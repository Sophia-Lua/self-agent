package parser

import (
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unicode"
)

// SubTask represents a single decomposed subtask.
type SubTask struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        TaskType `json:"type"`
	Priority    int      `json:"priority"`
	Files       []string `json:"files,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Agent       string   `json:"agent"`
	Status      TaskStatus `json:"status"`
}

// TaskType classifies the nature of the subtask.
type TaskType string

const (
	TypeImplementation TaskType = "implementation"
	TypeFeature        TaskType = "feature"
	TypeFix            TaskType = "fix"
	TypeRefactor       TaskType = "refactor"
	TypeTest           TaskType = "test"
	TypeConfig         TaskType = "config"
	TypeDoc            TaskType = "documentation"
)

// TaskStatus tracks the execution state of a subtask.
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskCompleted  TaskStatus = "completed"
	TaskFailed     TaskStatus = "failed"
	TaskSkipped    TaskStatus = "skipped"
)

// DecomposedTask represents a user task broken into subtasks.
type DecomposedTask struct {
	ID          string     `json:"id"`
	OriginalTask string   `json:"original_task"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	SubTasks    []SubTask `json:"subtasks"`
	CreatedAt   time.Time `json:"created_at"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Decompose breaks down a user task into structured subtasks.
func Decompose(taskDesc string) *DecomposedTask {
	dt := &DecomposedTask{
		ID:           generateTaskID(),
		OriginalTask: taskDesc,
		Title:        extractTitle(taskDesc),
		Description:  taskDesc,
		CreatedAt:    time.Now(),
		Metadata:     make(map[string]interface{}),
	}

	// Detect keywords and generate subtasks
	dt.SubTasks = generateSubTasks(taskDesc)

	return dt
}

// generateSubTasks analyzes the task description and creates appropriate subtasks.
func generateSubTasks(taskDesc string) []SubTask {
	var subTasks []SubTask
	taskLower := strings.ToLower(taskDesc)

	// Rule-based decomposition based on keywords
	if containsAny(taskLower, []string{"add", "new", "create", "implement"}) {
		subTasks = append(subTasks, analyzeSubTask(taskDesc, TypeFeature)...)
	}

	if containsAny(taskLower, []string{"fix", "bug", "error", "issue", "broken", "not working"}) {
		subTasks = append(subTasks, analyzeSubTask(taskDesc, TypeFix)...)
	}

	if containsAny(taskLower, []string{"refactor", "restructure", "optimize", "improve", "clean"}) {
		subTasks = append(subTasks, analyzeSubTask(taskDesc, TypeRefactor)...)
	}

	if containsAny(taskLower, []string{"test", "unit test", "integration test", "e2e"}) {
		subTasks = append(subTasks, analyzeSubTask(taskDesc, TypeTest)...)
	}

	if containsAny(taskLower, []string{"config", "setup", "initialize", "configure"}) {
		subTasks = append(subTasks, analyzeSubTask(taskDesc, TypeConfig)...)
	}

	// If no subtasks generated, create a single implementation task
	if len(subTasks) == 0 {
		subTasks = append(subTasks, SubTask{
			ID:          "task-001",
			Title:       "Implement requested functionality",
			Description: taskDesc,
			Type:        TypeImplementation,
			Priority:    1,
			Agent:       "agent-developer",
			Status:      TaskPending,
		})
	}

	// Add dependency chain
	for i := 1; i < len(subTasks); i++ {
		subTasks[i].DependsOn = append(subTasks[i].DependsOn, subTasks[i-1].ID)
	}

	return subTasks
}

// analyzeSubTask creates a subtask based on the task type.
func analyzeSubTask(taskDesc string, taskType TaskType) []SubTask {
	var tasks []SubTask

	switch taskType {
	case TypeFeature:
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("feat"),
			Title:       "Analyze requirements",
			Description: "Understand the feature requirements and identify dependencies",
			Type:        TypeFeature,
			Priority:    1,
			Agent:       "agent-parser",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("feat"),
			Title:       "Implement feature",
			Description: fmt.Sprintf("Build and implement the requested feature: %s", taskDesc),
			Type:        TypeFeature,
			Priority:    2,
			Agent:       "agent-developer",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("test"),
			Title:       "Add tests for feature",
			Description: "Write unit tests to validate the new feature works correctly",
			Type:        TypeTest,
			Priority:    3,
			Agent:       "agent-tester",
			Status:      TaskPending,
		})

	case TypeFix:
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("fix"),
			Title:       "Identify root cause",
			Description: "Analyze the reported issue and find the root cause",
			Type:        TypeFix,
			Priority:    1,
			Agent:       "agent-parser",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("fix"),
			Title:       "Fix the issue",
			Description: fmt.Sprintf("Apply fix for: %s", taskDesc),
			Type:        TypeFix,
			Priority:    2,
			Agent:       "agent-developer",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("test"),
			Title:       "Verify fix with tests",
			Description: "Create test cases to prevent regression",
			Type:        TypeTest,
			Priority:    3,
			Agent:       "agent-tester",
			Status:      TaskPending,
		})

	case TypeRefactor:
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("refactor"),
			Title:       "Analyze code structure",
			Description: "Review current implementation and identify refactoring opportunities",
			Type:        TypeRefactor,
			Priority:    1,
			Agent:       "agent-checker",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("refactor"),
			Title:       "Apply refactoring",
			Description: fmt.Sprintf("Refactor code: %s", taskDesc),
			Type:        TypeRefactor,
			Priority:    2,
			Agent:       "agent-developer",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("test"),
			Title:       "Run validation tests",
			Description: "Ensure refactored code passes all existing tests",
			Type:        TypeTest,
			Priority:    3,
			Agent:       "agent-tester",
			Status:      TaskPending,
		})

	case TypeTest:
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("test"),
			Title:       "Create test cases",
			Description: fmt.Sprintf("Write tests for: %s", taskDesc),
			Type:        TypeTest,
			Priority:    1,
			Agent:       "agent-tester",
			Status:      TaskPending,
		})
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("test"),
			Title:       "Run test suite",
			Description: "Execute tests and verify coverage",
			Type:        TypeTest,
			Priority:    2,
			Agent:       "agent-tester",
			Status:      TaskPending,
		})

	case TypeConfig:
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("config"),
			Title:       "Configure settings",
			Description: fmt.Sprintf("Setup configuration: %s", taskDesc),
			Type:        TypeConfig,
			Priority:    1,
			Agent:       "agent-developer",
			Status:      TaskPending,
		})

	default:
		tasks = append(tasks, SubTask{
			ID:          generateSubTaskID("impl"),
			Title:       "Implement task",
			Description: taskDesc,
			Type:        TypeImplementation,
			Priority:    1,
			Agent:       "agent-developer",
			Status:      TaskPending,
		})
	}

	return tasks
}

// containsAny checks if the string contains any of the keywords.
func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// extractTitle extracts a short title from the task description.
func extractTitle(taskDesc string) string {
	parts := strings.SplitN(taskDesc, " ", 4)
	if len(parts) > 0 {
		runes := []rune(parts[0])
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
		}
		return string(runes) + " " + strings.Join(parts[1:], " ")
	}
	return taskDesc
}

// generateTaskID generates a unique task ID.
func generateTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

// generateSubTaskID generates a unique subtask ID with prefix.
var subTaskCounter atomic.Int64

func generateSubTaskID(prefix string) string {
	n := subTaskCounter.Add(1)
	return fmt.Sprintf("%s-%03d", prefix, n)
}

// ValidateTask checks if a decomposed task is valid.
func ValidateTask(dt *DecomposedTask) error {
	if dt == nil {
		return fmt.Errorf("nil task")
	}
	if dt.Title == "" {
		return fmt.Errorf("task title is required")
	}
	if len(dt.SubTasks) == 0 {
		return fmt.Errorf("task must have at least one subtask")
	}

	// Validate subtask dependencies
	seen := make(map[string]bool)
	for _, st := range dt.SubTasks {
		seen[st.ID] = true
	}

	for _, st := range dt.SubTasks {
		for _, dep := range st.DependsOn {
			if !seen[dep] {
				return fmt.Errorf("subtask %s depends on missing subtask %s", st.ID, dep)
			}
		}
	}

	return nil
}

// MergeSubTasks merges multiple decomposed tasks into one.
func MergeSubTasks(tasks ...*DecomposedTask) *DecomposedTask {
	if len(tasks) == 0 {
		return nil
	}

	merged := &DecomposedTask{
		ID:          generateTaskID(),
		Title:       tasks[0].Title,
		Description: tasks[0].Description,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	for _, t := range tasks {
		merged.SubTasks = append(merged.SubTasks, t.SubTasks...)
		if t.OriginalTask != "" {
			merged.OriginalTask += "; " + t.OriginalTask
		}
	}

	// Rebuild dependency chain
	for i := 1; i < len(merged.SubTasks); i++ {
		merged.SubTasks[i].DependsOn = append(merged.SubTasks[i].DependsOn, merged.SubTasks[i-1].ID)
	}

	return merged
}

// SummarizeTaskPlan creates a human-readable summary of the decomposed task.
func SummarizeTaskPlan(dt *DecomposedTask) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Task: %s\n", dt.Title))
	b.WriteString(fmt.Sprintf("Subtasks (%d):\n", len(dt.SubTasks)))

	for _, st := range dt.SubTasks {
		deps := ""
		if len(st.DependsOn) > 0 {
			deps = fmt.Sprintf(" (depends on: %s)", strings.Join(st.DependsOn, ", "))
		}
		b.WriteString(fmt.Sprintf("  [%s] %s - %s%s\n", st.ID, strings.ToUpper(string(st.Type)), st.Title, deps))
	}

	return b.String()
}

// ExtractFilesFromTask attempts to identify files referenced in the task description.
func ExtractFilesFromTask(taskDesc string) []string {
	// Match common file path patterns
	filePatterns := []string{
		`[a-zA-Z0-9_\-/.]+\.go`,
		`[a-zA-Z0-9_\-/.]+\.py`,
		`[a-zA-Z0-9_\-/.]+\.js`,
		`[a-zA-Z0-9_\-/.]+\.ts`,
		`[a-zA-Z0-9_\-/.]+\.yaml`,
		`[a-zA-Z0-9_\-/.]+\.yml`,
		`[a-zA-Z0-9_\-/.]+\.json`,
		`[a-zA-Z0-9_\-/.]+\.md`,
		`[a-zA-Z0-9_\-/.]+\.toml`,
		`[a-zA-Z0-9_\-/.]+\.cfg`,
		`[a-zA-Z0-9_\-/.]+\.ini`,
	}

	var files []string
	seen := make(map[string]bool)

	for _, pattern := range filePatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(taskDesc, -1)
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				files = append(files, m)
			}
		}
	}

	return files
}
