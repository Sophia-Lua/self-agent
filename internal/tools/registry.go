package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"autodev/internal/core"
	"autodev/internal/mcp"
)

// Registry maps tool names to their implementation logic.
type Registry struct {
	mu      sync.RWMutex
	tools   map[string]core.Tool
	logic   map[string]func(ctx context.Context, args map[string]any) (any, error)
	mcpLogic map[string]*mcp.Client // Maps tool name to the MCP Client that owns it
}

// New creates a new Tool Registry.
func New() *Registry {
	return &Registry{
		tools:   make(map[string]core.Tool),
		logic:   make(map[string]func(ctx context.Context, args map[string]any) (any, error)),
		mcpLogic: make(map[string]*mcp.Client),
	}
}

// Register adds a local tool and its execution logic.
func (r *Registry) Register(tool core.Tool, exec func(ctx context.Context, args map[string]any) (any, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Function.Name
	r.tools[name] = tool
	r.logic[name] = exec
	delete(r.mcpLogic, name) // Ensure no conflict
}

// RegisterMCP registers a tool sourced from an MCP server.
func (r *Registry) RegisterMCP(tool core.Tool, client *mcp.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Function.Name
	r.tools[name] = tool
	r.mcpLogic[name] = client
	delete(r.logic, name) // Ensure no conflict
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
// Supports both local logic and MCP remote execution.
func (r *Registry) Execute(ctx context.Context, call core.ToolCall) (string, error) {
	r.mu.RLock()
	
	// 1. Check if it's a Local tool
	localLogic, isLocal := r.logic[call.Function.Name]
	
	// 2. Check if it's an MCP tool
	mcpClient, isMCP := r.mcpLogic[call.Function.Name]
	
	r.mu.RUnlock()

	var args map[string]any
	if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("invalid tool arguments: %w", err)
	}

	if isLocal {
		result, err := localLogic(ctx, args)
		if err != nil {
			return "", fmt.Errorf("tool execution failed: %w", err)
		}
		// Marshal result to string
		b, _ := json.Marshal(result)
		return string(b), nil
	}

	if isMCP && mcpClient != nil {
		result, err := mcpClient.CallTool(call.Function.Name, args)
		if err != nil {
			return "", fmt.Errorf("MCP tool execution failed: %w", err)
		}
		return result, nil
	}

	return "", fmt.Errorf("tool not found or not executable: %s", call.Function.Name)
}
