package memory

import (
	"context"
	"database/sql"
	
	_ "github.com/mattn/go-sqlite3"
)

// Store implements the persistence of agent memory and history.
type Store struct {
	db *sql.DB
}

// New creates a new in-memory or file-based SQLite store.
func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	
	// Initialize tables
	if err := db.Ping(); err != nil {
		return nil, err
	}
	
	if err := initSchema(db); err != nil {
		return nil, err
	}
	
	return &Store{db: db}, nil
}

func initSchema(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS memory (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		task_id TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		embedding BLOB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	CREATE INDEX IF NOT EXISTS idx_memory_task ON memory(task_id);
	CREATE INDEX IF NOT EXISTS idx_memory_key ON memory(key);
	`
	_, err := db.Exec(query)
	return err
}

// Save stores a key-value pair associated with a task.
func (s *Store) Save(ctx context.Context, taskID, key, value string) error {
	_, err := s.db.ExecContext(ctx, 
		"INSERT INTO memory (task_id, key, value) VALUES (?, ?, ?)", 
		taskID, key, value)
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
