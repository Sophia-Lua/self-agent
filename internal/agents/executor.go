package agents

import (
	"context"
	"fmt"

	ctxbuilder "autodev/internal/context"
	"autodev/internal/core"
	"autodev/internal/llm"
	"autodev/internal/tools"
)

// Executor is a generic Agent implementation backed by an LLM.
type Executor struct {
	AgentID      string
	AgentRole    core.Role
	AgentDesc    string
	Provider     llm.Provider
	SystemPrompt string
	ToolRegistry *tools.Registry
	Context      *ctxbuilder.Builder

	// MaxToolCalls limits the number of tool calls per execution
	MaxToolCalls int
}

func (e *Executor) ID() string          { return e.AgentID }
func (e *Executor) Role() core.Role     { return e.AgentRole }
func (e *Executor) Description() string { return e.AgentDesc }

func (e *Executor) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	if e.Provider == nil {
		return nil, fmt.Errorf("LLM provider not configured for agent %s", e.AgentID)
	}

	if e.MaxToolCalls == 0 {
		e.MaxToolCalls = 10
	}

	var messages []core.Message
	var err error

	// 1. Construct Context via ContextBuilder (handles tokenization & files)
	if e.Context != nil {
		messages, err = e.Context.Build(
			input.TaskDescription,
			e.SystemPrompt,
			input.History,
			input.Files,
		)
	} else {
		// Fallback: simple message construction
		messages = append(messages, core.Message{Role: "system", Content: e.SystemPrompt})
		messages = append(messages, input.History...)
		messages = append(messages, core.Message{Role: "user", Content: input.TaskDescription})
	}

	if err != nil {
		return nil, fmt.Errorf("context build failed: %w", err)
	}

	var toolsList []core.Tool
	if e.ToolRegistry != nil {
		toolsList = e.ToolRegistry.List()
	}

	finalContent := ""

	// 2. Tool Call Loop
	for i := 0; i < e.MaxToolCalls; i++ {
		resp, err := e.Provider.Chat(ctx, messages, toolsList)
		if err != nil {
			return nil, fmt.Errorf("LLM chat failed: %w", err)
		}

		if resp.Content != "" {
			finalContent = resp.Content
		}

		// If no tool calls, we are done
		if len(resp.ToolCalls) == 0 {
			break
		}

		// Execute tools
		for _, call := range resp.ToolCalls {
			var result string
			var execErr error

			if e.ToolRegistry != nil {
				result, execErr = e.ToolRegistry.Execute(ctx, call)
			} else {
				result = `{"error": "No tool registry available"}`
			}

			// Add assistant message with tool_calls stub (optional in some APIs, but required for context)
			messages = append(messages, core.Message{
				Role:    "assistant",
				Content: "", // Content is usually empty when tool_calls are present
			})

			// Add tool result message
			if execErr != nil {
				result = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
			}

			messages = append(messages, core.Message{
				Role:    "tool",
				Content: result,
			})
		}
	}

	return &core.Output{
		Status:  core.StatusSuccess,
		Message: finalContent,
	}, nil
}
