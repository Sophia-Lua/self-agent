package agents

import (
	"context"
	"fmt"
	"autodev/internal/core"
)

// ParserAgent converts natural language inputs into structured tasks.
type ParserAgent struct {
	id          string
	description string
}

// NewParser creates a new instance of ParserAgent.
func NewParser() *ParserAgent {
	return &ParserAgent{
		id:          "agent-parser",
		description: "Parses user intent into structured development tasks",
	}
}

func (a *ParserAgent) ID() string          { return a.id }
func (a *ParserAgent) Role() core.Role     { return core.RoleParser }
func (a *ParserAgent) Description() string { return a.description }

// Execute the parsing logic.
func (a *ParserAgent) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	if input.TaskDescription == "" {
		return &core.Output{Status: core.StatusFailure, Error: fmt.Errorf("empty task description")}, nil
	}

	fmt.Printf("[Parser] Analyzing task: %s\n", input.TaskDescription)

	taskContext := fmt.Sprintf("Task: %s\nFiles: %s\n", input.TaskDescription, input.Files)

	return &core.Output{
		Status:  core.StatusSuccess,
		Message: "Task parsed successfully",
		Data:    map[string]any{"context": taskContext},
	}, nil
}
