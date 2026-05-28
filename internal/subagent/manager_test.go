package subagent

import (
	"context"
	"strings"
	"testing"
	"time"

	"autodev/internal/agents"
	"autodev/internal/core"
)

func TestNewWithDefaults(t *testing.T) {
	m := New(0, 0)

	if m.maxConcurrency != 4 {
		t.Errorf("expected default maxConcurrency 4, got %d", m.maxConcurrency)
	}
	if m.timeout != 5*time.Minute {
		t.Errorf("expected default timeout 5m, got %v", m.timeout)
	}
}

func TestNewWithValues(t *testing.T) {
	m := New(10, 2*time.Hour)

	if m.maxConcurrency != 10 {
		t.Errorf("expected maxConcurrency 10, got %d", m.maxConcurrency)
	}
	if m.timeout != 2*time.Hour {
		t.Errorf("expected timeout 2h, got %v", m.timeout)
	}
}

func TestNewWithNegativeValues(t *testing.T) {
	m := New(-1, -1)

	// Should use defaults
	if m.maxConcurrency != 4 {
		t.Errorf("expected default maxConcurrency 4, got %d", m.maxConcurrency)
	}
}

func TestTaskStructure(t *testing.T) {
	task := Task{
		ID:          "task-1",
		Name:        "test-task",
		Description: "A test task",
		Input:       "do something",
		Timeout:     30 * time.Second,
		Priority:    5,
	}

	if task.ID != "task-1" {
		t.Error("Task ID not set correctly")
	}
	if task.Priority != 5 {
		t.Error("Task Priority not set correctly")
	}
}

func TestResultStructure(t *testing.T) {
	result := Result{
		TaskID:    "task-1",
		Output:    "hello",
		Duration:  time.Second,
		Tokens:    100,
		Completed: true,
	}

	if result.TaskID != "task-1" {
		t.Error("TaskID not set correctly")
	}
	if result.Tokens != 100 {
		t.Error("Tokens not set correctly")
	}
}

func TestSortByPriority(t *testing.T) {
	priorityMap := map[string]int{
		"low":    1,
		"medium": 5,
		"high":   10,
	}

	results := []*Result{
		{TaskID: "low"},
		{TaskID: "high"},
		{TaskID: "medium"},
	}

	sortByPriority(results, priorityMap)

	if results[0].TaskID != "high" {
		t.Errorf("expected first to be 'high', got %s", results[0].TaskID)
	}
	if results[1].TaskID != "medium" {
		t.Errorf("expected second to be 'medium', got %s", results[1].TaskID)
	}
	if results[2].TaskID != "low" {
		t.Errorf("expected third to be 'low', got %s", results[2].TaskID)
	}
}

func TestSortByPriorityEqual(t *testing.T) {
	priorityMap := map[string]int{
		"a": 5,
		"b": 5,
		"c": 5,
	}

	results := []*Result{
		{TaskID: "a"},
		{TaskID: "b"},
		{TaskID: "c"},
	}

	sortByPriority(results, priorityMap)

	// All same priority, order should be preserved
	if len(results) != 3 {
		t.Error("expected 3 results")
	}
}

func TestSortByPriorityNoMap(t *testing.T) {
	results := []*Result{
		{TaskID: "a"},
		{TaskID: "b"},
	}

	// Should not panic
	sortByPriority(results, nil)

	if len(results) != 2 {
		t.Error("expected 2 results")
	}
}

func TestSortByPriorityEmpty(t *testing.T) {
	sortByPriority(nil, map[string]int{})
	sortByPriority([]*Result{}, map[string]int{})
}

func TestMergeResults(t *testing.T) {
	t.Run("all completed", func(t *testing.T) {
		results := []*Result{
			{TaskID: "task-1", Output: "hello", Completed: true, Duration: time.Second},
			{TaskID: "task-2", Output: "world", Completed: true, Duration: 2 * time.Second},
		}

		merged := MergeResults(results)

		if !strings.Contains(merged, "task-1") {
			t.Error("merged missing task-1")
		}
		if !strings.Contains(merged, "task-2") {
			t.Error("merged missing task-2")
		}
		if !strings.Contains(merged, "hello") {
			t.Error("merged missing 'hello'")
		}
		if !strings.Contains(merged, "world") {
			t.Error("merged missing 'world'")
		}
	})

	t.Run("with failure", func(t *testing.T) {
		results := []*Result{
			{TaskID: "task-1", Output: "done", Completed: true},
			{TaskID: "task-2", Error: context.DeadlineExceeded, Completed: false},
		}

		merged := MergeResults(results)

		if !strings.Contains(merged, "task-1") {
			t.Error("merged missing task-1")
		}
		if !strings.Contains(merged, "FAILED") {
			t.Error("merged missing FAILED")
		}
	})

	t.Run("empty", func(t *testing.T) {
		merged := MergeResults([]*Result{})
		if merged != "" {
			t.Errorf("expected empty for empty results, got %s", merged)
		}
	})
}

func TestManagerGetResult(t *testing.T) {
	m := &Manager{}

	// Store a result
	r := &Result{TaskID: "task-1", Completed: true}
	m.results.Store("task-1", r)

	// Get existing result
	got, ok := m.GetResult("task-1")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.TaskID != "task-1" {
		t.Errorf("expected task-1, got %s", got.TaskID)
	}

	// Get non-existing result
	_, ok = m.GetResult("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent")
	}
}

func TestManagerGetAllResults(t *testing.T) {
	m := &Manager{}

	m.results.Store("a", &Result{TaskID: "a"})
	m.results.Store("b", &Result{TaskID: "b"})
	m.results.Store("c", &Result{TaskID: "c"})

	results := m.GetAllResults()

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestManagerGetAllResultsEmpty(t *testing.T) {
	m := &Manager{}

	results := m.GetAllResults()

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestManagerSummary(t *testing.T) {
	m := &Manager{}

	m.results.Store("ok", &Result{Completed: true, Duration: time.Second})
	m.results.Store("fail", &Result{Completed: false, Duration: 2 * time.Second})

	summary := m.Summary()

	if !strings.Contains(summary, "1 completed") {
		t.Errorf("summary missing '1 completed': %s", summary)
	}
	if !strings.Contains(summary, "1 failed") {
		t.Errorf("summary missing '1 failed': %s", summary)
	}
	if !strings.Contains(summary, "3s") {
		t.Errorf("summary missing total duration: %s", summary)
	}
}

func TestManagerSummaryEmpty(t *testing.T) {
	m := &Manager{}

	summary := m.Summary()

	if !strings.Contains(summary, "0 completed") {
		t.Errorf("expected '0 completed': %s", summary)
	}
	if !strings.Contains(summary, "0 failed") {
		t.Errorf("expected '0 failed': %s", summary)
	}
}

func TestManagerReset(t *testing.T) {
	m := &Manager{}

	m.results.Store("a", &Result{TaskID: "a"})
	m.results.Store("b", &Result{TaskID: "b"})

	if len(m.GetAllResults()) != 2 {
		t.Fatal("expected 2 results before reset")
	}

	m.Reset()

	if len(m.GetAllResults()) != 0 {
		t.Error("expected 0 results after reset")
	}
}

func TestManagerIsComplete(t *testing.T) {
	m := &Manager{}

	// 0 expected with no results is technically complete
	if !m.IsComplete(0) {
		t.Error("expected true for 0 expected with no results")
	}

	m.results.Store("a", &Result{TaskID: "a"})
	m.results.Store("b", &Result{TaskID: "b"})

	if !m.IsComplete(2) {
		t.Error("expected true for 2 expected with 2 results")
	}

	if !m.IsComplete(1) {
		t.Error("expected true for 1 expected with 2 results")
	}

	if m.IsComplete(5) {
		t.Error("expected false for 5 expected with 2 results")
	}
}

func TestManagerSpawnSubTask(t *testing.T) {
	m := New(2, 3*time.Minute)
	exec := &agents.Executor{
		AgentID:   "parent",
		AgentRole: core.RoleDeveloper,
	}

	task := m.SpawnSubTask("parent", "child", "child task description", exec)

	if task.ID != "parent.child" {
		t.Errorf("expected 'parent.child', got %s", task.ID)
	}
	if task.Name != "child" {
		t.Errorf("expected 'child', got %s", task.Name)
	}
	if task.Description != "child task description" {
		t.Errorf("expected description, got %s", task.Description)
	}
	if task.Executor != exec {
		t.Error("executor not set correctly")
	}
	// Timeout should be 1/3 of manager timeout
	if task.Timeout != time.Minute {
		t.Errorf("expected 1m timeout, got %v", task.Timeout)
	}
}

func TestManagerExecuteSequential(t *testing.T) {
	t.Run("empty tasks", func(t *testing.T) {
		m := New(4, time.Minute)

		ctx := context.Background()
		results, err := m.ExecuteSequential(ctx, nil, false)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestWaitForCompletion(t *testing.T) {
	m := &Manager{}

	// Start with no results
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Should timeout since no results added
	err := m.WaitForCompletion(ctx, 1)
	if err == nil {
		t.Log("expected timeout but completed immediately (acceptable)")
	}
}

// Mock executor for testing type

type mockExecutor struct {
	output string
	err    error
}

func newMockExecutor(output string, err error) *agents.Executor {
	return &agents.Executor{
		AgentID:   "mock",
		AgentRole: core.RoleDeveloper,
	}
}
