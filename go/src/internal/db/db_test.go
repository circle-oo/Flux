package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestOpen_InMemory(t *testing.T) {
	// Use a temp dir to test Open with real file
	dir := t.TempDir()
	path := dir + "/test.db"

	database, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	// Verify WAL mode
	var mode string
	err = database.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Errorf("expected journal_mode=wal, got %s", mode)
	}

	// Verify foreign_keys
	var fk int
	err = database.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("expected foreign_keys=1, got %d", fk)
	}
}

func TestCreateSchema(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	if err := CreateSchema(database); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Verify all 7 tables exist
	tables := []string{"goals", "projects", "tasks", "alerts", "usage_snapshots", "rate_limit_events", "service_metrics"}
	for _, table := range tables {
		var name string
		err := database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}

	// Verify idempotent: run again without error
	if err := CreateSchema(database); err != nil {
		t.Fatalf("CreateSchema (idempotent): %v", err)
	}
}

func TestCreateSchema_Indexes(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	if err := CreateSchema(database); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	indexes := []string{
		"idx_tasks_status_priority",
		"idx_tasks_project",
		"idx_tasks_pr_status",
		"idx_tasks_parent",
		"idx_usage_snapshots_type_time",
		"idx_metrics_service_time",
	}
	for _, idx := range indexes {
		var name string
		err := database.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}
