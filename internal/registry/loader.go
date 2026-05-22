package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"autodev/internal/agents"
	"autodev/internal/core"
	"gopkg.in/yaml.v3"
)

// AgentDef represents a YAML definition for a custom agent.
type AgentDef struct {
	Name        string            `yaml:"name"`
	Role        string            `yaml:"role"`
	Description string            `yaml:"description"`
	Model       string            `yaml:"model"`
	TimeoutSecs int               `yaml:"timeout_secs"`
	Prompt      PromptDef         `yaml:"prompt"`
}

type PromptDef struct {
	System string `yaml:"system"`
}

// Loader handles loading agents from YAML files.
type Loader struct {
	registry *agents.Registry
}

// New creates a new Agent Loader.
func New(reg *agents.Registry) *Loader {
	return &Loader{registry: reg}
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
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
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

	// Create a custom agent instance (placeholder for now)
	// In a full impl, this would parse Prompts and bind Models
	agent := &core.CustomAgent{
		IDVal:   def.Name,
		RoleVal:   core.Role(def.Role),
		DescVal: def.Description,
	}

	return l.registry.Register(agent)
}
