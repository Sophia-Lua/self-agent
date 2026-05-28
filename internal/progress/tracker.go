package progress

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"autodev/internal/core"
)

// Phase represents a distinct stage in the pipeline execution.
type Phase struct {
	Name        string
	Description string
	Status      Status
	StartedAt   time.Time
	CompletedAt time.Time
	TokensUsed  int
	Retries     int
}

// Status represents the execution state of a phase or the overall pipeline.
type Status string

const (
	StatusPending    Status = "pending"
	StatusRunning    Status = "running"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusRetrying   Status = "retrying"
	StatusSkipped    Status = "skipped"
)

// Report represents a snapshot of the current pipeline execution status.
type Report struct {
	TaskID         string
	TaskDesc       string
	OverallStatus  Status
	CurrentPhase   string
	Phases         []Phase
	TotalTokens    int
	ElapsedTime    time.Duration
	Errors         []string
	Warnings       []string
	Timestamp      time.Time
}

// Tracker monitors and records pipeline execution progress.
type Tracker struct {
	mu              sync.RWMutex
	taskID          string
	taskDesc        string
	startTime       time.Time
	phases          map[string]*Phase
	phaseOrder      []string
	currentPhase    string
	totalTokens     int
	errors          []string
	warnings        []string
	subscribers     []chan Report
	subscribersMu   sync.Mutex
	onComplete      func(Report)
}

// New creates a progress tracker for a task.
func New(taskID, taskDesc string) *Tracker {
	return &Tracker{
		taskID:    taskID,
		taskDesc:  taskDesc,
		startTime: time.Now(),
		phases:    make(map[string]*Phase),
	}
}

// RegisterPhase adds a phase to track.
func (t *Tracker) RegisterPhase(name, description string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.phases[name] = &Phase{
		Name:        name,
		Description: description,
		Status:      StatusPending,
	}
	t.phaseOrder = append(t.phaseOrder, name)
}

// StartPhase marks a phase as running.
func (t *Tracker) StartPhase(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if phase, exists := t.phases[name]; exists {
		phase.Status = StatusRunning
		phase.StartedAt = time.Now()
		t.currentPhase = name
		t.notify()
	}
}

// CompletePhase marks a phase as completed with token usage.
func (t *Tracker) CompletePhase(name string, tokensUsed int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if phase, exists := t.phases[name]; exists {
		phase.Status = StatusCompleted
		phase.CompletedAt = time.Now()
		phase.TokensUsed = tokensUsed
		t.totalTokens += tokensUsed
		t.notify()
	}
}

// FailPhase marks a phase as failed with an error message.
func (t *Tracker) FailPhase(name string, errMsg string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if phase, exists := t.phases[name]; exists {
		phase.Status = StatusFailed
		phase.CompletedAt = time.Now()
		t.errors = append(t.errors, fmt.Sprintf("[%s] %s", name, errMsg))
		t.notify()
	}
}

// RetryPhase marks a phase as retrying.
func (t *Tracker) RetryPhase(name string, attempt int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if phase, exists := t.phases[name]; exists {
		phase.Status = StatusRetrying
		phase.Retries = attempt
		phase.StartedAt = time.Now()
		t.notify()
	}
}

// SkipPhase marks a phase as skipped.
func (t *Tracker) SkipPhase(name string, reason string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if phase, exists := t.phases[name]; exists {
		phase.Status = StatusSkipped
		phase.CompletedAt = time.Now()
		t.warnings = append(t.warnings, fmt.Sprintf("[%s] Skipped: %s", name, reason))
		t.notify()
	}
}

// AddError records an error without failing a phase.
func (t *Tracker) AddError(err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errors = append(t.errors, err)
	t.notify()
}

// AddWarning records a warning.
func (t *Tracker) AddWarning(warn string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.warnings = append(t.warnings, warn)
	t.notify()
}

// Report returns a snapshot of current progress.
func (t *Tracker) Report() Report {
	t.mu.RLock()
	defer t.mu.RUnlock()

	phases := make([]Phase, 0, len(t.phaseOrder))
	for _, name := range t.phaseOrder {
		if phase, exists := t.phases[name]; exists {
			phases = append(phases, *phase)
		}
	}

	overallStatus := StatusRunning
	if len(t.errors) > 0 && t.isAnyRunning() {
		// Still running despite errors
	} else if len(t.errors) > 0 {
		overallStatus = StatusFailed
	} else if t.allCompleted() {
		overallStatus = StatusCompleted
	}

	return Report{
		TaskID:         t.taskID,
		TaskDesc:       t.taskDesc,
		OverallStatus:  overallStatus,
		CurrentPhase:   t.currentPhase,
		Phases:         phases,
		TotalTokens:    t.totalTokens,
		ElapsedTime:    time.Since(t.startTime),
		Errors:         append([]string(nil), t.errors...),
		Warnings:       append([]string(nil), t.warnings...),
		Timestamp:      time.Now(),
	}
}

// Subscribe adds a channel to receive progress updates.
func (t *Tracker) Subscribe(ch chan Report) {
	t.subscribersMu.Lock()
	defer t.subscribersMu.Unlock()
	t.subscribers = append(t.subscribers, ch)
}

// String returns a human-readable progress summary.
func (t *Tracker) String() string {
	report := t.Report()
	return renderReport(report)
}

// Summary returns a concise one-line summary.
func (t *Tracker) Summary() string {
	report := t.Report()
	completed := 0
	running := 0
	failed := 0

	for _, p := range report.Phases {
		switch p.Status {
		case StatusCompleted:
			completed++
		case StatusRunning, StatusRetrying:
			running++
		case StatusFailed:
			failed++
		}
	}

	total := len(report.Phases)
	pct := 0
	if total > 0 {
		pct = (completed * 100) / total
	}

	status := "Running"
	if failed > 0 {
		status = "Errors"
	} else if completed == total && total > 0 {
		status = "Done"
	}

	return fmt.Sprintf("[%s] %d/%d phases (%d%%) - Tokens: %d - Time: %v",
		status, completed, total, pct, report.TotalTokens, report.ElapsedTime.Round(time.Second))
}

// OnComplete sets a callback for when the pipeline finishes.
func (t *Tracker) OnComplete(fn func(Report)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onComplete = fn
}

// Finish marks the tracker as complete and triggers callbacks.
func (t *Tracker) Finish() {
	report := t.Report()
	if t.onComplete != nil {
		t.onComplete(report)
	}
	t.notify()
}

// StateToPhase maps pipeline states to phase names.
func StateToPhase(state core.PipelineState) string {
	switch state {
	case core.StateParsing:
		return "parse"
	case core.StateDeveloping:
		return "develop"
	case core.StateTesting:
		return "test"
	case core.StateChecking:
		return "check"
	case core.StateCompleted:
		return "completed"
	case core.StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// PhaseNames returns the standard phase names for the pipeline.
func PhaseNames() []string {
	return []string{
		string(core.StateParsing),
		string(core.StateDeveloping),
		string(core.StateTesting),
		string(core.StateChecking),
	}
}

// RegisterStandardPhases registers the standard pipeline phases.
func (t *Tracker) RegisterStandardPhases() {
	t.RegisterPhase(string(core.StateParsing), "Parse and understand the task")
	t.RegisterPhase(string(core.StateDeveloping), "Implement the solution")
	t.RegisterPhase(string(core.StateTesting), "Verify with tests")
	t.RegisterPhase(string(core.StateChecking), "Review code quality")
}

func (t *Tracker) notify() {
	// Build report data while holding read lock, then notify subscribers
	// without calling Report() which would try to re-acquire the write lock
	t.subscribersMu.Lock()
	defer t.subscribersMu.Unlock()

	phases := make([]Phase, 0, len(t.phaseOrder))
	for _, name := range t.phaseOrder {
		if phase, exists := t.phases[name]; exists {
			phases = append(phases, *phase)
		}
	}

	report := Report{
		TaskID:       t.taskID,
		TaskDesc:     t.taskDesc,
		CurrentPhase: t.currentPhase,
		Phases:       phases,
		TotalTokens:  t.totalTokens,
		ElapsedTime:  time.Since(t.startTime),
		Errors:       append([]string(nil), t.errors...),
		Warnings:     append([]string(nil), t.warnings...),
		Timestamp:    time.Now(),
	}

	for _, ch := range t.subscribers {
		select {
		case ch <- report:
		default:
			// Drop if channel full
		}
	}
}

func (t *Tracker) isAnyRunning() bool {
	for _, phase := range t.phases {
		if phase.Status == StatusRunning || phase.Status == StatusRetrying {
			return true
		}
	}
	return false
}

func (t *Tracker) allCompleted() bool {
	for _, phase := range t.phases {
		if phase.Status != StatusCompleted && phase.Status != StatusSkipped {
			return false
		}
	}
	return len(t.phases) > 0
}

// renderReport creates a text representation of the progress report.
func renderReport(r Report) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Task: %s\n", r.TaskDesc))
	b.WriteString(fmt.Sprintf("Status: %s | Phase: %s\n", r.OverallStatus, r.CurrentPhase))
	b.WriteString(fmt.Sprintf("Elapsed: %v | Tokens: %d\n\n", r.ElapsedTime.Round(time.Second), r.TotalTokens))

	for _, p := range r.Phases {
		icon := statusIcon(p.Status)
		duration := ""
		if !p.CompletedAt.IsZero() && !p.StartedAt.IsZero() {
			duration = fmt.Sprintf(" (%v)", p.CompletedAt.Sub(p.StartedAt).Round(time.Millisecond))
		}
		b.WriteString(fmt.Sprintf("  %s %-12s %s%s\n", icon, p.Name, p.Description, duration))
		if p.Retries > 0 {
			b.WriteString(fmt.Sprintf("              Retries: %d\n", p.Retries))
		}
	}

	if len(r.Errors) > 0 {
		b.WriteString("\nErrors:\n")
		for _, e := range r.Errors {
			b.WriteString(fmt.Sprintf("  - %s\n", e))
		}
	}

	if len(r.Warnings) > 0 {
		b.WriteString("\nWarnings:\n")
		for _, w := range r.Warnings {
			b.WriteString(fmt.Sprintf("  - %s\n", w))
		}
	}

	return b.String()
}

func statusIcon(s Status) string {
	switch s {
	case StatusCompleted:
		return "[+]"
	case StatusRunning:
		return "[~]"
	case StatusFailed:
		return "[!]"
	case StatusRetrying:
		return "[R]"
	case StatusSkipped:
		return "[-]"
	default:
		return "[ ]"
	}
}

// FormatBar returns a simple ASCII progress bar.
func FormatBar(completed, total int, width int) string {
	if width <= 0 {
		width = 20
	}
	if total == 0 {
		return strings.Repeat(" ", width)
	}
	filled := (completed * width) / total
	if filled > width {
		filled = width
	}
	return strings.Repeat("=", filled) + strings.Repeat("-", width-filled)
}
