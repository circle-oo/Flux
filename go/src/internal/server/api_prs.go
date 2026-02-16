package server

import (
	"database/sql"
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
	s.mux.Handle("POST /api/prs/{task_id}/approve", s.authMiddleware(http.HandlerFunc(s.handleApprovePR)))
	s.mux.Handle("POST /api/prs/{task_id}/request-changes", s.authMiddleware(http.HandlerFunc(s.handleRequestChanges)))
	s.mux.Handle("POST /api/prs/{task_id}/close", s.authMiddleware(http.HandlerFunc(s.handleClosePR)))
}

// handleListPendingPRs handles GET /api/prs/pending
// Returns tasks with pr_url set, optionally filtered by pr_status query parameter.
// If no status filter is provided, returns all PRs regardless of status.
func (s *Server) handleListPendingPRs(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")

	query := models.TaskSelectSQL + " WHERE pr_url != ''"

	var args []interface{}
	if statusFilter != "" {
		query += " AND pr_status = ?"
		args = append(args, statusFilter)
	}

	query += " ORDER BY priority ASC, created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		serverError(w, "failed to query pending PRs", "error", err)
		return
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := models.ScanTask(rows)
		if err != nil {
			serverError(w, "failed to scan task row", "error", err)
			return
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		serverError(w, "row iteration error", "error", err)
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
}

// handleApprovePR handles POST /api/prs/{task_id}/approve
// Merges the PR and updates the task.
func (s *Server) handleApprovePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := s.tasks.GetByID(taskID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get task", "id", taskID, "error", err)
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
		serverError(w, "failed to get project", "id", task.ProjectID, "error", err)
		return
	}

	owner, repo := extractOwnerRepoFromURL(project.RepoURL)
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid repo URL in project")
		return
	}

	if s.ghClient == nil {
		writeError(w, http.StatusServiceUnavailable, "GitHub not configured")
		return
	}

	if err := s.ghClient.MergePR(owner, repo, prNumber); err != nil {
		slog.Error("failed to merge PR", "pr", prNumber, "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("merge failed: %v", err))
		return
	}

	task.PRStatus = "MERGED"
	if err := s.tasks.Update(task); err != nil {
		serverError(w, "failed to update task PR status", "id", taskID, "error", err)
		return
	}

	s.ws.Broadcast(Event{Type: EventPRStatus, Data: task})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "merged",
		"task":   task,
	})
}

// handleClosePR handles POST /api/prs/{task_id}/close
// Closes the PR on GitHub and updates the task.
func (s *Server) handleClosePR(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := s.tasks.GetByID(taskID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get task", "id", taskID, "error", err)
		return
	}

	if task.PRUrl == "" {
		writeError(w, http.StatusBadRequest, "task has no PR")
		return
	}

	if task.PRStatus == "MERGED" || task.PRStatus == "CLOSED" {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("PR is already %s", task.PRStatus))
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
		serverError(w, "failed to get project", "id", task.ProjectID, "error", err)
		return
	}

	owner, repo := extractOwnerRepoFromURL(project.RepoURL)
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid repo URL in project")
		return
	}

	if s.ghClient == nil {
		writeError(w, http.StatusServiceUnavailable, "GitHub not configured")
		return
	}

	if err := s.ghClient.ClosePR(owner, repo, prNumber); err != nil {
		slog.Error("failed to close PR", "pr", prNumber, "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("close failed: %v", err))
		return
	}

	task.PRStatus = "CLOSED"
	if err := s.tasks.Update(task); err != nil {
		serverError(w, "failed to update task PR status", "id", taskID, "error", err)
		return
	}

	s.ws.Broadcast(Event{Type: EventPRStatus, Data: task})

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "closed",
		"task":   task,
	})
}

// handleRequestChanges handles POST /api/prs/{task_id}/request-changes
// Fetches PR comments, creates a fix task, and marks original as CHANGES_REQUESTED.
func (s *Server) handleRequestChanges(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	task, err := s.tasks.GetByID(taskID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get task", "id", taskID, "error", err)
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
		serverError(w, "failed to get project", "id", task.ProjectID, "error", err)
		return
	}

	owner, repo := extractOwnerRepoFromURL(project.RepoURL)
	if owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "invalid repo URL in project")
		return
	}

	if s.ghClient == nil {
		writeError(w, http.StatusServiceUnavailable, "GitHub not configured")
		return
	}

	// Fetch PR comments
	comments, err := s.ghClient.FetchPRComments(owner, repo, prNumber)
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
		serverError(w, "failed to create fix task", "error", err)
		return
	}

	// Update original task
	task.PRStatus = "CHANGES_REQUESTED"
	if err := s.tasks.Update(task); err != nil {
		serverError(w, "failed to update original task", "id", taskID, "error", err)
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
// TODO(integration): Replace with github.ExtractOwnerRepo after integration phase
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
