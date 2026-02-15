package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/circle-oo/flux/internal/github"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// RegisterPRRoutes registers PR API routes. Called from server setup.
// NOTE: These routes must be added to setupRoutes() in server.go during integration.
func (s *Server) RegisterPRRoutes() {
	s.mux.Handle("GET /api/prs/pending", s.authMiddleware(http.HandlerFunc(s.handleListPendingPRs)))
	s.mux.Handle("GET /api/prs", s.authMiddleware(http.HandlerFunc(s.handleListPRs)))
	s.mux.Handle("POST /api/prs/{task_id}/approve", s.authMiddleware(http.HandlerFunc(s.handleApprovePR)))
	s.mux.Handle("POST /api/prs/{task_id}/request-changes", s.authMiddleware(http.HandlerFunc(s.handleRequestChanges)))
	s.mux.Handle("POST /api/prs/{task_id}/close", s.authMiddleware(http.HandlerFunc(s.handleClosePR)))
}

// handleListPendingPRs handles GET /api/prs/pending
// Returns tasks with pr_status = 'OPEN'.
func (s *Server) handleListPendingPRs(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(
		`SELECT id, title, description, type, status, priority, source,
		 project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
		 result, error_log, executor_id, model, branch_name, pr_url, pr_status,
		 diff_lines, files_changed, test_passed, retry_count, crash_recovery,
		 tokens_used, cost_usd, created_at, updated_at, started_at, completed_at
		 FROM tasks WHERE pr_status = ? ORDER BY priority ASC, created_at DESC`,
		"OPEN",
	)
	if err != nil {
		slog.Error("failed to query pending PRs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := scanPRTask(rows)
		if err != nil {
			slog.Error("failed to scan task row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
}

// handleListPRs handles GET /api/prs
// Returns tasks that have a PR, optionally filtered by pr_status query param.
// Supports: ?status=OPEN, ?status=MERGED, ?status=CHANGES_REQUESTED, ?status=CLOSED
// Without a status param, returns all tasks with a non-empty pr_status.
func (s *Server) handleListPRs(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	var rows *sql.Rows
	var err error

	if statusFilter != "" {
		rows, err = s.db.Query(
			`SELECT id, title, description, type, status, priority, source,
			 project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
			 result, error_log, executor_id, model, branch_name, pr_url, pr_status,
			 diff_lines, files_changed, test_passed, retry_count, crash_recovery,
			 tokens_used, cost_usd, created_at, updated_at, started_at, completed_at
			 FROM tasks WHERE pr_status = ? ORDER BY updated_at DESC, created_at DESC`,
			statusFilter,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, title, description, type, status, priority, source,
			 project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
			 result, error_log, executor_id, model, branch_name, pr_url, pr_status,
			 diff_lines, files_changed, test_passed, retry_count, crash_recovery,
			 tokens_used, cost_usd, created_at, updated_at, started_at, completed_at
			 FROM tasks WHERE pr_status != '' ORDER BY updated_at DESC, created_at DESC`,
		)
	}
	if err != nil {
		slog.Error("failed to query PRs", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := scanPRTask(rows)
		if err != nil {
			slog.Error("failed to scan task row", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		slog.Error("row iteration error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
}

// handleClosePR handles POST /api/prs/{task_id}/close
// Closes the PR on GitHub without merging and updates the task.
func (s *Server) handleClosePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := s.tasks.GetByID(taskID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if task.PRUrl == "" {
		writeError(w, http.StatusBadRequest, "task has no PR")
		return
	}

	prNumber, err := github.ExtractPRNumber(task.PRUrl)
	if err != nil {
		slog.Error("failed to extract PR number", "url", task.PRUrl, "error", err)
		writeError(w, http.StatusBadRequest, "invalid PR URL")
		return
	}

	project, err := s.projects.GetByID(task.ProjectID)
	if err != nil {
		slog.Error("failed to get project", "id", task.ProjectID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	owner, repo := extractOwnerRepoFromURL(project.RepoURL)
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid repo URL in project")
		return
	}

	client := github.NewClient(s.config.GitHub.Token, s.config.GitHub.Username)
	if err := client.ClosePR(owner, repo, prNumber); err != nil {
		slog.Error("failed to close PR", "pr", prNumber, "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("close failed: %v", err))
		return
	}

	task.PRStatus = "CLOSED"
	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task PR status", "id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.ws.Broadcast(Event{Type: EventPRStatus, Data: task})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "closed",
		"task":   task,
	})
}

// handleApprovePR handles POST /api/prs/{task_id}/approve
// Merges the PR and updates the task.
func (s *Server) handleApprovePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := s.tasks.GetByID(taskID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if task.PRUrl == "" {
		writeError(w, http.StatusBadRequest, "task has no PR")
		return
	}

	prNumber, err := github.ExtractPRNumber(task.PRUrl)
	if err != nil {
		slog.Error("failed to extract PR number", "url", task.PRUrl, "error", err)
		writeError(w, http.StatusBadRequest, "invalid PR URL")
		return
	}

	// Get project to extract owner/repo
	project, err := s.projects.GetByID(task.ProjectID)
	if err != nil {
		slog.Error("failed to get project", "id", task.ProjectID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	owner, repo := extractOwnerRepoFromURL(project.RepoURL)
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid repo URL in project")
		return
	}

	client := github.NewClient(s.config.GitHub.Token, s.config.GitHub.Username)
	if err := client.MergePR(owner, repo, prNumber); err != nil {
		slog.Error("failed to merge PR", "pr", prNumber, "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("merge failed: %v", err))
		return
	}

	task.PRStatus = "MERGED"
	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task PR status", "id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.ws.Broadcast(Event{Type: EventPRStatus, Data: task})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "merged",
		"task":   task,
	})
}

// handleRequestChanges handles POST /api/prs/{task_id}/request-changes
// Fetches PR comments, creates a fix task, and marks original as CHANGES_REQUESTED.
func (s *Server) handleRequestChanges(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := s.tasks.GetByID(taskID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if task.PRUrl == "" {
		writeError(w, http.StatusBadRequest, "task has no PR")
		return
	}

	prNumber, err := github.ExtractPRNumber(task.PRUrl)
	if err != nil {
		slog.Error("failed to extract PR number", "url", task.PRUrl, "error", err)
		writeError(w, http.StatusBadRequest, "invalid PR URL")
		return
	}

	// Get project to extract owner/repo
	project, err := s.projects.GetByID(task.ProjectID)
	if err != nil {
		slog.Error("failed to get project", "id", task.ProjectID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	owner, repo := extractOwnerRepoFromURL(project.RepoURL)
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid repo URL in project")
		return
	}

	// Fetch PR comments
	client := github.NewClient(s.config.GitHub.Token, s.config.GitHub.Username)
	comments, err := client.FetchPRComments(owner, repo, prNumber)
	if err != nil {
		slog.Error("failed to fetch PR comments", "pr", prNumber, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch comments")
		return
	}

	// Build comment summary
	var commentSummary strings.Builder
	commentSummary.WriteString(fmt.Sprintf("## PR Review Comments for #%d\n\n", prNumber))
	if len(comments) == 0 {
		commentSummary.WriteString("No comments found. Please review the PR and address any issues.\n")
	} else {
		for _, c := range comments {
			commentSummary.WriteString(fmt.Sprintf("**%s** (%s):\n%s\n\n---\n\n", c.Author, c.CreatedAt, c.Body))
		}
	}

	// Create fix task
	fixTask := &models.Task{
		Title:       fmt.Sprintf("PR fix: %s", task.Title),
		Description: commentSummary.String(),
		Type:        task.Type,
		Priority:    6,
		Source:      models.TaskSourceOperator,
		Status:      models.TaskReady,
		ProjectID:   task.ProjectID,
		GoalID:      task.GoalID,
		BranchName:  task.BranchName,
		PRUrl:       task.PRUrl,
		ParentID:    task.ID,
	}

	if err := s.tasks.Create(fixTask); err != nil {
		slog.Error("failed to create fix task", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create fix task")
		return
	}

	// Update original task
	task.PRStatus = "CHANGES_REQUESTED"
	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update original task", "id", taskID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Notify via Discord
	if s.notifier != nil {
		_ = s.notifier.Send(notifier.LevelInfo,
			fmt.Sprintf("Changes requested on PR #%d - fix task %s created", prNumber, fixTask.ID))
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: fixTask})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "changes_requested",
		"fix_task_id": fixTask.ID,
		"fix_task":    fixTask,
	})
}

// extractOwnerRepoFromURL parses owner and repo from a GitHub URL.
func extractOwnerRepoFromURL(repoURL string) (owner, repo string) {
	// HTTPS: https://github.com/owner/repo.git or https://github.com/owner/repo
	repoURL = strings.TrimSuffix(repoURL, ".git")

	if strings.Contains(repoURL, "github.com/") {
		parts := strings.Split(repoURL, "github.com/")
		if len(parts) >= 2 {
			segments := strings.SplitN(parts[1], "/", 2)
			if len(segments) == 2 {
				return segments[0], segments[1]
			}
		}
	}

	// SSH: git@github.com:owner/repo.git
	if strings.Contains(repoURL, "github.com:") {
		parts := strings.Split(repoURL, "github.com:")
		if len(parts) >= 2 {
			segments := strings.SplitN(parts[1], "/", 2)
			if len(segments) == 2 {
				return segments[0], segments[1]
			}
		}
	}

	return "", ""
}

// scanPRTask scans a task row from the database. Duplicated here to avoid
// exporting internal scan functions from the models package.
func scanPRTask(rows *sql.Rows) (*models.Task, error) {
	var t models.Task
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

	// Parse JSON arrays; ignore errors to be resilient to bad data
	if dependsOnJSON != "" {
		_ = json.Unmarshal([]byte(dependsOnJSON), &t.DependsOn)
	}
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &t.Tags)
	}
	if t.DependsOn == nil {
		t.DependsOn = []string{}
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}

	return &t, nil
}
