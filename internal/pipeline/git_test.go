package pipeline

import (
	"os/exec"
	"strings"
	"testing"

	"autodev/internal/core"
)

func TestGetCurrentBranchInGitRepo(t *testing.T) {
	dir := t.TempDir()

	// Initialize a git repo
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "commit", "--allow-empty", "-m", "initial")
	runGit(t, dir, "checkout", "-b", "feature-branch")

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	branch := orch.getCurrentBranch()
	if branch != "feature-branch" {
		t.Errorf("expected 'feature-branch', got '%s'", branch)
	}
}

func TestGetCurrentBranchDefaultFallback(t *testing.T) {
	dir := t.TempDir()

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	// Not a git repo, should return default
	branch := orch.getCurrentBranch()
	if branch != "main" {
		t.Errorf("expected 'main', got '%s'", branch)
	}
}

func TestGetRemoteURLOrigin(t *testing.T) {
	dir := t.TempDir()

	// Initialize a git repo with a remote
	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", "https://github.com/owner/repo.git")

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	url, err := orch.getRemoteURL()
	if err != nil {
		t.Fatalf("getRemoteURL failed: %v", err)
	}
	if !strings.Contains(url, "github.com/owner/repo") {
		t.Errorf("expected URL to contain 'github.com/owner/repo', got '%s'", url)
	}
}

func TestGetRemoteURLNoRemote(t *testing.T) {
	dir := t.TempDir()

	// Initialize empty git repo (no remote)
	runGit(t, dir, "init")

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	_, err := orch.getRemoteURL()
	if err == nil {
		t.Error("expected error when no remote is configured")
	}
}

func TestGetRemoteURLNotGitRepo(t *testing.T) {
	dir := t.TempDir()

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	_, err := orch.getRemoteURL()
	if err == nil {
		t.Error("expected error when not a git repo")
	}
}

func TestGetRemoteURLWithSSHFormat(t *testing.T) {
	dir := t.TempDir()

	runGit(t, dir, "init")
	runGit(t, dir, "remote", "add", "origin", "git@github.com:org/project.git")

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	url, err := orch.getRemoteURL()
	if err != nil {
		t.Fatalf("getRemoteURL failed: %v", err)
	}
	if url != "git@github.com:org/project.git" {
		t.Errorf("expected 'git@github.com:org/project.git', got '%s'", url)
	}
}

func TestGetCurrentBranchDetachedHEAD(t *testing.T) {
	dir := t.TempDir()

	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "commit", "--allow-empty", "-m", "first")
	runGit(t, dir, "commit", "--allow-empty", "-m", "second")

	// Detach HEAD
	runGit(t, dir, "checkout", "--detach", "HEAD~1")

	cfg := &core.Config{WorkDir: dir}
	orch := &Orchestrator{cfg: cfg}

	branch := orch.getCurrentBranch()
	// Should fall back to main when HEAD is detached
	if branch != "main" {
		t.Errorf("expected 'main' for detached HEAD, got '%s'", branch)
	}
}

// Helper to run git command
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %v failed: %v", args, err)
	}
}
