package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"autodev/internal/agents"
	"autodev/internal/config"
	ctxbuilder "autodev/internal/context"
	"autodev/internal/core"
	"autodev/internal/crypto"
	"autodev/internal/diagnosis"
	"autodev/internal/events"
	"autodev/internal/git"
	"autodev/internal/llm"
	"autodev/internal/memory"
	"autodev/internal/mcp"
	"autodev/internal/notify"
	"autodev/internal/pipeline"
	"autodev/internal/project"
	"autodev/internal/registry"
	"autodev/internal/sandbox"
	"autodev/internal/session"
	"autodev/internal/tools"

	"github.com/spf13/cobra"
)

func main() {
	var provider, model, apiKey, agentsDir string
	var dryRun, failOnce bool
	var mcpConfigPath string
	var resumeSession string
	var createPR bool
	var prTargetBranch string
	var prDraft bool
	var prReviewers string
	var rootCmd = &cobra.Command{
		Use:   "autodev",
		Short: "Autonomous developer agent",
	}

	var runCmd = &cobra.Command{
		Use:   "run [task]",
		Short: "Execute a development task",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			task := args[0]
			return runPipeline(task, provider, model, apiKey, agentsDir, dryRun, failOnce, mcpConfigPath, resumeSession, createPR, prTargetBranch, prDraft, prReviewers)
		},
	}

	runCmd.Flags().StringVar(&provider, "provider", "openai", "LLM Provider (openai, claude, ollama, mock)")
	runCmd.Flags().StringVar(&model, "model", "gpt-4o", "LLM Model")
	runCmd.Flags().StringVar(&apiKey, "api-key", "", "LLM API Key (or set OPENAI_API_KEY)")
	runCmd.Flags().StringVar(&agentsDir, "agents-dir", "./agents", "Directory containing custom agent YAMLs")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Use mock LLM provider to test workflow")
	runCmd.Flags().BoolVar(&failOnce, "fail-once", false, "Simulate one LLM failure to test recovery")
	runCmd.Flags().StringVar(&mcpConfigPath, "mcp-config", "", "Path to MCP Servers JSON config")
	runCmd.Flags().StringVar(&resumeSession, "resume", "", "Resume execution from checkpoint")
	runCmd.Flags().BoolVar(&createPR, "create-pr", false, "Automatically create PR after completion")
	runCmd.Flags().StringVar(&prTargetBranch, "pr-target", "main", "Target branch for PR")
	runCmd.Flags().BoolVar(&prDraft, "pr-draft", false, "Create PR as draft")
	runCmd.Flags().StringVar(&prReviewers, "pr-reviewers", "", "Comma-separated list of reviewers")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newSessionCmd())
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newDiagnoseCmd())
	rootCmd.AddCommand(newEncryptCmd())
	rootCmd.AddCommand(newDecryptCmd())
	rootCmd.AddCommand(newSandboxCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newConfigCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runPipeline(task, provider, model, apiKey, agentsDir string, dryRun bool, failOnce bool, mcpConfigPath string, resumeSession string, createPR bool, prTargetBranch string, prDraft bool, prReviewers string) error {
	appCfg, _ := config.LoadConfig()

	var prov llm.Provider

	if dryRun || provider == "mock" {
		failCount := 0
		if failOnce {
			failCount = 1
		}
		prov = &llm.MockProvider{FailCount: failCount}
	} else {
		switch provider {
		case "openai":
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			}
			if apiKey == "" {
				return fmt.Errorf("api-key is required for openai provider or set OPENAI_API_KEY (or use --dry-run)")
			}
		case "claude", "anthropic":
			if apiKey == "" {
				apiKey = os.Getenv("ANTHROPIC_API_KEY")
			}
			if apiKey == "" {
				return fmt.Errorf("api-key is required for claude provider or set ANTHROPIC_API_KEY")
			}
		case "ollama", "local":
			if model == "" {
				model = "llama3"
			}
		}

		var err error
		enableCache := appCfg != nil && appCfg.LLM.EnableCache
		enableRateLimit := appCfg != nil && appCfg.LLM.EnableRateLimit
		prov, err = llm.NewProvider(llm.ProviderConfig{
			Provider:       provider,
			Model:          model,
			APIKey:         apiKey,
			EnableCache:    enableCache,
			MaxCacheSize:   appCfg.LLM.MaxCacheSize,
			EnableRateLimit: enableRateLimit,
			MaxRetries:     appCfg.LLM.MaxRetries,
			BaseDelayMs:    appCfg.LLM.BaseDelayMs,
		})
		if err != nil {
			return fmt.Errorf("failed to create provider: %w", err)
		}
	}

	store, err := memory.New(":memory:")
	if err != nil {
		return fmt.Errorf("memory init failed: %w", err)
	}

	bus := events.NewInMemoryBus()

	if appCfg != nil && appCfg.Webhook.Enabled && len(appCfg.Webhook.URLs) > 0 {
		sender := notify.NewWebhookSender(notify.SenderConfig{
			URLs:    appCfg.Webhook.URLs,
			Secret:  appCfg.Webhook.Secret,
			Timeout: appCfg.Webhook.Timeout,
			Retries: appCfg.Webhook.Retries,
		})
		if err := sender.SubscribeAll(bus); err != nil {
			log.Printf("[Webhook] subscribe warning: %v", err)
		}
	}

	// Load MCP configuration
	var mcpServers []mcp.ServerDef
	if mcpConfigPath != "" {
		data, err := os.ReadFile(mcpConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read MCP config: %w", err)
		}
		if err := json.Unmarshal(data, &mcpServers); err != nil {
			return fmt.Errorf("failed to parse MCP config JSON: %w", err)
		}
	}
	
	cfg := &core.Config{ WorkDir: "." }

	// Context Builder (Limits input size to LLM)
	ctxBuilder := &ctxbuilder.Builder{
		MaxTokens: 32000, // Conservative limit
	}

	toolReg := tools.New()
	tools.RegisterFileTools(toolReg, cfg.WorkDir)

	// 加载 MCP Servers
	if len(mcpServers) > 0 {
		log.Printf("Initializing %d MCP server(s)...", len(mcpServers))
		for _, srv := range mcpServers {
			log.Printf("Connecting to MCP server: %s (%s)", srv.Name, srv.Command)
			if err := tools.RegisterMCPServer(toolReg, srv.Command, srv.Args); err != nil {
				log.Printf("Warning: Failed to load MCP server %s: %v", srv.Name, err)
			}
		}
	}

	// Register Built-in Agents
	reg := agents.NewRegistry()
	reg.Register(&agents.Executor{
		AgentID: "agent-parser",
		AgentRole: core.RoleParser,
		AgentDesc: "Parses user intent into structured development tasks",
		Provider:  prov,
		SystemPrompt: "You are an expert Lead Developer. Analyze the request and outline the technical steps required.",
		ToolRegistry: toolReg,
		Context:      ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-developer",
		AgentRole: core.RoleDeveloper,
		AgentDesc: "Generates code based on structured tasks",
		Provider:  prov,
		SystemPrompt: "You are an expert AI Coding Agent. Write clean, efficient, and tested code.",
		ToolRegistry: toolReg,
		Context:      ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-tester",
		AgentRole: core.RoleTester,
		AgentDesc: "Validates code correctness",
		Provider:  prov,
		SystemPrompt: "You are an expert QA Engineer. Review the code for bugs and edge cases.",
		ToolRegistry: toolReg,
		Context:      ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-checker",
		AgentRole: core.RoleChecker,
		AgentDesc: "Validates code quality, coverage, and standards compliance",
		Provider:  prov,
		SystemPrompt: "You are an expert Code Reviewer. Check for quality issues, coverage gaps, and style violations.",
		ToolRegistry: toolReg,
		Context:      ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-recovery",
		AgentRole: core.RoleRecovery,
		AgentDesc: "Attempts to fix task failures",
		Provider:  prov,
		SystemPrompt: "You are an expert Recovery Agent. Analyze the error and fix it.",
		ToolRegistry: toolReg,
		Context:      ctxBuilder,
	})

	loader := registry.New(reg)
	if err := loader.LoadFromDir(agentsDir); err != nil {
		return fmt.Errorf("failed to load custom agents: %w", err)
	}

	// Create Orchestrator
	sessionID := resumeSession
	if sessionID == "" {
		sessionID = fmt.Sprintf("session-%d", time.Now().Unix())
	}

	orch := pipeline.New(cfg, store, bus, reg).
		WithSessionID(sessionID).
		WithAutoCheckpoint(true)

	// Execute
	ctx := context.Background()
	
	var input *core.Input
	
	if resumeSession != "" {
		// Resume from checkpoint
		sessMgr := session.New(cfg.WorkDir)
		resumedInput, _, history, retryCount, err := sessMgr.ResumeSession(resumeSession)
		if err != nil {
			return fmt.Errorf("failed to resume session: %w", err)
		}
		
		input = resumedInput
		input.History = history
		log.Printf("Resuming session %s (retries: %d)", resumeSession, retryCount)
	} else {
		input = &core.Input{ TaskDescription: task }
		log.Printf("Starting task: %s (session: %s)", task, sessionID)
	}

	// Configure PR creation if enabled
	if createPR {
		input.PRConfig = &core.PRConfig{
			Enabled:      true,
			TargetBranch: prTargetBranch,
			Draft:        prDraft,
		}
		if prReviewers != "" {
			input.PRConfig.Reviewers = strings.Split(prReviewers, ",")
		}
		log.Printf("[Pipeline] PR creation enabled (target: %s, draft: %t)", prTargetBranch, prDraft)
	}
	
	_, err = orch.Run(ctx, input)
	return err
}

func newAnalyzeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "analyze [dir]",
		Short: "Analyze project structure and dependencies",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			analyzer := project.New(dir)
			profile, err := analyzer.Analyze()
			if err != nil {
				return err
			}
			fmt.Println(profile.Summary())
			return nil
		},
	}
}

func newDiagnoseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diagnose [error]",
		Short: "Diagnose an error message and suggest fixes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			errMsg := args[0]
			engine := diagnosis.New()
			result := engine.Analyze(errMsg)
			fmt.Println(result.Summary)
			if result.Confidence < 0.5 {
				fmt.Println("\nLow confidence - consider manual review")
			}
			return nil
		},
	}
}

func newEncryptCmd() *cobra.Command {
	var password string
	cmd := &cobra.Command{
		Use:   "encrypt [text]",
		Short: "Encrypt a secret using AES-GCM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if password == "" {
				return fmt.Errorf("--password is required")
			}
			vault, err := crypto.New(password)
			if err != nil {
				return err
			}
			encrypted, err := vault.Encrypt(args[0])
			if err != nil {
				return err
			}
			fmt.Println(encrypted)
			return nil
		},
	}
	cmd.Flags().StringVarP(&password, "password", "p", "", "Encryption password")
	cmd.MarkFlagRequired("password")
	return cmd
}

func newDecryptCmd() *cobra.Command {
	var password string
	cmd := &cobra.Command{
		Use:   "decrypt [encrypted]",
		Short: "Decrypt an AES-GCM ciphertext",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if password == "" {
				return fmt.Errorf("--password is required")
			}
			vault, err := crypto.New(password)
			if err != nil {
				return err
			}
			decrypted, err := vault.Decrypt(args[0])
			if err != nil {
				return err
			}
			fmt.Println(decrypted)
			return nil
		},
	}
	cmd.Flags().StringVarP(&password, "password", "p", "", "Decryption password")
	cmd.MarkFlagRequired("password")
	return cmd
}

func newSandboxCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sandbox [command]",
		Short: "Execute a command in a sandboxed environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			executor, err := sandbox.New(nil, "/tmp/autodev-sandbox")
			if err != nil {
				return err
			}
			defer executor.Cleanup()

			ctx := context.Background()
			result, err := executor.Run(ctx, args[0], args[1:]...)
			if err != nil {
				return err
			}

			fmt.Printf("Exit Code: %d\n", result.ExitCode)
			if result.Stdout != "" {
				fmt.Printf("Stdout:\n%s\n", result.Stdout)
			}
			if result.Stderr != "" {
				fmt.Printf("Stderr:\n%s\n", result.Stderr)
			}
			fmt.Printf("Duration: %v\n", result.Duration)
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show git repository status",
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := git.New(".")
			if err != nil {
				return err
			}
			status, err := repo.Status()
			if err != nil {
				return err
			}
			if status.IsEmpty() {
				fmt.Println("Working tree is clean")
			} else {
				fmt.Printf("Branch: %s\n", status.Branch)
				fmt.Printf("Added: %d, Modified: %d, Deleted: %d, Untracked: %d\n",
					status.Added, status.Modified, status.Deleted, status.Untracked)
				if status.LastCommit != nil {
					fmt.Printf("Last Commit: %s - %s\n", status.LastCommit.Hash[:8], status.LastCommit.Subject)
				}
			}
			return nil
		},
	}
}

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show loaded configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			fmt.Printf("Provider: %s\n", cfg.LLM.Provider)
			fmt.Printf("Model: %s\n", cfg.LLM.Model)
			fmt.Printf("Session DB: %s\n", cfg.Session.DBPath)
			fmt.Printf("MCP Servers: %d\n", len(cfg.MCP.Servers))
			return nil
		},
	}
}
