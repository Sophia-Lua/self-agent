package agents

import (
	"context"
	"fmt"

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

	// 1. Construct Messages
	messages := make([]core.Message, 0)
	messages = append(messages, core.Message{
		Role:    "system",
		Content: e.SystemPrompt,
	})

	// Append history from previous agents
	for _, msg := range input.History {
		messages = append(messages, msg)
	}

	// Append current task
	messages = append(messages, core.Message{
		Role:    "user",
		Content: input.TaskDescription,
	})

	var toolsList []core.Tool
	if e.ToolRegistry != nil {
		toolsList = e.ToolRegistry.List()
	}

	finalContent := ""

	// Tool Call Loop
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

			// Add the assistant message that triggered the tool call
			messages = append(messages, core.Message{
				Role:    "assistant",
				Content: "", // Content is usually empty when tool_calls are present
			})

			// Add the tool result message
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
