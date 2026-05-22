package tools

import (
	"fmt"

	"autodev/internal/mcp"
)

// RegisterMCPServer connects to an MCP Server via Stdio, retrieves tools, and registers them into the given registry.
func RegisterMCPServer(reg *Registry, command string, args []string) error {
	client, err := mcp.NewClient(command, args)
	if err != nil {
		return fmt.Errorf("failed to create MCP client for %s: %w", command, err)
	}

	if err := client.Init(); err != nil {
		return fmt.Errorf("failed to initialize MCP client for %s: %w", command, err)
	}

	tools, err := client.ListTools()
	if err != nil {
		return fmt.Errorf("failed to list tools from MCP server %s: %w", command, err)
	}

	if len(tools) == 0 {
		return nil // No tools to register
	}

	for _, tool := range tools {
		reg.RegisterMCP(tool, client)
	}

	return nil
}
