package core

import "context"

// PipelineState represents the current stage of the autonomous execution loop.
type PipelineState string

const (
	StatePending     PipelineState = "pending"
	StateParsing     PipelineState = "parsing"
	StateDeveloping  PipelineState = "developing"
	StateTesting     PipelineState = "testing"
	StateChecking    PipelineState = "checking"
	StateRecovering  PipelineState = "recovering"
	StateRollback    PipelineState = "rollback"
	StateCompleted   PipelineState = "completed"
	StateCancelled   PipelineState = "cancelled"
	StateFailed      PipelineState = "failed"
)

// Snapshot encapsulates the state of the workspace at a specific point in time.
// It is used for rollback operations.
type Snapshot struct {
	ID        string            `json:"id"`
	TaskID    string            `json:"task_id"`
	Timestamp string            `json:"timestamp"`
	WorkDir   string            `json:"work_dir"`
	Files     map[string]string `json:"files"`            // Path -> Content Hash or Content
	GitRef    string            `json:"git_ref,omitempty"` // Commit hash if using Git
}

// Orchestrator defines the interface for the main pipeline execution engine.
type Orchestrator interface {
	// Run starts the pipeline execution for a given task.
	Run(ctx context.Context, input *Input) (*Output, error)

	// State returns the current state of the pipeline.
	State() PipelineState

	// Rollback restores the workspace to the last valid snapshot.
	Rollback() error

	// Pause interrupts execution and waits for User resume.
	Pause() error

	// Resume continues execution after a pause.
	Resume() error
}

// Snapshotter is responsible for creating and restoring workspace snapshots.
type Snapshotter interface {
	// CreateSnapshot saves the current state of the workspace.
	CreateSnapshot(taskID string) (*Snapshot, error)
	
	// RestoreSnapshot reverts the workspace to the given snapshot.
	RestoreSnapshot(snapshot *Snapshot) error
}
