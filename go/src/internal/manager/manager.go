package manager

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/executor"
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

// PopNextTask atomically retrieves and claims the next READY task.
// Uses BEGIN IMMEDIATE transaction for SQLite to ensure exclusive access.
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
	// Include READY, RETRY, and FAILED (but retryable) tasks
	// FAILED tasks are auto-retried if retry_count < max_retries
	orderAndLimit := `
			ORDER BY priority ASC,
				CASE WHEN goal_id = ? THEN 0 ELSE 1 END,
				created_at ASC
			LIMIT 10`
	query := models.TaskSelectSQL + " WHERE (status IN (?, ?) OR (status = ? AND retry_count < max_retries))" + orderAndLimit
	args := []interface{}{models.TaskReady, models.TaskRetry, models.TaskFailed, currentGoalID}

	// Query multiple candidates for dependency checking
	var rows *sql.Rows
	rows, err = tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	// Find first task with met dependencies
	var task *models.Task
	for rows.Next() {
		candidate, err := models.ScanTask(rows)
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

	slog.Debug("claiming task", "task_id", task.ID, "title", task.Title, "current_status", task.Status)

	// If task is FAILED but retryable, transition to RETRY first, then to RUNNING
	if task.Status == models.TaskFailed && task.IsRetryable() {
		slog.Info("auto-retrying failed task",
			"task_id", task.ID,
			"retry_count", task.RetryCount,
			"max_retries", task.MaxRetries)

		// Increment retry count and clear previous failure data
		_, err = tx.Exec(
			`UPDATE tasks SET status = ?, retry_count = retry_count + 1,
			 error_log = '', result = '', started_at = '', completed_at = '',
			 updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			models.TaskRetry, task.ID,
		)
		if err != nil {
			return nil, fmt.Errorf("update task to retry: %w", err)
		}
		task.RetryCount++
		task.Status = models.TaskRetry
		task.ErrorLog = ""
		task.Result = ""
		task.StartedAt = ""
		task.CompletedAt = ""
	}

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
// For subtasks (tasks with parent_id), this checks both:
// 1. Task-level dependencies (depends_on field) - existing behavior
// 2. Subtask dependencies within the same parent - if depends_on references sibling subtasks
func (m *Manager) areDependenciesMet(tx *sql.Tx, task *models.Task) (bool, error) {
	if len(task.DependsOn) == 0 {
		return true, nil
	}

	for _, depID := range task.DependsOn {
		var status string
		var parentID string
		err := tx.QueryRow(`SELECT status, parent_id FROM tasks WHERE id = ?`, depID).Scan(&status, &parentID)
		if err == sql.ErrNoRows {
			slog.Debug("dependency not found", "dep_id", depID, "task_id", task.ID)
			return false, fmt.Errorf("dependency not found: %s", depID)
		}
		if err != nil {
			return false, fmt.Errorf("query dependency %s: %w", depID, err)
		}

		// Log whether this is a sibling subtask dependency or a regular task dependency
		if task.ParentID != "" && parentID == task.ParentID {
			slog.Debug("checking subtask dependency (sibling)", "dep_id", depID, "status", status, "task_id", task.ID, "parent_id", task.ParentID)
		} else {
			slog.Debug("checking task dependency", "dep_id", depID, "status", status, "task_id", task.ID)
		}

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
		if !task.CrashRecovery && task.RetryCount >= task.MaxRetries {
			return fmt.Errorf("retry limit exceeded (max %d retries)", task.MaxRetries)
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
			"max_retries", task.MaxRetries,
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

	query := models.TaskSelectSQL + `
		WHERE status = ? AND (executor_id = '' OR executor_id IS NULL)
		ORDER BY priority ASC, created_at ASC
		LIMIT 1`

	task, err := models.ScanTask(tx.QueryRow(query, models.TaskPending))
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

// CheckParentCompletion checks if all subtasks of a parent are done,
// merges their branches into the parent branch, and auto-transitions the parent accordingly.
// Parent failure is deferred until all retry options are exhausted for failed subtasks.
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

	// Check completion status and merge completed subtasks
	allInTerminalState := true
	anyFailedPermanently := false
	anyRetryable := false

	for _, sub := range subtasks {
		switch sub.Status {
		case models.TaskCompleted, models.TaskArchived:
			// Merge completed subtask branch into parent branch
			if err := m.MergeCompletedSubtask(parent, sub); err != nil {
				slog.Error("failed to merge subtask into parent",
					"parent_id", parentID,
					"subtask_id", sub.ID,
					"error", err)
				// Continue processing other subtasks even if one merge fails
			}
		case models.TaskFailed:
			// Check if this failed task is retryable
			if sub.IsRetryable() {
				anyRetryable = true
				allInTerminalState = false
				slog.Debug("subtask is retryable, deferring parent failure",
					"parent_id", parentID,
					"subtask_id", sub.ID,
					"retry_count", sub.RetryCount,
					"max_retries", sub.MaxRetries)
			} else {
				// Failed with retries exhausted
				anyFailedPermanently = true
				slog.Debug("subtask failed permanently",
					"parent_id", parentID,
					"subtask_id", sub.ID,
					"retry_count", sub.RetryCount,
					"max_retries", sub.MaxRetries)
			}
		case models.TaskCancelled:
			// treat as done (terminal state)
		default:
			// Task is still pending, ready, running, or decomposed
			allInTerminalState = false
		}
	}

	// If any subtasks are still retryable or not in terminal state, keep parent as DECOMPOSED
	if !allInTerminalState {
		if anyRetryable {
			slog.Info("parent task still has retryable subtasks, keeping DECOMPOSED",
				"parent_id", parentID)
		} else {
			slog.Debug("parent task has non-terminal subtasks, keeping DECOMPOSED",
				"parent_id", parentID)
		}
		return nil
	}

	// All subtasks are in terminal states - aggregate results and transition parent
	if err := m.AggregateSubtaskResults(parentID); err != nil {
		slog.Error("failed to aggregate subtask results", "parent_id", parentID, "error", err)
		// Continue with transition even if aggregation fails
	}

	newStatus := models.TaskCompleted
	if anyFailedPermanently {
		newStatus = models.TaskFailed
	}

	slog.Info("auto-transitioning parent task", "parent_id", parentID, "new_status", newStatus)
	return m.TransitionTask(parentID, newStatus)
}

// AggregateSubtaskResults collects results from all completed subtasks
// and stores a unified summary in the parent task's Result field.
func (m *Manager) AggregateSubtaskResults(parentID string) error {
	parent, err := m.tasks.GetByID(parentID)
	if err != nil {
		return fmt.Errorf("get parent: %w", err)
	}

	subtasks, err := m.tasks.ListByParent(parentID)
	if err != nil {
		return fmt.Errorf("list subtasks: %w", err)
	}

	if len(subtasks) == 0 {
		return nil
	}

	// Build aggregated result as structured markdown
	var result strings.Builder
	result.WriteString(fmt.Sprintf("# Subtask Results Summary\n\n"))
	result.WriteString(fmt.Sprintf("Total subtasks: %d\n\n", len(subtasks)))

	completedCount := 0
	failedCount := 0
	cancelledCount := 0

	for i, sub := range subtasks {
		result.WriteString(fmt.Sprintf("## Subtask %d: %s\n\n", i+1, sub.Title))
		result.WriteString(fmt.Sprintf("**Status:** %s\n\n", sub.Status))

		if sub.Description != "" {
			result.WriteString(fmt.Sprintf("**Description:** %s\n\n", sub.Description))
		}

		if sub.Result != "" {
			result.WriteString(fmt.Sprintf("**Result:**\n%s\n\n", sub.Result))
		}

		if sub.ErrorLog != "" {
			result.WriteString(fmt.Sprintf("**Error:** %s\n\n", sub.ErrorLog))
		}

		// Track status counts
		switch sub.Status {
		case models.TaskCompleted, models.TaskArchived:
			completedCount++
		case models.TaskFailed:
			failedCount++
		case models.TaskCancelled:
			cancelledCount++
		}

		result.WriteString("---\n\n")
	}

	// Add summary statistics
	result.WriteString("## Overall Status\n\n")
	result.WriteString(fmt.Sprintf("- Completed: %d\n", completedCount))
	result.WriteString(fmt.Sprintf("- Failed: %d\n", failedCount))
	result.WriteString(fmt.Sprintf("- Cancelled: %d\n", cancelledCount))

	// Update parent task with aggregated result
	parent.Result = result.String()
	if err := m.tasks.Update(parent); err != nil {
		return fmt.Errorf("update parent result: %w", err)
	}

	slog.Info("aggregated subtask results",
		"parent_id", parentID,
		"subtask_count", len(subtasks),
		"completed", completedCount,
		"failed", failedCount,
		"cancelled", cancelledCount)

	return nil
}

// MergeCompletedSubtask merges a completed subtask's branch into the parent task's branch.
// This function ensures that subtask work is integrated into the parent before the parent creates a PR.
func (m *Manager) MergeCompletedSubtask(parent, subtask *models.Task) error {
	// Skip merge if subtask didn't produce a branch (decomposed tasks, for example)
	if subtask.BranchName == "" {
		slog.Debug("subtask has no branch, skipping merge", "subtask_id", subtask.ID)
		return nil
	}

	// Skip merge if parent has no branch yet
	if parent.BranchName == "" {
		slog.Warn("parent has no branch, cannot merge subtask",
			"parent_id", parent.ID,
			"subtask_id", subtask.ID)
		return fmt.Errorf("parent task has no branch")
	}

	// Get parent project to construct project name
	project, err := m.projects.GetByID(parent.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to get parent project: %w", err)
	}

	// Create worktree manager to perform the merge
	wm := executor.NewWorktreeManager(
		m.config.Orchestrator.WorkspaceBase,
		m.config.GitHub.Token,
		m.config.GitHub.Username,
	)

	slog.Info("merging subtask branch into parent",
		"parent_id", parent.ID,
		"parent_branch", parent.BranchName,
		"subtask_id", subtask.ID,
		"subtask_branch", subtask.BranchName,
		"project", project.Name)

	// Perform the merge
	if err := wm.MergeSubtaskIntoParent(project.Name, parent.ID, subtask.ID); err != nil {
		return fmt.Errorf("worktree merge failed: %w", err)
	}

	slog.Info("successfully merged subtask into parent",
		"parent_id", parent.ID,
		"subtask_id", subtask.ID)

	return nil
}

