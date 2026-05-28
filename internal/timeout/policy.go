package timeout

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Scope defines where a timeout applies.
type Scope string

const (
	ScopeTask     Scope = "task"
	ScopePhase    Scope = "phase"
	ScopeAgent    Scope = "agent"
	ScopeToolCall Scope = "tool_call"
	ScopeLLM      Scope = "llm_request"
)

// Policy defines timeout rules for different scopes.
type Policy struct {
	mu sync.RWMutex
	rules map[Scope]time.Duration
}

// DefaultPolicy returns a policy with sensible defaults.
func DefaultPolicy() *Policy {
	return &Policy{
		rules: map[Scope]time.Duration{
			ScopeTask:     30 * time.Minute,
			ScopePhase:    10 * time.Minute,
			ScopeAgent:    5 * time.Minute,
			ScopeToolCall: 30 * time.Second,
			ScopeLLM:      60 * time.Second,
		},
	}
}

// New creates an empty timeout policy.
func New() *Policy {
	return &Policy{
		rules: make(map[Scope]time.Duration),
	}
}

// Set configures a timeout for a specific scope.
func (p *Policy) Set(scope Scope, duration time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rules[scope] = duration
}

// Get returns the timeout for a scope.
func (p *Policy) Get(scope Scope) (time.Duration, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	d, ok := p.rules[scope]
	return d, ok
}

// WithTimeout returns a context with timeout applied for the given scope.
func (p *Policy) WithTimeout(parent context.Context, scope Scope) (context.Context, context.CancelFunc) {
	p.mu.RLock()
	duration, ok := p.rules[scope]
	p.mu.RUnlock()

	if !ok {
		return parent, func() {}
	}

	return context.WithTimeout(parent, duration)
}

// WithDeadline returns a context with an absolute deadline.
func (p *Policy) WithDeadline(parent context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	return context.WithDeadline(parent, deadline)
}

// Watchdog monitors operations and cancels them if they exceed their timeout.
type Watchdog struct {
	mu       sync.Mutex
	monitors map[string]*monitor
	policy   *Policy
	onTimeout func(id string, scope Scope, elapsed time.Duration)
}

type monitor struct {
	id        string
	scope     Scope
	startTime time.Time
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewWatchdog creates a watchdog with the given policy.
func NewWatchdog(policy *Policy) *Watchdog {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Watchdog{
		monitors: make(map[string]*monitor),
		policy:   policy,
	}
}

// Start begins monitoring an operation.
func (w *Watchdog) Start(id string, scope Scope) context.Context {
	w.mu.Lock()
	defer w.mu.Unlock()

	ctx, cancel := w.policy.WithTimeout(context.Background(), scope)

	m := &monitor{
		id:        id,
		scope:     scope,
		startTime: time.Now(),
		cancel:    cancel,
		done:      make(chan struct{}),
	}

	w.monitors[id] = m

	go w.watch(m)

	return ctx
}

// Stop stops monitoring an operation.
func (w *Watchdog) Stop(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if m, exists := w.monitors[id]; exists {
		m.cancel()
		close(m.done)
		delete(w.monitors, id)
	}
}

// OnTimeout sets a callback for when a timeout occurs.
func (w *Watchdog) OnTimeout(fn func(id string, scope Scope, elapsed time.Duration)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onTimeout = fn
}

// ActiveCount returns the number of actively monitored operations.
func (w *Watchdog) ActiveCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.monitors)
}

// Clear stops all monitors.
func (w *Watchdog) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, m := range w.monitors {
		m.cancel()
		close(m.done)
	}
	w.monitors = make(map[string]*monitor)
}

func (w *Watchdog) watch(m *monitor) {
	timeout, _ := w.policy.Get(m.scope)
	if timeout == 0 {
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			elapsed := time.Since(m.startTime)
			if elapsed > timeout {
				w.mu.Lock()
				fn := w.onTimeout
				w.mu.Unlock()

				if fn != nil {
					fn(m.id, m.scope, elapsed)
				}
				m.cancel()
				return
			}
		}
	}
}

// Guarded executes a function with a timeout applied.
func Guarded(ctx context.Context, scope Scope, timeout time.Duration, fn func(context.Context) error) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result := make(chan error, 1)
	go func() {
		result <- fn(ctx)
	}()

	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return fmt.Errorf("operation %q timed out after %v: %w", scope, timeout, ctx.Err())
	}
}

// DeadlineCalculator computes deadlines for phased execution.
type DeadlineCalculator struct {
	startTime    time.Time
	totalBudget  time.Duration
	phaseWeights map[Scope]float64
}

// NewDeadlineCalculator creates a calculator with equal weights.
func NewDeadlineCalculator(totalBudget time.Duration, phases []Scope) *DeadlineCalculator {
	dc := &DeadlineCalculator{
		startTime:    time.Now(),
		totalBudget:  totalBudget,
		phaseWeights: make(map[Scope]float64),
	}

	weight := 1.0 / float64(len(phases))
	for _, phase := range phases {
		dc.phaseWeights[phase] = weight
	}

	return dc
}

// WithWeight sets a custom weight for a phase.
func (dc *DeadlineCalculator) WithWeight(scope Scope, weight float64) {
	dc.phaseWeights[scope] = weight
}

// DeadlineFor returns the deadline for a specific phase.
func (dc *DeadlineCalculator) DeadlineFor(scope Scope) time.Time {
	weight, exists := dc.phaseWeights[scope]
	if !exists {
		return dc.startTime.Add(dc.totalBudget)
	}

	totalWeight := 0.0
	for _, w := range dc.phaseWeights {
		totalWeight += w
	}

	phaseDuration := time.Duration(float64(dc.totalBudget) * weight / totalWeight)
	return dc.startTime.Add(phaseDuration)
}

// Remaining returns the time remaining in the total budget.
func (dc *DeadlineCalculator) Remaining() time.Duration {
	elapsed := time.Since(dc.startTime)
	remaining := dc.totalBudget - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

// IsExpired checks if the total budget has been exhausted.
func (dc *DeadlineCalculator) IsExpired() bool {
	return time.Since(dc.startTime) >= dc.totalBudget
}
