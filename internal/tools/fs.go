package tools

import (
	"context"
	"autodev/internal/core"
)

// RegisterFileTools adds basic file manipulation tools to the registry.
func RegisterFileTools(reg *Registry) {
	// write_file
	reg.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "write_file",
			Description: "Write content to a file. If the file exists, it will be overwritten.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]string{"type": "string", "description": "File path"},
					"content": map[string]string{"type": "string", "description": "File content"},
				},
				"required": []string{"path", "content"},
			},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		path, ok := args["path"].(string)
		if !ok {
			return nil, ErrMissingPath
		}
		content, _ := args["content"].(string)
		// Mock implementation for now
		return map[string]string{"status": "success", "path": path, "bytes": string(rune(len(content)))}, nil
	})
}

type ToolResult struct {
	Status string `json:"status"`
	Message string `json:"message,omitempty"`
}

var ErrMissingPath = &ToolError{"missing 'path' argument"}

type ToolError struct {
	msg string
}

func (e *ToolError) Error() string {
	return e.msg
}
