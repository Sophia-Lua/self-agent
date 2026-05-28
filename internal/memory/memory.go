package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"autodev/internal/core"

	_ "github.com/mattn/go-sqlite3"
)

// Store implements the persistence of agent memory and history with FTS5 support.
type Store struct {
	db *sql.DB
}

// New creates a new in-memory or file-based SQLite store with full-text search.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrent performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := initSchema(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func initSchema(db *sql.DB) error {
	// Core memory table
	query := `
	CREATE TABLE IF NOT EXISTS memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		category TEXT DEFAULT '',
		priority INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_memory_task ON memory(task_id);
	CREATE INDEX IF NOT EXISTS idx_memory_key ON memory(key);
	CREATE INDEX IF NOT EXISTS idx_memory_category ON memory(category);
	CREATE INDEX IF NOT EXISTS idx_memory_created ON memory(created_at);
	`
	if _, err := db.Exec(query); err != nil {
		return err
	}

	// Attempt FTS5 virtual table; degrade gracefully if module unavailable
	if _, err := db.Exec("CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(value, content='memory', content_rowid='id', tokenize='porter')"); err != nil {
		// FTS5 not available; triggers will fail silently but LIKE fallback still works
		return nil
	}

	triggers := `
	CREATE TRIGGER IF NOT EXISTS memory_ai AFTER INSERT ON memory BEGIN
		INSERT INTO memory_fts(rowid, value) VALUES (new.id, new.value);
	END;

	CREATE TRIGGER IF NOT EXISTS memory_ad AFTER DELETE ON memory BEGIN
		INSERT INTO memory_fts(memory_fts, rowid, value) VALUES('delete', old.id, old.value);
	END;

	CREATE TRIGGER IF NOT EXISTS memory_au AFTER UPDATE ON memory BEGIN
		INSERT INTO memory_fts(memory_fts, rowid, value) VALUES('delete', old.id, old.value);
		INSERT INTO memory_fts(rowid, value) VALUES (new.id, new.value);
	END;
	`
	_, err := db.Exec(triggers)
	return err
}

// SaveContext stores a key-value pair associated with a task (implements MemoryProvider).
func (s *Store) SaveContext(ctx context.Context, taskID, key, value string) error {
	return s.Save(ctx, taskID, key, value)
}

// LoadContext retrieves a value by task and key (implements MemoryProvider).
func (s *Store) LoadContext(ctx context.Context, taskID, key string) (string, error) {
	return s.Load(ctx, taskID, key)
}

// SearchMemory performs full-text search across all memories (implements MemoryProvider).
func (s *Store) SearchMemory(ctx context.Context, query string, limit int) ([]core.MemoryResult, error) {
	return s.Search(ctx, query, limit)
}

// Save stores a key-value pair associated with a task.
func (s *Store) Save(ctx context.Context, taskID, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO memory (task_id, key, value) VALUES (?, ?, ?)",
		taskID, key, value)
	return err
}

// SaveWithMeta stores a memory entry with category and priority metadata.
func (s *Store) SaveWithMeta(ctx context.Context, taskID, key, category, value string, priority int) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO memory (task_id, key, category, value, priority) VALUES (?, ?, ?, ?, ?)",
		taskID, key, category, value, priority)
	return err
}

// Load retrieves a value by task and key.
func (s *Store) Load(ctx context.Context, taskID, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx,
		"SELECT value FROM memory WHERE task_id = ? AND key = ? ORDER BY created_at DESC LIMIT 1",
		taskID, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// Search performs full-text search and returns matching memories.
func (s *Store) Search(ctx context.Context, query string, limit int) ([]core.MemoryResult, error) {
	if limit <= 0 {
		limit = 10
	}
	// Normalize query for FTS5
	searchTerm := strings.ReplaceAll(query, "\"", "\"\"")

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.task_id, m.key, m.value, m.rank
		FROM memory_fts f
		JOIN memory m ON f.rowid = m.id
		WHERE memory_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?
	`, searchTerm, limit)
	if err != nil {
		// FTS5 search failed, fall back to LIKE search
		return s.searchFallback(ctx, query, limit)
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		var rank float64
		if err := rows.Scan(&r.TaskID, &r.Key, &r.Value, &rank); err != nil {
			continue
		}
		// Convert ranking: more negative = better match in FTS5
		r.Score = -rank
		results = append(results, r)
	}
	return results, rows.Err()
}

// searchFallback performs a LIKE-based search when FTS5 is unavailable.
func (s *Store) searchFallback(ctx context.Context, query string, limit int) ([]core.MemoryResult, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT task_id, key, value FROM memory
		WHERE value LIKE ?
		ORDER BY created_at DESC
		LIMIT ?
	`, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		if err := rows.Scan(&r.TaskID, &r.Key, &r.Value); err != nil {
			continue
		}
		r.Score = 0.5
		results = append(results, r)
	}
	return results, rows.Err()
}

// TaskHistory returns all memories for a specific task.
func (s *Store) TaskHistory(ctx context.Context, taskID string) ([]core.MemoryResult, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT task_id, key, value FROM memory WHERE task_id = ? ORDER BY created_at",
		taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		if err := rows.Scan(&r.TaskID, &r.Key, &r.Value); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ListByCategory retrieves all memories with a specific category.
func (s *Store) ListByCategory(ctx context.Context, category string) ([]core.MemoryResult, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT task_id, key, value FROM memory WHERE category = ? ORDER BY priority DESC, created_at",
		category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		if err := rows.Scan(&r.TaskID, &r.Key, &r.Value); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Recent returns the most recent N memories.
func (s *Store) Recent(ctx context.Context, limit int) ([]core.MemoryResult, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		"SELECT task_id, key, value FROM memory ORDER BY created_at DESC LIMIT ?",
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		if err := rows.Scan(&r.TaskID, &r.Key, &r.Value); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Delete removes a memory entry by task and key.
func (s *Store) Delete(ctx context.Context, taskID, key string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM memory WHERE task_id = ? AND key = ?",
		taskID, key)
	return err
}

// Stats returns memory statistics.
func (s *Store) Stats() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM memory").Scan(&count)
	return count, err
}

// Close releases database resources.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// MemoryEntry represents a full memory entry with metadata.
type MemoryEntry struct {
	ID        int
	TaskID    string
	Key       string
	Value     string
	Category  string
	Priority  int
	CreatedAt string
}

// GetEntries returns full entries for a task.
func (s *Store) GetEntries(ctx context.Context, taskID string) ([]MemoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, task_id, key, value, category, priority, created_at FROM memory WHERE task_id = ? ORDER BY created_at",
		taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Key, &e.Value, &e.Category, &e.Priority, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// FindSimilar returns memories semantically similar to query (based on keyword overlap).
func (s *Store) FindSimilar(ctx context.Context, query string, limit int) ([]core.MemoryResult, error) {
	// Get recent entries and score by keyword overlap
	entries, err := s.Recent(ctx, limit*4)
	if err != nil {
		return nil, err
	}

	queryTokens := tokenize(query)
	var scored []scoredResult
	for _, e := range entries {
		score := similarityScore(queryTokens, tokenize(e.Value))
		if score > 0.1 {
			scored = append(scored, scoredResult{core.MemoryResult{
				TaskID: e.TaskID,
				Key:    e.Key,
				Value:  e.Value,
				Score:  score,
			}, score})
		}
	}

	// Sort by score descending and return top N
	scored = sortResult(scored, limit)
	results := make([]core.MemoryResult, 0, len(scored))
	for _, s := range scored {
		results = append(results, s.MemoryResult)
	}
	return results, nil
}

type scoredResult struct {
	core.MemoryResult
	score float64
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	for _, word := range strings.Fields(text) {
		// Strip punctuation
		word = strings.Trim(word, ".,;:!?()[]{}\"'")
		if len(word) > 2 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

func similarityScore(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	set := make(map[string]bool)
	for _, w := range b {
		set[w] = true
	}
	match := 0
	for _, w := range a {
		if set[w] {
			match++
		}
	}
	return float64(match) / float64(len(a))
}

func sortResult(scored []scoredResult, limit int) []scoredResult {
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j].score > scored[j-1].score; j-- {
			scored[j], scored[j-1] = scored[j-1], scored[j]
		}
	}
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

// SaveFailure records a task failure for later diagnosis.
func (s *Store) SaveFailure(ctx context.Context, taskID, errorType, message string) error {
	return s.SaveWithMeta(ctx, taskID, "error:"+errorType, "errors", message, 10)
}

// SaveSuccess records a successful task completion.
func (s *Store) SaveSuccess(ctx context.Context, taskID, result string) error {
	return s.SaveWithMeta(ctx, taskID, "result:success", "successes", result, 5)
}

// LookupErrors returns error history for a task.
func (s *Store) LookupErrors(ctx context.Context, taskID string) ([]core.MemoryResult, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT task_id, key, value FROM memory WHERE task_id = ? AND category = 'errors' ORDER BY created_at",
		taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []core.MemoryResult
	for rows.Next() {
		var r core.MemoryResult
		if err := rows.Scan(&r.TaskID, &r.Key, &r.Value); err != nil {
			continue
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// Compact removes duplicate entries for the same task+key, keeping only the latest.
func (s *Store) Compact(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM memory WHERE id NOT IN (
			SELECT MAX(id) FROM memory GROUP BY task_id, key
		)
	`)
	return err
}

// ExportJSON exports all memories as JSON string.
func (s *Store) ExportJSON(ctx context.Context) (string, error) {
	type jsonEntry struct {
		TaskID   string `json:"task_id"`
		Key      string `json:"key"`
		Value    string `json:"value"`
		Category string `json:"category"`
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT task_id, key, value, category FROM memory ORDER BY created_at")
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var items []jsonEntry
	for rows.Next() {
		var e jsonEntry
		if err := rows.Scan(&e.TaskID, &e.Key, &e.Value, &e.Category); err != nil {
			return "", err
		}
		items = append(items, e)
	}

	if len(items) == 0 {
		return "{}", nil
	}

	type entry struct {
		TaskID   string `json:"task_id"`
		Key      string `json:"key"`
		Value    string `json:"value"`
		Category string `json:"category"`
	}

	entries := make([]entry, len(items))
	for i, item := range items {
		entries[i] = entry{
			TaskID:   item.TaskID,
			Key:      item.Key,
			Value:    item.Value,
			Category: item.Category,
		}
	}

	data, err := json.MarshalIndent(map[string][]entry{"entries": entries}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
