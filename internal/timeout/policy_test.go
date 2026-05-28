package timeout

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPolicySetGet(t *testing.T) {
	p := New()

	p.Set(ScopeTask, 5*time.Minute)
	d, ok := p.Get(ScopeTask)
	if !ok {
		t.Error("expected ScopeTask to exist")
	}
	if d != 5*time.Minute {
		t.Errorf("expected 5m, got %v", d)
	}

	_, ok = p.Get(ScopeAgent)
	if ok {
		t.Error("expected ScopeAgent to not exist")
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()

	d, ok := p.Get(ScopeTask)
	if !ok || d != 30*time.Minute {
		t.Errorf("default task timeout: got %v, ok=%v", d, ok)
	}

	d, ok = p.Get(ScopeLLM)
	if !ok || d != 60*time.Second {
		t.Errorf("default LLM timeout: got %v, ok=%v", d, ok)
	}
}

func TestPolicyWithTimeout(t *testing.T) {
	p := New()
	p.Set(ScopeTask, 50*time.Millisecond)

	ctx, cancel := p.WithTimeout(context.Background(), ScopeTask)
	defer cancel()

	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Error("context should have been cancelled")
	}
}

func TestPolicyWithTimeoutUnknownScope(t *testing.T) {
	p := New()

	ctx, cancel := p.WithTimeout(context.Background(), ScopeTask)
	defer cancel()

	select {
	case <-ctx.Done():
		t.Error("context should not be done for unknown scope")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestWatchdogStartStop(t *testing.T) {
	p := DefaultPolicy()
	w := NewWatchdog(p)

	ctx := w.Start("op-1", ScopeToolCall)
	if ctx == nil {
		t.Fatal("Start returned nil context")
	}

	if w.ActiveCount() != 1 {
		t.Errorf("expected 1 active monitor, got %d", w.ActiveCount())
	}

	w.Stop("op-1")

	if w.ActiveCount() != 0 {
		t.Errorf("expected 0 active monitors after Stop, got %d", w.ActiveCount())
	}
}

func TestWatchdogFireTimeout(t *testing.T) {
	p := New()
	p.Set(ScopeToolCall, 100*time.Millisecond)

	var (
		mu         sync.Mutex
		firedID    string
		firedScope Scope
		fired      bool
	)

	w := NewWatchdog(p)
	w.OnTimeout(func(id string, scope Scope, elapsed time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		firedID = id
		firedScope = scope
		fired = true
	})

	ctx := w.Start("timed-op", ScopeToolCall)

	// The watchdog ticker fires every 1 second, so we need to wait long enough
	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("context should have been cancelled by watchdog")
	}

	// Wait for the watchdog ticker (1s interval) to detect the timeout and invoke callback
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !fired {
		t.Fatal("onTimeout callback was not invoked")
	}
	if firedID != "timed-op" {
		t.Errorf("expected firedID=timed-op, got %s", firedID)
	}
	if firedScope != ScopeToolCall {
		t.Errorf("expected scope=ScopeToolCall, got %v", firedScope)
	}
	
	w.Clear()
}

func TestWatchdogClear(t *testing.T) {
	p := DefaultPolicy()
	w := NewWatchdog(p)

	w.Start("a", ScopeAgent)
	w.Start("b", ScopeAgent)
	w.Start("c", ScopeAgent)

	if w.ActiveCount() != 3 {
		t.Errorf("expected 3 active monitors, got %d", w.ActiveCount())
	}

	w.Clear()

	if w.ActiveCount() != 0 {
		t.Errorf("expected 0 after Clear, got %d", w.ActiveCount())
	}
}

func TestWatchdogOnTimeoutCallbackOutsideLock(t *testing.T) {
	p := New()
	p.Set(ScopeToolCall, 100*time.Millisecond)

	var callbackMu sync.Mutex
	callbackBlocked := make(chan struct{})
	callbackCanProceed := make(chan struct{})

	w := NewWatchdog(p)
	w.OnTimeout(func(id string, scope Scope, elapsed time.Duration) {
		callbackMu.Lock()
		callbackBlocked <- struct{}{}
		<-callbackCanProceed
		callbackMu.Unlock()
	})

	ctx := w.Start("lock-test", ScopeToolCall)
	<-ctx.Done()
	<-callbackBlocked

	// If the callback runs under the watchdog lock, calling Stop() would deadlock.
	// Use a timer to verify Stop returns promptly.
	done := make(chan struct{})
	go func() {
		w.Stop("lock-test")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Stop() appears to be deadlocked - callback likely invoked under lock")
	}

	close(callbackCanProceed)
	w.Clear()
}

func TestGuardedSuccess(t *testing.T) {
	err := Guarded(context.Background(), "test", 5*time.Second, func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestGuardedReturnsError(t *testing.T) {
	expected := context.Canceled
	err := Guarded(context.Background(), "test", 5*time.Second, func(ctx context.Context) error {
		return expected
	})
	if err != expected {
		t.Errorf("expected %v, got %v", expected, err)
	}
}

func TestGuardedTimeout(t *testing.T) {
	err := Guarded(context.Background(), ScopeToolCall, 50*time.Millisecond, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestDeadlineCalculatorEqualWeights(t *testing.T) {
	phases := []Scope{ScopeTask, ScopePhase, ScopeAgent}
	dc := NewDeadlineCalculator(30*time.Minute, phases)

	// All phases should have equal weight
	for _, s := range phases {
		d := dc.DeadlineFor(s)
		if d.IsZero() {
			t.Errorf("deadline for %v is zero", s)
		}
	}
}

func TestDeadlineCalculatorWeighted(t *testing.T) {
	dc := NewDeadlineCalculator(100*time.Second, []Scope{ScopeTask, ScopePhase})
	dc.WithWeight(ScopeTask, 0.7)
	dc.WithWeight(ScopePhase, 0.3)

	taskDeadline := dc.DeadlineFor(ScopeTask)
	phaseDeadline := dc.DeadlineFor(ScopePhase)

	// Task has higher weight, so its deadline should be further out
	if !taskDeadline.After(phaseDeadline) {
		t.Errorf("task deadline %v should be after phase deadline %v", taskDeadline, phaseDeadline)
	}
}

func TestDeadlineCalculatorUnknownPhase(t *testing.T) {
	dc := NewDeadlineCalculator(30*time.Minute, []Scope{ScopeTask})

	d := dc.DeadlineFor(ScopeAgent)
	total := dc.startTime.Add(dc.totalBudget)
	if !d.Equal(total) {
		t.Errorf("expected unknown phase deadline to equal total budget end")
	}
}

func TestDeadlineCalculatorRemaining(t *testing.T) {
	dc := NewDeadlineCalculator(10*time.Second, []Scope{ScopeTask})

	rem := dc.Remaining()
	if rem > 10*time.Second {
		t.Errorf("remaining %v should be <= 10s", rem)
	}
	if rem <= 0 {
		t.Errorf("remaining should be positive, got %v", rem)
	}
}

func TestDeadlineCalculatorIsExpired(t *testing.T) {
	dc := NewDeadlineCalculator(1*time.Millisecond, []Scope{ScopeTask})

	time.Sleep(5 * time.Millisecond)

	if !dc.IsExpired() {
		t.Error("calculator should be expired")
	}
}

func TestWatchdogNilPolicy(t *testing.T) {
	w := NewWatchdog(nil)
	if w.policy == nil {
		t.Fatal("expected default policy when nil is passed")
	}
}

func TestWatchdogStopNonExistent(t *testing.T) {
	w := NewWatchdog(DefaultPolicy())

	w.Stop("does-not-exist")

	if w.ActiveCount() != 0 {
		t.Error("stopping non-existent monitor should not affect count")
	}
}
