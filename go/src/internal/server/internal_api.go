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

	slog.Debug("internal API: next task requested", "pod_id", req.PodID, "pod_type", req.PodType)

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

	if task != nil {
		slog.Info("internal API: task dispatched", "pod_id", req.PodID, "task_id", task.ID, "task_title", task.Title)
	} else {
		slog.Debug("internal API: no task available", "pod_id", req.PodID, "pod_type", req.PodType)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"task": task})
}

// handleInternalTaskDone handles POST /internal/tasks/{id}/done
// Pod reports task completion.
func (s *Server) handleInternalTaskDone(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Status       string  `json:"status"`
		Result       string  `json:"result"`
		ErrorLog     string  `json:"error_log"`
		TokensUsed   int     `json:"tokens_used"`
		CostUSD      float64 `json:"cost_usd"`
		ExecutorID   string  `json:"executor_id"`
		Model        string  `json:"model"`
		BranchName   string  `json:"branch_name"`
		DiffLines    int     `json:"diff_lines"`
		FilesChanged int     `json:"files_changed"`
		TestPassed   *bool   `json:"test_passed"`
		PRUrl        string  `json:"pr_url"`
		PRStatus     string  `json:"pr_status"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slog.Info("internal API: task done reported", "task_id", id, "status", req.Status, "tokens_used", req.TokensUsed, "cost_usd", req.CostUSD)

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Use manager for state validation if available
	if mgr != nil && req.Status != "" {
		if err := mgr.TransitionTask(id, req.Status); err != nil {
			slog.Error("invalid state transition", "id", id, "status", req.Status, "error", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Reload task to get updated state
		task, err = s.tasks.GetByID(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
	} else {
		// Fallback: direct update without validation
		if req.Status != "" {
			task.Status = req.Status
		}
	}

	// Update result fields
	if req.Result != "" {
		task.Result = req.Result
	}
	if req.ErrorLog != "" {
		task.ErrorLog = req.ErrorLog
	}
	task.TokensUsed = req.TokensUsed
	task.CostUSD = req.CostUSD

	// Update execution detail fields
	if req.ExecutorID != "" {
		task.ExecutorID = req.ExecutorID
	}
	if req.Model != "" {
		task.Model = req.Model
	}
	if req.BranchName != "" {
		task.BranchName = req.BranchName
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
	if req.PRUrl != "" {
		task.PRUrl = req.PRUrl
	}
	if req.PRStatus != "" {
		task.PRStatus = req.PRStatus
	}

	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	slog.Info("internal API: task updated successfully", "task_id", id, "status", task.Status)

	slog.Debug("internal API: broadcasting task update", "task_id", id)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	// Check parent completion if this task has a parent
	if task.ParentID != "" && mgr != nil {
		if err := mgr.CheckParentCompletion(task.ParentID); err != nil {
			slog.Error("parent completion check failed", "parent_id", task.ParentID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleInternalTaskStarted handles POST /internal/tasks/{id}/started
// Executor reports execution start — sets executor_id, model, and branch immediately.
func (s *Server) handleInternalTaskStarted(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		ExecutorID string `json:"executor_id"`
		Model      string `json:"model"`
		BranchName string `json:"branch_name"`
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

	if req.ExecutorID != "" {
		task.ExecutorID = req.ExecutorID
	}
	if req.Model != "" {
		task.Model = req.Model
	}
	if req.BranchName != "" {
		task.BranchName = req.BranchName
	}

	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task on start", "task_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	slog.Info("internal API: task execution started",
		"task_id", id, "executor_id", req.ExecutorID, "model", req.Model, "branch", req.BranchName)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleInternalNextPending handles POST /internal/tasks/next-pending
// Triager requests next PENDING task.
func (s *Server) handleInternalNextPending(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TriagerID string `json:"triager_id"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := readJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	slog.Debug("internal API: next pending task requested", "triager_id", req.TriagerID)

	if mgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"task": nil})
		return
	}

	task, err := mgr.PopNextPending(req.TriagerID)
	if err != nil {
		slog.Error("failed to pop next pending task", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if task != nil {
		slog.Info("internal API: pending task dispatched to triager", "triager_id", req.TriagerID, "task_id", task.ID)
	} else {
		slog.Debug("internal API: no pending task available", "triager_id", req.TriagerID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"task": task})
}

// handleInternalTriaged handles POST /internal/tasks/{id}/triaged
// Triager reports triage completion — updates analysis/priority/description and promotes to READY.
func (s *Server) handleInternalTriaged(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Analysis    string `json:"analysis"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Model       string `json:"model"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	slog.Info("internal API: triage result reported", "task_id", id, "model", req.Model)

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	// Update triage results
	if req.Analysis != "" {
		task.TriageAnalysis = req.Analysis
	}
	if req.Description != "" && req.Description != task.Description {
		task.Description = req.Description
	}
	if req.Priority > 0 && req.Priority != task.Priority {
		slog.Info("triage adjusted priority", "task_id", id, "old", task.Priority, "new", req.Priority)
		task.Priority = req.Priority
	}
	if req.Model != "" {
		task.Model = req.Model
	}

	// Promote to READY
	task.Status = models.TaskReady
	task.ExecutorID = "" // Clear triager claim

	if err := s.tasks.Update(task); err != nil {
		slog.Error("failed to update task after triage", "task_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	slog.Info("internal API: task triaged and promoted to READY", "task_id", id)
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

	slog.Info("internal API: subtask creation requested", "parent_id", req.ParentID, "count", len(req.Subtasks))

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
		slog.Info("internal API: subtask created", "parent_id", parent.ID, "subtask_id", task.ID, "title", sub.Title)
		created = append(created, task)
	}

	slog.Info("internal API: all subtasks created", "parent_id", req.ParentID, "total", len(created))

	writeJSON(w, http.StatusCreated, map[string]interface{}{"tasks": created})
}

// handleInternalCreateTask handles POST /internal/tasks
// Executor creates a follow-up task (e.g., build failure bugfix) via Manager.
func (s *Server) handleInternalCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Type        string   `json:"type"`
		Priority    int      `json:"priority"`
		Source      string   `json:"source"`
		ProjectID   string   `json:"project_id"`
		GoalID      string   `json:"goal_id"`
		BranchName  string   `json:"branch_name"`
		Tags        []string `json:"tags"`
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
		BranchName:  req.BranchName,
		Tags:        req.Tags,
	}
	if task.Priority == 0 {
		task.Priority = 50
	}
	if task.Source == "" {
		task.Source = models.TaskSourceSystem
	}

	if err := s.tasks.Create(task); err != nil {
		slog.Error("failed to create internal task", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusCreated, task)
}

// handleInternalTaskStatus handles GET /internal/tasks/{id}/status
// Executor checks if a task was cancelled mid-execution.
func (s *Server) handleInternalTaskStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": task.Status})
}

// handleInternalGetModel handles GET /internal/model/{task_id}
// Pod queries which model to use for a task.
// Priority: triager recommendation > NeedsOpus heuristic > default sonnet.
func (s *Server) handleInternalGetModel(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")

	slog.Debug("internal API: model query", "task_id", taskID)

	if mgr != nil {
		task, err := mgr.GetTask(taskID)
		if err != nil {
			slog.Error("failed to get task", "id", taskID, "error", err)
			model := "sonnet"
			slog.Info("internal API: model assigned (fallback)", "task_id", taskID, "model", model)
			writeJSON(w, http.StatusOK, map[string]string{"model": model})
			return
		}

		// If triager already set a model, use it
		if task.Model != "" && task.Model != "sonnet" {
			slog.Info("internal API: model assigned (triager)", "task_id", taskID, "model", task.Model)
			writeJSON(w, http.StatusOK, map[string]string{"model": task.Model})
			return
		}

		// If task was triaged (has analysis), trust the triager's default sonnet
		if task.TriageAnalysis != "" {
			model := task.Model
			if model == "" {
				model = "sonnet"
			}
			slog.Info("internal API: model assigned (triaged)", "task_id", taskID, "model", model)
			writeJSON(w, http.StatusOK, map[string]string{"model": model})
			return
		}

		// Fallback heuristic for non-triaged tasks
		model := "sonnet"
		if task.NeedsOpus() {
			model = "opus"
		}
		slog.Info("internal API: model assigned (heuristic)", "task_id", taskID, "model", model)
		writeJSON(w, http.StatusOK, map[string]string{"model": model})
		return
	}

	// No manager: fallback
	model := "sonnet"
	slog.Info("internal API: model assigned (default)", "task_id", taskID, "model", model)
	writeJSON(w, http.StatusOK, map[string]string{"model": model})
}

// handleInternalGetProject handles GET /internal/projects/{id}
// Executor retrieves project info without auth.
func (s *Server) handleInternalGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	project, err := s.projects.GetByID(id)
	if err != nil {
		slog.Error("internal API: failed to get project", "id", id, "error", err)
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSON(w, http.StatusOK, project)
}
