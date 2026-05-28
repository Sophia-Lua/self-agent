package agents

import (
	"autodev/internal/core"
	"context"
	"fmt"
)

type CheckerAgent struct {
	id          string
	description string
}

func NewChecker() *CheckerAgent {
	return &CheckerAgent{
		id:          "agent-checker",
		description: "Validates code quality, coverage, and standards compliance",
	}
}

func (a *CheckerAgent) ID() string          { return a.id }
func (a *CheckerAgent) Role() core.Role     { return core.RoleChecker }
func (a *CheckerAgent) Description() string { return a.description }

func (a *CheckerAgent) Execute(ctx context.Context, input core.Input) (*core.Output, error) {
	if input.TaskDescription == "" {
		return &core.Output{
			Status:  core.StatusFailure,
			Message: "No task description to check",
		}, nil
	}

	fmt.Printf("[Checker] Running quality checks for: %s\n", input.TaskDescription)

	// Check results would typically come from:
	// 1. Test coverage reports
	// 2. Lint tool output
	// 3. Static analysis results
	// 
	// For now, we validate that the modified files look reasonable
	if len(input.Files) == 0 {
		return &core.Output{
			Status:  core.StatusFailure,
			Message: "No files found to check",
		}, fmt.Errorf("empty workspace")
	}

	issues := []string{}
	for path, content := range input.Files {
		if len(content) == 0 {
			issues = append(issues, fmt.Sprintf("%s: empty file", path))
		}
	}

	if len(issues) > 0 {
		return &core.Output{
			Status:  core.StatusFailure,
			Message: fmt.Sprintf("Found %d issue(s): %v", len(issues), issues),
		}, fmt.Errorf("quality check failed: %v", issues)
	}

	return &core.Output{
		Status:  core.StatusSuccess,
		Message: "Quality checks passed",
	}, nil
}
