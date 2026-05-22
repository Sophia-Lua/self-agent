package core

import "context"

// MemoryProvider handles long-term context storage.
type MemoryProvider interface {
	SaveContext(ctx context.Context, taskID string, key string, value string) error
	LoadContext(ctx context.Context, taskID string, key string) (string, error)
}

// ToolCall represents a request from the LLM to execute a tool.
type ToolCall struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Function  FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LLMProvider abstracts the underlying LLM API.
type LLMProvider interface {
	Name() string
	Chat(ctx context.Context, messages []Message, tools []Tool) (*Output, error)
}

// Tool defines an executable function available to the LLM.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}
