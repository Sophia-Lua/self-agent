package approval

import (
	"bufio"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewDefaultTimeout(t *testing.T) {
	mgr := New(ModeAuto, 0)
	if mgr.mode != ModeAuto {
		t.Errorf("mode = %d, want %d", mgr.mode, ModeAuto)
	}
	if mgr.timeout <= 0 {
		t.Error("timeout should be positive")
	}
}

func TestNewCustomTimeout(t *testing.T) {
	mgr := New(ModeStrict, 30*time.Second)
	if mgr.timeout != 30*time.Second {
		t.Errorf("timeout = %v, want 30s", mgr.timeout)
	}
}

func TestModeValues(t *testing.T) {
	if ModeAuto != 0 {
		t.Errorf("ModeAuto = %d, want 0", ModeAuto)
	}
	if ModePrompt != 1 {
		t.Errorf("ModePrompt = %d, want 1", ModePrompt)
	}
	if ModeStrict != 2 {
		t.Errorf("ModeStrict = %d, want 2", ModeStrict)
	}
}

func TestRiskLevelValues(t *testing.T) {
	if RiskLow != 0 {
		t.Errorf("RiskLow = %d", RiskLow)
	}
	if RiskMedium != 1 {
		t.Errorf("RiskMedium = %d", RiskMedium)
	}
	if RiskHigh != 2 {
		t.Errorf("RiskHigh = %d", RiskHigh)
	}
	if RiskCritical != 3 {
		t.Errorf("RiskCritical = %d", RiskCritical)
	}
}

func TestRiskLabels(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskLow, "LOW"},
		{RiskMedium, "MEDIUM"},
		{RiskHigh, "HIGH"},
		{RiskCritical, "CRITICAL"},
		{RiskLevel(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := riskLabel(tt.level); got != tt.want {
			t.Errorf("riskLabel(%d) = %s, want %s", tt.level, got, tt.want)
		}
	}
}

func TestSubmitAction(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	action := mgr.SubmitAction("act-1", "delete file", "remove /tmp/x", RiskHigh)
	if action.ID != "act-1" {
		t.Errorf("ID = %s", action.ID)
	}
	if action.Description != "delete file" {
		t.Errorf("Description = %s", action.Description)
	}
	if action.Details != "remove /tmp/x" {
		t.Errorf("Details = %s", action.Details)
	}
	if action.RiskLevel != RiskHigh {
		t.Errorf("RiskLevel = %d", action.RiskLevel)
	}
	if action.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if action.Deadline.Before(action.CreatedAt) {
		t.Error("Deadline should be after CreatedAt")
	}

	actions := mgr.PendingActions()
	if len(actions) != 1 {
		t.Errorf("pending count = %d, want 1", len(actions))
	}
}

func TestSubmitMultipleActions(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mgr.SubmitAction("a1", "act1", "", RiskLow)
	mgr.SubmitAction("a2", "act2", "", RiskMedium)
	mgr.SubmitAction("a3", "act3", "", RiskHigh)

	actions := mgr.PendingActions()
	if len(actions) != 3 {
		t.Errorf("pending count = %d, want 3", len(actions))
	}
}

func TestAutoApproveLowRisk(t *testing.T) {
	mgr := New(ModeAuto, time.Minute)

	action := mgr.SubmitAction("act-1", "safe change", "", RiskLow)
	decision, err := mgr.RequestApproval(context.Background(), action)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if !decision.Approved {
		t.Error("expected auto-approval for low risk in auto mode")
	}
	if decision.Comment != "auto-approved (low risk)" {
		t.Errorf("comment = %q", decision.Comment)
	}
}

func TestAutoApproveMediumRisk(t *testing.T) {
	mgr := New(ModeAuto, time.Minute)

	action := mgr.SubmitAction("act-1", "medium change", "", RiskMedium)
	decision, err := mgr.RequestApproval(context.Background(), action)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if !decision.Approved {
		t.Error("expected auto-approval for medium risk in auto mode")
	}
}

func TestStrictModeDoesNotAutoApprove(t *testing.T) {
	mgr := New(ModeStrict, time.Minute)

	// Use a mock requester that auto-approves
	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "strict check", "", RiskLow)
	_, err := mgr.RequestApproval(context.Background(), action)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}

	if !mockReq.PromptCalled {
		t.Error("expected Prompt to be called in strict mode")
	}
}

func TestRequestApprovalByID(t *testing.T) {
	mgr := New(ModeAuto, time.Minute)

	mgr.SubmitAction("act-1", "test action", "", RiskLow)

	decision, err := mgr.RequestApprovalByID(context.Background(), "act-1")
	if err != nil {
		t.Fatalf("RequestApprovalByID failed: %v", err)
	}
	if !decision.Approved {
		t.Error("expected approval")
	}

	// Non-existent ID
	_, err = mgr.RequestApprovalByID(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent action")
	}
}

func TestIsApproved(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "test", "", RiskHigh)
	mgr.RequestApproval(context.Background(), action)

	if !mgr.IsApproved("act-1") {
		t.Error("should be approved")
	}
	if mgr.IsApproved("nonexistent") {
		t.Error("non-existent action should not be approved")
	}
}

func TestShouldBlock(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
		"act-2": {ActionID: "act-2", Approved: false},
	}}
	mgr.WithRequester(mockReq)

	action1 := mgr.SubmitAction("act-1", "approved", "", RiskHigh)
	action2 := mgr.SubmitAction("act-2", "denied", "", RiskHigh)

	mgr.RequestApproval(context.Background(), action1)
	mgr.RequestApproval(context.Background(), action2)

	if mgr.ShouldBlock("act-1") {
		t.Error("approved action should not be blocked")
	}
	if !mgr.ShouldBlock("act-2") {
		t.Error("denied action should be blocked")
	}
	if mgr.ShouldBlock("nonexistent") {
		t.Error("non-existent action should not be blocked")
	}
}

func TestHistory(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "test", "", RiskHigh)
	mgr.RequestApproval(context.Background(), action)

	history := mgr.History()
	if len(history) != 1 {
		t.Errorf("history length = %d, want 1", len(history))
	}

	// Modify returned slice shouldn't affect internal state
	history = append(history, Decision{})
	if len(mgr.History()) == 2 {
		t.Error("History() should return a copy")
	}
}

func TestClearHistory(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "test", "", RiskHigh)
	mgr.RequestApproval(context.Background(), action)

	if len(mgr.History()) != 1 {
		t.Fatalf("expected 1 history entry")
	}

	mgr.ClearHistory()
	if len(mgr.History()) != 0 {
		t.Error("history should be empty after clear")
	}
}

func TestSummary(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action1 := mgr.SubmitAction("act-1", "approved", "", RiskHigh)
	mgr.RequestApproval(context.Background(), action1)

	mgr.SubmitAction("act-2", "pending", "", RiskHigh)

	summary := mgr.Summary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
	if !strings.Contains(summary, "1 approved") {
		t.Errorf("summary = %q, should contain '1 approved'", summary)
	}
	if !strings.Contains(summary, "1 pending") {
		t.Errorf("summary = %q, should contain '1 pending'", summary)
	}
}

func TestSetMode(t *testing.T) {
	mgr := New(ModeStrict, time.Minute)
	mgr.SetMode(ModeAuto)

	action := mgr.SubmitAction("act-1", "test", "", RiskLow)
	decision, err := mgr.RequestApproval(context.Background(), action)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if !decision.Approved {
		t.Error("auto mode should auto-approve low risk")
	}

	// Switch back to strict
	mgr.SetMode(ModeStrict)
	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-2": {ActionID: "act-2", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action2 := mgr.SubmitAction("act-2", "test", "", RiskLow)
	mgr.RequestApproval(context.Background(), action2)

	if !mockReq.PromptCalled {
		t.Error("changing back to strict mode should require prompting")
	}
}

func TestSetAutoApprove(t *testing.T) {
	mgr := New(ModeStrict, time.Minute)
	mgr.SetAutoApprove([]RiskLevel{RiskLow, RiskMedium})

	mockReq := &MockRequester{}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "test", "", RiskLow)
	decision, err := mgr.RequestApproval(context.Background(), action)
	if err != nil {
		t.Fatalf("RequestApproval failed: %v", err)
	}
	if !decision.Approved {
		t.Error("low risk should be auto-approved after SetAutoApprove")
	}
	if mockReq.PromptCalled {
		t.Error("should not prompt when auto-approved")
	}
}

func TestShouldPrompt(t *testing.T) {
	mgr := New(ModeStrict, time.Minute)
	if !mgr.ShouldPrompt("any") {
		t.Error("strict mode should always prompt")
	}

	mgr.SetMode(ModeAuto)
	if mgr.ShouldPrompt("any") {
		t.Error("auto mode should not prompt")
	}

	mgr.SetMode(ModePrompt)
	if !mgr.ShouldPrompt("any") {
		t.Error("prompt mode should always prompt")
	}
}

func TestPendingActionsEmpty(t *testing.T) {
	mgr := New(ModeAuto, time.Minute)
	actions := mgr.PendingActions()
	if len(actions) != 0 {
		t.Errorf("expected 0 pending, got %d", len(actions))
	}
}

func TestPendingActionsRemovedAfterApproval(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &MockRequester{Decisions: map[string]*Decision{
		"act-1": {ActionID: "act-1", Approved: true},
	}}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "test", "", RiskHigh)
	mgr.RequestApproval(context.Background(), action)

	actions := mgr.PendingActions()
	if len(actions) != 0 {
		t.Errorf("pending should be empty after approval, got %d", len(actions))
	}
}

func TestContextCancellation(t *testing.T) {
	mgr := New(ModePrompt, time.Minute)

	mockReq := &SlowMockRequester{Delay: 5 * time.Second}
	mgr.WithRequester(mockReq)

	action := mgr.SubmitAction("act-1", "test", "", RiskHigh)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := mgr.RequestApproval(ctx, action)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestStdinRequesterYesInput(t *testing.T) {
	input := "yes\n"
	req := &StdinRequester{Reader: bufio.NewReader(strings.NewReader(input))}

	action := &Action{ID: "act-1", Description: "test", RiskLevel: RiskHigh}
	decision, err := req.Prompt(context.Background(), action)
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if !decision.Approved {
		t.Error("'yes' input should approve")
	}
}

func TestStdinRequesterYInput(t *testing.T) {
	input := "y\n"
	req := &StdinRequester{Reader: bufio.NewReader(strings.NewReader(input))}

	action := &Action{ID: "act-1", Description: "test", RiskLevel: RiskHigh}
	decision, err := req.Prompt(context.Background(), action)
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if !decision.Approved {
		t.Error("'y' input should approve")
	}
}

func TestStdinRequesterEmptyInput(t *testing.T) {
	input := "\n"
	req := &StdinRequester{Reader: bufio.NewReader(strings.NewReader(input))}

	action := &Action{ID: "act-1", Description: "test", RiskLevel: RiskHigh}
	decision, err := req.Prompt(context.Background(), action)
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if decision.Approved {
		t.Error("empty input should deny")
	}
	if decision.Comment == "" {
		t.Error("denied comment should explain reason")
	}
}

func TestStdinRequesterCommentInput(t *testing.T) {
	input := "need more info\n"
	req := &StdinRequester{Reader: bufio.NewReader(strings.NewReader(input))}

	action := &Action{ID: "act-1", Description: "test", RiskLevel: RiskHigh}
	decision, err := req.Prompt(context.Background(), action)
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}
	if decision.Approved {
		t.Error("comment input should deny")
	}
	if !strings.Contains(decision.Comment, "need more info") {
		t.Errorf("comment = %q, should contain user input", decision.Comment)
	}
}

func TestStdinRequesterContextCancellation(t *testing.T) {
	r, w, _ := os.Pipe()
	defer r.Close()
	defer w.Close()

	req := &StdinRequester{Reader: bufio.NewReader(r)}
	action := &Action{ID: "act-1", Description: "test", RiskLevel: RiskHigh}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := req.Prompt(ctx, action)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
	_ = w
}

type MockRequester struct {
	Decisions   map[string]*Decision
	PromptCalled bool
	mu          sync.Mutex
}

func (m *MockRequester) Prompt(ctx context.Context, action *Action) (*Decision, error) {
	m.mu.Lock()
	m.PromptCalled = true
	m.mu.Unlock()

	if d, ok := m.Decisions[action.ID]; ok {
		d.ActionID = action.ID
		return d, nil
	}
	return &Decision{ActionID: action.ID, Approved: true}, nil
}

type SlowMockRequester struct {
	Delay time.Duration
}

func (s *SlowMockRequester) Prompt(ctx context.Context, action *Action) (*Decision, error) {
	select {
	case <-time.After(s.Delay):
		return &Decision{ActionID: action.ID, Approved: true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
