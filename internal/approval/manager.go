package approval

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Action represents a pending action requiring human approval.
type Action struct {
	ID          string
	Description string
	Details     string
	RiskLevel   RiskLevel
	CreatedAt   time.Time
	Deadline    time.Time
}

// RiskLevel indicates how dangerous an action is.
type RiskLevel int

const (
	RiskLow RiskLevel = iota
	RiskMedium
	RiskHigh
	RiskCritical
)

// Decision represents a human's response to an approval request.
type Decision struct {
	ActionID string
	Approved bool
	Comment  string
	DecidedAt time.Time
}

// Mode defines how the approval system behaves.
type Mode int

const (
	// ModeAuto automatically approves low-risk actions.
	ModeAuto Mode = iota
	// ModePrompt prompts the user via stdin for approval.
	ModePrompt
	// ModeStrict requires explicit approval for all actions.
	ModeStrict
)

// Requester defines how approval requests are presented to the user.
type Requester interface {
	Prompt(ctx context.Context, action *Action) (*Decision, error)
}

// StdinRequester reads approval decisions from stdin.
type StdinRequester struct {
	Reader *bufio.Reader
}

// NewStdinRequester creates a requester that reads from standard input.
func NewStdinRequester() *StdinRequester {
	return &StdinRequester{Reader: bufio.NewReader(os.Stdin)}
}

// Prompt displays the action and waits for user input.
func (s *StdinRequester) Prompt(ctx context.Context, action *Action) (*Decision, error) {
	fmt.Printf("\n[APPROVAL REQUIRED] %s\n", action.Description)
	fmt.Printf("Risk Level: %s\n", riskLabel(action.RiskLevel))
	if action.Details != "" {
		fmt.Printf("Details: %s\n", action.Details)
	}
	fmt.Print("Approve? [y/N/comment]: ")

	// Use a channel to make the read interruptible
	type result struct {
		decision *Decision
		err      error
	}

	ch := make(chan result, 1)
	go func() {
		line, err := s.Reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil {
			ch <- result{nil, err}
			return
		}

		decision := &Decision{
			ActionID:  action.ID,
			DecidedAt: time.Now(),
		}

		switch strings.ToLower(line) {
		case "y", "yes":
			decision.Approved = true
		case "":
			decision.Approved = false
			decision.Comment = "defaulted to deny (empty input)"
		default:
			decision.Approved = false
			decision.Comment = line
		}

		ch <- result{decision, nil}
	}()

	select {
	case <-ctx.Done():
		return &Decision{
			ActionID:  action.ID,
			Approved:  false,
			Comment:   "cancelled (context done)",
			DecidedAt: time.Now(),
		}, ctx.Err()
	case res := <-ch:
		return res.decision, res.err
	}
}

// Manager coordinates human approval gates in the pipeline.
type Manager struct {
	mu        sync.Mutex
	mode      Mode
	requester Requester
	pending   map[string]*Action
	history   []Decision
	timeout   time.Duration
	autoApprove map[RiskLevel]bool
}

// New creates an approval manager with the specified mode.
func New(mode Mode, timeout time.Duration) *Manager {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	mgr := &Manager{
		mode:      mode,
		pending:   make(map[string]*Action),
		timeout:   timeout,
		autoApprove: make(map[RiskLevel]bool),
	}

	// Default auto-approval for low risk in auto mode
	if mode == ModeAuto {
		mgr.autoApprove[RiskLow] = true
	}

	return mgr
}

// WithRequester sets the requester implementation.
func (m *Manager) WithRequester(r Requester) *Manager {
	m.requester = r
	return m
}

// SubmitAction registers an action requiring approval.
func (m *Manager) SubmitAction(id, description, details string, risk RiskLevel) *Action {
	action := &Action{
		ID:          id,
		Description: description,
		Details:     details,
		RiskLevel:   risk,
		CreatedAt:   time.Now(),
		Deadline:    time.Now().Add(m.timeout),
	}

	m.mu.Lock()
	m.pending[id] = action
	m.mu.Unlock()

	return action
}

// RequestApproval asks for human approval of an action.
func (m *Manager) RequestApproval(ctx context.Context, action *Action) (*Decision, error) {
	// Check auto-approval rules
	if m.autoApprove[action.RiskLevel] {
		return &Decision{
			ActionID:  action.ID,
			Approved:  true,
			Comment:   "auto-approved (low risk)",
			DecidedAt: time.Now(),
		}, nil
	}

	// Skip requester in auto mode for medium risk as well
	if m.mode == ModeAuto && action.RiskLevel <= RiskMedium {
		return &Decision{
			ActionID:  action.ID,
			Approved:  true,
			Comment:   "auto-approved (medium risk in auto mode)",
			DecidedAt: time.Now(),
		}, nil
	}

	if m.requester == nil {
		m.requester = NewStdinRequester()
	}

	decision, err := m.requester.Prompt(ctx, action)

	m.mu.Lock()
	defer m.mu.Unlock()

	if err == nil {
		m.history = append(m.history, *decision)
		delete(m.pending, action.ID)
	}

	return decision, err
}

// RequestApprovalByID looks up a pending action and requests approval.
func (m *Manager) RequestApprovalByID(ctx context.Context, actionID string) (*Decision, error) {
	m.mu.Lock()
	action, exists := m.pending[actionID]
	m.mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("action %q not found", actionID)
	}

	return m.RequestApproval(ctx, action)
}

// IsApproved checks if an action was approved.
func (m *Manager) IsApproved(actionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.history {
		if d.ActionID == actionID && d.Approved {
			return true
		}
	}
	return false
}

// PendingActions returns all actions awaiting approval.
func (m *Manager) PendingActions() []*Action {
	m.mu.Lock()
	defer m.mu.Unlock()

	actions := make([]*Action, 0, len(m.pending))
	for _, a := range m.pending {
		actions = append(actions, a)
	}
	return actions
}

// History returns all past decisions.
func (m *Manager) History() []Decision {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]Decision(nil), m.history...)
}

// ClearHistory removes all past decisions.
func (m *Manager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.history = nil
}

// SetMode changes the approval mode.
func (m *Manager) SetMode(mode Mode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = mode
}

// SetAutoApprove configures which risk levels are auto-approved.
func (m *Manager) SetAutoApprove(levels []RiskLevel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoApprove = make(map[RiskLevel]bool)
	for _, level := range levels {
		m.autoApprove[level] = true
	}
}

// Summary returns a summary of approval statistics.
func (m *Manager) Summary() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	approved := 0
	denied := 0
	for _, d := range m.history {
		if d.Approved {
			approved++
		} else {
			denied++
		}
	}

	return fmt.Sprintf("Approval Stats: %d approved, %d denied, %d pending",
		approved, denied, len(m.pending))
}

// ShouldBlock checks if an action must be blocked based on approval history.
func (m *Manager) ShouldBlock(actionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, d := range m.history {
		if d.ActionID == actionID && !d.Approved {
			return true
		}
	}
	return false
}

func riskLevel(r RiskLevel) string {
	switch r {
	case RiskLow:
		return "LOW"
	case RiskMedium:
		return "MEDIUM"
	case RiskHigh:
		return "HIGH"
	case RiskCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func riskLabel(r RiskLevel) string {
	return riskLevel(r)
}

// CommonActions defines standard approval points in the pipeline.
type CommonActions struct {
	DeleteFiles     bool
	ModifyConfig    bool
	ExecuteCommands bool
	Deploy          bool
	AccessSecrets   bool
}

// ShouldPrompt checks if a common action requires approval.
func (m *Manager) ShouldPrompt(actionType string) bool {
	switch m.mode {
	case ModeStrict:
		return true
	case ModeAuto:
		return false
	case ModePrompt:
		return true
	default:
		return false
	}
}
