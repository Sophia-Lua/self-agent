package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	logger := New()
	if logger == nil {
		t.Fatal("logger is nil")
	}
	if logger.Count() != 0 {
		t.Error("new logger should have 0 events")
	}
}

func TestNewWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audit.log")

	logger, err := NewWithFile(path)
	if err != nil {
		t.Fatalf("NewWithFile failed: %v", err)
	}
	defer logger.Close()

	if logger.file == nil {
		t.Fatal("file should be set")
	}

	// Write an error to trigger file write
	logger.Error("test-agent", "test action", nil, nil)
	logger.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("audit log file should not be empty")
	}
}

func TestNewWithFileInvalidPath(t *testing.T) {
	_, err := NewWithFile("/dev/null/invalid/path/audit.log")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestInfo(t *testing.T) {
	logger := New()
	logger.Info("agent-1", "task started", map[string]interface{}{"task_id": "123"})

	events := logger.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].Level != LevelInfo {
		t.Errorf("level = %s, want %s", events[0].Level, LevelInfo)
	}
	if events[0].Agent != "agent-1" {
		t.Errorf("agent = %s", events[0].Agent)
	}
	if events[0].Category != "" {
		// Info doesn't set category by default but might have a value from log function
	}
}

func TestWarning(t *testing.T) {
	logger := New()
	logger.Warning("agent-1", "slow operation", nil)

	events := logger.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].Level != LevelWarning {
		t.Errorf("level = %s", events[0].Level)
	}
}

func TestError(t *testing.T) {
	logger := New()
	testErr := &testError{"something failed"}
	logger.Error("agent-1", "task failed", map[string]interface{}{"detail": "x"}, testErr)

	events := logger.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].Level != LevelError {
		t.Errorf("level = %s", events[0].Level)
	}
	if events[0].Details["error"] != "something failed" {
		t.Errorf("error detail = %v", events[0].Details["error"])
	}
}

func TestErrorNilDetails(t *testing.T) {
	logger := New()
	logger.Error("agent-1", "task failed", nil, &testError{"oops"})

	events := logger.Events()
	if events[0].Details == nil {
		t.Fatal("details should not be nil")
	}
}

func TestToolCall(t *testing.T) {
	logger := New()
	logger.ToolCall("agent-1", "read_file", `{"path":"x.go"}`, "content", 10*time.Millisecond, nil)

	events := logger.Events()
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}
	if events[0].Category != CategoryToolCall {
		t.Errorf("category = %s", events[0].Category)
	}
	if events[0].Details["tool"] != "read_file" {
		t.Errorf("tool = %v", events[0].Details["tool"])
	}
}

func TestToolCallError(t *testing.T) {
	logger := New()
	logger.ToolCall("agent-1", "write_file", "", "output", 0, &testError{"permission denied"})

	events := logger.Events()
	if events[0].Details["error"] != "permission denied" {
		t.Errorf("error = %v", events[0].Details["error"])
	}
}

func TestToolCallTruncatesOutput(t *testing.T) {
	logger := New()
	longOutput := strings.Repeat("a", 1000)
	logger.ToolCall("agent-1", "cmd", "args", longOutput, 0, nil)

	events := logger.Events()
	output := events[0].Details["output"].(string)
	if len(output) > 503 { // 500 + 3 for "..."
		t.Errorf("output length = %d, should be <= 503", len(output))
	}
}

func TestFileModify(t *testing.T) {
	logger := New()
	logger.FileModify("agent-1", "main.go", 100, 200)

	events := logger.Events()
	if events[0].Category != CategoryFileModify {
		t.Errorf("category = %s", events[0].Category)
	}
	if events[0].Details["path"] != "main.go" {
		t.Errorf("path = %v", events[0].Details["path"])
	}
	if events[0].Details["before_size"] != 100 {
		t.Errorf("before_size = %v", events[0].Details["before_size"])
	}
	if events[0].Details["after_size"] != 200 {
		t.Errorf("after_size = %v", events[0].Details["after_size"])
	}
}

func TestLLMRequest(t *testing.T) {
	logger := New()
	logger.LLMRequest("agent-1", "gpt-4", 100, 50, 500*time.Millisecond)

	events := logger.Events()
	if events[0].Category != CategoryLLMRequest {
		t.Errorf("category = %s", events[0].Category)
	}
	if events[0].Details["model"] != "gpt-4" {
		t.Errorf("model = %v", events[0].Details["model"])
	}
	if events[0].Details["total_tokens"] != 150 {
		t.Errorf("total_tokens = %v", events[0].Details["total_tokens"])
	}
}

func TestStateChange(t *testing.T) {
	logger := New()
	logger.StateChange("agent-1", "parsing", "developing")

	events := logger.Events()
	if events[0].Category != CategoryStateChange {
		t.Errorf("category = %s", events[0].Category)
	}
	if events[0].Details["from"] != "parsing" {
		t.Errorf("from = %v", events[0].Details["from"])
	}
	if events[0].Details["to"] != "developing" {
		t.Errorf("to = %v", events[0].Details["to"])
	}
}

func TestApprovalLog(t *testing.T) {
	logger := New()
	logger.Approval("agent-1", "act-1", true, "looks good")

	events := logger.Events()
	if events[0].Category != CategoryApproval {
		t.Errorf("category = %s", events[0].Category)
	}
	if events[0].Details["approved"] != true {
		t.Errorf("approved = %v", events[0].Details["approved"])
	}
	if events[0].Details["comment"] != "looks good" {
		t.Errorf("comment = %v", events[0].Details["comment"])
	}
}

func TestSnapshotLog(t *testing.T) {
	logger := New()
	logger.Snapshot("agent-1", "snap-1", 42)

	events := logger.Events()
	if events[0].Category != CategorySnapshot {
		t.Errorf("category = %s", events[0].Category)
	}
	if events[0].Details["file_count"] != 42 {
		t.Errorf("file_count = %v", events[0].Details["file_count"])
	}
}

func TestSecurity(t *testing.T) {
	logger := New()
	logger.Security("agent-1", "suspicious activity", map[string]interface{}{"ip": "1.2.3.4"})

	events := logger.Events()
	if events[0].Level != LevelCritical {
		t.Errorf("level = %s", events[0].Level)
	}
	if events[0].Category != CategorySecurity {
		t.Errorf("category = %s", events[0].Category)
	}
}

func TestEventsByAgent(t *testing.T) {
	logger := New()
	logger.Info("agent-a", "event 1", nil)
	logger.Info("agent-b", "event 2", nil)
	logger.Info("agent-a", "event 3", nil)

	eventsA := logger.EventsByAgent("agent-a")
	if len(eventsA) != 2 {
		t.Errorf("agent-a events = %d, want 2", len(eventsA))
	}

	eventsB := logger.EventsByAgent("agent-b")
	if len(eventsB) != 1 {
		t.Errorf("agent-b events = %d, want 1", len(eventsB))
	}
}

func TestEventsByCategory(t *testing.T) {
	logger := New()
	logger.Info("agent-1", "start", nil)
	logger.StateChange("agent-1", "a", "b")
	logger.FileModify("agent-1", "x.go", 0, 100)

	cats := logger.EventsByCategory(CategoryStateChange)
	if len(cats) != 1 {
		t.Errorf("state change events = %d, want 1", len(cats))
	}
}

func TestSince(t *testing.T) {
	logger := New()
	before := time.Now()
	time.Sleep(10 * time.Millisecond)
	logger.Info("agent-1", "after", nil)

	events := logger.Since(before)
	if len(events) != 1 {
		t.Errorf("events since before = %d, want 1", len(events))
	}
}

func TestSummary(t *testing.T) {
	logger := New()
	logger.Info("a", "info", nil)
	logger.Warning("a", "warn", nil)
	logger.StateChange("a", "x", "y")

	summary := logger.Summary()
	if !strings.Contains(summary, "3 total events") {
		t.Errorf("summary = %q", summary)
	}
}

func TestExportJSON(t *testing.T) {
	logger := New()
	logger.Info("agent-1", "test", nil)

	data, err := logger.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("JSON should not be empty")
	}
}

func TestClose(t *testing.T) {
	logger := New()
	err := logger.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestReset(t *testing.T) {
	logger := New()
	logger.Info("agent-1", "event 1", nil)
	logger.Info("agent-1", "event 2", nil)

	logger.Reset()
	if len(logger.Events()) != 0 {
		t.Error("events should be empty after reset")
	}
}

func TestCount(t *testing.T) {
	logger := New()
	if logger.Count() != 0 {
		t.Errorf("count = %d, want 0", logger.Count())
	}

	logger.Info("a", "1", nil)
	logger.Info("a", "2", nil)
	logger.Info("a", "3", nil)

	if logger.Count() != 3 {
		t.Errorf("count = %d, want 3", logger.Count())
	}
}

func TestLevels(t *testing.T) {
	if LevelInfo != "INFO" {
		t.Errorf("LevelInfo = %q", LevelInfo)
	}
	if LevelWarning != "WARNING" {
		t.Errorf("LevelWarning = %q", LevelWarning)
	}
	if LevelError != "ERROR" {
		t.Errorf("LevelError = %q", LevelError)
	}
	if LevelCritical != "CRITICAL" {
		t.Errorf("LevelCritical = %q", LevelCritical)
	}
}

func TestCategories(t *testing.T) {
	if CategoryToolCall != "tool_call" {
		t.Errorf("CategoryToolCall = %q", CategoryToolCall)
	}
	if CategoryFileModify != "file_modify" {
		t.Errorf("CategoryFileModify = %q", CategoryFileModify)
	}
	if CategoryLLMRequest != "llm_request" {
		t.Errorf("CategoryLLMRequest = %q", CategoryLLMRequest)
	}
	if CategoryStateChange != "state_change" {
		t.Errorf("CategoryStateChange = %q", CategoryStateChange)
	}
	if CategoryApproval != "approval" {
		t.Errorf("CategoryApproval = %q", CategoryApproval)
	}
	if CategorySnapshot != "snapshot" {
		t.Errorf("CategorySnapshot = %q", CategorySnapshot)
	}
	if CategoryError != "error" {
		t.Errorf("CategoryError = %q", CategoryError)
	}
	if CategorySecurity != "security" {
		t.Errorf("CategorySecurity = %q", CategorySecurity)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Error("short string should not be truncated")
	}
	if truncate(strings.Repeat("a", 10), 5) != "aaaaa..." {
		t.Error("long string should be truncated with ...")
	}
	if truncate("", 5) != "" {
		t.Error("empty string should stay empty")
	}
}

func TestEventIDGenerated(t *testing.T) {
	logger := New()
	logger.Info("agent-1", "test", nil)

	events := logger.Events()
	if events[0].ID == "" {
		t.Error("event ID should not be empty")
	}
}

func TestEventTimestamp(t *testing.T) {
	logger := New()
	logger.Info("agent-1", "test", nil)

	events := logger.Events()
	if events[0].Timestamp.IsZero() {
		t.Error("event timestamp should be set")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
