package models

import (
	"database/sql"
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
	TaskPending    = "PENDING"
	TaskReady      = "READY"
	TaskRunning    = "RUNNING"
	TaskCompleted  = "COMPLETED"
	TaskFailed     = "FAILED"
	TaskCancelled  = "CANCELLED"
	TaskRetry      = "RETRY"
	TaskArchived   = "ARCHIVED"
	TaskDecomposed = "DECOMPOSED"
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
	TriageAnalysis    string  `json:"triage_analysis"`
	TriageDescription string  `json:"triage_description"`
	TriageTitle       string  `json:"triage_title"`
	Plan              string  `json:"plan"`
	TestPassed    *bool    `json:"test_passed"`
	RetryCount    int      `json:"retry_count"`
	MaxRetries    int      `json:"max_retries"`
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

// RequiresTest returns true if this task requires tests.
// Tests are required by default unless the task has a "skip-tests" tag.
func (t *Task) RequiresTest() bool {
	// Check for explicit skip-tests tag
	for _, tag := range t.Tags {
		if tag == "skip-tests" || tag == "no-tests" {
			return false
		}
	}
	// By default, require tests
	return true
}

// IsRetryable returns true if this task can be retried (failed but within retry limits).
func (t *Task) IsRetryable() bool {
	return t.Status == TaskFailed && t.RetryCount < t.MaxRetries
}

// HasRetriesExhausted returns true if this task has failed and exhausted all retries.
func (t *Task) HasRetriesExhausted() bool {
	return t.Status == TaskFailed && t.RetryCount >= t.MaxRetries
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
	if t.MaxRetries == 0 {
		t.MaxRetries = 3
	}

	// Operator tasks stay PENDING until triage completes, then move to READY.
	// Non-operator tasks (SYSTEM, SELF) go directly to READY.
	if t.Source != TaskSourceOperator && t.Status == TaskPending {
		t.Status = TaskReady
	}

	dependsOnJSON, err := marshalStringSlice("depends_on", t.DependsOn)
	if err != nil {
		return err
	}
	tagsJSON, err := marshalStringSlice("tags", t.Tags)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`INSERT INTO tasks (id, title, description, type, status, priority, source,
		 project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
		 result, error_log, executor_id, model, branch_name, pr_url, pr_status,
		 diff_lines, files_changed, triage_analysis, triage_description, triage_title, plan, test_passed,
		 retry_count, max_retries, crash_recovery, tokens_used, cost_usd, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Title, t.Description, t.Type, t.Status, t.Priority, t.Source,
		t.ProjectID, t.ParentID, t.Depth, t.AlertID, t.GoalID,
		dependsOnJSON, tagsJSON, t.Prompt,
		t.Result, t.ErrorLog, t.ExecutorID, t.Model, t.BranchName,
		t.PRUrl, t.PRStatus, t.DiffLines, t.FilesChanged,
		t.TriageAnalysis, t.TriageDescription, t.TriageTitle, t.Plan, t.TestPassed,
		t.RetryCount, t.MaxRetries, t.CrashRecovery, t.TokensUsed, t.CostUSD,
		t.StartedAt, t.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("insert task: %w", err)
	}
	return nil
}

// GetByID retrieves a task by its ID.
func (s *TaskStore) GetByID(id string) (*Task, error) {
	row := s.DB.QueryRow(TaskSelectSQL+" WHERE id = ?", id)
	return ScanTask(row)
}

// ListFilter holds optional filter parameters for listing tasks.
type ListFilter struct {
	Status          string
	ProjectID       string
	ExcludeSubtasks bool
	Page            int
	Limit           int
}

// List retrieves tasks matching the given filters.
func (s *TaskStore) List(f ListFilter) ([]*Task, error) {
	query := TaskSelectSQL
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
	if f.ExcludeSubtasks {
		conditions = append(conditions, "(parent_id = '' OR parent_id IS NULL)")
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
		t, err := ScanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// Update modifies an existing task.
func (s *TaskStore) Update(t *Task) error {
	dependsOnJSON, err := marshalStringSlice("depends_on", t.DependsOn)
	if err != nil {
		return err
	}
	tagsJSON, err := marshalStringSlice("tags", t.Tags)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`UPDATE tasks SET title = ?, description = ?, type = ?, status = ?, priority = ?,
		 source = ?, project_id = ?, parent_id = ?, depth = ?, alert_id = ?, goal_id = ?,
		 depends_on = ?, tags = ?, prompt = ?, result = ?, error_log = ?, executor_id = ?,
		 model = ?, branch_name = ?, pr_url = ?, pr_status = ?, diff_lines = ?,
		 files_changed = ?, triage_analysis = ?, triage_description = ?, triage_title = ?, plan = ?,
		 test_passed = ?, retry_count = ?, max_retries = ?, crash_recovery = ?,
		 tokens_used = ?, cost_usd = ?, updated_at = CURRENT_TIMESTAMP,
		 started_at = ?, completed_at = ? WHERE id = ?`,
		t.Title, t.Description, t.Type, t.Status, t.Priority,
		t.Source, t.ProjectID, t.ParentID, t.Depth, t.AlertID, t.GoalID,
		dependsOnJSON, tagsJSON, t.Prompt, t.Result, t.ErrorLog,
		t.ExecutorID, t.Model, t.BranchName, t.PRUrl, t.PRStatus, t.DiffLines,
		t.FilesChanged, t.TriageAnalysis, t.TriageDescription, t.TriageTitle, t.Plan,
		t.TestPassed, t.RetryCount, t.MaxRetries, t.CrashRecovery,
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

// Cancel sets a task to CANCELLED with "cancelled by operator" error log.
// Only cancels tasks that are in a cancellable state (PENDING, READY, RUNNING).
func (s *TaskStore) Cancel(id string) error {
	result, err := s.DB.Exec(
		`UPDATE tasks SET status = ?, error_log = 'cancelled by operator',
		 updated_at = CURRENT_TIMESTAMP, completed_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status IN (?, ?, ?, ?)`,
		TaskCancelled, id, TaskPending, TaskReady, TaskRunning, TaskDecomposed,
	)
	if err != nil {
		return fmt.Errorf("cancel task: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %s is not in a cancellable state", id)
	}
	return nil
}

// Archive sets a task to ARCHIVED. Only completed, failed, or cancelled tasks can be archived.
func (s *TaskStore) Archive(id string) error {
	result, err := s.DB.Exec(
		`UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND status IN (?, ?, ?)`,
		TaskArchived, id, TaskCompleted, TaskFailed, TaskCancelled,
	)
	if err != nil {
		return fmt.Errorf("archive task: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("task %s is not in an archivable state", id)
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
		 WHERE id = ? AND status IN (?, ?) AND retry_count < max_retries`,
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

// ListByParent retrieves all subtasks for a given parent task.
func (s *TaskStore) ListByParent(parentID string) ([]*Task, error) {
	query := TaskSelectSQL + " WHERE parent_id = ? ORDER BY priority ASC, created_at ASC"
	rows, err := s.DB.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("query subtasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := ScanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ListPending retrieves tasks with PENDING status, ordered by priority.
func (s *TaskStore) ListPending() ([]*Task, error) {
	query := TaskSelectSQL + " WHERE status = ? ORDER BY priority ASC, created_at ASC"
	rows, err := s.DB.Query(query, TaskPending)
	if err != nil {
		return nil, fmt.Errorf("query pending tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := ScanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// CancelChildren cancels all active subtasks of a parent task.
func (s *TaskStore) CancelChildren(parentID string) (int64, error) {
	result, err := s.DB.Exec(
		`UPDATE tasks SET status = ?, error_log = 'parent cancelled',
		 updated_at = CURRENT_TIMESTAMP, completed_at = CURRENT_TIMESTAMP
		 WHERE parent_id = ? AND status IN (?, ?, ?, ?)`,
		TaskCancelled, parentID, TaskPending, TaskReady, TaskRunning, TaskDecomposed,
	)
	if err != nil {
		return 0, fmt.Errorf("cancel children: %w", err)
	}
	return result.RowsAffected()
}

// ArchiveChildren archives all subtasks of a parent task, regardless of their current status.
// This is used when a parent task is cancelled or retried to clear out stale subtask data.
func (s *TaskStore) ArchiveChildren(parentID string) (int64, error) {
	result, err := s.DB.Exec(
		`UPDATE tasks SET status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE parent_id = ? AND status != ?`,
		TaskArchived, parentID, TaskArchived,
	)
	if err != nil {
		return 0, fmt.Errorf("archive children: %w", err)
	}
	return result.RowsAffected()
}

// CountByStatus returns the number of tasks with the given status.
func (s *TaskStore) CountByStatus(status string) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = ?`, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by status: %w", err)
	}
	slog.Debug("count by status", "status", status, "count", count)
	return count, nil
}

// CountBySource returns the number of tasks with the given source.
func (s *TaskStore) CountBySource(source string) (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM tasks WHERE source = ?`, source).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by source: %w", err)
	}
	slog.Debug("count by source", "source", source, "count", count)
	return count, nil
}

// CountByPriority returns the number of tasks with priority in the range [minP, maxP].
func (s *TaskStore) CountByPriority(minP, maxP int) (int, error) {
	var count int
	err := s.DB.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE priority >= ? AND priority <= ?`,
		minP, maxP,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by priority: %w", err)
	}
	slog.Debug("count by priority", "min_priority", minP, "max_priority", maxP, "count", count)
	return count, nil
}

// ListByPRStatus returns all tasks with the given PR status.
func (s *TaskStore) ListByPRStatus(prStatus string) ([]*Task, error) {
	query := TaskSelectSQL + `
		WHERE pr_status = ?
		ORDER BY priority ASC, created_at ASC`

	rows, err := s.DB.Query(query, prStatus)
	if err != nil {
		return nil, fmt.Errorf("query by pr_status: %w", err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := ScanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	slog.Debug("list by pr status", "pr_status", prStatus, "count", len(tasks))
	return tasks, rows.Err()
}

// TaskSelectSQL is the canonical SELECT column list for tasks.
// All queries that read full task rows should use this constant to avoid
// column-list drift (the root cause of PR #23 where triage_analysis was
// missing from manager queries).
const TaskSelectSQL = `SELECT id, title, description, type, status, priority, source,
	project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
	result, error_log, executor_id, model, branch_name, pr_url, pr_status,
	diff_lines, files_changed, triage_analysis, triage_description, triage_title, plan,
	test_passed, retry_count, max_retries, crash_recovery, tokens_used, cost_usd,
	created_at, updated_at, started_at, completed_at
	FROM tasks`

// SubtaskDependency represents a dependency relationship between subtasks.
type SubtaskDependency struct {
	DependentID  string `json:"dependent_id"`
	DependencyID string `json:"dependency_id"`
}

// GetSubtaskDependencies retrieves all dependencies for subtasks of a given parent task.
func (s *TaskStore) GetSubtaskDependencies(parentID string) ([]*SubtaskDependency, error) {
	query := `
		SELECT sd.dependent_id, sd.dependency_id
		FROM subtask_dependencies sd
		INNER JOIN tasks t1 ON sd.dependent_id = t1.id
		WHERE t1.parent_id = ?
		ORDER BY sd.dependent_id, sd.dependency_id
	`
	rows, err := s.DB.Query(query, parentID)
	if err != nil {
		return nil, fmt.Errorf("query subtask dependencies: %w", err)
	}
	defer rows.Close()

	var deps []*SubtaskDependency
	for rows.Next() {
		var d SubtaskDependency
		if err := rows.Scan(&d.DependentID, &d.DependencyID); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, &d)
	}
	return deps, rows.Err()
}

// ScanTask scans a task from any source that implements Scan (works with
// both *sql.Row and *sql.Rows). This is the single source of truth for
// the scan field order — it must match TaskSelectSQL.
func ScanTask(scanner interface{ Scan(...interface{}) error }) (*Task, error) {
	var t Task
	var dependsOnJSON, tagsJSON string
	err := scanner.Scan(
		&t.ID, &t.Title, &t.Description, &t.Type, &t.Status, &t.Priority, &t.Source,
		&t.ProjectID, &t.ParentID, &t.Depth, &t.AlertID, &t.GoalID,
		&dependsOnJSON, &tagsJSON, &t.Prompt,
		&t.Result, &t.ErrorLog, &t.ExecutorID, &t.Model, &t.BranchName,
		&t.PRUrl, &t.PRStatus, &t.DiffLines, &t.FilesChanged,
		&t.TriageAnalysis, &t.TriageDescription, &t.TriageTitle, &t.Plan,
		&t.TestPassed, &t.RetryCount, &t.MaxRetries, &t.CrashRecovery, &t.TokensUsed, &t.CostUSD,
		&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, err
	}
	t.DependsOn = unmarshalStringSlice("depends_on", dependsOnJSON)
	t.Tags = unmarshalStringSlice("tags", tagsJSON)
	return &t, nil
}

// ValidateSubtaskDAG validates that adding a subtask with given dependencies won't create a cycle.
// It checks that the dependency graph for all subtasks under parentID remains acyclic.
// If taskID is provided, it validates updating that task's dependencies; otherwise it validates a new task.
// Returns an error if a cycle would be created.
func (s *TaskStore) ValidateSubtaskDAG(parentID string, dependencies []string) error {
	return s.ValidateSubtaskDAGWithUpdate(parentID, "", dependencies)
}

// ValidateSubtaskDAGWithUpdate validates DAG property when updating an existing task or adding a new one.
// If taskID is empty, it simulates adding a new task; otherwise it simulates updating the existing task.
func (s *TaskStore) ValidateSubtaskDAGWithUpdate(parentID string, taskID string, dependencies []string) error {
	// Build the current dependency graph for this parent's subtasks
	subtasks, err := s.ListByParent(parentID)
	if err != nil {
		return fmt.Errorf("list subtasks: %w", err)
	}

	// Build adjacency list: taskID -> []dependencyIDs
	graph := make(map[string][]string)
	taskIDs := make(map[string]bool)

	for _, task := range subtasks {
		taskIDs[task.ID] = true
		if task.ID == taskID {
			// Use the updated dependencies for this task
			graph[task.ID] = dependencies
		} else {
			graph[task.ID] = task.DependsOn
		}
	}

	// If taskID is empty, we're adding a new task
	if taskID == "" && len(dependencies) > 0 {
		newTaskID := "new-task-placeholder"
		taskIDs[newTaskID] = true
		graph[newTaskID] = dependencies
	}

	// Verify all dependencies reference valid subtasks under the same parent
	for _, depID := range dependencies {
		if !taskIDs[depID] {
			return fmt.Errorf("dependency %s is not a subtask of parent %s", depID, parentID)
		}
	}

	// Detect cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for id := range taskIDs {
		if !visited[id] {
			if hasCycle(id, graph, visited, recStack) {
				return fmt.Errorf("adding dependencies would create a cycle in subtask DAG")
			}
		}
	}

	return nil
}

// GetTopologicalOrder returns subtasks of the given parent in topological order.
// Tasks that can execute in parallel (no dependency relationship) are ordered by priority.
// Returns an error if the dependency graph contains a cycle.
func (s *TaskStore) GetTopologicalOrder(parentID string) ([]*Task, error) {
	subtasks, err := s.ListByParent(parentID)
	if err != nil {
		return nil, fmt.Errorf("list subtasks: %w", err)
	}

	if len(subtasks) == 0 {
		return []*Task{}, nil
	}

	// Build adjacency list and in-degree map
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	taskMap := make(map[string]*Task)

	for _, task := range subtasks {
		taskMap[task.ID] = task
		inDegree[task.ID] = 0
		graph[task.ID] = []string{}
	}

	// Build graph edges (task -> tasks that depend on it)
	for _, task := range subtasks {
		for _, depID := range task.DependsOn {
			if _, exists := taskMap[depID]; exists {
				graph[depID] = append(graph[depID], task.ID)
				inDegree[task.ID]++
			}
		}
	}

	// Kahn's algorithm for topological sort
	var queue []*Task
	for _, task := range subtasks {
		if inDegree[task.ID] == 0 {
			queue = append(queue, task)
		}
	}

	var result []*Task
	for len(queue) > 0 {
		// Process current level (tasks with no remaining dependencies)
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for dependent tasks
		for _, dependentID := range graph[current.ID] {
			inDegree[dependentID]--
			if inDegree[dependentID] == 0 {
				queue = append(queue, taskMap[dependentID])
			}
		}
	}

	// If not all tasks are in result, there's a cycle
	if len(result) != len(subtasks) {
		return nil, fmt.Errorf("subtask dependency graph contains a cycle")
	}

	return result, nil
}

// AddSubtaskDependency adds a dependency edge from dependentID to dependencyID.
// It validates that both tasks are subtasks of the same parent and that adding
// the dependency won't create a cycle.
func (s *TaskStore) AddSubtaskDependency(dependentID, dependencyID string) error {
	if dependentID == dependencyID {
		return fmt.Errorf("task cannot depend on itself")
	}

	// Get both tasks
	dependent, err := s.GetByID(dependentID)
	if err != nil {
		return fmt.Errorf("get dependent task: %w", err)
	}

	dependency, err := s.GetByID(dependencyID)
	if err != nil {
		return fmt.Errorf("get dependency task: %w", err)
	}

	// Verify both tasks have the same parent
	if dependent.ParentID == "" {
		return fmt.Errorf("dependent task %s is not a subtask", dependentID)
	}
	if dependent.ParentID != dependency.ParentID {
		return fmt.Errorf("tasks must be subtasks of the same parent")
	}

	// Check if dependency already exists
	for _, depID := range dependent.DependsOn {
		if depID == dependencyID {
			return nil // Already exists, no-op
		}
	}

	// Add the new dependency and validate no cycles
	newDependencies := append(dependent.DependsOn, dependencyID)
	if err := s.ValidateSubtaskDAGWithUpdate(dependent.ParentID, dependentID, newDependencies); err != nil {
		return err
	}

	// Update the task with the new dependency
	dependent.DependsOn = newDependencies
	return s.Update(dependent)
}
