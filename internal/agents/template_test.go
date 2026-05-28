package agents

import (
	"strings"
	"testing"
)

func TestRenderTemplateBasic(t *testing.T) {
	tmpl := "Hello {{.name}}, welcome to {{.project}}!"
	config := TemplateConfig{
		Variables: map[string]string{
			"name":    "Alice",
			"project": "AutoDev",
		},
	}

	result, err := RenderTemplate(tmpl, config)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	expected := "Hello Alice, welcome to AutoDev!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRenderTemplateEmpty(t *testing.T) {
	result, err := RenderTemplate("", TemplateConfig{})
	if err != nil {
		t.Fatalf("Should handle empty string: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty string, got '%s'", result)
	}
}

func TestRenderTemplateMissingVariable(t *testing.T) {
	tmpl := "Hello {{.name}}"
	config := TemplateConfig{
		Variables: map[string]string{},
		StrictMode: false,
	}

	// Go templates show <no value> for missing variables by default
	result, err := RenderTemplate(tmpl, config)
	if err != nil {
		t.Fatalf("Should not fail in non-strict mode: %v", err)
	}

	// In non-strict mode with missing variables, result contains <no value>
	if !strings.Contains(result, "<no value>") {
		t.Errorf("Expected '<no value>' in result, got '%s'", result)
	}
}

func TestRenderTemplateCustomDelimiters(t *testing.T) {
	tmpl := "Hello [[.name]]!"
	config := TemplateConfig{
		Variables: map[string]string{
			"name": "World",
		},
		Delimiters: [2]string{"[[", "]]"},
	}

	result, err := RenderTemplate(tmpl, config)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	expected := "Hello World!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRenderTemplateWithDefaultFunc(t *testing.T) {
	// The default function takes: default defaultValue actualValue
	// Test with empty value to trigger default
	tmpl := "Hello {{default \"Guest\" .name}}!"
	config := TemplateConfig{
		Variables: map[string]string{
			"name": "", // Empty value should trigger default
		},
	}

	result, err := RenderTemplate(tmpl, config)
	if err != nil {
		t.Fatalf("RenderTemplate failed: %v", err)
	}

	expected := "Hello Guest!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestResolveVariables(t *testing.T) {
	tmpl := "Hello {{.name}}, your project is {{.project}}!"
	config := TemplateConfig{
		Variables: map[string]string{
			"name": "Alice",
		},
	}

	missing := ResolveVariables(tmpl, config)

	if len(missing) != 1 {
		t.Errorf("Expected 1 missing variable, got %d: %v", len(missing), missing)
	}
	if missing[0] != "project" {
		t.Errorf("Expected 'project' to be missing, got '%s'", missing[0])
	}
}

func TestHelperFunctions(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		{"upper", toUpper, "hello", "HELLO"},
		{"lower", toLower, "HELLO", "hello"},
		{"title", toTitle, "hello world", "Hello World"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.fn(tc.input)
			if result != tc.expected {
				t.Errorf("%s('%s') = '%s', want '%s'", tc.name, tc.input, result, tc.expected)
			}
		})
	}
}

func TestApplyTemplateToSystemPrompt(t *testing.T) {
	executor := &Executor{
		SystemPrompt: "You are a {{.role}} agent working on {{.project}}",
	}

	config := TemplateConfig{
		Variables: map[string]string{
			"role":    "developer",
			"project": "AutoDev",
		},
	}

	err := executor.ApplyTemplateToSystemPrompt(config)
	if err != nil {
		t.Fatalf("ApplyTemplateToSystemPrompt failed: %v", err)
	}

	expected := "You are a developer agent working on AutoDev"
	if executor.SystemPrompt != expected {
		t.Errorf("Expected '%s', got '%s'", expected, executor.SystemPrompt)
	}
}

func TestApplyTemplateEmptyPrompt(t *testing.T) {
	executor := &Executor{
		SystemPrompt: "",
	}

	err := executor.ApplyTemplateToSystemPrompt(TemplateConfig{})
	if err != nil {
		t.Fatalf("Should handle empty prompt: %v", err)
	}
	if executor.SystemPrompt != "" {
		t.Error("Empty prompt should remain empty")
	}
}

func TestIndexOf(t *testing.T) {
	tests := []struct {
		s        string
		substr   string
		expected int
	}{
		{"hello world", "world", 6},
		{"hello world", "hello", 0},
		{"hello world", "xyz", -1},
		{"", "test", -1},
	}

	for _, tc := range tests {
		result := indexOf(tc.s, tc.substr)
		if result != tc.expected {
			t.Errorf("indexOf('%s', '%s') = %d, want %d", tc.s, tc.substr, result, tc.expected)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"\thello\n", "hello"},
		{"hello", "hello"},
		{"   ", ""},
		{"", ""},
	}

	for _, tc := range tests {
		result := trimSpace(tc.input)
		if result != tc.expected {
			t.Errorf("trimSpace('%s') = '%s', want '%s'", tc.input, result, tc.expected)
		}
	}
}

func TestPrepareAgentTemplate(t *testing.T) {
	executor := &Executor{}

	config := executor.PrepareAgentTemplate()

	if config.Variables == nil {
		t.Error("Variables should be initialized")
	}
	if config.StrictMode {
		t.Error("StrictMode should be false by default")
	}
}
