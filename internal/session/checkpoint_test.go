package session

import (
	"os"
	"path/filepath"
	"testing"

	"autodev/internal/core"
)

func TestSaveAndLoadCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{
		TaskDescription: "Add user authentication",
		Files:           map[string]string{"main.go": "package main"},
	}
	history := []core.Message{
		{Role: "user", Content: "task"},
	}

	cp, err := mgr.SaveCheckpoint("test-session-1", core.StateDeveloping, input, history, 0)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	if cp.SessionID != "test-session-1" {
		t.Errorf("Expected session ID 'test-session-1', got '%s'", cp.SessionID)
	}
	if cp.State != core.StateDeveloping {
		t.Errorf("Expected state 'developing', got '%s'", cp.State)
	}

	// Load the checkpoint
	loaded, err := mgr.LoadCheckpoint("test-session-1")
	if err != nil {
		t.Fatalf("LoadCheckpoint failed: %v", err)
	}

	if loaded.TaskDesc != "Add user authentication" {
		t.Errorf("Expected task 'Add user authentication', got '%s'", loaded.TaskDesc)
	}
	if len(loaded.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(loaded.Files))
	}
}

func TestLoadCheckpointNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	_, err := mgr.LoadCheckpoint("nonexistent")
	if err == nil {
		t.Error("Should return error for nonexistent checkpoint")
	}
}

func TestResumeSession(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{
		TaskDescription: "Fix login bug",
		Files:           map[string]string{"auth.go": "package auth"},
	}
	history := []core.Message{{Role: "user", Content: "history entry"}}

	_, err := mgr.SaveCheckpoint("resume-test", core.StateTesting, input, history, 2)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	resumedInput, state, hist, retries, err := mgr.ResumeSession("resume-test")
	if err != nil {
		t.Fatalf("ResumeSession failed: %v", err)
	}

	if resumedInput.TaskDescription != "Fix login bug" {
		t.Errorf("Expected task 'Fix login bug', got '%s'", resumedInput.TaskDescription)
	}
	if state != core.StateTesting {
		t.Errorf("Expected state 'testing', got '%s'", state)
	}
	if len(hist) != 1 {
		t.Errorf("Expected 1 history entry, got %d", len(hist))
	}
	if retries != 2 {
		t.Errorf("Expected 2 retries, got %d", retries)
	}
}

func TestDeleteCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{TaskDescription: "Test"}
	_, err := mgr.SaveCheckpoint("delete-test", core.StateParsing, input, nil, 0)
	if err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}

	// Delete should succeed
	if err := mgr.DeleteCheckpoint("delete-test"); err != nil {
		t.Fatalf("DeleteCheckpoint failed: %v", err)
	}

	// Loading after delete should fail
	_, err = mgr.LoadCheckpoint("delete-test")
	if err == nil {
		t.Error("Loading deleted checkpoint should fail")
	}
}

func TestListCheckpoints(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{TaskDescription: "Task 1"}
	mgr.SaveCheckpoint("session-a", core.StateParsing, input, nil, 0)
	mgr.SaveCheckpoint("session-b", core.StateDeveloping, input, nil, 0)

	checkpoints, err := mgr.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}

	if len(checkpoints) != 2 {
		t.Errorf("Expected 2 checkpoints, got %d", len(checkpoints))
	}
}

func TestListCheckpointsEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	checkpoints, err := mgr.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints should handle empty dir: %v", err)
	}
	if len(checkpoints) != 0 {
		t.Errorf("Expected 0 checkpoints, got %d", len(checkpoints))
	}
}

func TestResumeFromAny(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	_, err := mgr.ResumeFromAny()
	if err == nil {
		t.Error("Should fail when no checkpoints exist")
	}

	input := &core.Input{TaskDescription: "Task"}
	mgr.SaveCheckpoint("first", core.StateParsing, input, nil, 0)
	mgr.SaveCheckpoint("second", core.StateDeveloping, input, nil, 0)

	cp, err := mgr.ResumeFromAny()
	if err != nil {
		t.Fatalf("ResumeFromAny failed: %v", err)
	}
	if cp.SessionID != "second" {
		t.Errorf("Expected most recent checkpoint 'second', got '%s'", cp.SessionID)
	}
}

func TestAutoCheckpointSkipsPending(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{TaskDescription: "Task"}

	err := mgr.AutoCheckpoint("skip-test", core.StatePending, input, nil, 0)
	if err != nil {
		t.Errorf("AutoCheckpoint should not error for pending state: %v", err)
	}

	// Should not have saved a checkpoint
	checkpoints, _ := mgr.ListCheckpoints()
	if len(checkpoints) != 0 {
		t.Error("AutoCheckpoint should skip pending state")
	}
}

func TestAutoCheckpointCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{TaskDescription: "Task"}

	err := mgr.AutoCheckpoint("cancel-test", core.StateCancelled, input, nil, 0)
	if err != nil {
		t.Errorf("AutoCheckpoint should not error for cancelled state: %v", err)
	}

	checkpoints, _ := mgr.ListCheckpoints()
	if len(checkpoints) != 0 {
		t.Error("AutoCheckpoint should skip cancelled state")
	}
}

func TestCheckpointDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := New(tmpDir)

	input := &core.Input{TaskDescription: "Task"}
	_, err := mgr.SaveCheckpoint("dir-test", core.StateDeveloping, input, nil, 0)
	if err != nil {
		t.Fatalf("Failed to save checkpoint: %v", err)
	}

	// Verify directory was created
	checkpointDir := filepath.Join(tmpDir, ".autodev", "checkpoints")
	info, err := os.Stat(checkpointDir)
	if err != nil {
		t.Fatalf("Checkpoint directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Should be a directory")
	}
}
