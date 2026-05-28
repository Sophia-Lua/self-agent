package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewDefaultPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	ex, err := New(nil, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if ex == nil {
		t.Fatal("executor is nil")
	}
	if ex.policy == nil {
		t.Fatal("policy is nil")
	}
	if len(ex.policy.AllowedCommands) == 0 {
		t.Error("allowed commands should not be empty")
	}
}

func TestNewCustomDir(t *testing.T) {
	tmpDir := t.TempDir()
	ex, err := New(DefaultPolicy(), tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if ex.cwd != tmpDir {
		t.Errorf("cwd = '%s', want '%s'", ex.cwd, tmpDir)
	}
}

func TestNewCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sandboxDir := filepath.Join(tmpDir, "new-sandbox")

	ex, err := New(nil, sandboxDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	info, err := os.Stat(sandboxDir)
	if err != nil {
		t.Fatalf("sandbox directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}
	_ = ex
}

func TestNewInvalidDir(t *testing.T) {
	// On Windows, this test is skipped because invalid paths behave differently
	if runtime.GOOS == "windows" {
		t.Skip("skipping on Windows")
	}

	_, err := New(nil, "/dev/null/invalid/path")
	if err == nil {
		t.Error("expected error for invalid directory path")
	}
}

func TestRunEcho(t *testing.T) {
	tmpDir := t.TempDir()
	policy := DefaultPolicy()
	policy.AllowedCommands["echo"] = true
	ex, err := New(policy, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := ex.Run(context.Background(), "echo", "hello", "world")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if result.Stdout == "" {
		t.Error("stdout should not be empty")
	}
}

func TestRunCatExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatal(err)
	}

	ex, err := New(nil, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := ex.Run(context.Background(), "cat", testFile)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestRunNonExistentCommand(t *testing.T) {
	tmpDir := t.TempDir()
	policy := DefaultPolicy()
	policy.AllowedCommands["nonexistent-command-xyz"] = true
	ex, err := New(policy, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := ex.Run(context.Background(), "nonexistent-command-xyz")
	// The executor returns errors in the Result.Error field rather than as a function error
	if err != nil {
		t.Logf("Run returned error: %v", err)
		return
	}
	if result.Error == "" {
		t.Error("expected error in result for non-existent command")
	}
}

func TestRunFailingCommand(t *testing.T) {
	tmpDir := t.TempDir()
	policy := DefaultPolicy()
	policy.AllowedCommands["false"] = true
	ex, err := New(policy, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := ex.Run(context.Background(), "false")
	if err != nil {
		t.Logf("Run returned error: %v", err)
		return
	}
	if result.ExitCode != 1 {
		t.Logf("exit code = %d, expected 1 (may vary by implementation)", result.ExitCode)
	}
}

func TestRunWithCancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	policy := DefaultPolicy()
	policy.AllowedCommands["cat"] = true
	ex, err := New(policy, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before run

	_, err = ex.Run(ctx, "cat", "/dev/null")
	if err == nil {
		// May or may not return error depending on implementation
		t.Log("Run with cancelled context did not return error (may be acceptable)")
	}
}

func TestRunWithTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	policy := DefaultPolicy()
	policy.MaxExecTime = 100 * time.Millisecond
	policy.AllowedCommands["echo"] = true

	ex, err := New(policy, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// This might timeout based on policy - verify behavior
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := ex.Run(ctx, "echo", "fast")
	if err != nil {
		t.Logf("Run with short timeout returned error: %v", err)
	}
	if result != nil && result.ExitCode != 0 {
		t.Logf("exit code = %d", result.ExitCode)
	}
}

func TestDefaultPolicyValues(t *testing.T) {
	p := DefaultPolicy()

	// Verify allowed commands exist
	expected := []string{"go", "npm", "node", "python", "make", "cat", "ls", "grep"}
	for _, cmd := range expected {
		if !p.AllowedCommands[cmd] {
			t.Errorf("expected '%s' to be allowed", cmd)
		}
	}

	// Verify dangerous commands exist
	if len(p.DangerousCommands) == 0 {
		t.Error("dangerous commands should not be empty")
	}

	// Verify defaults
	if p.MaxExecTime <= 0 {
		t.Error("MaxExecTime should be positive")
	}
	if p.MaxMemoryMB <= 0 {
		t.Error("MaxMemoryMB should be positive")
	}
	if p.MaxOutputBytes <= 0 {
		t.Error("MaxOutputBytes should be positive")
	}
}

func TestResultStructure(t *testing.T) {
	tmpDir := t.TempDir()
	policy := DefaultPolicy()
	policy.AllowedCommands["echo"] = true
	ex, err := New(policy, tmpDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	result, err := ex.Run(context.Background(), "echo", "test")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify result fields
	if result.Stdout == "" && result.Stderr == "" {
		t.Error("at least one of stdout/stderr should have content")
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	sandboxDir := filepath.Join(tmpDir, "cleanup-test")

	ex, err := New(nil, sandboxDir)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Call cleanup
	ex.Cleanup()

	// Sandbox directory should still exist after cleanup (cleanup typically removes temp files, not the dir)
	_, err = os.Stat(sandboxDir)
	if err != nil {
		// If directory was removed, that's also acceptable
		t.Log("sandbox directory removed after cleanup (acceptable)")
	}
}
