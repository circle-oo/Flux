package testutil

import (
	"database/sql"
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
