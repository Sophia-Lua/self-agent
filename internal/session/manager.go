package session

import (
	"fmt"
	"path/filepath"
	"time"

	"autodev/internal/core"
)

// Manager handles saving and restoring workspace snapshots.
type Manager struct {
	workDir   string
	snapshots []*core.Snapshot
}

// New creates a new Session Manager.
func New(workDir string) *Manager {
	return &Manager{
		workDir:   workDir,
		snapshots: make([]*core.Snapshot, 0),
	}
}

// Snapshot captures the current state of the workspace (mocked for now).
func (m *Manager) CreateSnapshot(taskID string) (*core.Snapshot, error) {
	s := &core.Snapshot{
		ID:        fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		TaskID:    taskID,
		Timestamp: time.Now().Format(time.RFC3339),
		WorkDir:   filepath.Clean(m.workDir),
		Files:     make(map[string]string), // In real impl, scan files
	}
	m.snapshots = append(m.snapshots, s)
	return s, nil
}

// Restore reverts the workspace to a specific snapshot.
func (m *Manager) RestoreSnapshot(s *core.Snapshot) error {
	// Implementation would:
	// 1. Delete current modified files
	// 2. Restore files from s.Files
	// 3. Reset git state if s.GitRef is present
	return nil
}
