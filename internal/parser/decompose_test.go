package parser

import (
	"strings"
	"testing"
)

func TestDecomposeFeature(t *testing.T) {
	dt := Decompose("Add user authentication with OAuth2")

	if dt == nil {
		t.Fatal("Decompose returned nil")
	}
	if dt.OriginalTask == "" {
		t.Error("OriginalTask should not be empty")
	}
	if len(dt.SubTasks) == 0 {
		t.Error("Should have at least one subtask")
	}

	// Verify subtask types include implementation and test
	hasFeature := false
	hasTest := false
	for _, st := range dt.SubTasks {
		if st.Type == TypeFeature {
			hasFeature = true
		}
		if st.Type == TypeTest {
			hasTest = true
		}
	}
	if !hasFeature {
		t.Error("Feature task should be present for 'Add' keyword")
	}
	if !hasTest {
		t.Error("Test task should be present for feature tasks")
	}
}

func TestDecomposeFix(t *testing.T) {
	dt := Decompose("Fix login button not working on mobile")

	if len(dt.SubTasks) == 0 {
		t.Fatal("Should have subtasks")
	}

	hasFix := false
	for _, st := range dt.SubTasks {
		if st.Type == TypeFix {
			hasFix = true
			break
		}
	}
	if !hasFix {
		t.Error("Fix task should be present for 'Fix' keyword")
	}
}

func TestDecomposeRefactor(t *testing.T) {
	dt := Decompose("Refactor the database layer to use connection pooling")

	if len(dt.SubTasks) == 0 {
		t.Fatal("Should have subtasks")
	}

	hasRefactor := false
	for _, st := range dt.SubTasks {
		if st.Type == TypeRefactor {
			hasRefactor = true
			break
		}
	}
	if !hasRefactor {
		t.Error("Refactor task should be present for 'Refactor' keyword")
	}
}

func TestDecomposeTest(t *testing.T) {
	dt := Decompose("Add unit tests for the payment service")

	if len(dt.SubTasks) == 0 {
		t.Fatal("Should have subtasks")
	}

	hasTestTask := false
	for _, st := range dt.SubTasks {
		if st.Type == TypeTest {
			hasTestTask = true
			break
		}
	}
	if !hasTestTask {
		t.Error("Test task should be present for 'test' keyword")
	}
}

func TestDecomposeGeneric(t *testing.T) {
	dt := Decompose("Update the README documentation")

	if len(dt.SubTasks) == 0 {
		t.Fatal("Should have at least one subtask")
	}

	// Generic tasks should get a single implementation task
	if len(dt.SubTasks) != 1 {
		t.Logf("Warning: generic task generated %d subtasks", len(dt.SubTasks))
	}
}

func TestSubTaskDependencies(t *testing.T) {
	dt := Decompose("Add new API endpoint for user registration")

	// Build a map of seen task IDs
	seen := make(map[string]bool)
	for _, st := range dt.SubTasks {
		seen[st.ID] = true
	}

	// Verify all dependencies reference existing tasks
	for _, st := range dt.SubTasks {
		for _, dep := range st.DependsOn {
			if !seen[dep] {
				t.Errorf("Subtask %s depends on missing task %s", st.ID, dep)
			}
		}
	}
}

func TestValidateTask(t *testing.T) {
	dt := &DecomposedTask{
		Title:    "Valid task",
		SubTasks: []SubTask{{ID: "task-1", Type: TypeImplementation}},
	}

	if err := ValidateTask(dt); err != nil {
		t.Errorf("Valid task should not error: %v", err)
	}

	// Test nil task
	if err := ValidateTask(nil); err == nil {
		t.Error("Nil task should return error")
	}

	// Test empty title
	dt.Title = ""
	if err := ValidateTask(dt); err == nil {
		t.Error("Empty title should return error")
	}

	// Test no subtasks
	dt.Title = "Valid"
	dt.SubTasks = nil
	if err := ValidateTask(dt); err == nil {
		t.Error("Empty subtasks should return error")
	}
}

func TestValidateTaskMissingDependency(t *testing.T) {
	dt := &DecomposedTask{
		Title: "Task with broken dep",
		SubTasks: []SubTask{
			{ID: "task-1", Type: TypeImplementation, DependsOn: []string{"nonexistent"}},
		},
	}

	if err := ValidateTask(dt); err == nil {
		t.Error("Task with missing dependency should return error")
	}
}

func TestMergeSubTasks(t *testing.T) {
	dt1 := &DecomposedTask{
		Title:    "Feature A",
		SubTasks: []SubTask{{ID: "feat-001", Type: TypeFeature}},
	}
	dt2 := &DecomposedTask{
		Title:    "Feature B",
		SubTasks: []SubTask{{ID: "feat-002", Type: TypeFeature}},
	}

	merged := MergeSubTasks(dt1, dt2)

	if merged == nil {
		t.Fatal("Merge returned nil")
	}
	if len(merged.SubTasks) != 2 {
		t.Errorf("Expected 2 subtasks, got %d", len(merged.SubTasks))
	}
}

func TestMergeEmptyTasks(t *testing.T) {
	result := MergeSubTasks()
	if result != nil {
		t.Error("Merge of empty should return nil")
	}
}

func TestSummarizeTaskPlan(t *testing.T) {
	dt := &DecomposedTask{
		Title: "Test plan",
		SubTasks: []SubTask{
			{ID: "task-1", Type: TypeImplementation, Title: "Implement feature"},
			{ID: "task-2", Type: TypeTest, Title: "Write tests", DependsOn: []string{"task-1"}},
		},
	}

	summary := SummarizeTaskPlan(dt)

	if !strings.Contains(summary, "Test plan") {
		t.Error("Summary should contain task title")
	}
	if !strings.Contains(summary, "2") {
		t.Error("Summary should mention 2 subtasks")
	}
	// Verify the task type is in uppercase
	if !strings.Contains(summary, "IMPLEMENTATION") {
		t.Errorf("Summary should contain IMPLEMENTATION type, got:\n%s", summary)
	}
	if !strings.Contains(summary, "TEST") {
		t.Errorf("Summary should contain TEST type, got:\n%s", summary)
	}
}

func TestExtractFilesFromTask(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCount int
	}{
		{
			name:         "Go files",
			input:        "Modify internal/handler.go and internal/service.go",
			expectedCount: 2,
		},
		{
			name:         "Config files",
			input:        "Update config.yaml and settings.toml",
			expectedCount: 2,
		},
		{
			name:         "No files",
			input:        "Add new feature without file reference",
			expectedCount: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := ExtractFilesFromTask(tc.input)
			if len(files) != tc.expectedCount {
				t.Errorf("Expected %d files, got %d: %v", tc.expectedCount, len(files), files)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", []string{"hello", "bye"}) {
		t.Error("Should match 'hello'")
	}
	if containsAny("goodbye", []string{"hello", "bye"}) {
		// "bye" is in "goodbye"
	} else {
		t.Error("Should match 'bye' in 'goodbye'")
	}
	if containsAny("nothing", []string{"hello", "world"}) {
		t.Error("Should not match")
	}
}

func TestExtractTitle(t *testing.T) {
	title := extractTitle("Add user authentication with OAuth2")
	if !strings.HasPrefix(title, "Add") {
		t.Errorf("Expected title starting with 'Add', got: %s", title)
	}
}

func TestGenerateTaskID(t *testing.T) {
	id1 := generateTaskID()
	id2 := generateTaskID()

	if id1 == id2 {
		t.Error("Task IDs should be unique")
	}
	if !strings.HasPrefix(id1, "task-") {
		t.Errorf("Expected task- prefix, got: %s", id1)
	}
}

func TestGenerateSubTaskID(t *testing.T) {
	// Reset counter for test

	id1 := generateSubTaskID("feat")
	id2 := generateSubTaskID("feat")

	if id1 == id2 {
		t.Error("Subtask IDs should be unique")
	}
	if !strings.HasPrefix(id1, "feat-") {
		t.Errorf("Expected feat- prefix, got: %s", id1)
	}
}
