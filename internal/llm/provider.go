package llm

import (
	"context"
	"autodev/internal/core"
)

// Provider defines the interface for interacting with LLMs.
type Provider interface {
	Name() string
	// Chat sends a list of messages to the LLM and returns the response.
	// If tools are provided, the LLM may return ToolCalls instead of content.
	Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error)
}
