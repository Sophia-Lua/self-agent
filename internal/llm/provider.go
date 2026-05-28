package llm

import (
	"context"
	"autodev/internal/core"
)

// AgentModelOption specifies per-agent model preferences (alias for ChatOptions).
type AgentModelOption = ChatOptions

// ChatOptions configures per-request LLM parameters.
type ChatOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

// Provider defines the interface for interacting with LLMs.
type Provider interface {
	Name() string
	// Chat sends a list of messages to the LLM and returns the response.
	// If tools are provided, the LLM may return ToolCalls instead of content.
	Chat(ctx context.Context, messages []core.Message, tools []core.Tool) (*core.AgentOutput, error)
	// Capabilities returns the model's capabilities and limits.
	Capabilities() core.Capabilities
}

// ChatWithOpts is an optional interface implemented by providers that support per-call options.
type ChatWithOpts interface {
	ChatWithOptions(ctx context.Context, messages []core.Message, tools []core.Tool, opts ChatOptions) (*core.AgentOutput, error)
}
