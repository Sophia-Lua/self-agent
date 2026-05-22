package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"autodev/internal/agents"
	ctxbuilder "autodev/internal/context"
	"autodev/internal/core"
	"autodev/internal/events"
	"autodev/internal/llm"
	"autodev/internal/memory"
	"autodev/internal/pipeline"
	"autodev/internal/registry"
	"autodev/internal/tools"

	"github.com/spf13/cobra"
)

func main() {
	var provider, model, apiKey, agentsDir string
	var dryRun, failOnce bool
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
			return runPipeline(task, provider, model, apiKey, agentsDir, dryRun, failOnce)
		},
	}

	runCmd.Flags().StringVar(&provider, "provider", "openai", "LLM Provider (openai, mock)")
	runCmd.Flags().StringVar(&model, "model", "gpt-4o", "LLM Model")
	runCmd.Flags().StringVar(&apiKey, "api-key", "", "LLM API Key (or set OPENAI_API_KEY)")
	runCmd.Flags().StringVar(&agentsDir, "agents-dir", "./agents", "Directory containing custom agent YAMLs")
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Use mock LLM provider to test workflow")
	runCmd.Flags().BoolVar(&failOnce, "fail-once", false, "Simulate one LLM failure to test recovery")

	rootCmd.AddCommand(runCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runPipeline(task, provider, model, apiKey, agentsDir string, dryRun bool, failOnce bool) error {
	if !dryRun && apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if !dryRun && apiKey == "" {
		return fmt.Errorf("api-key is required or set OPENAI_API_KEY (or use --dry-run)")
	}

	var prov llm.Provider
	if dryRun || provider == "mock" {
		failCount := 0
		if failOnce {
			failCount = 1
		}
		prov = &llm.MockProvider{FailCount: failCount}
	} else if provider == "openai" {
		prov = &llm.OpenAIProvider{
			APIKey:  apiKey,
			Model:   model,
			BaseURL: "https://api.openai.com/v1",
		}
	} else {
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	store, err := memory.New(":memory:")
	if err != nil {
		return fmt.Errorf("memory init failed: %w", err)
	}

	bus := events.NewInMemoryBus()
	cfg := &core.Config{ WorkDir: "." }

	// Context Builder (Limits input size to LLM)
	ctxBuilder := &ctxbuilder.Builder{
		MaxTokens: 32000, // Conservative limit
	}

	toolReg := tools.New()
	tools.RegisterFileTools(toolReg)

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
		SystemPrompt: "You are an expert AI Coding Agent. Write clean, efficient, and tested Go code.",
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
		AgentID: "agent-recovery",
		AgentRole: core.RoleRecovery,
		AgentDesc: "Attempts to fix task failures",
		Provider:  prov,
		SystemPrompt: "You are an expert Recovery Agent. Analyze the error and fix it.",
		ToolRegistry: toolReg,
		Context:      ctxBuilder,
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-developer",
		AgentRole:       core.RoleDeveloper,
		AgentDesc: "Generates code based on structured tasks",
		Provider:   prov,
		SystemPrompt: "You are an expert AI Coding Agent. Write clean, efficient, and tested Go code.",
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-tester",
		AgentRole:       core.RoleTester,
		AgentDesc: "Validates code correctness",
		Provider:   prov,
		SystemPrompt: "You are an expert QA Engineer. Review the code for bugs and edge cases.",
	})
	reg.Register(&agents.Executor{
		AgentID: "agent-recovery",
		AgentRole:       core.RoleRecovery,
		AgentDesc: "Attempts to fix task failures",
		Provider:   prov,
		SystemPrompt: "You are an expert Recovery Agent. Analyze the error and fix it.",
	})

	loader := registry.New(reg)
	if err := loader.LoadFromDir(agentsDir); err != nil {
		return fmt.Errorf("failed to load custom agents: %w", err)
	}

	// Create Orchestrator
	orch := pipeline.New(cfg, store, bus, reg)

	// Execute
	ctx := context.Background()
	log.Printf("Starting task: %s", task)
	
	input := &core.Input{ TaskDescription: task }
	
	_, err = orch.Run(ctx, input)
	return err
}
