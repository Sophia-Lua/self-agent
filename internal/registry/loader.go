package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"autodev/internal/agents"
	"autodev/internal/core"
	"gopkg.in/yaml.v3"
)

// AgentDef represents a YAML definition for a custom agent.
type AgentDef struct {
	ID           string            `yaml:"id"`
	Name         string            `yaml:"name"`
	Role         string            `yaml:"role"`
	Description  string            `yaml:"description"`
	Provider     string            `yaml:"provider,omitempty"`
	Model        string            `yaml:"model,omitempty"`
	MaxTokens    int               `yaml:"max_tokens,omitempty"`
	MaxToolCalls int               `yaml:"max_tool_calls,omitempty"`
	SystemPrompt string            `yaml:"system_prompt"`
	Temperature  float64           `yaml:"temperature,omitempty"`
	Tools        []string          `yaml:"tools,omitempty"`
	Variables    map[string]string `yaml:"variables,omitempty"`
	Prompt       PromptDef         `yaml:"prompt"`
}

type PromptDef struct {
	System string `yaml:"system"`
}

// Loader handles loading agents from YAML files.
type Loader struct {
	registry       *agents.Registry
	variableValues map[string]string
}

// New creates a new Agent Loader.
func New(reg *agents.Registry) *Loader {
	return &Loader{
		registry:       reg,
		variableValues: make(map[string]string),
	}
}

// WithVariables sets template variable values for agent loading.
func (l *Loader) WithVariables(vars map[string]string) *Loader {
	l.variableValues = vars
	return l
}

// LoadFromDir scans a directory for *.yaml files and registers agents.
func (l *Loader) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, e := range entries {
		if e.IsDir() || (filepath.Ext(e.Name()) != ".yaml" && filepath.Ext(e.Name()) != ".yml") {
			continue
		}

		if err := l.loadFile(filepath.Join(dir, e.Name())); err != nil {
			fmt.Printf("warning: failed to load agent %s: %v\n", e.Name(), err)
		}
	}
	return nil
}

func (l *Loader) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var def AgentDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return err
	}

	// Determine agent ID (prefer id field, fallback to name, then filename)
	agentID := def.ID
	if agentID == "" {
		agentID = def.Name
	}
	if agentID == "" {
		agentID = filepath.Base(path)
		agentID = agentID[:len(agentID)-len(filepath.Ext(agentID))]
	}

	// Use prompt.system as fallback for system_prompt
	systemPrompt := def.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = def.Prompt.System
	}

	// Apply template variable resolution to system prompt
	if systemPrompt != "" && len(def.Variables) > 0 {
		templateConfig := agents.TemplateConfig{
			Delimiters: [2]string{"{{", "}}"},
			StrictMode: false,
		}

		// Merge YAML variables with loader-level variables
		mergedVars := make(map[string]string)
		for k, v := range def.Variables {
			mergedVars[k] = v
		}
		for k, v := range l.variableValues {
			mergedVars[k] = v
		}
		templateConfig.Variables = mergedVars

		rendered, err := agents.RenderTemplate(systemPrompt, templateConfig)
		if err == nil {
			systemPrompt = rendered
		}
	}

	// Create custom agent with resolved prompt
	agent := &core.CustomAgent{
		IDVal:   agentID,
		RoleVal:   core.Role(def.Role),
		DescVal: def.Description,
		ExecFunc: func(ctx context.Context, input core.Input) (*core.Output, error) {
			return &core.Output{
				Status:  core.StatusSuccess,
				Message: fmt.Sprintf("Custom agent %s executed: %s", agentID, input.TaskDescription),
			}, nil
		},
	}

	return l.registry.Register(agent)
}
