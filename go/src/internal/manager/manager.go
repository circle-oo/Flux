package manager

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/models"
)

// validTransitions defines the state machine for task status transitions.
var validTransitions = map[string][]string{
	models.TaskPending:    {models.TaskReady, models.TaskCancelled},
	models.TaskReady:      {models.TaskRunning, models.TaskCancelled},
	models.TaskRunning:    {models.TaskCompleted, models.TaskFailed, models.TaskRetry, models.TaskCancelled, models.TaskDecomposed},
	models.TaskFailed:     {models.TaskRetry, models.TaskArchived},
	models.TaskRetry:      {models.TaskRunning},
	models.TaskCompleted:  {models.TaskArchived},
	models.TaskCancelled:  {models.TaskArchived},
	models.TaskDecomposed: {models.TaskCompleted, models.TaskFailed, models.TaskCancelled},
}

// Manager coordinates task distribution and state transitions.
type Manager struct {
	db       *sql.DB
	config   *config.Config
	goals    *models.GoalStore
	tasks    *models.TaskStore
	projects *models.ProjectStore
}

// NewManager creates a new Manager instance.
func NewManager(db *sql.DB, cfg *config.Config) *Manager {
	slog.Debug("creating new manager",
		"database_path", cfg.Database.Path,
		"max_total_pods", cfg.Orchestrator.MaxTotalPods)
	return &Manager{
		db:       db,
		config:   cfg,
		goals:    models.NewGoalStore(db),
		tasks:    models.NewTaskStore(db),
		projects: models.NewProjectStore(db),
	}
}

// PopNextTask atomically retrieves and claims the next READY task for the given pod type.
// Uses BEGIN IMMEDIATE transaction for SQLite to ensure exclusive access.
// Executor pods: any type except RESEARCH.
// Researcher pods: RESEARCH type only.
// Retries up to 5 times on SQLITE_BUSY to handle concurrent access.
func (m *Manager) PopNextTask(podType string) (*models.Task, error) {
	slog.Debug("pop next task started", "pod_type", podType)
	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		slog.Debug("pop next task attempt", "attempt", attempt+1, "max_retries", maxRetries)
		task, err := m.popNextTaskOnce(podType)
		if err == nil {
			if task != nil {
				slog.Info("task successfully popped",
					"task_id", task.ID,
					"title", task.Title,
					"type", task.Type,
					"pod_type", podType)
			} else {
				slog.Debug("no task available", "pod_type", podType)
			}
			return task, nil
		}
		// Retry on SQLite busy errors
		if isSQLiteBusy(err) {
			slog.Warn("sqlite busy, retrying",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"backoff_ms", 10*(attempt+1))
			time.Sleep(time.Duration(10*(attempt+1)) * time.Millisecond)
			continue
		}
		return nil, err
	}
	slog.Error("pop next task: max retries exceeded",
		"pod_type", podType,
		"max_retries", maxRetries)
	return nil, fmt.Errorf("pop next task: max retries exceeded due to database contention")
}

func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "database is locked") || strings.Contains(s, "SQLITE_BUSY")
}

func (m *Manager) popNextTaskOnce(podType string) (*models.Task, error) {
	// Get current goal ID for goal boost (tiebreaker) - BEFORE starting transaction
	currentGoalID := GetCurrentGoalID(m.goals)
	slog.Debug("pop next task once started", "current_goal_id", currentGoalID, "pod_type", podType)

	tx, err := m.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Build query based on pod type with goal boost and extended limit for dependency checking
	var query string
	if podType == "RESEARCHER" {
		query = `SELECT id, title, description, type, status, priority, source,
			project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
			result, error_log, executor_id, model, branch_name, pr_url, pr_status,
			diff_lines, files_changed, triage_analysis, plan, test_passed, retry_count,
			crash_recovery, tokens_used, cost_usd, created_at, updated_at, started_at,
			completed_at
			FROM tasks
			WHERE status = ? AND type = ?
			ORDER BY priority ASC,
				CASE WHEN goal_id = ? THEN 0 ELSE 1 END,
				created_at ASC
			LIMIT 10`
	} else {
		// Executor: any type except RESEARCH
		query = `SELECT id, title, description, type, status, priority, source,
			project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
			result, error_log, executor_id, model, branch_name, pr_url, pr_status,
			diff_lines, files_changed, triage_analysis, plan, test_passed, retry_count,
			crash_recovery, tokens_used, cost_usd, created_at, updated_at, started_at,
			completed_at
			FROM tasks
			WHERE status = ? AND type != ?
			ORDER BY priority ASC,
				CASE WHEN goal_id = ? THEN 0 ELSE 1 END,
				created_at ASC
			LIMIT 10`
	}

	// Query multiple candidates for dependency checking
	var rows *sql.Rows
	if podType == "RESEARCHER" {
		rows, err = tx.Query(query, models.TaskReady, models.TaskTypeResearch, currentGoalID)
	} else {
		rows, err = tx.Query(query, models.TaskReady, models.TaskTypeResearch, currentGoalID)
	}
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	// Find first task with met dependencies
	var task *models.Task
	for rows.Next() {
		candidate, err := scanTaskFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		slog.Debug("evaluating candidate task",
			"task_id", candidate.ID,
			"title", candidate.Title,
			"priority", candidate.Priority)

		// Check if dependencies are met
		met, err := m.areDependenciesMet(tx, candidate)
		if err != nil {
			return nil, fmt.Errorf("check dependencies: %w", err)
		}
		if met {
			task = candidate
			break
		} else {
			slog.Debug("dependency not met for candidate", "task_id", candidate.ID)
		}
	}

	if task == nil {
		return nil, nil // No task with met dependencies available
	}

	slog.Debug("claiming task", "task_id", task.ID, "title", task.Title)

	// Transition to RUNNING and set started_at
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`UPDATE tasks SET status = ?, started_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		models.TaskRunning, now, task.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update task status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// Update task object with new status
	task.Status = models.TaskRunning
	task.StartedAt = now

	return task, nil
}

// areDependenciesMet checks if all dependencies of a task are COMPLETED or ARCHIVED.
func (m *Manager) areDependenciesMet(tx *sql.Tx, task *models.Task) (bool, error) {
	if len(task.DependsOn) == 0 {
		return true, nil
	}

	for _, depID := range task.DependsOn {
		var status string
		err := tx.QueryRow(`SELECT status FROM tasks WHERE id = ?`, depID).Scan(&status)
		if err == sql.ErrNoRows {
			slog.Debug("dependency not found", "dep_id", depID, "task_id", task.ID)
			return false, fmt.Errorf("dependency not found: %s", depID)
		}
		if err != nil {
			return false, fmt.Errorf("query dependency %s: %w", depID, err)
		}

		slog.Debug("checking dependency", "dep_id", depID, "status", status, "task_id", task.ID)

		if status != models.TaskCompleted && status != models.TaskArchived {
			return false, nil
		}
	}

	return true, nil
}

// TransitionTask enforces valid state transitions and retry logic.
func (m *Manager) TransitionTask(taskID, newStatus string) error {
	task, err := m.tasks.GetByID(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	// Check if transition is valid
	validNextStates, ok := validTransitions[task.Status]
	if !ok {
		return fmt.Errorf("no valid transitions from status %s", task.Status)
	}

	isValid := false
	for _, state := range validNextStates {
		if state == newStatus {
			isValid = true
			break
		}
	}
	if !isValid {
		slog.Warn("invalid transition attempt",
			"task_id", taskID,
			"from_status", task.Status,
			"to_status", newStatus)
		return fmt.Errorf("invalid transition from %s to %s", task.Status, newStatus)
	}

	// RETRY validation
	if newStatus == models.TaskRetry {
		// Check if cancelled
		if task.ErrorLog == "cancelled by operator" {
			return fmt.Errorf("cannot retry cancelled task")
		}

		// Check retry count limit (unless crash recovery)
		if !task.CrashRecovery && task.RetryCount >= 3 {
			return fmt.Errorf("retry limit exceeded (max 3 retries)")
		}

		// Increment retry count unless crash recovery
		if !task.CrashRecovery {
			task.RetryCount++
		} else {
			// Reset crash recovery flag
			task.CrashRecovery = false
		}

		slog.Info("task retry",
			"task_id", taskID,
			"retry_count", task.RetryCount,
			"crash_recovery", task.CrashRecovery)
	}

	// Update status
	oldStatus := task.Status
	task.Status = newStatus

	// Set timestamps
	now := time.Now().UTC().Format(time.RFC3339)
	if newStatus == models.TaskRunning {
		task.StartedAt = now
	}
	if newStatus == models.TaskCompleted || newStatus == models.TaskFailed {
		task.CompletedAt = now
	}

	// Save to database
	if err := m.tasks.Update(task); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	slog.Info("task state transition",
		"task_id", taskID,
		"from_status", oldStatus,
		"to_status", newStatus)

	return nil
}

// CreateTask delegates to TaskStore.Create.
func (m *Manager) CreateTask(task *models.Task) error {
	err := m.tasks.Create(task)
	if err == nil {
		slog.Info("task created",
			"task_id", task.ID,
			"title", task.Title,
			"type", task.Type,
			"priority", task.Priority,
			"project_id", task.ProjectID)
	}
	return err
}

// GetCurrentGoal returns the single ACTIVE goal.
func (m *Manager) GetCurrentGoal() (*models.Goal, error) {
	goal, err := m.goals.GetCurrent()
	if err == nil {
		if goal != nil {
			slog.Debug("current goal retrieved", "goal_id", goal.ID, "title", goal.Title)
		} else {
			slog.Debug("no current goal found")
		}
	}
	return goal, err
}

// GetTask retrieves a task by ID.
func (m *Manager) GetTask(taskID string) (*models.Task, error) {
	return m.tasks.GetByID(taskID)
}

// GetProject retrieves a project by ID.
func (m *Manager) GetProject(projectID string) (*models.Project, error) {
	return m.projects.GetByID(projectID)
}

// PopNextPending atomically retrieves and claims the next PENDING task for the triager.
// Sets executor_id to the triager's ID to prevent other triagers from grabbing it.
func (m *Manager) PopNextPending(triagerID string) (*models.Task, error) {
	const maxRetries = 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		task, err := m.popNextPendingOnce(triagerID)
		if err == nil {
			return task, nil
		}
		if isSQLiteBusy(err) {
			time.Sleep(time.Duration(10*(attempt+1)) * time.Millisecond)
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("pop next pending: max retries exceeded")
}

func (m *Manager) popNextPendingOnce(triagerID string) (*models.Task, error) {
	tx, err := m.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `SELECT id, title, description, type, status, priority, source,
		project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
		result, error_log, executor_id, model, branch_name, pr_url, pr_status,
		diff_lines, files_changed, triage_analysis, plan, test_passed, retry_count,
		crash_recovery, tokens_used, cost_usd, created_at, updated_at, started_at,
		completed_at
		FROM tasks
		WHERE status = ? AND (executor_id = '' OR executor_id IS NULL)
		ORDER BY priority ASC, created_at ASC
		LIMIT 1`

	task, err := scanTaskFromRows(tx.QueryRow(query, models.TaskPending))
	if err != nil {
		return nil, nil // No pending task available
	}

	// Claim the task by setting executor_id to the triager
	_, err = tx.Exec(
		`UPDATE tasks SET executor_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		triagerID, task.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("claim pending task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	task.ExecutorID = triagerID
	slog.Info("triager claimed pending task", "task_id", task.ID, "triager_id", triagerID)
	return task, nil
}

// CheckParentCompletion checks if all subtasks of a parent are done
// and auto-transitions the parent accordingly.
func (m *Manager) CheckParentCompletion(parentID string) error {
	parent, err := m.tasks.GetByID(parentID)
	if err != nil {
		return fmt.Errorf("get parent: %w", err)
	}

	// Only auto-transition DECOMPOSED parents
	if parent.Status != models.TaskDecomposed {
		return nil
	}

	subtasks, err := m.tasks.ListByParent(parentID)
	if err != nil {
		return fmt.Errorf("list subtasks: %w", err)
	}
	if len(subtasks) == 0 {
		return nil
	}

	allDone := true
	anyFailed := false
	for _, sub := range subtasks {
		switch sub.Status {
		case models.TaskCompleted, models.TaskArchived:
			// done
		case models.TaskFailed:
			anyFailed = true
		case models.TaskCancelled:
			// treat as done
		default:
			allDone = false
		}
	}

	if !allDone {
		return nil
	}

	newStatus := models.TaskCompleted
	if anyFailed {
		newStatus = models.TaskFailed
	}

	slog.Info("auto-transitioning parent task", "parent_id", parentID, "new_status", newStatus)
	return m.TransitionTask(parentID, newStatus)
}

// scanTaskFromRow is a helper to scan a task from a sql.Row.
func scanTaskFromRow(row *sql.Row) (*models.Task, error) {
	var t models.Task
	var dependsOnJSON, tagsJSON string
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.Type, &t.Status, &t.Priority, &t.Source,
		&t.ProjectID, &t.ParentID, &t.Depth, &t.AlertID, &t.GoalID,
		&dependsOnJSON, &tagsJSON, &t.Prompt,
		&t.Result, &t.ErrorLog, &t.ExecutorID, &t.Model, &t.BranchName,
		&t.PRUrl, &t.PRStatus, &t.DiffLines, &t.FilesChanged,
		&t.TriageAnalysis, &t.Plan, &t.TestPassed,
		&t.RetryCount, &t.CrashRecovery, &t.TokensUsed, &t.CostUSD,
		&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if err := parseJSONField(dependsOnJSON, &t.DependsOn); err != nil {
		t.DependsOn = []string{}
	}
	if err := parseJSONField(tagsJSON, &t.Tags); err != nil {
		t.Tags = []string{}
	}

	return &t, nil
}
