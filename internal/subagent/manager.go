package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"autodev/internal/agents"
	"autodev/internal/core"
)

// Task describes a unit of work for a subagent.
type Task struct {
	ID          string
	Name        string
	Description string
	Executor    *agents.Executor
	Input       string
	Timeout     time.Duration
	Priority    int
}

// Result holds the output of a completed subagent task.
type Result struct {
	TaskID    string
	Output    string
	Error     error
	Duration  time.Duration
	Tokens    int
	Completed bool
}

// Manager coordinates multiple subagents executing tasks concurrently or sequentially.
type Manager struct {
	maxConcurrency int
	timeout        time.Duration
	results        sync.Map
}

// New creates a SubAgent manager.
func New(maxConcurrency int, globalTimeout time.Duration) *Manager {
	if maxConcurrency <= 0 {
		maxConcurrency = 4
	}
	if globalTimeout <= 0 {
		globalTimeout = 5 * time.Minute
	}
	return &Manager{
		maxConcurrency: maxConcurrency,
		timeout:        globalTimeout,
	}
}

// Execute runs a single subagent task synchronously.
func (m *Manager) Execute(ctx context.Context, task *Task) (*Result, error) {
	start := time.Now()

	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	input := core.Input{TaskDescription: task.Input}
	output, err := task.Executor.Execute(ctx, input)
	duration := time.Since(start)

	result := &Result{
		TaskID:    task.ID,
		Output:    output.Message,
		Error:     err,
		Duration:  duration,
		Completed: err == nil,
	}

	m.results.Store(task.ID, result)
	return result, err
}

// ExecuteConcurrent runs multiple tasks concurrently up to maxConcurrency limit.
func (m *Manager) ExecuteConcurrent(ctx context.Context, tasks []*Task) ([]*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	sem := make(chan struct{}, m.maxConcurrency)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []*Result
	)

	for _, task := range tasks {
		sem <- struct{}{}
		wg.Add(1)

		go func(t *Task) {
			defer wg.Done()
			defer func() { <-sem }()

			result, err := m.Execute(ctx, t)
			if err != nil && result != nil {
				result.Error = err
				result.Completed = false
			}
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(task)
	}

	wg.Wait()

	// Build priority map from tasks
	priorityMap := make(map[string]int)
	for _, t := range tasks {
		priorityMap[t.ID] = t.Priority
	}

	// Sort results by priority (higher priority first)
	sortByPriority(results, priorityMap)

	return results, nil
}

// ExecuteSequential runs tasks one after another in order.
func (m *Manager) ExecuteSequential(ctx context.Context, tasks []*Task, stopOnError bool) ([]*Result, error) {
	var results []*Result

	for _, task := range tasks {
		result, err := m.Execute(ctx, task)
		results = append(results, result)

		if stopOnError && err != nil {
			return results, fmt.Errorf("task %s failed: %w", task.ID, err)
		}
	}

	return results, nil
}

// GetResult retrieves the result of a specific task.
func (m *Manager) GetResult(taskID string) (*Result, bool) {
	val, ok := m.results.Load(taskID)
	if !ok {
		return nil, false
	}
	result, ok := val.(*Result)
	return result, ok
}

// GetAllResults returns all completed results.
func (m *Manager) GetAllResults() []*Result {
	var results []*Result
	m.results.Range(func(_, value interface{}) bool {
		if r, ok := value.(*Result); ok {
			results = append(results, r)
		}
		return true
	})
	return results
}

// Summary returns a summary of all results.
func (m *Manager) Summary() string {
	var completed, failed int
	var totalDuration time.Duration

	m.results.Range(func(_, value interface{}) bool {
		if r, ok := value.(*Result); ok {
			if r.Completed {
				completed++
			} else {
				failed++
			}
			totalDuration += r.Duration
		}
		return true
	})

	return fmt.Sprintf("SubAgent Summary: %d completed, %d failed, total duration: %v",
		completed, failed, totalDuration)
}

// Reset clears all results.
func (m *Manager) Reset() {
	m.results = sync.Map{}
}

// SpawnSubTask creates a new subtask from the parent task context.
func (m *Manager) SpawnSubTask(parentID, name, description string, executor *agents.Executor) *Task {
	return &Task{
		ID:          fmt.Sprintf("%s.%s", parentID, name),
		Name:        name,
		Description: description,
		Executor:    executor,
		Timeout:     m.timeout / 3,
	}
}

// MergeResults combines multiple subagent outputs into a single context string.
func MergeResults(results []*Result) string {
	var output string
	for _, r := range results {
		if r.Completed {
			output += fmt.Sprintf("\n=== Result from %s (took %v) ===\n%s\n",
				r.TaskID, r.Duration, r.Output)
		} else {
			output += fmt.Sprintf("\n=== Task %s FAILED: %v ===\n",
				r.TaskID, r.Error)
		}
	}
	return output
}

// sortByPriority orders results by task priority descending.
func sortByPriority(results []*Result, priorityMap map[string]int) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			priA := priorityMap[results[i].TaskID]
			priB := priorityMap[results[j].TaskID]
			if priA < priB {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// IsComplete checks if all tasks finished.
func (m *Manager) IsComplete(expectedTasks int) bool {
	count := 0
	m.results.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count >= expectedTasks
}

// WaitForCompletion blocks until all expected tasks complete or timeout.
func (m *Manager) WaitForCompletion(ctx context.Context, expectedTasks int) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if m.IsComplete(expectedTasks) {
				return nil
			}
		}
	}
}
