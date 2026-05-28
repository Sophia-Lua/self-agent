package core

import "context"

// MemoryProvider handles long-term context storage.
type MemoryProvider interface {
	SaveContext(ctx context.Context, taskID string, key string, value string) error
	LoadContext(ctx context.Context, taskID string, key string) (string, error)
	SearchMemory(ctx context.Context, query string, limit int) ([]MemoryResult, error)
}

// MemoryResult represents a single search result from MemoryProvider.
type MemoryResult struct {
	Key     string
	Value   string
	Score   float64
	TaskID  string
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

// ToolExecutor handles the execution of ToolCalls.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (string, error)
}

// ChatOptions configures an LLM API request.
type ChatOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
	TopP        float64
	Stop        []string
}

// Capabilities describes what an LLM provider supports.
type Capabilities struct {
	MaxTokens      int
	ContextWindow  int
	Streaming      bool
	Vision         bool
	FunctionCall   bool
}

// LLMProvider abstracts the underlying LLM API.
type LLMProvider interface {
	Name() string
	Chat(ctx context.Context, messages []Message, tools []Tool) (*AgentOutput, error)
	Capabilities() Capabilities
}

// AgentOutput represents the LLM response.
type AgentOutput struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
	Model     string
}
