package concurrency

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work.
type Task func(ctx context.Context) error

// Result holds the result of a task execution.
type Result struct {
	Index    int
	Error    error
	Duration time.Duration
}

// Group manages a collection of goroutines working on tasks.
type Group struct {
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	mu      sync.Mutex
	errors  []error
	results []Result
	running int32
	maxConc int
	sem     chan struct{}
}

// New creates a Group with the given context and concurrency limit.
func New(ctx context.Context, maxConcurrent int) *Group {
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}

	ctx, cancel := context.WithCancel(ctx)
	return &Group{
		ctx:     ctx,
		cancel:  cancel,
		maxConc: maxConcurrent,
		sem:     make(chan struct{}, maxConcurrent),
	}
}

// Go adds a task to the group.
func (g *Group) Go(fn Task) {
	g.wg.Add(1)
	g.sem <- struct{}{}

	go func() {
		defer g.wg.Done()
		defer func() { <-g.sem }()

		atomic.AddInt32(&g.running, 1)
		defer atomic.AddInt32(&g.running, -1)

		start := time.Now()
		err := fn(g.ctx)
		duration := time.Since(start)

		g.mu.Lock()
		g.results = append(g.results, Result{
			Index:    len(g.results),
			Error:    err,
			Duration: duration,
		})
		if err != nil {
			g.errors = append(g.errors, err)
		}
		g.mu.Unlock()
	}()
}

// Wait blocks until all tasks complete.
func (g *Group) Wait() []Result {
	g.wg.Wait()
	g.cancel()
	return g.Results()
}

// WaitWithContext waits and returns early if context is done.
func (g *Group) WaitWithContext(ctx context.Context) ([]Result, error) {
	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		g.cancel()
		return g.Results(), ctx.Err()
	case <-done:
		return g.Results(), nil
	}
}

// Results returns all task results.
func (g *Group) Results() []Result {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]Result(nil), g.results...)
}

// Errors returns all errors that occurred.
func (g *Group) Errors() []error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]error(nil), g.errors...)
}

// HasErrors checks if any task failed.
func (g *Group) HasErrors() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return len(g.errors) > 0
}

// Error returns the first error that occurred.
func (g *Group) Error() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.errors) == 0 {
		return nil
	}
	return g.errors[0]
}

// Running returns the number of currently executing tasks.
func (g *Group) Running() int {
	return int(atomic.LoadInt32(&g.running))
}

// Cancel cancels all running tasks.
func (g *Group) Cancel() {
	g.cancel()
}

// Summary returns a summary of execution.
func (g *Group) Summary() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	total := len(g.results)
	successful := total - len(g.errors)

	var totalDuration time.Duration
	for _, r := range g.results {
		totalDuration += r.Duration
	}
	avgDuration := time.Duration(0)
	if total > 0 {
		avgDuration = totalDuration / time.Duration(total)
	}

	return fmt.Sprintf("Group: %d total, %d successful, %d failed, avg duration: %v",
		total, successful, len(g.errors), avgDuration.Round(time.Millisecond))
}

// FanOut distributes work across multiple workers.
func FanOut[T any](ctx context.Context, items []T, workers int, fn func(context.Context, T) error) ([]Result, error) {
	if workers <= 0 {
		workers = 4
	}

	g := New(ctx, workers)

	for _, item := range items {
		item := item // Capture for closure
		g.Go(func(ctx context.Context) error {
			return fn(ctx, item)
		})
	}

	return g.Wait(), g.Error()
}

// FanIn collects results from multiple input channels.
func FanIn[T any](ctx context.Context, inputs []<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup

	output := func(ch <-chan T) {
		defer wg.Done()
		for v := range ch {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}

	wg.Add(len(inputs))
	for _, ch := range inputs {
		go output(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// Pipeline chains stages of processing.
func Pipeline[T any](ctx context.Context, input <-chan T, stages ...func(context.Context, T) (T, error)) <-chan T {
	current := input
	for _, stage := range stages {
		next := make(chan T)
		go func(s func(context.Context, T) (T, error), in <-chan T, out chan<- T) {
			defer close(out)
			for v := range in {
				select {
				case <-ctx.Done():
					return
				default:
					result, err := s(ctx, v)
					if err == nil {
						out <- result
					}
				}
			}
		}(stage, current, next)
		current = next
	}
	return current
}

// Semaphore limits concurrent operations.
type Semaphore struct {
	sem chan struct{}
}

// NewSemaphore creates a semaphore with the given limit.
func NewSemaphore(limit int) *Semaphore {
	return &Semaphore{
		sem: make(chan struct{}, limit),
	}
}

// Acquire blocks until a slot is available.
func (s *Semaphore) Acquire() {
	s.sem <- struct{}{}
}

// Release frees a slot.
func (s *Semaphore) Release() {
	<-s.sem
}

// TryAcquire tries to acquire without blocking.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.sem <- struct{}{}:
		return true
	default:
		return false
	}
}

// WithSemaphore wraps a function with semaphore acquisition.
func (s *Semaphore) WithSemaphore(fn func() error) error {
	s.Acquire()
	defer s.Release()
	return fn()
}

// WorkerPool manages a pool of reusable workers.
type WorkerPool struct {
	tasks    chan Task
	results  chan Result
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	workers  int
}

// NewWorkerPool creates a pool with the given number of workers.
func NewWorkerPool(ctx context.Context, workers, queueSize int) *WorkerPool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = workers * 2
	}

	ctx, cancel := context.WithCancel(ctx)
	pool := &WorkerPool{
		tasks:   make(chan Task, queueSize),
		results: make(chan Result, queueSize),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		pool.wg.Add(1)
		go pool.worker(i)
	}

	return pool
}

// Submit adds a task to the pool.
func (p *WorkerPool) Submit(fn Task) error {
	select {
	case p.tasks <- fn:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Results returns the results channel.
func (p *WorkerPool) Results() <-chan Result {
	return p.results
}

// Close shuts down the pool.
func (p *WorkerPool) Close() {
	close(p.tasks)
	p.wg.Wait()
	close(p.results)
}

// Shutdown cancels all pending tasks.
func (p *WorkerPool) Shutdown() {
	p.cancel()
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	for task := range p.tasks {
		select {
		case <-p.ctx.Done():
			return
		default:
			start := time.Now()
			err := task(p.ctx)
			p.results <- Result{
				Index:    id,
				Error:    err,
				Duration: time.Since(start),
			}
		}
	}
}

// Batch processes items in batches.
func Batch[T, R any](ctx context.Context, items []T, batchSize int, fn func(context.Context, []T) ([]R, error)) ([]R, error) {
	if batchSize <= 0 {
		batchSize = 10
	}

	var allResults []R

	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		results, err := fn(ctx, batch)
		if err != nil {
			return allResults, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// Retry executes a function with retry logic.
func Retry(ctx context.Context, maxAttempts int, delay time.Duration, fn func(context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if attempt < maxAttempts-1 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			delay *= 2 // Exponential backoff
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", maxAttempts, lastErr)
}

// Debounce delays execution until no calls have been made for the specified duration.
func Debounce(fn func(), delay time.Duration) func() {
	var mu sync.Mutex
	var timer *time.Timer

	return func() {
		mu.Lock()
		defer mu.Unlock()

		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(delay, fn)
	}
}

// Throttle limits execution to once per interval.
func Throttle(fn func(), interval time.Duration) func() {
	var mu sync.Mutex
	var lastExec time.Time

	return func() {
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		if now.Sub(lastExec) >= interval {
			fn()
			lastExec = now
		}
	}
}
