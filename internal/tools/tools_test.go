package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"autodev/internal/core"
)

func TestNewRegistry(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("registry is nil")
	}
}

func TestRegisterAndList(t *testing.T) {
	r := New()

	r.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "test_tool",
			Description: "A test tool",
			Parameters:  map[string]any{},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		return "result", nil
	})

	tools := r.List()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Function.Name != "test_tool" {
		t.Errorf("tool name = '%s', want 'test_tool'", tools[0].Function.Name)
	}
}

func TestExecuteLocalTool(t *testing.T) {
	r := New()

	r.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "calculator",
			Description: "Adds two numbers",
			Parameters:  map[string]any{},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		a, _ := args["a"].(float64)
		b, _ := args["b"].(float64)
		return map[string]float64{"sum": a + b}, nil
	})

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "calculator",
			Arguments: `{"a": 3, "b": 5}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	var m map[string]float64
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m["sum"] != 8 {
		t.Errorf("sum = %f, want 8", m["sum"])
	}
}

func TestExecuteNotFound(t *testing.T) {
	r := New()

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "nonexistent_tool",
			Arguments: `{}`,
		},
	}

	_, err := r.Execute(context.Background(), call)
	if err == nil {
		t.Error("expected error for nonexistent tool")
	}
}

func TestExecuteInvalidArguments(t *testing.T) {
	r := New()

	r.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name:        "test_tool",
			Description: "A test tool",
			Parameters:  map[string]any{},
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		return "ok", nil
	})

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "test_tool",
			Arguments: `{invalid json`,
		},
	}

	_, err := r.Execute(context.Background(), call)
	if err == nil {
		t.Error("expected error for invalid JSON arguments")
	}
}

func TestWriteFileAndReadFile(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	// Write
	writeCall := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "write_file",
			Arguments: `{"path": "test.txt", "content": "hello world"}`,
		},
	}

	result, err := r.Execute(context.Background(), writeCall)
	if err != nil {
		t.Fatalf("write_file failed: %v", err)
	}
	if result == "" {
		t.Error("write result should not be empty")
	}

	// Read
	readCall := core.ToolCall{
		ID:   "call-2",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "read_file",
			Arguments: `{"path": "test.txt"}`,
		},
	}

	result, err = r.Execute(context.Background(), readCall)
	if err != nil {
		t.Fatalf("read_file failed: %v", err)
	}

	var readResult map[string]string
	if err := json.Unmarshal([]byte(result), &readResult); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if readResult["content"] != "hello world" {
		t.Errorf("content = '%s', want 'hello world'", readResult["content"])
	}
}

func TestWriteFileCreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "write_file",
			Arguments: `{"path": "a/b/c/deep.txt", "content": "deep nested"}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("write_file failed: %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, "a/b/c/deep.txt")
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(content) != "deep nested" {
		t.Errorf("content = '%s', want 'deep nested'", string(content))
	}
	_ = result
}

func TestReadFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "read_file",
			Arguments: `{"path": "nonexistent.txt"}`,
		},
	}

	_, err := r.Execute(context.Background(), call)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestWriteFileMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "write_file",
			Arguments: `{"content": "test"}`,
		},
	}

	_, err := r.Execute(context.Background(), call)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestReadFileMissingPath(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "read_file",
			Arguments: `{}`,
		},
	}

	_, err := r.Execute(context.Background(), call)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestToolError(t *testing.T) {
	err := ErrMissingPath
	if err == nil {
		t.Fatal("ErrMissingPath is nil")
	}
	if err.Error() == "" {
		t.Error("error message should not be empty")
	}

	customErr := &ToolError{msg: "custom error"}
	if customErr.Error() != "custom error" {
		t.Errorf("error = '%s', want 'custom error'", customErr.Error())
	}
}

func TestListEmptyRegistry(t *testing.T) {
	r := New()
	tools := r.List()
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestRegisterOverwritesLocalWithMCP(t *testing.T) {
	r := New()

	// Register a local tool
	r.Register(core.Tool{
		Type: "function",
		Function: core.ToolFunction{
			Name: "existing_tool",
		},
	}, func(ctx context.Context, args map[string]any) (any, error) {
		return "local", nil
	})

	// Execute should return local result
	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "existing_tool",
			Arguments: `{}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != `"local"` {
		t.Errorf("result = '%s', want '\"local\"'", result)
	}
}

func TestRegisterMultipleTools(t *testing.T) {
	r := New()

	for i := 0; i < 10; i++ {
		r.Register(core.Tool{
			Type: "function",
			Function: core.ToolFunction{
				Name:        string(rune('a' + i)),
				Description: "tool",
				Parameters:  map[string]any{},
			},
		}, func(ctx context.Context, args map[string]any) (any, error) {
			return "ok", nil
		})
	}

	tools := r.List()
	if len(tools) != 10 {
		t.Errorf("expected 10 tools, got %d", len(tools))
	}
}

func TestPathTraversalPrevention(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	attempts := []string{
		"../etc/passwd",
		"../../etc/shadow",
		"foo/../../etc/hosts",
	}

	for _, path := range attempts {
		call := core.ToolCall{
			ID:   "call-1",
			Type: "function",
			Function: core.FunctionCall{
				Name:      "read_file",
				Arguments: `{"path": "` + path + `"}`,
			},
		}

		_, err := r.Execute(context.Background(), call)
		if err == nil {
			t.Errorf("expected error for path traversal attempt: %s", path)
		}
	}
}

func TestWriteFileTraversalPrevention(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "write_file",
			Arguments: `{"path": "../escape.txt", "content": "escaped"}`,
		},
	}

	_, err := r.Execute(context.Background(), call)
	if err == nil {
		t.Error("expected error for write path traversal attempt")
	}
}

func TestListFilesWithPattern(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.md"), []byte(""), 0644)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "list_files",
			Arguments: `{"path": ".", "pattern": "*.go"}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("list_files failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	count, _ := m["count"].(float64)
	if int(count) != 1 {
		t.Errorf("expected 1 .go file, got %d", int(count))
	}
}

func TestSearchFiles(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "hello.go"), []byte("package main\nfunc Hello() { }"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other.py"), []byte("def other(): pass"), 0644)

	// Search by name pattern
	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "search_files",
			Arguments: `{"path": ".", "name_pattern": "*.go"}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("search_files failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	count, _ := m["count"].(float64)
	if int(count) != 1 {
		t.Errorf("expected 1 .go file, got %d", int(count))
	}
}

func TestExecuteCommand(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "execute_command",
			Arguments: `{"command": "echo hello"}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("execute_command failed: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	output, _ := m["output"].(string)
	if strings.TrimSpace(output) != "hello" {
		t.Errorf("expected 'hello', got %q", output)
	}
}

func TestExecuteCommandBadExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	r := New()
	RegisterFileTools(r, tmpDir)

	call := core.ToolCall{
		ID:   "call-1",
		Type: "function",
		Function: core.FunctionCall{
			Name:      "execute_command",
			Arguments: `{"command": "exit 42"}`,
		},
	}

	result, err := r.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("execute_command should not error on non-zero exit: %v", err)
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	exitCode, _ := m["exit_code"].(float64)
	if int(exitCode) != 42 {
		t.Errorf("expected exit code 42, got %d", int(exitCode))
	}
}
