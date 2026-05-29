package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"autodev/internal/core"
)

// Manager handles saving and restoring workspace snapshots.
type Manager struct {
	workDir   string
	mu        sync.Mutex
	snapshots []*core.Snapshot
}

// New creates a new Session Manager.
func New(workDir string) *Manager {
	return &Manager{
		workDir:   filepath.Clean(workDir),
		snapshots: make([]*core.Snapshot, 0),
	}
}

// Snapshot captures the current state of the workspace files in memory.
func (m *Manager) CreateSnapshot(taskID string) (*core.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	files := make(map[string]string)
	
	// Walk the directory (simple mock: just read root or specific files)
	// In a real scenario, we would traverse .autodev/ignored paths.
	err := filepath.Walk(m.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.Contains(path, ".git") || strings.Contains(path, ".autodev") {
				return filepath.SkipDir
			}
			return nil
		}
		
		// Read small files only
		if info.Size() < 1024*1024 { // 1MB limit
			content, err := os.ReadFile(path)
			if err == nil {
				relPath, _ := filepath.Rel(m.workDir, path)
				files[relPath] = string(content)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	s := &core.Snapshot{
		ID:        fmt.Sprintf("snap-%d", len(m.snapshots)),
		TaskID:    taskID,
		Timestamp: time.Now().Format(time.RFC3339),
		WorkDir:   m.workDir,
		Files:     files,
	}
	m.snapshots = append(m.snapshots, s)
	return s, nil
}

// Restore reverts the workspace to a specific snapshot.
func (m *Manager) RestoreSnapshot(s *core.Snapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s == nil {
		return fmt.Errorf("no snapshot provided")
	}

	for path, content := range s.Files {
		target := filepath.Join(m.workDir, path)
		
		// Create directories if needed
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		
		if err := os.WriteFile(target, []byte(content), 0644); err != nil {
			return err
		}
	}
	
	// Note: This does not delete new files created after the snapshot.
	// A full restore would require comparing the current state with the snapshot.
	return nil
}
