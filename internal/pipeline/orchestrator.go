package pipeline

import (
	"context"
	"fmt"
	"log"

	"autodev/internal/agents"
	"autodev/internal/core"
	"autodev/internal/events"
	"autodev/internal/memory"
	"autodev/internal/session"
)

// Orchestrator manages the execution of the autonomous pipeline.
type Orchestrator struct {
	state      core.PipelineState
	cfg        *core.Config
	memory     *memory.Store
	bus        events.Bus
	registry   *agents.Registry
	snapshot   *session.Manager
	retries    int
	maxRetries int
	history    []core.Message

	// Workflow state
	lastError error
}

// New creates a new Pipeline Orchestrator.
func New(cfg *core.Config, memory *memory.Store, bus events.Bus, reg *agents.Registry) *Orchestrator {
	sessMgr := session.New(cfg.WorkDir)
	return &Orchestrator{
		cfg:        cfg,
		memory:     memory,
		bus:        bus,
		registry:   reg,
		snapshot:   sessMgr,
		state:      core.StatePending,
		maxRetries: 3,
		history:    make([]core.Message, 0),
	}
}

// Run starts the autonomous execution loop.
func (o *Orchestrator) Run(ctx context.Context, input *core.Input) (*core.Output, error) {
	o.transition(core.StateParsing)

	// Initialize the input data
	if input.Files == nil {
		input.Files = make(map[string]string)
	}

	// Main Loop
	for {
		select {
		case <-ctx.Done():
			o.transition(core.StateCancelled)
			return nil, ctx.Err()
		default:
		}

		var err error

		switch o.state {
		case core.StateParsing:
			_, err = o.runAgent(ctx, "agent-parser", input)
			if err != nil {
				o.lastError = err
				o.transition(core.StateRecovering)
				continue
			}
			o.transition(core.StateDeveloping)

		case core.StateDeveloping:
			_, err = o.runAgent(ctx, "agent-developer", input)
			if err != nil {
				o.lastError = err
				o.transition(core.StateRecovering)
				continue
			}
			o.transition(core.StateTesting)

		case core.StateTesting:
			_, err = o.runAgent(ctx, "agent-tester", input)
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
			o.transition(core.StateDeveloping)

		case core.StateCompleted:
			return &core.Output{
				Status:  core.StatusSuccess,
				Message: "Pipeline completed successfully",
			}, nil

		case core.StateRollback:
			if err := o.snapshot.RestoreSnapshot(nil); err != nil {
				log.Println("Rollback failed") // Should log error
			}
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

	output, err := agent.Execute(ctx, agentInput)
	if err != nil {
		o.bus.Publish(ctx, events.Event{
			Type:  events.TypeAgentError,
			Agent: agentID,
			Payload: map[string]interface{}{"error": err.Error()},
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
	o.state = s
	o.bus.Publish(context.TODO(), events.Event{
		Type: events.TypeStateChange,
		Payload: map[string]any{
			"from": string(o.state), // Note: this logs previous state
			"to":   string(s),
		},
	})
	log.Printf("[Pipeline] Transitioned to %s", s)
}
