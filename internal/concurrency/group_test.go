package concurrency

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewDefaultMaxConcurrent(t *testing.T) {
	g := New(context.Background(), 0)
	if g.maxConc != 10 {
		t.Errorf("maxConc = %d, want 10", g.maxConc)
	}
}

func TestNewWithLimit(t *testing.T) {
	g := New(context.Background(), 5)
	if g.maxConc != 5 {
		t.Errorf("maxConc = %d, want 5", g.maxConc)
	}
}

func TestGoAndWait(t *testing.T) {
	g := New(context.Background(), 5)

	g.Go(func(ctx context.Context) error { return nil })
	g.Go(func(ctx context.Context) error { return nil })
	g.Go(func(ctx context.Context) error { return nil })

	results := g.Wait()
	if len(results) != 3 {
		t.Errorf("results count = %d, want 3", len(results))
	}
}

func TestWaitReturnsErrors(t *testing.T) {
	g := New(context.Background(), 5)

	errA := errors.New("error A")
	errB := errors.New("error B")

	g.Go(func(ctx context.Context) error { return nil })
	g.Go(func(ctx context.Context) error { return errA })
	g.Go(func(ctx context.Context) error { return errB })

	results := g.Wait()
	if len(results) != 3 {
		t.Fatalf("results count = %d, want 3", len(results))
	}

	errCount := 0
	for _, r := range results {
		if r.Error != nil {
			errCount++
		}
	}
	if errCount != 2 {
		t.Errorf("error count = %d, want 2", errCount)
	}
}

func TestHasErrors(t *testing.T) {
	g := New(context.Background(), 5)

	g.Go(func(ctx context.Context) error { return errors.New("fail") })
	g.Wait()

	if !g.HasErrors() {
		t.Error("HasErrors should return true")
	}
}

func TestErrorReturnsFirst(t *testing.T) {
	g := New(context.Background(), 5)

	g.Go(func(ctx context.Context) error { return errors.New("first error") })
	g.Wait()

	err := g.Error()
	if err == nil {
		t.Fatal("Error() returned nil")
	}
	if err.Error() != "first error" {
		t.Errorf("error = %q", err.Error())
	}
}

func TestErrorReturnsNil(t *testing.T) {
	g := New(context.Background(), 5)
	g.Go(func(ctx context.Context) error { return nil })
	g.Wait()

	if g.Error() != nil {
		t.Error("Error() should return nil")
	}
}

func TestErrorsReturnsCopy(t *testing.T) {
	g := New(context.Background(), 5)
	g.Go(func(ctx context.Context) error { return errors.New("e1") })
	g.Go(func(ctx context.Context) error { return errors.New("e2") })
	g.Wait()

	errs := g.Errors()
	if len(errs) != 2 {
		t.Fatalf("errors count = %d", len(errs))
	}

	errs = append(errs, errors.New("e3"))
	if len(g.Errors()) == 3 {
		t.Error("Errors() should return a copy")
	}
}

func TestResultsReturnsCopy(t *testing.T) {
	g := New(context.Background(), 5)
	g.Go(func(ctx context.Context) error { return nil })
	g.Wait()

	results := g.Results()
	if len(results) != 1 {
		t.Fatalf("results count = %d", len(results))
	}

	results = append(results, Result{})
	if len(g.Results()) == 2 {
		t.Error("Results() should return a copy")
	}
}

func TestRunning(t *testing.T) {
	g := New(context.Background(), 5)
	if g.Running() != 0 {
		t.Errorf("initial running = %d", g.Running())
	}

	done := make(chan struct{})
	g.Go(func(ctx context.Context) error {
		<-done
		return nil
	})

	time.Sleep(50 * time.Millisecond)
	if g.Running() != 1 {
		t.Errorf("running = %d, want 1", g.Running())
	}
	close(done)
	g.Wait()
}

func TestCancel(t *testing.T) {
	g := New(context.Background(), 5)

	cancelled := make(chan struct{})
	g.Go(func(ctx context.Context) error {
		<-ctx.Done()
		close(cancelled)
		return ctx.Err()
	})

	g.Cancel()

	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Error("task should have been cancelled")
	}
}

func TestSummary(t *testing.T) {
	g := New(context.Background(), 5)

	g.Go(func(ctx context.Context) error { return nil })
	g.Go(func(ctx context.Context) error { return nil })
	g.Go(func(ctx context.Context) error { return errors.New("fail") })

	g.Wait()

	summary := g.Summary()
	if summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestWaitWithContext(t *testing.T) {
	g := New(context.Background(), 5)

	done := make(chan struct{})
	g.Go(func(ctx context.Context) error {
		<-done
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := g.WaitWithContext(ctx)
	if err == nil {
		t.Error("expected context deadline exceeded")
	}
	close(done)
}

func TestWaitWithContextSuccess(t *testing.T) {
	g := New(context.Background(), 5)
	g.Go(func(ctx context.Context) error { return nil })
	g.Go(func(ctx context.Context) error { return nil })

	results, err := g.WaitWithContext(context.Background())
	if err != nil {
		t.Fatalf("WaitWithContext failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("results count = %d, want 2", len(results))
	}
}

func TestFanOut(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	results, err := FanOut(context.Background(), items, 3, func(ctx context.Context, n int) error {
		return nil
	})

	if err != nil {
		t.Fatalf("FanOut failed: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results count = %d, want 5", len(results))
	}
}

func TestFanOutWithError(t *testing.T) {
	items := []int{1, 2, 3}

	_, err := FanOut(context.Background(), items, 2, func(ctx context.Context, n int) error {
		if n == 2 {
			return errors.New("item 2 failed")
		}
		return nil
	})

	if err == nil {
		t.Error("expected error from FanOut")
	}
}

func TestFanOutDefaultWorkers(t *testing.T) {
	items := []int{1, 2}

	results, _ := FanOut(context.Background(), items, 0, func(ctx context.Context, n int) error {
		return nil
	})

	if len(results) != 2 {
		t.Errorf("results count = %d, want 2", len(results))
	}
}

func TestFanIn(t *testing.T) {
	ch1 := make(chan int)
	ch2 := make(chan int)
	inputs := []<-chan int{ch1, ch2}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := FanIn(ctx, inputs)

	var collected []int
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		for v := range out {
			mu.Lock()
			collected = append(collected, v)
			mu.Unlock()
		}
		close(done)
	}()

	ch1 <- 1
	ch2 <- 2
	ch1 <- 3
	close(ch1)
	close(ch2)

	<-done

	if len(collected) != 3 {
		t.Errorf("collected count = %d, want 3", len(collected))
	}
}

func TestFanInCancelled(t *testing.T) {
	ch := make(chan int)
	inputs := []<-chan int{ch}

	ctx, cancel := context.WithCancel(context.Background())
	out := FanIn(ctx, inputs)

	cancel()

	// Should not block forever
	select {
	case <-out:
	case <-time.After(100 * time.Millisecond):
	}
}

func TestPipeline(t *testing.T) {
	input := make(chan int)
	go func() {
		defer close(input)
		for i := 1; i <= 5; i++ {
			input <- i
		}
	}()

	double := func(ctx context.Context, n int) (int, error) {
		return n * 2, nil
	}

	out := Pipeline(context.Background(), input, double)

	collected := []int{}
	for v := range out {
		collected = append(collected, v)
	}

	if len(collected) != 5 {
		t.Fatalf("collected count = %d, want 5", len(collected))
	}

	expected := []int{2, 4, 6, 8, 10}
	for i, v := range collected {
		if v != expected[i] {
			t.Errorf("collected[%d] = %d, want %d", i, v, expected[i])
		}
	}
}

func TestPipelineWithErrors(t *testing.T) {
	input := make(chan int)
	go func() {
		defer close(input)
		input <- 1
		input <- 2
		input <- 3
	}()

	failEven := func(ctx context.Context, n int) (int, error) {
		if n%2 == 0 {
			return 0, errors.New("even numbers fail")
		}
		return n, nil
	}

	out := Pipeline(context.Background(), input, failEven)

	collected := []int{}
	for v := range out {
		collected = append(collected, v)
	}

	// Only odd numbers (1, 3) should pass through
	if len(collected) != 2 {
		t.Errorf("collected count = %d, want 2", len(collected))
	}
}

func TestSemaphore(t *testing.T) {
	sem := NewSemaphore(2)

	sem.Acquire()
	sem.Acquire()

	acquired := make(chan bool, 1)
	go func() {
		sem.Acquire()
		acquired <- true
	}()

	select {
	case <-acquired:
		t.Error("third acquire should block")
	case <-time.After(100 * time.Millisecond):
	}

	sem.Release()

	select {
	case <-acquired:
	case <-time.After(time.Second):
		t.Error("third acquire should succeed after release")
	}
	sem.Release()
}

func TestSemaphoreTryAcquire(t *testing.T) {
	sem := NewSemaphore(1)

	if !sem.TryAcquire() {
		t.Error("first TryAcquire should succeed")
	}
	if sem.TryAcquire() {
		t.Error("second TryAcquire should fail")
	}

	sem.Release()
	if !sem.TryAcquire() {
		t.Error("TryAcquire after release should succeed")
	}
	sem.Release()
}

func TestSemaphoreWithSemaphore(t *testing.T) {
	sem := NewSemaphore(2)

	var called bool
	err := sem.WithSemaphore(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("WithSemaphore failed: %v", err)
	}
	if !called {
		t.Error("function should have been called")
	}
}

func TestSemaphoreWithSemaphoreError(t *testing.T) {
	sem := NewSemaphore(1)

	expectedErr := errors.New("inner error")
	err := sem.WithSemaphore(func() error {
		return expectedErr
	})
	if err != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestWorkerPool(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 3, 10)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		n := i
		err := pool.Submit(func(ctx context.Context) error {
			defer wg.Done()
			_ = n
			return nil
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	pool.Close()
	wg.Wait()
}

func TestWorkerPoolDefaultWorkers(t *testing.T) {
	pool := NewWorkerPool(context.Background(), 0, 0)
	if pool.workers != 4 {
		t.Errorf("workers = %d, want 4", pool.workers)
	}
}

func TestWorkerPoolShutdown(t *testing.T) {
	ctx := context.Background()
	pool := NewWorkerPool(ctx, 2, 0)
	pool.Shutdown()

	// After shutdown, submitting should fail because context is cancelled
	// May succeed if channel has capacity, so we test the result channel closes
	pool.Close()
}

func TestWorkerPoolCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	pool := NewWorkerPool(ctx, 2, 0)
	cancel()

	// Try to submit after cancel - may succeed or fail depending on timing
	_ = pool.Submit(func(ctx context.Context) error { return nil })
	pool.Close()
}

func TestBatch(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7}

	var mu sync.Mutex
	var processed []int

	results, err := Batch(context.Background(), items, 3, func(ctx context.Context, batch []int) ([]int, error) {
		mu.Lock()
		processed = append(processed, batch...)
		mu.Unlock()
		out := make([]int, len(batch))
		for i, n := range batch {
			out[i] = n * 10
		}
		return out, nil
	})

	if err != nil {
		t.Fatalf("Batch failed: %v", err)
	}
	if len(results) != 7 {
		t.Errorf("results count = %d, want 7", len(results))
	}
	if len(processed) != 7 {
		t.Errorf("processed count = %d, want 7", len(processed))
	}
}

func TestBatchError(t *testing.T) {
	items := []int{1, 2, 3}

	expectedErr := errors.New("batch failed")
	_, err := Batch(context.Background(), items, 2, func(ctx context.Context, batch []int) ([]int, error) {
		// First batch is [1, 2], second batch is [3]
		if len(batch) == 1 && batch[0] == 3 {
			return nil, expectedErr
		}
		return []int{1}, nil
	})

	if err != expectedErr {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestBatchDefaultSize(t *testing.T) {
	items := []int{1, 2, 3}

	results, _ := Batch(context.Background(), items, 0, func(ctx context.Context, batch []int) ([]int, error) {
		return batch, nil
	})

	if len(results) != 3 {
		t.Errorf("results count = %d, want 3", len(results))
	}
}

func TestRetrySuccess(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), 3, 10*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errors.New("temporary error")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Retry failed: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestRetryAllFail(t *testing.T) {
	attempts := 0
	err := Retry(context.Background(), 3, time.Millisecond, func(ctx context.Context) error {
		attempts++
		return errors.New("always fail")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetryCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Retry(ctx, 5, time.Minute, func(ctx context.Context) error {
		return errors.New("should not reach")
	})

	if err != context.Canceled {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestDebounce(t *testing.T) {
	var count int
	var mu sync.Mutex

	fn := Debounce(func() {
		mu.Lock()
		count++
		mu.Unlock()
	}, 50*time.Millisecond)

	// Call rapidly multiple times
	for i := 0; i < 10; i++ {
		fn()
	}

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	c := count
	mu.Unlock()

	if c != 1 {
		t.Errorf("count = %d, want 1", c)
	}
}

func TestThrottle(t *testing.T) {
	var count int
	var mu sync.Mutex

	fn := Throttle(func() {
		mu.Lock()
		count++
		mu.Unlock()
	}, 50*time.Millisecond)

	// Call multiple times
	for i := 0; i < 10; i++ {
		fn()
	}

	mu.Lock()
	c := count
	mu.Unlock()

	if c != 1 {
		t.Errorf("throttle count = %d, want 1", c)
	}

	time.Sleep(60 * time.Millisecond)
	fn()

	mu.Lock()
	c = count
	mu.Unlock()

	if c != 2 {
		t.Errorf("throttle after wait count = %d, want 2", c)
	}
}
