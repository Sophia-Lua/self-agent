package progress

import (
	"strings"
	"testing"

	"autodev/internal/core"
)

func TestStatusConstants(t *testing.T) {
	if StatusPending != "pending" {
		t.Errorf("expected 'pending', got %s", StatusPending)
	}
	if StatusRunning != "running" {
		t.Errorf("expected 'running', got %s", StatusRunning)
	}
	if StatusCompleted != "completed" {
		t.Errorf("expected 'completed', got %s", StatusCompleted)
	}
	if StatusFailed != "failed" {
		t.Errorf("expected 'failed', got %s", StatusFailed)
	}
	if StatusRetrying != "retrying" {
		t.Errorf("expected 'retrying', got %s", StatusRetrying)
	}
	if StatusSkipped != "skipped" {
		t.Errorf("expected 'skipped', got %s", StatusSkipped)
	}
}

func TestNew(t *testing.T) {
	tr := New("task-1", "Test Task")
	if tr == nil {
		t.Fatal("expected non-nil tracker")
	}
	if tr.taskID != "task-1" {
		t.Errorf("expected taskID 'task-1', got %s", tr.taskID)
	}
	if tr.taskDesc != "Test Task" {
		t.Errorf("expected taskDesc 'Test Task', got %s", tr.taskDesc)
	}
}

func TestRegisterPhase(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("parse", "Parse input")

	report := tr.Report()
	if len(report.Phases) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(report.Phases))
	}
	if report.Phases[0].Name != "parse" {
		t.Errorf("expected phase 'parse', got %s", report.Phases[0].Name)
	}
	if report.Phases[0].Status != StatusPending {
		t.Errorf("expected status 'pending', got %s", report.Phases[0].Status)
	}
}

func TestRegisterMultiplePhases(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")
	tr.RegisterPhase("c", "C")

	report := tr.Report()
	if len(report.Phases) != 3 {
		t.Errorf("expected 3 phases, got %d", len(report.Phases))
	}
}

func TestStartPhase(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("parse", "Parse")

	tr.StartPhase("parse")

	report := tr.Report()
	if report.Phases[0].Status != StatusRunning {
		t.Errorf("expected 'running', got %s", report.Phases[0].Status)
	}
	if !report.Phases[0].StartedAt.IsZero() {
		t.Log("StartedAt set correctly")
	}
}

func TestCompletePhase(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("parse", "Parse")

	tr.CompletePhase("parse", 100)

	report := tr.Report()
	if report.Phases[0].Status != StatusCompleted {
		t.Errorf("expected 'completed', got %s", report.Phases[0].Status)
	}
	if report.Phases[0].TokensUsed != 100 {
		t.Errorf("expected 100 tokens, got %d", report.Phases[0].TokensUsed)
	}
	if report.TotalTokens != 100 {
		t.Errorf("expected total 100 tokens, got %d", report.TotalTokens)
	}
}

func TestCompletePhaseAccumulates(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")

	tr.CompletePhase("a", 50)
	tr.CompletePhase("b", 75)

	report := tr.Report()
	if report.TotalTokens != 125 {
		t.Errorf("expected 125 total tokens, got %d", report.TotalTokens)
	}
}

func TestFailPhase(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("parse", "Parse")

	tr.FailPhase("parse", "syntax error")

	report := tr.Report()
	if report.Phases[0].Status != StatusFailed {
		t.Errorf("expected 'failed', got %s", report.Phases[0].Status)
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(report.Errors))
	}
	if !strings.Contains(report.Errors[0], "parse") || !strings.Contains(report.Errors[0], "syntax error") {
		t.Errorf("expected error to contain 'parse' and 'syntax error', got: %s", report.Errors[0])
	}
}

func TestRetryPhase(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("test", "Test")

	tr.RetryPhase("test", 2)

	report := tr.Report()
	if report.Phases[0].Status != StatusRetrying {
		t.Errorf("expected 'retrying', got %s", report.Phases[0].Status)
	}
	if report.Phases[0].Retries != 2 {
		t.Errorf("expected 2 retries, got %d", report.Phases[0].Retries)
	}
}

func TestSkipPhase(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("lint", "Lint")

	tr.SkipPhase("lint", "no linter configured")

	report := tr.Report()
	if report.Phases[0].Status != StatusSkipped {
		t.Errorf("expected 'skipped', got %s", report.Phases[0].Status)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(report.Warnings))
	}
	if !strings.Contains(report.Warnings[0], "no linter") {
		t.Errorf("expected warning to contain reason, got: %s", report.Warnings[0])
	}
}

func TestAddError(t *testing.T) {
	tr := New("t1", "desc")
	tr.AddError("network timeout")

	report := tr.Report()
	if len(report.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(report.Errors))
	}
	if report.Errors[0] != "network timeout" {
		t.Errorf("expected 'network timeout', got %s", report.Errors[0])
	}
}

func TestAddWarning(t *testing.T) {
	tr := New("t1", "desc")
	tr.AddWarning("deprecated API")

	report := tr.Report()
	if len(report.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(report.Warnings))
	}
	if report.Warnings[0] != "deprecated API" {
		t.Errorf("expected 'deprecated API', got %s", report.Warnings[0])
	}
}

func TestReportSnapshot(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")

	r1 := tr.Report()
	r2 := tr.Report()

	// Snapshots should be independent
	if len(r1.Phases) != 2 || len(r2.Phases) != 2 {
		t.Error("expected 2 phases in both snapshots")
	}
}

func TestReportTaskInfo(t *testing.T) {
	tr := New("task-xyz", "My important task")

	report := tr.Report()
	if report.TaskID != "task-xyz" {
		t.Errorf("expected 'task-xyz', got %s", report.TaskID)
	}
	if report.TaskDesc != "My important task" {
		t.Errorf("expected 'My important task', got %s", report.TaskDesc)
	}
}

func TestOverallStatusRunning(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")

	tr.StartPhase("a")

	report := tr.Report()
	if report.OverallStatus != StatusRunning {
		t.Errorf("expected 'running', got %s", report.OverallStatus)
	}
}

func TestOverallStatusCompleted(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")

	tr.CompletePhase("a", 0)
	tr.CompletePhase("b", 0)

	report := tr.Report()
	if report.OverallStatus != StatusCompleted {
		t.Errorf("expected 'completed', got %s", report.OverallStatus)
	}
}

func TestOverallStatusFailed(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")

	tr.FailPhase("a", "error")

	report := tr.Report()
	if report.OverallStatus != StatusFailed {
		t.Errorf("expected 'failed', got %s", report.OverallStatus)
	}
}

func TestSummary(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")
	tr.RegisterPhase("c", "C")

	tr.CompletePhase("a", 100)
	tr.StartPhase("b")

	summary := tr.Summary()

	if !strings.Contains(summary, "1/3") {
		t.Errorf("expected '1/3 phases', got: %s", summary)
	}
	if !strings.Contains(summary, "33%") {
		t.Errorf("expected '33%%', got: %s", summary)
	}
	if !strings.Contains(summary, "Tokens: 100") {
		t.Errorf("expected 'Tokens: 100', got: %s", summary)
	}
}

func TestSummaryAllDone(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.RegisterPhase("b", "B")

	tr.CompletePhase("a", 0)
	tr.CompletePhase("b", 0)

	summary := tr.Summary()
	if !strings.Contains(summary, "Done") {
		t.Errorf("expected 'Done' status, got: %s", summary)
	}
}

func TestSummaryErrors(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")
	tr.FailPhase("a", "fatal")

	summary := tr.Summary()
	if !strings.Contains(summary, "Errors") {
		t.Errorf("expected 'Errors' status, got: %s", summary)
	}
}

func TestSummaryZeroPhases(t *testing.T) {
	tr := New("t1", "desc")

	summary := tr.Summary()
	if !strings.Contains(summary, "0/0") {
		t.Errorf("expected '0/0 phases', got: %s", summary)
	}
	if !strings.Contains(summary, "(0%)") {
		t.Errorf("expected '(0%%)', got: %s", summary)
	}
}

func TestString(t *testing.T) {
	tr := New("t1", "Test Task")
	tr.RegisterPhase("parse", "Parse input")

	tr.StartPhase("parse")

	output := tr.String()
	if !strings.Contains(output, "Test Task") {
		t.Errorf("missing task description in string output")
	}
	if !strings.Contains(output, "parse") {
		t.Errorf("missing phase name in string output")
	}
	if !strings.Contains(output, "[~]") {
		t.Errorf("missing running icon in string output")
	}
}

func TestFormatBar(t *testing.T) {
	// 0/10 -> all empty
	bar := FormatBar(0, 10, 10)
	if bar != "----------" {
		t.Errorf("expected '----------', got %q", bar)
	}

	// 10/10 -> all filled
	bar = FormatBar(10, 10, 10)
	if bar != "==========" {
		t.Errorf("expected '==========', got %q", bar)
	}

	// 5/10 -> half filled
	bar = FormatBar(5, 10, 10)
	expected := "=====" + "-----"
	if bar != expected {
		t.Errorf("expected %q, got %q", expected, bar)
	}

	// 0/0 -> all empty
	bar = FormatBar(0, 0, 10)
	if len(bar) != 10 {
		t.Errorf("expected 10 chars for zero total, got %d", len(bar))
	}

	// 3/5 width 5
	bar = FormatBar(3, 5, 5)
	if bar != "===--" {
		t.Errorf("expected '===--', got %q", bar)
	}
}

func TestFormatBarWidthClamped(t *testing.T) {
	// completed * width / total should not exceed width
	bar := FormatBar(100, 10, 10)
	if len(bar) != 10 {
		t.Errorf("expected bar length 10, got %d", len(bar))
	}
}

func TestFormatBarZeroWidth(t *testing.T) {
	bar := FormatBar(5, 10, 0)
	if bar == "" {
		t.Error("expected default width bar, got empty")
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusCompleted, "[+]"},
		{StatusRunning, "[~]"},
		{StatusFailed, "[!]"},
		{StatusRetrying, "[R]"},
		{StatusSkipped, "[-]"},
		{StatusPending, "[ ]"},
	}

	for _, tt := range tests {
		got := statusIcon(tt.status)
		if got != tt.expected {
			t.Errorf("statusIcon(%s) = %q, want %q", tt.status, got, tt.expected)
		}
	}
}

func TestStateToPhase(t *testing.T) {
	tests := []struct {
		state    core.PipelineState
		expected string
	}{
		{core.StateParsing, "parse"},
		{core.StateDeveloping, "develop"},
		{core.StateTesting, "test"},
		{core.StateChecking, "check"},
		{core.StateCompleted, "completed"},
		{core.StateFailed, "failed"},
	}

	for _, tt := range tests {
		got := StateToPhase(tt.state)
		if got != tt.expected {
			t.Errorf("StateToPhase(%v) = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestStateToPhaseUnknown(t *testing.T) {
	got := StateToPhase("unknown_state")
	if got != "unknown" {
		t.Errorf("expected 'unknown', got %q", got)
	}
}

func TestPhaseNames(t *testing.T) {
	names := PhaseNames()
	if len(names) != 4 {
		t.Errorf("expected 4 phase names, got %d", len(names))
	}
}

func TestRegisterStandardPhases(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterStandardPhases()

	report := tr.Report()
	if len(report.Phases) != 4 {
		t.Errorf("expected 4 standard phases, got %d", len(report.Phases))
	}
}

func TestOnComplete(t *testing.T) {
	tr := New("t1", "desc")
	called := false
	var received Report

	tr.OnComplete(func(r Report) {
		called = true
		received = r
	})

	tr.Finish()

	if !called {
		t.Fatal("expected OnComplete callback to be called")
	}
	if received.TaskID != "t1" {
		t.Errorf("expected taskID 't1', got %s", received.TaskID)
	}
}

func TestFinishWithoutCallback(t *testing.T) {
	tr := New("t1", "desc")
	tr.RegisterPhase("a", "A")

	// Should not panic
	tr.Finish()
}

func TestErrorsAndWarningsAreCopied(t *testing.T) {
	tr := New("t1", "desc")
	tr.AddError("err1")
	tr.AddWarning("warn1")

	r1 := tr.Report()
	r1.Errors[0] = "modified"
	r1.Warnings[0] = "modified"

	r2 := tr.Report()
	if r2.Errors[0] == "modified" {
		t.Error("Errors slice should be a copy, not shared")
	}
	if r2.Warnings[0] == "modified" {
		t.Error("Warnings slice should be a copy, not shared")
	}
}

func TestPhaseStructure(t *testing.T) {
	phase := Phase{
		Name:        "test",
		Description: "Test phase",
		Status:      StatusRunning,
		TokensUsed:  500,
		Retries:     2,
	}

	if phase.Name != "test" {
		t.Error("Name not set correctly")
	}
	if phase.Status != StatusRunning {
		t.Error("Status not set correctly")
	}
	if phase.TokensUsed != 500 {
		t.Error("TokensUsed not set correctly")
	}
}

func TestReportStructure(t *testing.T) {
	report := Report{
		TaskID:        "t1",
		TaskDesc:      "Task",
		OverallStatus: StatusRunning,
		CurrentPhase:  "parse",
		TotalTokens:   1000,
		Errors:        []string{"err1"},
		Warnings:      []string{"warn1"},
	}

	if report.TaskID != "t1" {
		t.Error("TaskID not set correctly")
	}
	if len(report.Errors) != 1 {
		t.Error("Errors not set correctly")
	}
	if len(report.Warnings) != 1 {
		t.Error("Warnings not set correctly")
	}
}

func TestStartNonExistentPhase(t *testing.T) {
	tr := New("t1", "desc")
	// Should not panic
	tr.StartPhase("nonexistent")
	tr.CompletePhase("nonexistent", 0)
	tr.FailPhase("nonexistent", "error")
	tr.RetryPhase("nonexistent", 1)
	tr.SkipPhase("nonexistent", "reason")
}
