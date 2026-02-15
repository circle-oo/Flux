package server

import (
	"log/slog"
	"net/http"

	"github.com/circle-oo/flux/internal/manager"
	"github.com/circle-oo/flux/internal/models"
)

// mgr is the package-level manager instance set during initialization.
var mgr *manager.Manager

// SetManager sets the manager for internal API handlers.
func SetManager(m *manager.Manager) {
	mgr = m
}

// handleInternalNextTask handles POST /internal/tasks/next
// Pod requests next task from Manager.
func (s *Server) handleInternalNextTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PodID   string `json:"pod_id"`
		PodType string `json:"pod_type"`
	}
	// Accept empty body gracefully (backwards compat with Phase 1 stub tests)
	if r.Body != nil && r.ContentLength != 0 {
		if err := readJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	// If manager not set, fall back to Phase 1 stub behavior
	if mgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"task": nil})
		return
	}

	task, err := mgr.PopNextTask(req.PodType)
	if err != nil {
		slog.Error("failed to pop next task", "pod_type", req.PodType, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"task": task})
}

// handleInternalTaskDone handles POST /internal/tasks/{id}/done
// Pod reports task completion with all execution metadata.
func (s *Server) handleInternalTaskDone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Status       string  `json:"status"`
		Result       string  `json:"result"`
		ErrorLog     string  `json:"error_log"`
		TokensUsed   int     `json:"tokens_used"`
		CostUSD      float64 `json:"cost_usd"`
		Model        string  `json:"model"`
		BranchName   string  `json:"branch_name"`
		PRUrl        string  `json:"pr_url"`
		PRStatus     string  `json:"pr_status"`
		DiffLines    int     `json:"diff_lines"`
		FilesChanged int     `json:"files_changed"`
		TestPassed   *bool   `json:"test_passed"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Use manager for state validation if available
	if mgr != nil && req.Status != "" {
		if err := mgr.TransitionTask(id, req.Status); err != nil {
			slog.Error("invalid state transition", "id", id, "status", req.Status, "error", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	// Reload task to get state after transition (or current state if no manager)
	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Fallback: direct status update without validation when no manager
	if mgr == nil && req.Status != "" {
		task.Status = req.Status
	}

	// Apply execution result fields
	if req.Result != "" {
		task.Result = req.Result
	}
	if req.ErrorLog != "" {
		task.ErrorLog = req.ErrorLog
	}
	task.TokensUsed = req.TokensUsed
	task.CostUSD = req.CostUSD

	// Apply execution metadata fields
	if req.Model != "" {
		task.Model = req.Model
	}
	if req.BranchName != "" {
		task.BranchName = req.BranchName
	}
	if req.PRUrl != "" {
		task.PRUrl = req.PRUrl
	}
	if req.PRStatus != "" {
		task.PRStatus = req.PRStatus
	}
	if req.DiffLines != 0 {
		task.DiffLines = req.DiffLines
	}
	if req.FilesChanged != 0 {
		task.FilesChanged = req.FilesChanged
	}
	if req.TestPassed != nil {
		task.TestPassed = req.TestPassed
	}

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
func (s *Server) handleInternalGetModel(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	// If manager available, use task.NeedsOpus() logic
	if mgr != nil {
		task, err := mgr.GetTask(taskID)
		if err != nil {
			slog.Error("failed to get task", "id", taskID, "error", err)
			writeJSON(w, http.StatusOK, map[string]string{"model": "sonnet"})
			return
		}

		model := "sonnet"
		if task.NeedsOpus() {
			model = "opus"
		}
		writeJSON(w, http.StatusOK, map[string]string{"model": model})
		return
	}

	// Fallback: always sonnet
	writeJSON(w, http.StatusOK, map[string]string{"model": "sonnet"})
}
