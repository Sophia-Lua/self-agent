package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"autodev/internal/agents"
	"autodev/internal/audit"
	"autodev/internal/core"
	"autodev/internal/events"
	"autodev/internal/git"
	"autodev/internal/memory"
	"autodev/internal/parser"
	"autodev/internal/progress"
	"autodev/internal/session"
	"autodev/internal/timeout"
)

// Orchestrator manages the execution of the autonomous pipeline.
type Orchestrator struct {
	state      core.PipelineState
	cfg        *core.Config
	memory     *memory.Store
	bus        events.Bus
	auditLog   *audit.Logger
	registry   *agents.Registry
	snapshot   *session.Manager
	tracker    *progress.Tracker
	timePolicy *timeout.Policy
	retries    int
	maxRetries int
	history    []core.Message
	sessionID  string

	// Workflow state
	lastError       error
	lastSnapshot    *core.Snapshot
	decomposedTask  *parser.DecomposedTask
	autoCheckpoint  bool
}

// New creates a new Pipeline Orchestrator.
func New(cfg *core.Config, memory *memory.Store, bus events.Bus, reg *agents.Registry) *Orchestrator {
	sessMgr := session.New(cfg.WorkDir)
	o := &Orchestrator{
		cfg:            cfg,
		memory:         memory,
		bus:            bus,
		auditLog:       audit.New(),
		registry:       reg,
		snapshot:       sessMgr,
		tracker:        progress.New("", "Pipeline"),
		timePolicy:     timeout.DefaultPolicy(),
		state:          core.StatePending,
		maxRetries:     3,
		history:        make([]core.Message, 0),
		autoCheckpoint: true,
	}

	// Register standard pipeline phases for progress tracking
	o.tracker.RegisterPhase("agent-parser", "Decompose task into subtasks")
	o.tracker.RegisterPhase("agent-developer", "Implement the solution")
	o.tracker.RegisterPhase("agent-tester", "Write and execute tests")
	o.tracker.RegisterPhase("agent-checker", "Review code quality")
	o.tracker.RegisterPhase("agent-recovery", "Attempt to fix failures")

	return o
}

// WithSessionID sets the session ID for checkpoint tracking.
func (o *Orchestrator) WithSessionID(id string) *Orchestrator {
	o.sessionID = id
	return o
}

// WithAutoCheckpoint enables/disables automatic checkpoint saving.
func (o *Orchestrator) WithAutoCheckpoint(enabled bool) *Orchestrator {
	o.autoCheckpoint = enabled
	return o
}

// WithTimeout sets the timeout policy for agent execution.
func (o *Orchestrator) WithTimeout(policy *timeout.Policy) *Orchestrator {
	o.timePolicy = policy
	return o
}

// Run starts the autonomous execution loop.
func (o *Orchestrator) Run(ctx context.Context, input *core.Input) (*core.Output, error) {
	o.bus.Publish(ctx, events.Event{
		Type: events.TypePipelineStart,
		Payload: map[string]interface{}{
			"task": input.TaskDescription,
		},
	})

	o.transition(core.StateParsing)

	// Initialize input files from the current workspace if empty
	if len(input.Files) == 0 {
		if snap, err := o.snapshot.CreateSnapshot(""); err == nil {
			input.Files = snap.Files
		}
	}

	// Create initial snapshot for rollback
	// We do this now so that Rollback has a point to return to (pre-parsing state)
	if snap, err := o.snapshot.CreateSnapshot(input.TaskDescription); err == nil {
		o.lastSnapshot = snap
	}

	// Main Loop
	for {
		select {
		case <-ctx.Done():
			o.transition(core.StateCancelled)
			o.bus.Publish(ctx, events.Event{
				Type: events.TypePipelineEnd,
				Payload: map[string]interface{}{
					"status": "cancelled",
					"reason": ctx.Err().Error(),
				},
			})
			return nil, ctx.Err()
		default:
		}

		// Refresh files before each agent step to capture changes (e.g. from Developer's write_file)
		// Optimization: only do this if the previous step wasn't Parsing (which doesn't write)
		if o.state != core.StateParsing {
			if snap, err := o.snapshot.CreateSnapshot(""); err == nil {
				input.Files = snap.Files
			}
		}

		var err error

		switch o.state {
		case core.StateParsing:
			// Use parser module to decompose the task before agent execution
			o.decomposedTask = parser.Decompose(input.TaskDescription)
			log.Printf("[Parser] Decomposed into %d subtasks: %s", len(o.decomposedTask.SubTasks), o.decomposedTask.Title)

			// Append task decomposition to input context
			taskPlan := parser.SummarizeTaskPlan(o.decomposedTask)
			input.Context = taskPlan

			_, err = o.runAgent(ctx, "agent-parser", input)
			if err != nil {
				o.lastError = err
				o.transition(core.StateRecovering)
				continue
			}
			o.saveCheckpoint(input)
			o.transition(core.StateDeveloping)

		case core.StateDeveloping:
			_, err = o.runAgent(ctx, "agent-developer", input)
			if err != nil {
				o.lastError = err
				o.transition(core.StateRecovering)
				continue
			}
			o.saveCheckpoint(input)
			o.transition(core.StateTesting)

	case core.StateTesting:
		_, err = o.runAgent(ctx, "agent-tester", input)
		if err != nil {
			o.lastError = err
			o.transition(core.StateRecovering)
			continue
		}
		o.saveCheckpoint(input)
		o.transition(core.StateChecking)

	case core.StateChecking:
		_, err = o.runAgent(ctx, "agent-checker", input)
		if err != nil {
			o.lastError = err
			o.transition(core.StateRecovering)
			continue
		}
		o.transition(core.StateCompleted)

		case core.StateRecovering:
			if o.retries >= o.maxRetries {
				log.Println("Max retries reached, rolling back.")
				o.transition(core.StateRollback)
				continue
			}

			// Set error context for recovery agent
			errContext := fmt.Sprintf("Previous task failed with error: %v\nOriginal Task: %s\nPlease analyze this failure, suggest a fix, and update the files.", 
				o.lastError, input.TaskDescription)
			
			recoveryInput := &core.Input{
				TaskDescription: errContext,
				History:         o.history,
				Files:           input.Files,
			}

			// Attempt recovery
			o.retries++
			log.Printf("[Pipeline] Attempting recovery (%d/%d)", o.retries, o.maxRetries)
			_, err = o.runAgent(ctx, "agent-recovery", recoveryInput)
			if err != nil {
				// Recovery agent itself failed
				o.transition(core.StateRollback)
				continue
			}
			
			// Retry the previous phase (simplified: go back to Developing)
			o.saveCheckpoint(input)
			o.transition(core.StateDeveloping)

	case core.StateCompleted:
		o.saveCheckpoint(input)

		output := &core.Output{
			Status:  core.StatusSuccess,
			Message: "Pipeline completed successfully",
		}

		// Save run result to memory
		if o.memory != nil {
			resultSummary := fmt.Sprintf("Pipeline completed. Files: %d, PR: %s", len(input.Files), func() string {
				if output.PRInfo != nil {
					return output.PRInfo.URL
				}
				return "none"
			}())
			_ = o.memory.Save(context.TODO(), "default", "last_result", resultSummary)
		}

		// Create PR if configured
		if input.PRConfig != nil && input.PRConfig.Enabled {
			log.Println("[Pipeline] Creating Pull Request...")
			prURL, prNum, prErr := o.createPR(input)
			if prErr != nil {
				log.Printf("[Pipeline] PR creation failed: %v", prErr)
				output.Message = fmt.Sprintf("Pipeline completed, but PR creation failed: %v", prErr)
			} else {
				output.Message = fmt.Sprintf("Pipeline completed. PR created: %s", prURL)
				output.PRInfo = &core.PRResult{
					URL:      prURL,
					Number:   prNum,
					Platform: input.PRConfig.Platform,
				}
			}
		}

		o.bus.Publish(ctx, events.Event{
			Type: events.TypePipelineEnd,
			Payload: map[string]interface{}{
				"status": "completed",
				"pr_url": func() string {
					if output.PRInfo != nil {
						return output.PRInfo.URL
					}
					return ""
				}(),
			},
		})

		return output, nil

	case core.StateRollback:
		if o.lastSnapshot != nil {
			if err := o.snapshot.RestoreSnapshot(o.lastSnapshot); err != nil {
				log.Println("Rollback failed:", err)
			} else {
				log.Println("Workspace restored to pre-parsing state")
			}
		}

		o.bus.Publish(ctx, events.Event{
			Type: events.TypePipelineEnd,
			Payload: map[string]interface{}{
				"status": "failed",
				"error":  o.lastError,
			},
		})

		return nil, fmt.Errorf("pipeline failed: %v", o.lastError)
		}
	}
}

// runAgent looks up and executes a registered agent.
func (o *Orchestrator) runAgent(ctx context.Context, agentID string, input *core.Input) (*core.Output, error) {
	agent, err := o.registry.Get(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	// Apply per-agent timeout from the policy
	agentCtx, cancel := o.timePolicy.WithTimeout(ctx, timeout.ScopeAgent)
	defer cancel()

	// Start progress tracking for this agent phase
	o.tracker.StartPhase(agentID)
	defer o.tracker.CompletePhase(agentID, 0)

	o.bus.Publish(ctx, events.Event{
		Type:  events.TypeAgentStart,
		Agent: agentID,
		Payload: map[string]interface{}{
			"task": input.TaskDescription,
		},
	})

	// Build the agent input with history
	agentInput := core.Input{
		TaskDescription: input.TaskDescription,
		History:         o.history,
		Files:           input.Files,
	}

	startTime := time.Now()
	output, err := agent.Execute(agentCtx, agentInput)
	duration := time.Since(startTime)

	if err != nil {
		o.tracker.FailPhase(agentID, err.Error())
		o.bus.Publish(ctx, events.Event{
			Type:  events.TypeAgentError,
			Agent: agentID,
			Payload: map[string]interface{}{"error": err.Error(), "duration_ms": duration.Milliseconds()},
		})
		o.lastError = err
		return nil, err
	}

	// Update history
	o.history = append(o.history, core.Message{
		Role:    "user",
		Content: input.TaskDescription,
	})
	
	if output != nil {
		o.history = append(o.history, core.Message{
			Role:    "assistant",
			Content: output.Message,
		})
	}

	o.bus.Publish(ctx, events.Event{
		Type:  events.TypeAgentComplete,
		Agent: agentID,
		Payload: map[string]interface{}{
			"status": string(output.Status),
		},
	})

	return output, nil
}

// transition updates state and publishes event.
func (o *Orchestrator) transition(s core.PipelineState) {
	prev := o.state
	o.state = s
	o.bus.Publish(context.TODO(), events.Event{
		Type: events.TypeStateChange,
		Payload: map[string]any{
			"from": string(prev),
			"to":   string(s),
		},
	})
	o.auditLog.StateChange("orchestrator", string(prev), string(s))
	log.Printf("[Pipeline] Transitioned %s -> %s", prev, s)
}

// saveCheckpoint automatically saves execution state if checkpointing is enabled.
func (o *Orchestrator) saveCheckpoint(input *core.Input) {
	if !o.autoCheckpoint || o.sessionID == "" {
		return
	}

	if err := o.snapshot.AutoCheckpoint(o.sessionID, o.state, input, o.history, o.retries); err != nil {
		log.Printf("[Pipeline] Checkpoint save warning: %v", err)
	} else {
		o.auditLog.Snapshot("orchestrator", o.sessionID, len(input.Files))
	}
}

// createPR creates a pull request after pipeline completion.
func (o *Orchestrator) createPR(input *core.Input) (url string, number int, err error) {
	cfg := input.PRConfig

	determinePlatform := git.Platform(cfg.Platform)
	if determinePlatform == "" {
		// Auto-detect from git remote
		remoteURL, rerr := o.getRemoteURL()
		if rerr != nil {
			return "", 0, fmt.Errorf("auto-detect platform failed: %w", rerr)
		}
		platform, owner, repo := git.DetectPlatform(remoteURL)
		if platform == git.PlatformUnknown {
			return "", 0, fmt.Errorf("could not detect Git platform")
		}
		determinePlatform = platform
		cfg.Owner = owner
		cfg.Repo = repo
	}

	// Get token from env if not provided
	token := cfg.Token
	if token == "" {
		token = git.GetTokenFromEnv(determinePlatform)
		if token == "" {
			return "", 0, fmt.Errorf("no token provided and no env token found")
		}
	}

	client := git.NewPRClient(determinePlatform, token, cfg.Owner, cfg.Repo)

	// Collect git diff for PR description
	var stagedDiff, unstagedDiff string
	if repo, err := git.New(o.cfg.WorkDir); err == nil {
		stagedDiff, _ = repo.DiffStaged()
		unstagedDiff, _ = repo.Diff()
	}
	fullDiff := stagedDiff + "\n" + unstagedDiff

	// Generate title and description from task and diff
	title, description := git.GeneratePRDescription(input.TaskDescription, fullDiff, "")

	prCfg := git.PRConfig{
		Title:       title,
		Description: description,
		SourceBranch: o.getCurrentBranch(),
		TargetBranch: cfg.TargetBranch,
		Labels:       cfg.Labels,
		Reviewers:    cfg.Reviewers,
		Draft:        cfg.Draft,
	}

	ctx := context.Background()
	pr, err := client.CreatePR(ctx, prCfg)
	if err != nil {
		return "", 0, err
	}

	return pr.URL, pr.Number, nil
}

// getRemoteURL gets the git remote URL for platform detection.
func (o *Orchestrator) getRemoteURL() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = o.cfg.WorkDir
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	if err := cmd.Run(); err != nil {
		// Fallback: try git remote -v
		cmd = exec.Command("git", "remote", "-v")
		cmd.Dir = o.cfg.WorkDir
		cmd.Stdout = new(bytes.Buffer)
		cmd.Stderr = new(bytes.Buffer)
		if err := cmd.Run(); err != nil {
			// Check if work dir is not a git repo
			_, statErr := os.Stat(filepath.Join(o.cfg.WorkDir, ".git"))
			if os.IsNotExist(statErr) {
				return "", fmt.Errorf("not a git repository")
			}
			return "", fmt.Errorf("failed to get remote URL: %w", err)
		}
		// Parse first line of git remote -v output
		output := strings.TrimSpace(cmd.Stdout.(*bytes.Buffer).String())
		lines := strings.Split(output, "\n")
		if len(lines) > 0 {
			fields := strings.Fields(lines[0])
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
		return "", fmt.Errorf("no remote configured")
	}

	return strings.TrimSpace(cmd.Stdout.(*bytes.Buffer).String()), nil
}

// getCurrentBranch gets the current git branch name.
func (o *Orchestrator) getCurrentBranch() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = o.cfg.WorkDir
	cmd.Stdout = new(bytes.Buffer)
	cmd.Stderr = new(bytes.Buffer)

	if err := cmd.Run(); err != nil {
		log.Printf("[Pipeline] Failed to get current branch: %v", err)
		return "main" // Default fallback
	}

	branch := strings.TrimSpace(cmd.Stdout.(*bytes.Buffer).String())
	if branch == "" || branch == "HEAD" {
		return "main"
	}
	return branch
}

// State returns the current state of the pipeline.
func (o *Orchestrator) State() core.PipelineState {
	return o.state
}

// Rollback restores the workspace to the last valid snapshot.
func (o *Orchestrator) Rollback() error {
	if o.lastSnapshot != nil {
		return o.snapshot.RestoreSnapshot(o.lastSnapshot)
	}
	return fmt.Errorf("no snapshot available for rollback")
}

// Pause interrupts execution and waits for User resume.
func (o *Orchestrator) Pause() error {
	prev := o.state
	o.transition(core.StateCancelled) // Simplified pause - saves checkpoint
	log.Printf("[Pipeline] Paused from state: %s", prev)
	return nil
}

// Resume continues execution after a pause.
func (o *Orchestrator) Resume() error {
	// Resume by restarting from last checkpoint
	if o.lastSnapshot != nil {
		log.Printf("[Pipeline] Resuming from snapshot: %s", o.lastSnapshot.TaskID)
		o.transition(core.StateDeveloping)
	}
	return nil
}
