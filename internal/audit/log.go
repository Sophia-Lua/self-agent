package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Level represents the severity of an audit event.
type Level string

const (
	LevelInfo     Level = "INFO"
	LevelWarning  Level = "WARNING"
	LevelError    Level = "ERROR"
	LevelCritical Level = "CRITICAL"
)

// Category represents the type of audited action.
type Category string

const (
	CategoryToolCall      Category = "tool_call"
	CategoryFileModify    Category = "file_modify"
	CategoryLLMRequest    Category = "llm_request"
	CategoryStateChange   Category = "state_change"
	CategoryApproval      Category = "approval"
	CategorySnapshot      Category = "snapshot"
	CategoryError         Category = "error"
	CategorySecurity      Category = "security"
)

// Event represents a single auditable action.
type Event struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     Level                  `json:"level"`
	Category  Category               `json:"category"`
	Agent     string                 `json:"agent"`
	Action    string                 `json:"action"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
}

// Logger provides audit logging for the autodev pipeline.
type Logger struct {
	mu      sync.Mutex
	events  []Event
	file    *os.File
	flushed int
}

// New creates an audit logger.
func New() *Logger {
	return &Logger{
		events: make([]Event, 0, 1024),
	}
}

// NewWithFile creates an audit logger that writes to a file.
func NewWithFile(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &Logger{
		events: make([]Event, 0, 1024),
		file:   f,
	}, nil
}

// Info logs an informational event.
func (l *Logger) Info(agent, action string, details map[string]interface{}) {
	l.log(LevelInfo, "", agent, action, details, 0)
}

// Warning logs a warning event.
func (l *Logger) Warning(agent, action string, details map[string]interface{}) {
	l.log(LevelWarning, "", agent, action, details, 0)
}

// Error logs an error event.
func (l *Logger) Error(agent, action string, details map[string]interface{}, err error) {
	d := details
	if d == nil {
		d = make(map[string]interface{})
	}
	if err != nil {
		d["error"] = err.Error()
	}
	l.log(LevelError, "", agent, action, d, 0)
}

// ToolCall logs a tool execution.
func (l *Logger) ToolCall(agent, toolName string, args string, output string, duration time.Duration, err error) {
	details := map[string]interface{}{
		"tool":   toolName,
		"args":   args,
		"output": truncate(output, 500),
	}
	if err != nil {
		details["error"] = err.Error()
	}
	l.log(LevelInfo, CategoryToolCall, agent, fmt.Sprintf("tool.%s", toolName), details, duration)
}

// FileModify logs a file modification.
func (l *Logger) FileModify(agent, path string, beforeLen, afterLen int) {
	l.log(LevelInfo, CategoryFileModify, agent, "file.modify", map[string]interface{}{
		"path":        path,
		"before_size": beforeLen,
		"after_size":  afterLen,
	}, 0)
}

// LLMRequest logs an LLM API call.
func (l *Logger) LLMRequest(agent, model string, promptTokens, completionTokens int, duration time.Duration) {
	l.log(LevelInfo, CategoryLLMRequest, agent, "llm.request", map[string]interface{}{
		"model":            model,
		"prompt_tokens":    promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":     promptTokens + completionTokens,
	}, duration)
}

// StateChange logs a pipeline state transition.
func (l *Logger) StateChange(agent, fromState, toState string) {
	l.log(LevelInfo, CategoryStateChange, agent, "state.change", map[string]interface{}{
		"from": fromState,
		"to":   toState,
	}, 0)
}

// Approval logs an approval decision.
func (l *Logger) Approval(agent, actionID string, approved bool, comment string) {
	l.log(LevelInfo, CategoryApproval, agent, "approval.decision", map[string]interface{}{
		"action_id": actionID,
		"approved":  approved,
		"comment":   comment,
	}, 0)
}

// Snapshot logs a workspace snapshot.
func (l *Logger) Snapshot(agent, snapshotID string, fileCount int) {
	l.log(LevelInfo, CategorySnapshot, agent, "snapshot.create", map[string]interface{}{
		"snapshot_id": snapshotID,
		"file_count":  fileCount,
	}, 0)
}

// Security logs a security-related event.
func (l *Logger) Security(agent, action string, details map[string]interface{}) {
	l.log(LevelCritical, CategorySecurity, agent, action, details, 0)
}

// Events returns all logged events.
func (l *Logger) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Event(nil), l.events...)
}

// EventsByAgent returns events for a specific agent.
func (l *Logger) EventsByAgent(agent string) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []Event
	for _, e := range l.events {
		if e.Agent == agent {
			result = append(result, e)
		}
	}
	return result
}

// EventsByCategory returns events of a specific category.
func (l *Logger) EventsByCategory(cat Category) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []Event
	for _, e := range l.events {
		if e.Category == cat {
			result = append(result, e)
		}
	}
	return result
}

// Since returns events after the given time.
func (l *Logger) Since(t time.Time) []Event {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []Event
	for _, e := range l.events {
		if e.Timestamp.After(t) {
			result = append(result, e)
		}
	}
	return result
}

// Summary returns a summary of audit events.
func (l *Logger) Summary() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	counts := make(map[Category]int)
	levels := make(map[Level]int)
	for _, e := range l.events {
		counts[e.Category]++
		levels[e.Level]++
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Audit Summary: %d total events\n", len(l.events)))
	b.WriteString("By Category:\n")
	for cat, count := range counts {
		b.WriteString(fmt.Sprintf("  %-15s %d\n", cat, count))
	}
	b.WriteString("By Level:\n")
	for lev, count := range levels {
		b.WriteString(fmt.Sprintf("  %-10s %d\n", lev, count))
	}
	return b.String()
}

// ExportJSON exports all events as JSON.
func (l *Logger) ExportJSON() ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return json.MarshalIndent(l.events, "", "  ")
}

// Close flushes and closes the log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Reset clears all events.
func (l *Logger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = l.events[:0]
	l.flushed = 0
}

// Count returns the number of logged events.
func (l *Logger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.events)
}

func (l *Logger) log(level Level, category Category, agent, action string, details map[string]interface{}, duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	event := Event{
		ID:        generateID(),
		Timestamp: time.Now(),
		Level:     level,
		Category:  category,
		Agent:     agent,
		Action:    action,
		Details:   details,
		Duration:  duration,
	}

	l.events = append(l.events, event)

	// Write to file if configured
	if level == LevelError || level == LevelCritical {
		l.writeToFile(event)
	}
}

func (l *Logger) writeToFile(event Event) {
	if l.file == nil {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	l.file.Write(data)
	l.file.Write([]byte("\n"))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

var idCounter int
var idMu sync.Mutex

func generateID() string {
	idMu.Lock()
	defer idMu.Unlock()
	idCounter++
	return fmt.Sprintf("audit-%d-%d", time.Now().UnixNano(), idCounter)
}
