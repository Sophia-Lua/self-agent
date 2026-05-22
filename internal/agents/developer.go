package agents

import (
	"context"
	"fmt"
	"autodev/internal/core"
)

// DeveloperAgent is a placeholder for the AI coding logic.
type DeveloperAgent struct {
	id          string
	description string
}

// NewDeveloper creates a new instance of DeveloperAgent.
func NewDeveloper() *DeveloperAgent {
	return &DeveloperAgent{
		id:          "agent-developer",
		description: "Generates code based on structured tasks",
	}
}

func (a *DeveloperAgent) ID() string          { return a.id }
func (a *DeveloperAgent) Role() core.Role     { return core.RoleDeveloper }
func (a *DeveloperAgent) Description() string { return a.description }

// Execute the development logic.
func (a *DeveloperAgent) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	fmt.Println("[Developer] Generating code...")
	// In reality, this calls the LLM with the context and history
	return &core.Output{
		Status:  core.StatusSuccess,
		Message: "Code generated",
	}, nil
}
