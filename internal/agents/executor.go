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

	// ModelConfig specifies agent-specific model settings (optional).
	// These override the provider's default settings at request time.
	ModelOption llm.AgentModelOption
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

	messages, err = buildMessages(e, input)
	if err != nil {
		return nil, fmt.Errorf("context build failed: %w", err)
	}

	var toolsList []core.Tool
	if e.ToolRegistry != nil {
		toolsList = e.ToolRegistry.List()
	}

	opts := llm.ChatOptions{
		Model:       e.ModelOption.Model,
		Temperature: e.ModelOption.Temperature,
		MaxTokens:   e.ModelOption.MaxTokens,
	}

	finalContent := ""

	for i := 0; i < e.MaxToolCalls; i++ {
		resp, err := chatWithOpts(e.Provider, ctx, messages, toolsList, opts)
		if err != nil {
			return nil, fmt.Errorf("LLM chat failed: %w", err)
		}

		if resp.Content != "" {
			finalContent = resp.Content
		}

		if len(resp.ToolCalls) == 0 {
			break
		}

		for _, call := range resp.ToolCalls {
			var result string
			var execErr error

			if e.ToolRegistry != nil {
				result, execErr = e.ToolRegistry.Execute(ctx, call)
			} else {
				result = `{"error": "No tool registry available"}`
			}

			messages = append(messages, core.Message{
				Role:    "assistant",
				Content: "",
			})

			if execErr != nil {
				result = fmt.Sprintf(`{"error": "%s"}`, execErr.Error())
			}

			messages = append(messages, core.Message{
				Role:       "tool",
				Name:       call.Function.Name,
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}

	return &core.Output{
		Status:  core.StatusSuccess,
		Message: finalContent,
	}, nil
}

func buildMessages(e *Executor, input core.Input) ([]core.Message, error) {
	if e.Context != nil {
		maxTokens := e.Context.MaxTokens
		if maxTokens == 0 {
			caps := e.Provider.Capabilities()
			maxTokens = caps.ContextWindow
			if maxTokens == 0 {
				maxTokens = 128000
			}
		}
		safeLimit := int(float64(maxTokens) * 0.8)
		if e.ModelOption.MaxTokens > 0 && e.ModelOption.MaxTokens < safeLimit {
			safeLimit = e.ModelOption.MaxTokens
		}

		return e.Context.Build(
			input.TaskDescription,
			e.SystemPrompt,
			input.History,
			input.Files,
		)
	}

	messages := append([]core.Message(nil), input.History...)
	messages = append([]core.Message{{Role: "system", Content: e.SystemPrompt}}, messages...)
	messages = append(messages, core.Message{Role: "user", Content: input.TaskDescription})
	return messages, nil
}

func chatWithOpts(provider llm.Provider, ctx context.Context, messages []core.Message, tools []core.Tool, opts llm.ChatOptions) (*core.AgentOutput, error) {
	if optsProvider, ok := provider.(llm.ChatWithOpts); ok {
		return optsProvider.ChatWithOptions(ctx, messages, tools, opts)
	}
	return provider.Chat(ctx, messages, tools)
}
