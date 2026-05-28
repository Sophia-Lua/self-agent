package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Policy defines what operations are allowed in the sandbox.
type Policy struct {
	AllowedCommands   map[string]bool
	AllowedDirs       []string
	MaxExecTime       time.Duration
	MaxMemoryMB       int
	MaxOutputBytes    int
	NetworkAccess     bool
	FileWriteAccess   bool
	DangerousCommands []string
}

// DefaultPolicy returns a restrictive but usable default policy.
func DefaultPolicy() *Policy {
	return &Policy{
		AllowedCommands: map[string]bool{
			"go":    true,
			"npm":   true,
			"node":  true,
			"python": true,
			"pip":   true,
			"make":  true,
			"cat":   true,
			"ls":    true,
			"find":  true,
			"grep":  true,
			"sed":   true,
			"awk":   true,
			"diff":  true,
			"head":  true,
			"tail":  true,
			"wc":    true,
			"sort":  true,
			"uniq":  true,
			"echo":  true,
		},
		AllowedDirs:    []string{"/workspace"},
		MaxExecTime:    30 * time.Second,
		MaxMemoryMB:    512,
		MaxOutputBytes: 10 * 1024 * 1024, // 10MB
		NetworkAccess:  false,
		FileWriteAccess: true,
		DangerousCommands: []string{
			"rm -rf", "shutdown", "reboot", "mkfs", "fdisk",
			"dd", "chmod 777", "chown root", "sudo", "su",
			"iptables", "kill -9", "curl", "wget",
		},
	}
}

// Executor runs commands within a sandboxed environment.
type Executor struct {
	mu     sync.Mutex
	policy *Policy
	cwd    string
	env    []string
}

// New creates a sandbox executor with the given policy.
func New(policy *Policy, workDir string) (*Executor, error) {
	if policy == nil {
		policy = DefaultPolicy()
	}

	if workDir == "" {
		workDir = "/tmp/autodev-sandbox"
	}

	absDir, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("invalid sandbox directory: %w", err)
	}

	if err := os.MkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directory: %w", err)
	}

	return &Executor{
		policy: policy,
		cwd:    absDir,
		env:    os.Environ(),
	}, nil
}

// Run executes a command within the sandbox.
func (e *Executor) Run(ctx context.Context, cmd string, args ...string) (*Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate command
	if err := e.validate(cmd, args); err != nil {
		return nil, err
	}

	// Apply timeout
	timeout := e.policy.MaxExecTime
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Build command
	command := exec.CommandContext(ctx, cmd, args...)
	command.Dir = e.cwd
	command.Env = e.env

	// Restrict network if policy disallows
	if !e.policy.NetworkAccess {
		// Network restriction is handled by validation
	}

	// Capture output
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr

	// Set resource limits if available (Linux)
	if e.policy.MaxMemoryMB > 0 {
		setResourceLimit(command, e.policy.MaxMemoryMB)
	}

	startTime := time.Now()
	err := command.Run()
	duration := time.Since(startTime)

	result := &Result{
		Command:    cmd,
		Args:       args,
		ExitCode:   0,
		Stdout:     truncate(stdout.String(), e.policy.MaxOutputBytes),
		Stderr:     truncate(stderr.String(), e.policy.MaxOutputBytes),
		Duration:   duration,
		TimedOut:   ctx.Err() == context.DeadlineExceeded,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Error = err.Error()
		}
	}

	return result, nil
}

// RunShell executes a shell command string within the sandbox.
func (e *Executor) RunShell(ctx context.Context, command string) (*Result, error) {
	return e.Run(ctx, "sh", "-c", command)
}

// WriteFile writes content to a file within the sandbox.
func (e *Executor) WriteFile(path, content string) error {
	if !e.isPathAllowed(path) {
		return fmt.Errorf("path %q is outside allowed directories", path)
	}

	absPath, err := filepath.Abs(filepath.Join(e.cwd, path))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(absPath, []byte(content), 0644)
}

// ReadFile reads a file within the sandbox.
func (e *Executor) ReadFile(path string) (string, error) {
	if !e.isPathAllowed(path) {
		return "", fmt.Errorf("path %q is outside allowed directories", path)
	}

	absPath, err := filepath.Abs(filepath.Join(e.cwd, path))
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// ListFiles lists files matching a pattern within the sandbox.
func (e *Executor) ListFiles(pattern string) ([]string, error) {
	if strings.Contains(pattern, "..") {
		return nil, fmt.Errorf("pattern contains path traversal")
	}

	absPattern := filepath.Join(e.cwd, pattern)
	matches, err := filepath.Glob(absPattern)
	if err != nil {
		return nil, err
	}

	// Return relative paths
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, err := filepath.Rel(e.cwd, m)
		if err == nil {
			result = append(result, rel)
		}
	}

	return result, nil
}

// SetEnv sets an environment variable for sandbox commands.
func (e *Executor) SetEnv(key, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, env := range e.env {
		if strings.HasPrefix(env, key+"=") {
			e.env[i] = key + "=" + value
			return
		}
	}
	e.env = append(e.env, key+"="+value)
}

// GetCwd returns the sandbox working directory.
func (e *Executor) GetCwd() string {
	return e.cwd
}

// Cleanup removes the sandbox directory.
func (e *Executor) Cleanup() error {
	return os.RemoveAll(e.cwd)
}

// Result holds the outcome of a sandbox command execution.
type Result struct {
	Command  string
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Error    string
	Duration time.Duration
	TimedOut bool
}

// Success checks if the command succeeded.
func (r *Result) Success() bool {
	return r.ExitCode == 0 && r.Error == ""
}

func (e *Executor) validate(cmd string, args []string) error {
	fullCmd := strings.Join(append([]string{cmd}, args...), " ")

	// Check dangerous commands
	for _, dangerous := range e.policy.DangerousCommands {
		if strings.Contains(strings.ToLower(fullCmd), strings.ToLower(dangerous)) {
			return fmt.Errorf("command %q is blocked by sandbox policy", dangerous)
		}
	}

	// Check allowed commands
	if !e.policy.AllowedCommands[cmd] {
		return fmt.Errorf("command %q is not allowed by sandbox policy", cmd)
	}

	// Check for network commands if network is disabled
	if !e.policy.NetworkAccess {
		for _, arg := range args {
			if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
				return fmt.Errorf("network access is disabled by sandbox policy")
			}
		}
	}

	return nil
}

func (e *Executor) isPathAllowed(path string) bool {
	if strings.Contains(path, "..") {
		return false
	}

	for _, allowedDir := range e.policy.AllowedDirs {
		if strings.HasPrefix(path, allowedDir) {
			return true
		}
	}

	// Check if relative path stays within allowed directories
	absPath := filepath.Clean(filepath.Join(e.cwd, path))
	for _, allowedDir := range e.policy.AllowedDirs {
		if strings.HasPrefix(absPath, filepath.Clean(allowedDir)) {
			return true
		}
	}

	return false
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "... [truncated]"
}

func setResourceLimit(cmd *exec.Cmd, maxMemMB int) {
	if maxMemMB <= 0 || runtime.GOOS != "linux" {
		return
	}

	prlimitPath, err := exec.LookPath("prlimit")
	if err != nil {
		return
	}

	maxBytes := maxMemMB * 1024 * 1024
	args := []string{
		prlimitPath,
		"--as=" + strconv.Itoa(maxBytes),
		"--",
		cmd.Path,
	}
	args = append(args, cmd.Args[1:]...)

	cmd.Path = prlimitPath
	cmd.Args = args
}
