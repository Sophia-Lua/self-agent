package agents

import (
	"context"
	"fmt"

	"autodev/internal/core"
	"autodev/internal/llm"
)

// Executor is a generic Agent implementation backed by an LLM.
type Executor struct {
	AgentID      string
	AgentRole    core.Role
	AgentDesc    string
	Provider     llm.Provider
	SystemPrompt string
}

func (e *Executor) ID() string          { return e.AgentID }
func (e *Executor) Role() core.Role     { return e.AgentRole }
func (e *Executor) Description() string { return e.AgentDesc }

func (e *Executor) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	if e.Provider == nil {
		return nil, fmt.Errorf("LLM provider not configured for agent %s", e.AgentID)
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

	// 2. Call LLM
	resp, err := e.Provider.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM chat failed: %w", err)
	}

	return &core.Output{
		Status:  core.StatusSuccess,
		Message: resp,
	}, nil
}
