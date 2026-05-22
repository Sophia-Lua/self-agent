package mcp

// ServerDef defines how to start an MCP server via stdio.
type ServerDef struct {
	Command string   `json:"command" yaml:"command"`
	Args    []string `json:"args" yaml:"args,omitempty"`
	Name    string   `json:"name" yaml:"name"`
}
