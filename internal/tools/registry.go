package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"autodev/internal/core"
)

// Registry maps tool names to their implementation logic.
type Registry struct {
	mu      sync.RWMutex
	tools   map[string]core.Tool
	logic   map[string]func(ctx context.Context, args map[string]any) (any, error)
}

// New creates a new Tool Registry.
func New() *Registry {
	return &Registry{
		tools: make(map[string]core.Tool),
		logic: make(map[string]func(ctx context.Context, args map[string]any) (any, error)),
	}
}

// Register adds a tool and its execution logic.
func (r *Registry) Register(tool core.Tool, exec func(ctx context.Context, args map[string]any) (any, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[tool.Function.Name] = tool
	r.logic[tool.Function.Name] = exec
}

// List returns all registered tools in the format required by the LLM.
func (r *Registry) List() []core.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	res := make([]core.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		res = append(res, t)
	}
	return res
}

// Execute runs a ToolCall based on the function name.
func (r *Registry) Execute(ctx context.Context, call core.ToolCall) (string, error) {
	r.mu.RLock()
	logic, exists := r.logic[call.Function.Name]
	r.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("tool not found: %s", call.Function.Name)
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	result, err := logic(ctx, args)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	// Marshal result to string for LLM consumption
	b, _ := json.Marshal(result)
	return string(b), nil
}
