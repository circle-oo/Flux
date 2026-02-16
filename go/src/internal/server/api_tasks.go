package server

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/circle-oo/flux/internal/models"
)

// handleCreateTask handles POST /api/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priority    int      `json:"priority"`
		Source      string   `json:"source"`
		ProjectID   string   `json:"project_id"`
		GoalID      string   `json:"goal_id"`
		DependsOn   []string `json:"depends_on"`
		Tags        []string `json:"tags"`
		Prompt      string   `json:"prompt"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	// Validate input for security
	if err := ValidateTaskInput(req.Title, req.Description); err != nil {
		slog.Warn("task creation validation failed", "error", err, "title_len", len(req.Title), "desc_len", len(req.Description))
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Prompt != "" {
		if err := ValidatePrompt(req.Prompt); err != nil {
			slog.Warn("task creation prompt validation failed", "error", err, "prompt_len", len(req.Prompt))
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	task := &models.Task{
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Source:      req.Source,
		ProjectID:   req.ProjectID,
		GoalID:      req.GoalID,
		DependsOn:   req.DependsOn,
		Tags:        req.Tags,
		Prompt:      req.Prompt,
	}
	if task.Priority == 0 {
		task.Priority = 40
	}
	if task.Source == "" {
		task.Source = models.TaskSourceOperator
	}

	if err := s.tasks.Create(task); err != nil {
		serverError(w, "failed to create task", "error", err)
		return
	}

	// Broadcast task update via WebSocket
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	// If triager is enabled, operator tasks stay PENDING for the triager to pick up.
	// If triager is disabled but executor is configured, run inline triage.
	// Otherwise, promote directly to READY.
	if task.Source == models.TaskSourceOperator && task.Status == models.TaskPending {
		if s.config.Triager.Enabled {
			// Triager component will poll and pick up PENDING tasks
			slog.Info("task created as PENDING, triager will process", "task_id", task.ID)
		} else {
			// No triage available — promote directly to READY
			s.promoteToReady(task.ID)
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
		serverError(w, "failed to list tasks", "error", err)
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
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get task", "id", id, "error", err)
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// handleUpdateTask handles PATCH /api/tasks/{id}
func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	task, err := s.tasks.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get task", "id", id, "error", err)
		return
	}

	var req struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Status      *string  `json:"status"`
		Priority    *int     `json:"priority"`
		ProjectID   *string  `json:"project_id"`
		GoalID      *string  `json:"goal_id"`
		DependsOn   []string `json:"depends_on"`
		Tags        []string `json:"tags"`
		Prompt      *string  `json:"prompt"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}

	// Validate updated fields
	if err := ValidateTaskInput(task.Title, task.Description); err != nil {
		slog.Warn("task update validation failed", "task_id", id, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
		serverError(w, "failed to update task", "id", id, "error", err)
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}

// handleDeleteTask handles DELETE /api/tasks/{id}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Delete(id); err != nil {
		serverError(w, "failed to delete task", "id", id, "error", err)
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
			writeError(w, http.StatusInternalServerError, errInternalServer)
		}
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		serverError(w, "failed to get task after cancel", "id", id, "error", err)
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

	// Archive all subtasks after cancelling
	archived, err := s.tasks.ArchiveChildren(id)
	if err != nil {
		slog.Error("failed to archive subtasks", "task_id", id, "error", err)
	} else if archived > 0 {
		slog.Info("archived subtasks after cancel", "parent_id", id, "count", archived)
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
		serverError(w, "failed to list subtasks", "parent_id", id, "error", err)
		return
	}
	if subtasks == nil {
		subtasks = []*models.Task{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": subtasks})
}

// handleGetSubtaskDependencies handles GET /api/tasks/{id}/subtasks/dependencies
func (s *Server) handleGetSubtaskDependencies(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Get subtasks
	subtasks, err := s.tasks.ListByParent(id)
	if err != nil {
		serverError(w, "failed to list subtasks", "parent_id", id, "error", err)
		return
	}
	if subtasks == nil {
		subtasks = []*models.Task{}
	}

	// Get dependencies
	dependencies, err := s.tasks.GetSubtaskDependencies(id)
	if err != nil {
		serverError(w, "failed to get subtask dependencies", "parent_id", id, "error", err)
		return
	}
	if dependencies == nil {
		dependencies = []*models.SubtaskDependency{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": subtasks,
		"edges": dependencies,
	})
}

// promoteToReady moves a PENDING task to READY status.
// Takes taskID and re-reads from DB to avoid stale state.
func (s *Server) promoteToReady(taskID string) {
	task, err := s.tasks.GetByID(taskID)
	if err != nil {
		slog.Error("promoteToReady: failed to read task", "task_id", taskID, "error", err)
		return
	}
	task.Status = models.TaskReady
	task.ExecutorID = "" // Clear any claim
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

	// Archive all previous subtasks before retrying
	archived, err := s.tasks.ArchiveChildren(id)
	if err != nil {
		slog.Error("failed to archive subtasks before retry", "task_id", id, "error", err)
	} else if archived > 0 {
		slog.Info("archived subtasks before retry", "parent_id", id, "count", archived)
	}

	if err := s.tasks.Retry(id); err != nil {
		slog.Error("failed to retry task", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		serverError(w, "failed to get task after retry", "id", id, "error", err)
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
			writeError(w, http.StatusInternalServerError, errInternalServer)
		}
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		serverError(w, "failed to get task after archive", "id", id, "error", err)
		return
	}

	slog.Info("task archived", "task_id", id, "title", task.Title)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}
