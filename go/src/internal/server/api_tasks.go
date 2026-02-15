package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/triager"
)

// handleCreateTask handles POST /api/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Type        string   `json:"type"`
		Priority    int      `json:"priority"`
		Source      string   `json:"source"`
		ProjectID   string   `json:"project_id"`
		GoalID      string   `json:"goal_id"`
		DependsOn   []string `json:"depends_on"`
		Tags        []string `json:"tags"`
		Prompt      string   `json:"prompt"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	task := &models.Task{
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Priority:    req.Priority,
		Source:      req.Source,
		ProjectID:   req.ProjectID,
		GoalID:      req.GoalID,
		DependsOn:   req.DependsOn,
		Tags:        req.Tags,
		Prompt:      req.Prompt,
	}
	if task.Priority == 0 {
		task.Priority = 50
	}
	if task.Source == "" {
		task.Source = models.TaskSourceOperator
	}

	if err := s.tasks.Create(task); err != nil {
		slog.Error("failed to create task", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Broadcast task update via WebSocket
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	// If triager is enabled, operator tasks stay PENDING for the triager to pick up.
	// If triager is disabled, run inline triage or promote directly.
	if task.Source == models.TaskSourceOperator && task.Status == models.TaskPending {
		if s.config.Triager.Enabled {
			// Triager component will poll and pick up PENDING tasks
			slog.Info("task created as PENDING, triager will process", "task_id", task.ID)
		} else if s.config.Executor.MaxExecutionTime > 0 {
			// Fallback: inline triage if triager is disabled but executor is configured
			go s.triageTask(task)
		} else {
			// No triage available — promote directly to READY
			s.promoteToReady(task)
		}
	}

	writeJSON(w, http.StatusCreated, task)
}

// handleListTasks handles GET /api/tasks
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	filter := models.ListFilter{
		Status:          q.Get("status"),
		ProjectID:       q.Get("project_id"),
		ExcludeSubtasks: q.Get("exclude_subtasks") == "true",
		Page:            page,
		Limit:           limit,
	}

	tasks, err := s.tasks.List(filter)
	if err != nil {
		slog.Error("failed to list tasks", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": tasks})
}

// handleGetTask handles GET /api/tasks/{id}
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	task, err := s.tasks.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleUpdateTask handles PATCH /api/tasks/{id}
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	task, err := s.tasks.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		slog.Error("failed to get task", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var req struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Type        *string  `json:"type"`
		Status      *string  `json:"status"`
		Priority    *int     `json:"priority"`
		ProjectID   *string  `json:"project_id"`
		GoalID      *string  `json:"goal_id"`
		DependsOn   []string `json:"depends_on"`
		Tags        []string `json:"tags"`
		Prompt      *string  `json:"prompt"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Type != nil {
		task.Type = *req.Type
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.ProjectID != nil {
		task.ProjectID = *req.ProjectID
	}
	if req.GoalID != nil {
		task.GoalID = *req.GoalID
	}
	if req.DependsOn != nil {
		task.DependsOn = req.DependsOn
	}
	if req.Tags != nil {
		task.Tags = req.Tags
	}
	if req.Prompt != nil {
		task.Prompt = *req.Prompt
	}

	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}

// handleDeleteTask handles DELETE /api/tasks/{id}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Delete(id); err != nil {
		slog.Error("failed to delete task", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleCancelTask handles POST /api/tasks/{id}/cancel
func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Cancel(id); err != nil {
		slog.Error("failed to cancel task", "id", id, "error", err)
		if strings.Contains(err.Error(), "not in a cancellable state") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		slog.Error("failed to get task after cancel", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	slog.Info("task cancelled by operator", "task_id", id, "title", task.Title)

	// Cascade cancel to children
	cancelled, err := s.tasks.CancelChildren(id)
	if err != nil {
		slog.Error("failed to cascade cancel to children", "task_id", id, "error", err)
	} else if cancelled > 0 {
		slog.Info("cascade cancelled subtasks", "parent_id", id, "count", cancelled)
	}

	if s.notifier != nil {
		s.notifier.Send("info", fmt.Sprintf("Task cancelled: %s", task.Title))
	}
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}

// handleListSubtasks handles GET /api/tasks/{id}/subtasks
func (s *Server) handleListSubtasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	subtasks, err := s.tasks.ListByParent(id)
	if err != nil {
		slog.Error("failed to list subtasks", "parent_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if subtasks == nil {
		subtasks = []*models.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": subtasks})
}

// triageTask runs async triage on a task using Claude to analyze requirements,
// rewrite the description, and suggest priority. After triage completes (or fails),
// the task is moved from PENDING to READY so the executor can pick it up.
// Concurrency is limited by triageSem (max 10 concurrent triages).
func (s *Server) triageTask(task *models.Task) {
	// Acquire semaphore slot (blocks if 10 triages are already running)
	s.triageSem <- struct{}{}
	defer func() { <-s.triageSem }()

	slog.Info("starting async triage", "task_id", task.ID, "title", task.Title)

	runner := executor.NewClaudeCodeRunner(&s.config.Executor)
	ctx := context.Background()

	result, err := triager.TriageTask(ctx, runner, task)
	if err != nil {
		slog.Warn("triage failed, task will use original description", "task_id", task.ID, "error", err)
		// Even on failure, move PENDING -> READY so the task doesn't get stuck
		s.promoteToReady(task)
		return
	}

	// Update task with triage results
	task.TriageAnalysis = result.Analysis
	if result.Description != "" && result.Description != task.Description {
		task.Description = result.Description
	}
	if result.Priority != task.Priority {
		slog.Info("triage adjusted priority", "task_id", task.ID, "old", task.Priority, "new", result.Priority)
		task.Priority = result.Priority
	}

	// Move to READY after triage
	task.Status = models.TaskReady

	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task after triage", "task_id", task.ID, "error", err)
		return
	}

	slog.Info("triage complete, task updated", "task_id", task.ID)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})
}

// promoteToReady moves a PENDING task to READY status.
func (s *Server) promoteToReady(task *models.Task) {
	task.Status = models.TaskReady
	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to promote task to READY", "task_id", task.ID, "error", err)
		return
	}
	slog.Info("task promoted to READY", "task_id", task.ID)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})
}

// handleRetryTask handles POST /api/tasks/{id}/retry
func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Retry(id); err != nil {
		slog.Error("failed to retry task", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		slog.Error("failed to get task after retry", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}

// handleArchiveTask handles POST /api/tasks/{id}/archive
func (s *Server) handleArchiveTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Archive(id); err != nil {
		slog.Error("failed to archive task", "id", id, "error", err)
		if strings.Contains(err.Error(), "not in an archivable state") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		slog.Error("failed to get task after archive", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	slog.Info("task archived", "task_id", id, "title", task.Title)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}
