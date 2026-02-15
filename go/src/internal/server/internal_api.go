package server

import (
	"log/slog"
	"net/http"

	"github.com/circle-oo/flux/internal/models"
)

// handleInternalNextTask handles POST /internal/tasks/next
// Pod requests next task from Manager.
// Phase 1 stub: always returns null task.
func (s *Server) handleInternalNextTask(w http.ResponseWriter, r *http.Request) {
	// Phase 1 stub: no task queue management yet
	writeJSON(w, http.StatusOK, map[string]interface{}{"task": nil})
}

// handleInternalTaskDone handles POST /internal/tasks/{id}/done
// Pod reports task completion.
func (s *Server) handleInternalTaskDone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Status    string  `json:"status"`
		Result    string  `json:"result"`
		ErrorLog  string  `json:"error_log"`
		TokensUsed int    `json:"tokens_used"`
		CostUSD   float64 `json:"cost_usd"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	if req.Status != "" {
		task.Status = req.Status
	}
	if req.Result != "" {
		task.Result = req.Result
	}
	if req.ErrorLog != "" {
		task.ErrorLog = req.ErrorLog
	}
	task.TokensUsed = req.TokensUsed
	task.CostUSD = req.CostUSD

	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleInternalCreateSubtasks handles POST /internal/subtasks
// Executor creates subtasks via Manager. Validates depth <= 1, max 5 per parent.
func (s *Server) handleInternalCreateSubtasks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID string `json:"parent_id"`
		Subtasks []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"subtasks"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ParentID == "" {
		writeError(w, http.StatusBadRequest, "parent_id is required")
		return
	}

	// Get parent task to inherit properties and check depth
	parent, err := s.tasks.GetByID(req.ParentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "parent task not found")
		return
	}

	// Validate depth: subtask depth must be <= 1
	if parent.Depth >= s.config.Subtask.MaxDepth {
		writeError(w, http.StatusBadRequest, "maximum subtask depth exceeded")
		return
	}

	// Validate count: max subtasks per parent
	existingCount, err := s.tasks.CountByParent(req.ParentID)
	if err != nil {
		slog.Error("failed to count subtasks", "parent_id", req.ParentID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if existingCount+len(req.Subtasks) > s.config.Subtask.MaxPerTask {
		writeError(w, http.StatusBadRequest, "maximum subtasks per parent exceeded")
		return
	}

	var created []*models.Task
	for _, sub := range req.Subtasks {
		task := &models.Task{
			Title:     sub.Title,
			Description: sub.Description,
			Type:      parent.Type,
			Priority:  parent.Priority,
			Source:    models.TaskSourceSelf,
			ProjectID: parent.ProjectID,
			ParentID:  parent.ID,
			Depth:     parent.Depth + 1,
			GoalID:    parent.GoalID,
		}
		if err := s.tasks.Create(task); err != nil {
			slog.Error("failed to create subtask", "parent_id", parent.ID, "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		created = append(created, task)
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"tasks": created})
}

// handleInternalGetModel handles GET /internal/model/{task_id}
// Pod queries which model to use for a task.
// Phase 1 stub: always returns "sonnet".
func (s *Server) handleInternalGetModel(w http.ResponseWriter, r *http.Request) {
	// Phase 1 stub: always sonnet
	writeJSON(w, http.StatusOK, map[string]string{"model": "sonnet"})
}
