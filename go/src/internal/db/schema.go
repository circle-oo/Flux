package db

import (
	"database/sql"
	"fmt"
)

// CreateSchema creates all tables and indexes if they don't exist.
func CreateSchema(db *sql.DB) error {
	for _, stmt := range schemaStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("execute schema statement: %w\nSQL: %s", err, stmt)
		}
	}
	return nil
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS goals (
		id           TEXT PRIMARY KEY,
		title        TEXT NOT NULL,
		description  TEXT DEFAULT '',
		priorities   TEXT DEFAULT '[]',
		metrics      TEXT DEFAULT '[]',
		status       TEXT NOT NULL DEFAULT 'PROPOSED',
		source       TEXT NOT NULL DEFAULT 'ORCHESTRATOR',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		active_since DATETIME DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS projects (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL UNIQUE,
		type        TEXT NOT NULL,
		repo_url    TEXT DEFAULT '',
		description TEXT DEFAULT '',
		vault_path  TEXT DEFAULT '',
		status      TEXT NOT NULL DEFAULT 'ACTIVE',
		tech_stack  TEXT DEFAULT '[]',
		inspiration TEXT DEFAULT '',
		goal_id     TEXT DEFAULT '',
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS tasks (
		id              TEXT PRIMARY KEY,
		title           TEXT NOT NULL,
		description     TEXT DEFAULT '',
		type            TEXT NOT NULL,
		status          TEXT NOT NULL DEFAULT 'PENDING',
		priority        INTEGER NOT NULL DEFAULT 50,
		source          TEXT NOT NULL DEFAULT 'SYSTEM',
		project_id      TEXT DEFAULT '',
		parent_id       TEXT DEFAULT '',
		depth           INTEGER NOT NULL DEFAULT 0,
		alert_id        TEXT DEFAULT '',
		goal_id         TEXT DEFAULT '',
		depends_on      TEXT DEFAULT '[]',
		tags            TEXT DEFAULT '[]',
		prompt          TEXT DEFAULT '',
		result          TEXT DEFAULT '',
		error_log       TEXT DEFAULT '',
		executor_id     TEXT DEFAULT '',
		model           TEXT DEFAULT 'sonnet',
		branch_name     TEXT DEFAULT '',
		pr_url          TEXT DEFAULT '',
		pr_status       TEXT DEFAULT '',
		diff_lines      INTEGER DEFAULT 0,
		files_changed   INTEGER DEFAULT 0,
		test_passed     BOOLEAN DEFAULT NULL,
		triage_analysis TEXT DEFAULT '',
		plan            TEXT DEFAULT '',
		retry_count     INTEGER DEFAULT 0,
		crash_recovery  BOOLEAN DEFAULT FALSE,
		tokens_used     INTEGER DEFAULT 0,
		cost_usd        REAL DEFAULT 0,
		created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
		started_at      DATETIME DEFAULT '',
		completed_at    DATETIME DEFAULT ''
	)`,

	`CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_pr_status ON tasks(pr_status)`,
	`CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id)`,

	`CREATE TABLE IF NOT EXISTS alerts (
		id           TEXT PRIMARY KEY,
		service_name TEXT NOT NULL,
		severity     TEXT NOT NULL,
		type         TEXT NOT NULL,
		message      TEXT DEFAULT '',
		task_id      TEXT DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'ACTIVE',
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		resolved_at  DATETIME DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS usage_snapshots (
		id            TEXT PRIMARY KEY,
		type          TEXT NOT NULL,
		data          TEXT NOT NULL,
		total_tokens  INTEGER DEFAULT 0,
		total_cost    REAL DEFAULT 0,
		recorded_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE INDEX IF NOT EXISTS idx_usage_snapshots_type_time ON usage_snapshots(type, recorded_at)`,

	`CREATE TABLE IF NOT EXISTS rate_limit_events (
		id          TEXT PRIMARY KEY,
		tokens_used INTEGER DEFAULT 0,
		active_pods INTEGER DEFAULT 0,
		occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS service_metrics (
		id           TEXT PRIMARY KEY,
		service_name TEXT NOT NULL,
		latency_ms   INTEGER DEFAULT 0,
		status_code  INTEGER DEFAULT 0,
		is_healthy   BOOLEAN DEFAULT TRUE,
		recorded_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE INDEX IF NOT EXISTS idx_metrics_service_time ON service_metrics(service_name, recorded_at)`,
}
