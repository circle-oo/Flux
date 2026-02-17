package models

import (
	"database/sql"
	"fmt"
)

// TaskAttempt is a snapshot of a task's execution state before a retry clears it.
type TaskAttempt struct {
	ID                int     `json:"id"`
	TaskID            string  `json:"task_id"`
	Attempt           int     `json:"attempt"`
	Status            string  `json:"status"`
	Result            string  `json:"result"`
	ErrorLog          string  `json:"error_log"`
	ExecutorID        string  `json:"executor_id"`
	Model             string  `json:"model"`
	BranchName        string  `json:"branch_name"`
	PRUrl             string  `json:"pr_url"`
	PRStatus          string  `json:"pr_status"`
	DiffLines         int     `json:"diff_lines"`
	FilesChanged      int     `json:"files_changed"`
	TestPassed        *bool   `json:"test_passed"`
	TokensUsed        int     `json:"tokens_used"`
	CostUSD           float64 `json:"cost_usd"`
	TriageAnalysis    string  `json:"triage_analysis"`
	TriageDescription string  `json:"triage_description"`
	TriageTitle       string  `json:"triage_title"`
	StartedAt         string  `json:"started_at"`
	CompletedAt       string  `json:"completed_at"`
	CreatedAt         string  `json:"created_at"`
}

// TaskAttemptStore provides operations for task attempt history.
type TaskAttemptStore struct {
	DB *sql.DB
}

// NewTaskAttemptStore creates a new TaskAttemptStore.
func NewTaskAttemptStore(db *sql.DB) *TaskAttemptStore {
	return &TaskAttemptStore{DB: db}
}

// SaveAttempt snapshots the current task state as an attempt record.
// This should be called before a retry clears the task's execution fields.
func (s *TaskAttemptStore) SaveAttempt(taskID string, task *Task) error {
	_, err := s.DB.Exec(
		`INSERT INTO task_attempts (task_id, attempt, status, result, error_log,
		 executor_id, model, branch_name, pr_url, pr_status, diff_lines, files_changed,
		 test_passed, tokens_used, cost_usd, triage_analysis, triage_description,
		 triage_title, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, task.RetryCount, task.Status, task.Result, task.ErrorLog,
		task.ExecutorID, task.Model, task.BranchName, task.PRUrl, task.PRStatus,
		task.DiffLines, task.FilesChanged, task.TestPassed, task.TokensUsed, task.CostUSD,
		task.TriageAnalysis, task.TriageDescription, task.TriageTitle,
		task.StartedAt, task.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("save task attempt: %w", err)
	}
	return nil
}

// SaveAttemptTx snapshots the current task state within an existing transaction.
func (s *TaskAttemptStore) SaveAttemptTx(tx *sql.Tx, taskID string, task *Task) error {
	_, err := tx.Exec(
		`INSERT INTO task_attempts (task_id, attempt, status, result, error_log,
		 executor_id, model, branch_name, pr_url, pr_status, diff_lines, files_changed,
		 test_passed, tokens_used, cost_usd, triage_analysis, triage_description,
		 triage_title, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, task.RetryCount, task.Status, task.Result, task.ErrorLog,
		task.ExecutorID, task.Model, task.BranchName, task.PRUrl, task.PRStatus,
		task.DiffLines, task.FilesChanged, task.TestPassed, task.TokensUsed, task.CostUSD,
		task.TriageAnalysis, task.TriageDescription, task.TriageTitle,
		task.StartedAt, task.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("save task attempt (tx): %w", err)
	}
	return nil
}

// ListByTask returns all attempts for a given task, ordered by attempt number.
func (s *TaskAttemptStore) ListByTask(taskID string) ([]*TaskAttempt, error) {
	rows, err := s.DB.Query(
		`SELECT id, task_id, attempt, status, result, error_log, executor_id, model,
		 branch_name, pr_url, pr_status, diff_lines, files_changed, test_passed,
		 tokens_used, cost_usd, triage_analysis, triage_description, triage_title,
		 started_at, completed_at, created_at
		 FROM task_attempts WHERE task_id = ? ORDER BY attempt ASC`, taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("query task attempts: %w", err)
	}
	defer rows.Close()

	var attempts []*TaskAttempt
	for rows.Next() {
		var a TaskAttempt
		if err := rows.Scan(
			&a.ID, &a.TaskID, &a.Attempt, &a.Status, &a.Result, &a.ErrorLog,
			&a.ExecutorID, &a.Model, &a.BranchName, &a.PRUrl, &a.PRStatus,
			&a.DiffLines, &a.FilesChanged, &a.TestPassed, &a.TokensUsed, &a.CostUSD,
			&a.TriageAnalysis, &a.TriageDescription, &a.TriageTitle,
			&a.StartedAt, &a.CompletedAt, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan task attempt: %w", err)
		}
		attempts = append(attempts, &a)
	}
	return attempts, rows.Err()
}

// TotalTokensAndCost returns the sum of tokens_used and cost_usd across all attempts for a task.
func (s *TaskAttemptStore) TotalTokensAndCost(taskID string) (int, float64, error) {
	var tokens int
	var cost float64
	err := s.DB.QueryRow(
		`SELECT COALESCE(SUM(tokens_used), 0), COALESCE(SUM(cost_usd), 0)
		 FROM task_attempts WHERE task_id = ?`, taskID,
	).Scan(&tokens, &cost)
	if err != nil {
		return 0, 0, fmt.Errorf("sum task attempt usage: %w", err)
	}
	return tokens, cost, nil
}
