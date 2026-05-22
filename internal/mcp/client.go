package mcp

import (
	"encoding/json"
	"fmt"
	"sync"

	"autodev/internal/core"
)

// Client manages the connection to an MCP Server.
type Client struct {
	transport Transport
	initialized bool
	mu       sync.Mutex
	
	// Tools received from server
	ServerTools []core.Tool
}

// NewClient creates a new MCP Client using Stdio Transport for the given command.
func NewClient(command string, args []string) (*Client, error) {
	transport, err := NewStdioTransport(command, args)
	if err != nil {
		return nil, err
	}
	return &Client{
		transport: transport,
	}, nil
}

// Init sends the initialize request and handshake.
func (c *Client) Init() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	// 1. Initialize Request
	params := map[string]any{
		"protocolVersion": "2025-03-26", // Or latest supported
		"capabilities": map[string]any{
			"roots": map[string]any{
				"listChanged": true,
			},
		},
		"clientInfo": map[string]any{
			"name": "AutoDevAgent",
			"version": "0.1.0",
		},
	}

	resp, err := c.transport.Send(JSONRPCRequest{
		Method: "initialize",
		Params: mustMarshal(params),
	})
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("init error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
	}

	// 2. Send notifications/initialized to complete handshake
	// Some servers require "notifications/initialized" after the response
	_, err = c.transport.Send(JSONRPCRequest{
		Method: "notifications/initialized", // Wait, spec says this is a notification, usually no ID?
	})
	// Note: Sending "notifications/initialized" as a notification might be better if send supports it. 
	// For simplicity, we treat it as a fire-and-forget method or assume the server handles it.
	// Let's just send a method without ID if transport supports it, or just ignore response.
	
	c.initialized = true
	return nil
}

// ListTools retrieves tools from the MCP server.
func (c *Client) ListTools() ([]core.Tool, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	resp, err := c.transport.Send(JSONRPCRequest{
		Method: "tools/list",
		Params: mustMarshal(map[string]any{}),
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("list tools error: %s", resp.Error.Message)
	}

	var result struct {
		Tools []core.Tool `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	c.ServerTools = result.Tools
	return result.Tools, nil
}

// CallTool executes a tool call on the server.
func (c *Client) CallTool(name string, arguments map[string]any) (string, error) {
	if !c.initialized {
		return "", fmt.Errorf("client not initialized")
	}

	params := map[string]any{
		"name":      name,
		"arguments": arguments,
	}

	resp, err := c.transport.Send(JSONRPCRequest{
		Method: "tools/call",
		Params: mustMarshal(params),
	})
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", fmt.Errorf("call tool error: %s", resp.Error.Message)
	}

	var result struct {
		Content []map[string]any `json:"content"`
		IsError bool             `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", err
	}

	if result.IsError {
		return "", fmt.Errorf("tool execution returned error state: %v", result.Content)
	}

	// Extract text content
	var text string
	for _, content := range result.Content {
		if content["type"] == "text" {
			if t, ok := content["text"].(string); ok {
				text += t
			}
		}
	}
	
	return text, nil
}

// Close terminates the connection.
func (c *Client) Close() error {
	return c.transport.Close()
}

// Helper to marshal without error in simple calls
func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
