package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
)

// Task type constants
const (
	TaskTypeCoding      = "CODING"
	TaskTypeResearch    = "RESEARCH"
	TaskTypeDocument    = "DOCUMENT"
	TaskTypeMaintenance = "MAINTENANCE"
	TaskTypeDeploy      = "DEPLOY"
	TaskTypeBugfix      = "BUGFIX"
	TaskTypePlanning    = "PLANNING"
)

// Task status constants
const (
	TaskPending   = "PENDING"
	TaskReady     = "READY"
	TaskRunning   = "RUNNING"
	TaskCompleted = "COMPLETED"
	TaskFailed    = "FAILED"
	TaskRetry     = "RETRY"
	TaskArchived  = "ARCHIVED"
)

// Task source constants
const (
	TaskSourceOperator   = "OPERATOR"
	TaskSourceResearcher = "RESEARCHER"
	TaskSourceSelf       = "SELF"
	TaskSourceSystem     = "SYSTEM"
)

// Task represents a unit of work in the system.
type Task struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Status        string   `json:"status"`
	Priority      int      `json:"priority"`
	Source        string   `json:"source"`
	ProjectID     string   `json:"project_id"`
	ParentID      string   `json:"parent_id"`
	Depth         int      `json:"depth"`
	AlertID       string   `json:"alert_id"`
	GoalID        string   `json:"goal_id"`
	DependsOn     []string `json:"depends_on"`
	Tags          []string `json:"tags"`
	Prompt        string   `json:"prompt"`
	Result        string   `json:"result"`
	ErrorLog      string   `json:"error_log"`
	ExecutorID    string   `json:"executor_id"`
	Model         string   `json:"model"`
	BranchName    string   `json:"branch_name"`
	PRUrl         string   `json:"pr_url"`
	PRStatus      string   `json:"pr_status"`
	DiffLines     int      `json:"diff_lines"`
	FilesChanged  int      `json:"files_changed"`
	TestPassed    *bool    `json:"test_passed"`
	RetryCount    int      `json:"retry_count"`
	CrashRecovery bool    `json:"crash_recovery"`
	TokensUsed    int      `json:"tokens_used"`
	CostUSD       float64  `json:"cost_usd"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
	StartedAt     string   `json:"started_at"`
	CompletedAt   string   `json:"completed_at"`
}

// NeedsOpus returns true if this task should use the Opus model.
func (t *Task) NeedsOpus() bool {
	if t.Priority <= 5 {
		return true
	}
	if t.Source == TaskSourceOperator && t.hasComplexKeywords() {
		return true
	}
	if t.hasTag("initial-design") {
		return true
	}
	if t.hasTag("goal-strategy") {
		return true
	}
	return false
}

// RequiresTest returns true if this task type requires tests.
func (t *Task) RequiresTest() bool {
	return t.Type == TaskTypeCoding || t.Type == TaskTypeBugfix || t.Type == TaskTypeMaintenance
}

func (t *Task) hasComplexKeywords() bool {
	keywords := []string{"architect", "refactor", "redesign", "migration", "security", "overhaul"}
	lower := strings.ToLower(t.Title + " " + t.Description)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func (t *Task) hasTag(tag string) bool {
	for _, tt := range t.Tags {
		if tt == tag {
			return true
		}
	}
	return false
}

// TaskStore provides CRUD operations for tasks.
type TaskStore struct {
	DB *sql.DB
}

// NewTaskStore creates a new TaskStore.
func NewTaskStore(db *sql.DB) *TaskStore {
	return &TaskStore{DB: db}
}

// Create inserts a new task.
func (s *TaskStore) Create(t *Task) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	if t.Status == "" {
		t.Status = TaskPending
	}
	if t.Source == "" {
		t.Source = TaskSourceSystem
	}
	if t.Model == "" {
		t.Model = "sonnet"
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}

	// Operator tasks go directly to READY
	if t.Source == TaskSourceOperator && t.Status == TaskPending {
		t.Status = TaskReady
	}

	dependsOnJSON, err := json.Marshal(t.DependsOn)
	if err != nil {
		return fmt.Errorf("marshal depends_on: %w", err)
	}
	tagsJSON, err := json.Marshal(t.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.DB.Exec(
		`INSERT INTO tasks (id, title, description, type, status, priority, source,
		 project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
		 result, error_log, executor_id, model, branch_name, pr_url, pr_status,
		 diff_lines, files_changed, test_passed, retry_count, crash_recovery,
		 tokens_used, cost_usd, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Type, t.Status, t.Priority, t.Source,
		t.ProjectID, t.ParentID, t.Depth, t.AlertID, t.GoalID,
		string(dependsOnJSON), string(tagsJSON), t.Prompt,
		t.Result, t.ErrorLog, t.ExecutorID, t.Model, t.BranchName,
		t.PRUrl, t.PRStatus, t.DiffLines, t.FilesChanged, t.TestPassed,
		t.RetryCount, t.CrashRecovery, t.TokensUsed, t.CostUSD,
		t.StartedAt, t.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetByID retrieves a task by its ID.
func (s *TaskStore) GetByID(id string) (*Task, error) {
	row := s.DB.QueryRow(taskSelectSQL+" WHERE id = ?", id)
	return scanTask(row)
}

// ListFilter holds optional filter parameters for listing tasks.
type ListFilter struct {
	Status    string
	ProjectID string
	Page      int
	Limit     int
}

// List retrieves tasks matching the given filters.
func (s *TaskStore) List(f ListFilter) ([]*Task, error) {
	query := taskSelectSQL
	var args []interface{}
	var conditions []string

	if f.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, f.Status)
	}
	if f.ProjectID != "" {
		conditions = append(conditions, "project_id = ?")
		args = append(args, f.ProjectID)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY priority ASC, created_at DESC"

	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	offset := (f.Page - 1) * f.Limit
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, offset)

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update modifies an existing task.
func (s *TaskStore) Update(t *Task) error {
	dependsOnJSON, err := json.Marshal(t.DependsOn)
	if err != nil {
		return fmt.Errorf("marshal depends_on: %w", err)
	}
	tagsJSON, err := json.Marshal(t.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}

	_, err = s.DB.Exec(
		`UPDATE tasks SET title = ?, description = ?, type = ?, status = ?, priority = ?,
		 source = ?, project_id = ?, parent_id = ?, depth = ?, alert_id = ?, goal_id = ?,
		 depends_on = ?, tags = ?, prompt = ?, result = ?, error_log = ?, executor_id = ?,
		 model = ?, branch_name = ?, pr_url = ?, pr_status = ?, diff_lines = ?,
		 files_changed = ?, test_passed = ?, retry_count = ?, crash_recovery = ?,
		 tokens_used = ?, cost_usd = ?, updated_at = CURRENT_TIMESTAMP,
		 started_at = ?, completed_at = ? WHERE id = ?`,
		t.Title, t.Description, t.Type, t.Status, t.Priority,
		t.Source, t.ProjectID, t.ParentID, t.Depth, t.AlertID, t.GoalID,
		string(dependsOnJSON), string(tagsJSON), t.Prompt, t.Result, t.ErrorLog,
		t.ExecutorID, t.Model, t.BranchName, t.PRUrl, t.PRStatus, t.DiffLines,
		t.FilesChanged, t.TestPassed, t.RetryCount, t.CrashRecovery,
		t.TokensUsed, t.CostUSD, t.StartedAt, t.CompletedAt, t.ID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	return nil
}

// Delete removes a task by its ID.
func (s *TaskStore) Delete(id string) error {
	_, err := s.DB.Exec(`DELETE FROM tasks WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// Cancel sets a task to FAILED with "cancelled by operator" error log.
func (s *TaskStore) Cancel(id string) error {
	_, err := s.DB.Exec(
		`UPDATE tasks SET status = ?, error_log = 'cancelled by operator',
		 updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		TaskFailed, id,
	)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}
	return nil
}

// Retry resets a FAILED task back to READY so it can be picked up again.
func (s *TaskStore) Retry(id string) error {
	result, err := s.DB.Exec(
		`UPDATE tasks SET status = ?, error_log = '', result = '',
		 retry_count = retry_count + 1, started_at = '', completed_at = '',
		 executor_id = '', branch_name = '', pr_url = '', pr_status = '',
		 updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status IN (?, ?)`,
		TaskReady, id, TaskFailed, TaskRetry,
	)
	if err != nil {
		return fmt.Errorf("retry task: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %s is not in a retryable state", id)
	}
	return nil
}

// CountByParent counts the number of subtasks for a given parent task.
func (s *TaskStore) CountByParent(parentID string) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM tasks WHERE parent_id = ?`, parentID).Scan(&count)
	return count, err
}

const taskSelectSQL = `SELECT id, title, description, type, status, priority, source,
	project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
	result, error_log, executor_id, model, branch_name, pr_url, pr_status,
	diff_lines, files_changed, test_passed, retry_count, crash_recovery,
	tokens_used, cost_usd, created_at, updated_at, started_at, completed_at
	FROM tasks`

func scanTask(row *sql.Row) (*Task, error) {
	var t Task
	var dependsOnJSON, tagsJSON string
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.Type, &t.Status, &t.Priority, &t.Source,
		&t.ProjectID, &t.ParentID, &t.Depth, &t.AlertID, &t.GoalID,
		&dependsOnJSON, &tagsJSON, &t.Prompt,
		&t.Result, &t.ErrorLog, &t.ExecutorID, &t.Model, &t.BranchName,
		&t.PRUrl, &t.PRStatus, &t.DiffLines, &t.FilesChanged, &t.TestPassed,
		&t.RetryCount, &t.CrashRecovery, &t.TokensUsed, &t.CostUSD,
		&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(dependsOnJSON), &t.DependsOn); err != nil {
		slog.Warn("corrupt JSON in DB", "field", "depends_on", "error", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
		slog.Warn("corrupt JSON in DB", "field", "tags", "error", err)
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}

func scanTaskRow(rows *sql.Rows) (*Task, error) {
	var t Task
	var dependsOnJSON, tagsJSON string
	err := rows.Scan(
		&t.ID, &t.Title, &t.Description, &t.Type, &t.Status, &t.Priority, &t.Source,
		&t.ProjectID, &t.ParentID, &t.Depth, &t.AlertID, &t.GoalID,
		&dependsOnJSON, &tagsJSON, &t.Prompt,
		&t.Result, &t.ErrorLog, &t.ExecutorID, &t.Model, &t.BranchName,
		&t.PRUrl, &t.PRStatus, &t.DiffLines, &t.FilesChanged, &t.TestPassed,
		&t.RetryCount, &t.CrashRecovery, &t.TokensUsed, &t.CostUSD,
		&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(dependsOnJSON), &t.DependsOn); err != nil {
		slog.Warn("corrupt JSON in DB", "field", "depends_on", "error", err)
	}
	if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
		slog.Warn("corrupt JSON in DB", "field", "tags", "error", err)
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	return &t, nil
}
