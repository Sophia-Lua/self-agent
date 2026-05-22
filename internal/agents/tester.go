package agents

import (
	"context"
	"fmt"
	"autodev/internal/core"
)

// TesterAgent checks correctness of the generated code.
type TesterAgent struct {
	id          string
	description string
}

// NewTester creates a new instance of TesterAgent.
func NewTester() *TesterAgent {
	return &TesterAgent{
		id:          "agent-tester",
		description: "Validates code correctness",
	}
}

func (a *TesterAgent) ID() string          { return a.id }
func (a *TesterAgent) Role() core.Role     { return core.RoleTester }
func (a *TesterAgent) Description() string { return a.description }

// Execute the testing logic.
func (a *TesterAgent) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	fmt.Println("[Tester] Validating output...")
	// In reality, this runs unit tests or static analysis
	return &core.Output{
		Status:  core.StatusSuccess,
		Message: "Validation passed",
	}, nil
}
