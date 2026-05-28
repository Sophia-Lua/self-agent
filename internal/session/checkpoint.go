package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"autodev/internal/core"
)

// Checkpoint represents a persisted execution checkpoint.
type Checkpoint struct {
	SessionID     string                 `json:"session_id"`
	State         core.PipelineState     `json:"state"`
	TaskID        string                 `json:"task_id"`
	TaskDesc      string                 `json:"task_description"`
	History       []core.Message         `json:"history"`
	Files         map[string]string      `json:"files"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	RetryCount    int                    `json:"retry_count"`
	LastSnapshotID string                `json:"last_snapshot_id"`
}

// SaveCheckpoint persists the current execution state.
func (m *Manager) SaveCheckpoint(sessionID string, state core.PipelineState, task *core.Input, history []core.Message, retryCount int) (*Checkpoint, error) {
	// Create checkpoint directory
	checkpointDir := filepath.Join(m.workDir, ".autodev", "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	checkpoint := &Checkpoint{
		SessionID:  sessionID,
		State:      state,
		TaskID:     task.TaskDescription,
		TaskDesc:   task.TaskDescription,
		History:    history,
		Files:      task.Files,
		Metadata:   make(map[string]interface{}),
		CreatedAt:  time.Now(),
		RetryCount: retryCount,
	}

	// Write checkpoint file
	checkpointPath := filepath.Join(checkpointDir, sessionID+".json")
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to write checkpoint: %w", err)
	}

	// Generate snapshot ID without full snapshot (to avoid deadlock with mutex)
	m.mu.Lock()
	snapshotID := fmt.Sprintf("snap-%d", len(m.snapshots))
	m.snapshots = append(m.snapshots, &core.Snapshot{
		ID:        snapshotID,
		TaskID:    task.TaskDescription,
		Timestamp: checkpoint.CreatedAt.String(),
		WorkDir:   m.workDir,
		Files:     task.Files,
	})
	m.mu.Unlock()
	checkpoint.LastSnapshotID = snapshotID

	return checkpoint, nil
}

// LoadCheckpoint loads a checkpoint by session ID.
func (m *Manager) LoadCheckpoint(sessionID string) (*Checkpoint, error) {
	checkpointPath := filepath.Join(m.workDir, ".autodev", "checkpoints", sessionID+".json")

	data, err := os.ReadFile(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("checkpoint not found for session: %s", sessionID)
		}
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// ResumeSession restores execution state from a checkpoint and returns the input ready to continue.
func (m *Manager) ResumeSession(sessionID string) (*core.Input, core.PipelineState, []core.Message, int, error) {
	checkpoint, err := m.LoadCheckpoint(sessionID)
	if err != nil {
		return nil, "", nil, 0, err
	}

	input := &core.Input{
		TaskDescription: checkpoint.TaskDesc,
		History:         checkpoint.History,
		Files:           checkpoint.Files,
	}

	return input, checkpoint.State, checkpoint.History, checkpoint.RetryCount, nil
}

// DeleteCheckpoint removes a saved checkpoint.
func (m *Manager) DeleteCheckpoint(sessionID string) error {
	checkpointPath := filepath.Join(m.workDir, ".autodev", "checkpoints", sessionID+".json")
	
	if err := os.Remove(checkpointPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete checkpoint: %w", err)
		}
	}

	return nil
}

// ListCheckpoints returns all saved checkpoints.
func (m *Manager) ListCheckpoints() ([]Checkpoint, error) {
	checkpointDir := filepath.Join(m.workDir, ".autodev", "checkpoints")

	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Checkpoint{}, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint directory: %w", err)
	}

	var checkpoints []Checkpoint

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		checkpointPath := filepath.Join(checkpointDir, entry.Name())
		data, err := os.ReadFile(checkpointPath)
		if err != nil {
			continue
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal(data, &checkpoint); err == nil {
			checkpoints = append(checkpoints, checkpoint)
		}
	}

	return checkpoints, nil
}

// AutoCheckpoint creates a checkpoint automatically based on session state.
func (m *Manager) AutoCheckpoint(sessionID string, state core.PipelineState, task *core.Input, history []core.Message, retryCount int) error {
	// Only checkpoint on meaningful state transitions
	if state == core.StatePending || state == core.StateCancelled {
		return nil
	}

	_, err := m.SaveCheckpoint(sessionID, state, task, history, retryCount)
	return err
}

// ResumeFromAny tries to resume the most recent checkpoint.
func (m *Manager) ResumeFromAny() (*Checkpoint, error) {
	checkpoints, err := m.ListCheckpoints()
	if err != nil {
		return nil, err
	}

	if len(checkpoints) == 0 {
		return nil, fmt.Errorf("no checkpoints found")
	}

	// Return the most recent one
	return &checkpoints[len(checkpoints)-1], nil
}
