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

	// Verify all 8 tables exist
	tables := []string{"goals", "projects", "tasks", "subtask_dependencies", "alerts", "usage_snapshots", "rate_limit_events", "service_metrics"}
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
		"idx_subtask_deps_dependent",
		"idx_subtask_deps_dependency",
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

func TestSubtaskDependencies(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	// Enable foreign key constraints
	if _, err := database.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	if err := CreateSchema(database); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Insert test tasks
	_, err = database.Exec(`INSERT INTO tasks (id, title, type, status, priority, source) VALUES
		('task-1', 'Task 1', 'CODING', 'PENDING', 40, 'SYSTEM'),
		('task-2', 'Task 2', 'CODING', 'PENDING', 40, 'SYSTEM'),
		('task-3', 'Task 3', 'CODING', 'PENDING', 40, 'SYSTEM')`)
	if err != nil {
		t.Fatalf("insert tasks: %v", err)
	}

	// Insert dependency: task-1 depends on task-2
	_, err = database.Exec(`INSERT INTO subtask_dependencies (dependent_id, dependency_id) VALUES ('task-1', 'task-2')`)
	if err != nil {
		t.Fatalf("insert dependency: %v", err)
	}

	// Verify dependency exists
	var count int
	err = database.QueryRow(`SELECT COUNT(*) FROM subtask_dependencies WHERE dependent_id = 'task-1' AND dependency_id = 'task-2'`).Scan(&count)
	if err != nil {
		t.Fatalf("query dependency: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 dependency, got %d", count)
	}

	// Test unique constraint: cannot insert duplicate
	_, err = database.Exec(`INSERT INTO subtask_dependencies (dependent_id, dependency_id) VALUES ('task-1', 'task-2')`)
	if err == nil {
		t.Error("expected error on duplicate dependency, got nil")
	}

	// Test foreign key constraint: cannot insert dependency with non-existent task
	_, err = database.Exec(`INSERT INTO subtask_dependencies (dependent_id, dependency_id) VALUES ('task-1', 'nonexistent')`)
	if err == nil {
		t.Error("expected foreign key error, got nil")
	}

	// Test cascade delete: deleting a task should remove its dependencies
	_, err = database.Exec(`INSERT INTO subtask_dependencies (dependent_id, dependency_id) VALUES ('task-2', 'task-3')`)
	if err != nil {
		t.Fatalf("insert second dependency: %v", err)
	}

	_, err = database.Exec(`DELETE FROM tasks WHERE id = 'task-2'`)
	if err != nil {
		t.Fatalf("delete task: %v", err)
	}

	// Both dependencies involving task-2 should be deleted
	err = database.QueryRow(`SELECT COUNT(*) FROM subtask_dependencies WHERE dependent_id = 'task-2' OR dependency_id = 'task-2'`).Scan(&count)
	if err != nil {
		t.Fatalf("query after delete: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 dependencies after cascade delete, got %d", count)
	}
}

func TestSubtaskDependencies_Indexes(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	if err := CreateSchema(database); err != nil {
		t.Fatalf("CreateSchema: %v", err)
	}

	// Insert test data
	_, err = database.Exec(`INSERT INTO tasks (id, title, type, status, priority, source) VALUES
		('task-1', 'Task 1', 'CODING', 'PENDING', 40, 'SYSTEM'),
		('task-2', 'Task 2', 'CODING', 'PENDING', 40, 'SYSTEM'),
		('task-3', 'Task 3', 'CODING', 'PENDING', 40, 'SYSTEM')`)
	if err != nil {
		t.Fatalf("insert tasks: %v", err)
	}

	_, err = database.Exec(`INSERT INTO subtask_dependencies (dependent_id, dependency_id) VALUES
		('task-1', 'task-2'),
		('task-1', 'task-3'),
		('task-2', 'task-3')`)
	if err != nil {
		t.Fatalf("insert dependencies: %v", err)
	}

	// Query by dependent_id (should use idx_subtask_deps_dependent)
	rows, err := database.Query(`SELECT dependency_id FROM subtask_dependencies WHERE dependent_id = 'task-1' ORDER BY dependency_id`)
	if err != nil {
		t.Fatalf("query by dependent: %v", err)
	}
	defer rows.Close()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			t.Fatalf("scan: %v", err)
		}
		deps = append(deps, dep)
	}
	if len(deps) != 2 || deps[0] != "task-2" || deps[1] != "task-3" {
		t.Errorf("expected ['task-2', 'task-3'], got %v", deps)
	}

	// Query by dependency_id (should use idx_subtask_deps_dependency)
	rows, err = database.Query(`SELECT dependent_id FROM subtask_dependencies WHERE dependency_id = 'task-3' ORDER BY dependent_id`)
	if err != nil {
		t.Fatalf("query by dependency: %v", err)
	}
	defer rows.Close()

	deps = nil
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			t.Fatalf("scan: %v", err)
		}
		deps = append(deps, dep)
	}
	if len(deps) != 2 || deps[0] != "task-1" || deps[1] != "task-2" {
		t.Errorf("expected ['task-1', 'task-2'], got %v", deps)
	}
}
