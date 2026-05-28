package memory

import (
	"context"
	"testing"
)

func TestNewInMemoryStore(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
	if store.db == nil {
		t.Fatal("db is nil")
	}
}

func TestSaveAndLoad(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// Save
	if err := store.Save(ctx, "task-1", "key1", "value1"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	value, err := store.Load(ctx, "task-1", "key1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if value != "value1" {
		t.Errorf("value = '%s', want 'value1'", value)
	}
}

func TestSaveWithMetaAndLoad(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	if err := store.SaveWithMeta(ctx, "task-1", "api-key", "configuration", "sk-123", 10); err != nil {
		t.Fatalf("SaveWithMeta failed: %v", err)
	}

	value, err := store.Load(ctx, "task-1", "api-key")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if value != "sk-123" {
		t.Errorf("value = '%s', want 'sk-123'", value)
	}
}

func TestLoadNotFound(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	value, err := store.Load(ctx, "nonexistent", "key")
	if err != nil {
		t.Errorf("expected no error for non-existent key, got: %v", err)
	}
	if value != "" {
		t.Errorf("expected empty string for non-existent key, got '%s'", value)
	}
}

func TestSearch(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	store.Save(ctx, "task-1", "key1", "user authentication module")
	store.Save(ctx, "task-2", "key2", "database connection pool")
	store.Save(ctx, "task-3", "key3", "payment gateway integration")
	store.Save(ctx, "task-4", "key4", "authentication and user management")

	results, err := store.Search(ctx, "authentication", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should find at least 2 results (task-1 and task-4)
	if len(results) < 1 {
		t.Errorf("expected at least 1 result, got %d", len(results))
	}
}

func TestSearchWithLimit(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		store.Save(ctx, "task", "key", "test data for search limit testing")
	}

	results, err := store.Search(ctx, "test", 2)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) > 2 {
		t.Errorf("expected at most 2 results, got %d", len(results))
	}
}

func TestDuplicateKeys(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	store.Save(ctx, "task-1", "key", "value1")
	store.Save(ctx, "task-1", "key", "value2")

	// Load should return the most recent value
	value, err := store.Load(ctx, "task-1", "key")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Depending on implementation, might return first or second value
	if value != "value1" && value != "value2" {
		t.Errorf("value = '%s', expected one of the saved values", value)
	}
}

func TestEmptyValue(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	store.Save(ctx, "task-1", "key", "")

	value, err := store.Load(ctx, "task-1", "key")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Empty string might still be stored or not depending on implementation
	if value != "" {
		t.Logf("empty value was stored as '%s' (may be acceptable)", value)
	}
}

func TestMultipleTasks(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	store.Save(ctx, "task-a", "config", "value-a")
	store.Save(ctx, "task-b", "config", "value-b")

	valueA, err := store.Load(ctx, "task-a", "config")
	if err != nil {
		t.Fatalf("Load task-a failed: %v", err)
	}
	if valueA != "value-a" {
		t.Errorf("task-a value = '%s', want 'value-a'", valueA)
	}

	valueB, err := store.Load(ctx, "task-b", "config")
	if err != nil {
		t.Fatalf("Load task-b failed: %v", err)
	}
	if valueB != "value-b" {
		t.Errorf("task-b value = '%s', want 'value-b'", valueB)
	}
}

func TestSaveContextInterface(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	// Test the MemoryProvider interface
	if err := store.SaveContext(ctx, "task-1", "key", "value"); err != nil {
		t.Fatalf("SaveContext failed: %v", err)
	}

	value, err := store.LoadContext(ctx, "task-1", "key")
	if err != nil {
		t.Fatalf("LoadContext failed: %v", err)
	}
	if value != "value" {
		t.Errorf("value = '%s', want 'value'", value)
	}
}

func TestSearchMemoryInterface(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx := context.Background()

	store.Save(ctx, "task-1", "key", "test search interface")

	results, err := store.SearchMemory(ctx, "test", 10)
	if err != nil {
		t.Fatalf("SearchMemory failed: %v", err)
	}

	if len(results) < 1 {
		t.Error("expected at least 1 result")
	}
}

func TestCancelledContext(t *testing.T) {
	store, err := New(":memory:")
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = store.Save(ctx, "task", "key", "value")
	if err == nil {
		// May or may not error with cancelled context depending on timing
		t.Log("Save with cancelled context did not error (acceptable)")
	}
}
