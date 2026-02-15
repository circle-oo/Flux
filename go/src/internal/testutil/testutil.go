package testutil

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/circle-oo/flux/internal/db"
)

// NewTestDB creates an in-memory SQLite database with the full schema applied.
// The database is automatically closed when the test completes.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}

	// Set pragmas (WAL not needed for in-memory but keep foreign keys)
	if _, err := database.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("set foreign_keys: %v", err)
	}

	if err := db.CreateSchema(database); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
	})

	return database
}

// NewTestDBFile creates a file-based SQLite test database in the given directory.
// Use this for concurrent access tests where in-memory DBs fail (separate DB per connection).
func NewTestDBFile(dir string) (*sql.DB, error) {
	dbPath := filepath.Join(dir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open test database: %w", err)
	}

	if err := db.CreateSchema(database); err != nil {
		database.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return database, nil
}
