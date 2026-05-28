package agents

import (
	"bytes"
	"fmt"
	"text/template"
)

// TemplateConfig holds template rendering configuration.
type TemplateConfig struct {
	Variables   map[string]string
	Delimiters  [2]string // Custom delimiters, defaults to "{{", "}}"
	StrictMode  bool      // Error on missing variables
}

// RenderTemplate renders a template string with the given variables.
func RenderTemplate(tmpl string, config TemplateConfig) (string, error) {
	if tmpl == "" {
		return "", nil
	}

	delims := config.Delimiters
	if delims[0] == "" {
		delims = [2]string{"{{", "}}"}
	}

	// Add helper functions
	funcMap := template.FuncMap{
		"upper": func(s string) string {
			return toUpper(s)
		},
		"lower": func(s string) string {
			return toLower(s)
		},
		"title": func(s string) string {
			return toTitle(s)
		},
		"default": func(def, val string) string {
			if val == "" {
				return def
			}
			return val
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"neq": func(a, b interface{}) bool {
			return a != b
		},
		"join": func(sep string, items []string) string {
			result := ""
			for i, item := range items {
				if i > 0 {
					result += sep
				}
				result += item
			}
			return result
		},
	}

	// Parse template with functions
	t, err := template.New("prompt").Delims(delims[0], delims[1]).Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}

	// Build template data
	data := make(map[string]interface{})
	for k, v := range config.Variables {
		data[k] = v
	}

	// Execute template
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		if config.StrictMode {
			return "", fmt.Errorf("template render error: %w", err)
		}
		// In non-strict mode, return original template
		return tmpl, nil
	}

	return buf.String(), nil
}

// ResolveVariables validates all required variables are present.
func ResolveVariables(templateStr string, config TemplateConfig) []string {
	var missing []string

	// Apply default delimiters
	delims := config.Delimiters
	if delims[0] == "" {
		delims = [2]string{"{{", "}}"}
	}

	// Simple scan for {{ var }} patterns
	tmpl := templateStr
	for {
		start := indexOf(tmpl, delims[0])
		if start == -1 {
			break
		}

		end := indexOf(tmpl[start+len(delims[0]):], delims[1])
		if end == -1 {
			break
		}

		varName := trimSpace(tmpl[start+len(delims[0]) : start+len(delims[0])+end])
		// Strip leading dot if present (e.g., ".name" -> "name")
		if len(varName) > 1 && varName[0] == '.' {
			varName = varName[1:]
		}

		// Check if variable is defined
		if _, ok := config.Variables[varName]; !ok {
			missing = append(missing, varName)
		}

		tmpl = tmpl[start+len(delims[0])+end+len(delims[1]):]
	}

	return missing
}

// ApplyTemplateToSystemPrompt updates the executor's system prompt with template rendering.
func (e *Executor) ApplyTemplateToSystemPrompt(config TemplateConfig) error {
	if e.SystemPrompt == "" {
		return nil
	}

	rendered, err := RenderTemplate(e.SystemPrompt, config)
	if err != nil {
		return err
	}

	e.SystemPrompt = rendered
	return nil
}

// PrepareAgentTemplate prepares a template configuration for the agent.
func (e *Executor) PrepareAgentTemplate() TemplateConfig {
	return TemplateConfig{
		Variables:  make(map[string]string),
		StrictMode: false,
	}
}

// Helper functions for template

func toUpper(s string) string {
	result := make([]byte, len(s))
	for i, c := range s {
		if c >= 'a' && c <= 'z' {
			result[i] = byte(c - 32)
		} else {
			result[i] = byte(c)
		}
	}
	return string(result)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			result[i] = byte(c + 32)
		} else {
			result[i] = byte(c)
		}
	}
	return string(result)
}

func toTitle(s string) string {
	if len(s) == 0 {
		return s
	}
	result := make([]byte, len(s))
	capitalize := true
	for i, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			capitalize = true
			result[i] = byte(c)
		} else {
			if capitalize {
				if c >= 'a' && c <= 'z' {
					result[i] = byte(c - 32)
				} else {
					result[i] = byte(c)
				}
				capitalize = false
			} else {
				result[i] = byte(c)
			}
		}
	}
	return string(result)
}

func indexOf(s string, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
