package core

import (
	"context"
	"fmt"
)

// Role defines the specific capability and responsibility of an agent within the pipeline.
type Role string

const (
	RoleParser   Role = "parser"
	RoleDeveloper Role = "developer"
	RoleTester   Role = "tester"
	RoleChecker  Role = "checker"
	RoleRecovery Role = "recovery"
	RoleCustom   Role = "custom"
)

// Input represents the standardized input payload for any agent execution.
type Input struct {
	TaskDescription string            `json:"task_description"`
	Context         string            `json:"context,omitempty"`
	History         []Message         `json:"history,omitempty"`
	Files           map[string]string `json:"files,omitempty"`        // Path -> Content
	Config          map[string]any    `json:"config,omitempty"`       // Agent-specific overrides
	MaxRetries      int               `json:"max_retries,omitempty"`
}

// Output represents the standardized result returned by an agent.
type Output struct {
	Status       ExecutionStatus     `json:"status"`
	Message      string              `json:"message,omitempty"`
	ModifiedFiles map[string]string  `json:"modified_files,omitempty"` // Path -> New Content
	NextState    PipelineState       `json:"next_state,omitempty"`     // Suggested state transition
	Data         map[string]any      `json:"data,omitempty"`           // Arbitrary structured data
	Error        error               `json:"-"`                        // Internal error
}

// Message represents a single turn in the conversation history.
type Message struct {
	Role    string `json:"role"`    // user, assistant, system
	Content string `json:"content"`
}

// ExecutionStatus indicates the outcome of an agent's internal logic.
type ExecutionStatus string

const (
	StatusSuccess ExecutionStatus = "success"
	StatusFailure ExecutionStatus = "failure"
	StatusWaiting ExecutionStatus = "waiting" // Awaiting user confirmation
)

// Agent defines the contract that all pipeline agents must implement.
type Agent interface {
	ID() string
	Role() Role
	Description() string
	Execute(ctx context.Context, input Input) (*Output, error)
}

// CustomAgent is a dynamically loaded agent from YAML configuration.
type CustomAgent struct {
	IDVal   string
	RoleVal Role
	DescVal string
	ExecFunc func(ctx context.Context, input Input) (*Output, error)
}

func (a *CustomAgent) ID() string { return a.IDVal }
func (a *CustomAgent) Role() Role { return a.RoleVal }
func (a *CustomAgent) Description() string { return a.DescVal }
func (a *CustomAgent) Execute(ctx context.Context, input Input) (*Output, error) {
	if a.ExecFunc != nil {
		return a.ExecFunc(ctx, input)
	}
	return &Output{Status: StatusFailure, Error: fmt.Errorf("not implemented")}, nil
}
