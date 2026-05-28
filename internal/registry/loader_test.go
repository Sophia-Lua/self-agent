package registry

import (
	"os"
	"path/filepath"
	"testing"

	"autodev/internal/agents"
	"autodev/internal/core"
)

func TestNewLoader(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	if l == nil {
		t.Fatal("New returned nil")
	}
	if l.registry != reg {
		t.Error("registry not set correctly")
	}
	if l.variableValues == nil {
		t.Error("variableValues should be initialized")
	}
}

func TestLoaderWithVariables(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	vars := map[string]string{
		"name":    "test-agent",
		"version": "1.0",
	}
	l2 := l.WithVariables(vars)

	if l2 != l {
		t.Error("WithVariables should return the same loader")
	}
	if l.variableValues["name"] != "test-agent" {
		t.Errorf("expected name=test-agent, got %s", l.variableValues["name"])
	}
}

func TestLoadFromDirNonExistent(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	err := l.LoadFromDir("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Errorf("expected nil error for non-existent directory, got %v", err)
	}
}

func TestLoadFromDirEmptyDir(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	tmpDir := t.TempDir()

	err := l.LoadFromDir(tmpDir)
	if err != nil {
		t.Errorf("expected nil error for empty directory, got %v", err)
	}
}

func TestLoadFileValidYAML(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yamlContent := `
id: test-agent
name: Test Agent
role: developer
description: A test agent
system_prompt: "You are a test agent"
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test-agent.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := l.loadFile(yamlPath)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	_, err = reg.Get("test-agent")
	if err != nil {
		t.Errorf("expected agent to be registered, got error: %v", err)
	}
}

func TestLoadFileInvalidYAML(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yamlContent := `
id: bad-agent
name: Invalid
  - this is not: valid: yaml
    broken: [
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "bad-agent.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := l.loadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadFileWithVariables(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg).WithVariables(map[string]string{
		"NAME": "World",
	})

	yamlContent := `
id: var-agent
name: Variable Agent
role: developer
description: Agent with variables
system_prompt: "Hello {{NAME}}"
variables:
  NAME: "{{NAME}}"
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "var-agent.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := l.loadFile(yamlPath)
	if err != nil {
		t.Fatalf("expected nil error with variables, got %v", err)
	}

	agent, err := reg.Get("var-agent")
	if err != nil {
		t.Fatalf("expected agent to be registered, got: %v", err)
	}
	if agent.ID() != "var-agent" {
		t.Errorf("expected agent ID var-agent, got: %s", agent.ID())
	}
}

func TestLoadFilePromptFallback(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yamlContent := `
id: prompt-fallback-agent
name: Prompt Fallback Agent
role: developer
description: Uses prompt.system fallback
prompt:
  system: "Use prompt.system as system_prompt"
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "prompt-fallback.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := l.loadFile(yamlPath)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	agent, err := reg.Get("prompt-fallback-agent")
	if err != nil {
		t.Fatalf("expected agent to be registered, got: %v", err)
	}
	if agent.Role() != core.RoleDeveloper {
		t.Errorf("expected developer role, got: %s", agent.Role())
	}
}

func TestLoadFileSystemPromptTakesPrecedence(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yamlContent := `
id: precedence-agent
name: Precedence Agent
role: developer
description: Both fields present
system_prompt: "System prompt value"
prompt:
  system: "Prompt.system value"
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "precedence.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := l.loadFile(yamlPath)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	agent, err := reg.Get("precedence-agent")
	if err != nil {
		t.Fatalf("expected agent to be registered, got: %v", err)
	}
	if agent.Description() != "Both fields present" {
		t.Errorf("unexpected description: %s", agent.Description())
	}
}

func TestLoadFileIDFallback(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yamlContent := `
name: fallback-id-agent
role: developer
description: Uses name as ID
system_prompt: "Hello"
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "some-file.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err := l.loadFile(yamlPath)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	_, err = reg.Get("fallback-id-agent")
	if err != nil {
		t.Errorf("expected agent registered with name as ID, got: %v", err)
	}
}

func TestLoadFileDuplicateRegistration(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yamlContent := `
id: dup-agent
name: Dup Agent
role: developer
description: Duplicate test
system_prompt: "Hello"
`
	tmpDir := t.TempDir()

	path1 := filepath.Join(tmpDir, "dup1.yaml")
	path2 := filepath.Join(tmpDir, "dup2.yaml")
	if err := os.WriteFile(path1, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(path2, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	err1 := l.loadFile(path1)
	if err1 != nil {
		t.Fatalf("first load should succeed, got: %v", err1)
	}

	err2 := l.loadFile(path2)
	if err2 == nil {
		t.Error("second load should fail with duplicate error")
	}
}

func TestLoadFromDirMultipleFiles(t *testing.T) {
	reg := agents.NewRegistry()
	l := New(reg)

	yaml1 := `id: agent-a
name: Agent A
role: developer
description: Agent A
system_prompt: "A"`
	yaml2 := `id: agent-b
name: Agent B
role: developer
description: Agent B
system_prompt: "B"`
	yaml3 := `invalid yaml: [`

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte(yaml1), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.yml"), []byte(yaml2), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.yaml"), []byte(yaml3), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("not yaml"), 0644)

	err := l.LoadFromDir(tmpDir)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	count := len(reg.List())
	if count != 2 {
		t.Errorf("expected 2 agents loaded, got %d", count)
	}
}
