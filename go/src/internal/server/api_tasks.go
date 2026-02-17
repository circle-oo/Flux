package server

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
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
	applyTaskDefaults(task, models.TaskSourceOperator)

	if err := s.tasks.Create(task); err != nil {
		serverError(w, "failed to create task", "error", err)
		return
	}

	// Broadcast task update via WebSocket
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	// All tasks stay PENDING until a triager picks them up and promotes to READY.
	slog.Info("task created as PENDING, awaiting triage", "task_id", task.ID)

	writeJSON(w, http.StatusCreated, task)
}

// handleListTasks handles GET /api/tasks
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page := queryInt(q, "page")
	limit := queryInt(q, "limit")

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
	tasks = sliceOrEmpty(tasks)
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

	task, err := s.tasks.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get task before delete", "id", id, "error", err)
		return
	}

	switch task.Status {
	case models.TaskArchived:
		// Already soft-deleted.
	case models.TaskCompleted, models.TaskFailed, models.TaskCancelled:
		if err := s.tasks.Archive(id); err != nil {
			serverError(w, "failed to archive task", "id", id, "error", err)
			return
		}
	default:
		if err := s.tasks.Cancel(id); err != nil {
			if strings.Contains(err.Error(), "not in a cancellable state") {
				writeError(w, http.StatusConflict, err.Error())
			} else {
				serverError(w, "failed to cancel task for soft delete", "id", id, "error", err)
			}
			return
		}
		if err := s.tasks.Archive(id); err != nil {
			serverError(w, "failed to archive task after cancel", "id", id, "error", err)
			return
		}
	}

	updatedTask, err := s.tasks.GetByID(id)
	if err != nil {
		serverError(w, "failed to get task after soft delete", "id", id, "error", err)
		return
	}
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: updatedTask})

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
	subtasks = sliceOrEmpty(subtasks)
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
	subtasks = sliceOrEmpty(subtasks)

	// Get dependencies
	dependencies, err := s.tasks.GetSubtaskDependencies(id)
	if err != nil {
		serverError(w, "failed to get subtask dependencies", "parent_id", id, "error", err)
		return
	}
	dependencies = sliceOrEmpty(dependencies)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"nodes": subtasks,
		"edges": dependencies,
	})
}

// handleTaskStats handles GET /api/tasks/stats
// Returns task counts for today and yesterday for delta indicators.
func (s *Server) handleTaskStats(w http.ResponseWriter, r *http.Request) {
	type StatusCounts struct {
		Completed int `json:"completed"`
		Failed    int `json:"failed"`
		Running   int `json:"running"`
		Ready     int `json:"ready"`
		Pending   int `json:"pending"`
	}

	scanCounts := func(dateFilter string) (StatusCounts, error) {
		var c StatusCounts
		rows, err := s.db.Query(fmt.Sprintf(`
			SELECT status, COUNT(*) FROM tasks
			WHERE %s
			GROUP BY status
		`, dateFilter))
		if err != nil {
			return c, err
		}
		defer rows.Close()
		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				return c, err
			}
			switch status {
			case "COMPLETED":
				c.Completed = count
			case "FAILED":
				c.Failed = count
			case "RUNNING":
				c.Running = count
			case "READY":
				c.Ready = count
			case "PENDING":
				c.Pending = count
			}
		}
		return c, rows.Err()
	}

	today, err := scanCounts("date(completed_at) = date('now') OR (status IN ('RUNNING','READY','PENDING') AND date(created_at) = date('now'))")
	if err != nil {
		serverError(w, "failed to get today stats", "error", err)
		return
	}

	yesterday, err := scanCounts("date(completed_at) = date('now', '-1 day') OR (date(created_at) = date('now', '-1 day') AND status NOT IN ('RUNNING','READY','PENDING'))")
	if err != nil {
		serverError(w, "failed to get yesterday stats", "error", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"today":     today,
		"yesterday": yesterday,
	})
}

// handleRetryTask handles POST /api/tasks/{id}/retry
func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Snapshot the current attempt before retry clears the fields
	task, err := s.tasks.GetByID(id)
	if err != nil {
		serverError(w, "failed to get task for attempt snapshot", "id", id, "error", err)
		return
	}
	// Reconcile usage from events before snapshotting, in case the task
	// row has stale zeros (prior bug: ReportTaskDone passed 0,0).
	if totalTokens, totalCost, err := s.taskUsageEvents.SumByTask(id); err == nil && (totalTokens > 0 || totalCost > 0) {
		task.TokensUsed = totalTokens
		task.CostUSD = totalCost
	}
	if err := s.taskAttempts.SaveAttempt(id, task); err != nil {
		slog.Error("failed to save attempt before retry", "task_id", id, "error", err)
		// Non-fatal: continue with retry
	}

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

	task, err = s.tasks.GetByID(id)
	if err != nil {
		serverError(w, "failed to get task after retry", "id", id, "error", err)
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}

// handleListAttempts handles GET /api/tasks/{id}/attempts
func (s *Server) handleListAttempts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	attempts, err := s.taskAttempts.ListByTask(id)
	if err != nil {
		serverError(w, "failed to list task attempts", "task_id", id, "error", err)
		return
	}
	attempts = sliceOrEmpty(attempts)

	// Use usage events as the source of truth for totals across all attempts.
	// This is more accurate than summing attempt snapshots (which may have stale zeros).
	totalTokens, totalCost, err := s.taskUsageEvents.SumByTask(id)
	if err != nil {
		slog.Error("failed to compute usage totals", "task_id", id, "error", err)
		// Fallback: sum from attempt snapshots + current task
		totalTokens, totalCost, _ = s.taskAttempts.TotalTokensAndCost(id)
		if task, err := s.tasks.GetByID(id); err == nil {
			totalTokens += task.TokensUsed
			totalCost += task.CostUSD
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"attempts":          attempts,
		"total_tokens_used": totalTokens,
		"total_cost_usd":    totalCost,
	})
}

// handleListUsageEvents handles GET /api/tasks/{id}/usage
func (s *Server) handleListUsageEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	events, err := s.taskUsageEvents.ListByTask(id)
	if err != nil {
		serverError(w, "failed to list usage events", "task_id", id, "error", err)
		return
	}
	events = sliceOrEmpty(events)

	totalTokens, totalCost, err := s.taskUsageEvents.SumByTask(id)
	if err != nil {
		serverError(w, "failed to sum usage", "task_id", id, "error", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"events":       events,
		"total_tokens": totalTokens,
		"total_cost":   totalCost,
	})
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
