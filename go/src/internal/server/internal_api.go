package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/circle-oo/flux/internal/manager"
	"github.com/circle-oo/flux/internal/models"
)

// SetManager is deprecated. Use NewServer with mgr parameter instead.
// Kept for backwards compatibility during integration.
func SetManager(m *manager.Manager) {
	// TODO(integration): Remove this function after main.go is updated
	slog.Warn("SetManager is deprecated, pass mgr to NewServer instead")
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
			writeError(w, http.StatusBadRequest, errInvalidBody)
			return
		}
	}

	slog.Debug("internal API: next task requested", "pod_id", req.PodID, "pod_type", req.PodType)

	// If manager not set, fall back to Phase 1 stub behavior
	if s.mgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"task": nil})
		return
	}

	task, err := s.mgr.PopNextTask(req.PodType)
	if err != nil {
		serverError(w, "failed to pop next task", "pod_type", req.PodType, "error", err)
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
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	slog.Info("internal API: task done reported", "task_id", id, "status", req.Status, "tokens_used", req.TokensUsed, "cost_usd", req.CostUSD)

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}

	// Special case: if task was already cancelled, log it and skip state transition
	if task.Status == models.TaskCancelled {
		slog.Info("task was cancelled during execution, keeping CANCELLED status",
			"task_id", id,
			"attempted_status", req.Status,
			"executor_id", req.ExecutorID)
		// Don't change status, but still update cost/tokens for accounting
		// This allows the executor to move on and pop the next task
	} else if s.mgr != nil && req.Status != "" {
		// Use manager for state validation if available
		if err := s.mgr.TransitionTask(id, req.Status); err != nil {
			slog.Error("invalid state transition", "id", id, "status", req.Status, "error", err)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Reload task to get updated state
		task, err = s.tasks.GetByID(id)
		if err != nil {
			writeError(w, http.StatusNotFound, errTaskNotFound)
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
		serverError(w, "failed to update task", "id", id, "error", err)
		return
	}

	slog.Info("internal API: task updated successfully", "task_id", id, "status", task.Status)

	slog.Debug("internal API: broadcasting task update", "task_id", id)
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	// Check parent completion if this task has a parent
	if task.ParentID != "" && s.mgr != nil {
		if err := s.mgr.CheckParentCompletion(task.ParentID); err != nil {
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
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, errTaskNotFound)
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
		serverError(w, "failed to update task on start", "task_id", id, "error", err)
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
			writeError(w, http.StatusBadRequest, errInvalidBody)
			return
		}
	}

	slog.Debug("internal API: next pending task requested", "triager_id", req.TriagerID)

	if s.mgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"task": nil})
		return
	}

	task, err := s.mgr.PopNextPending(req.TriagerID)
	if err != nil {
		serverError(w, "failed to pop next pending task", "error", err)
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
// Triager reports triage completion — updates analysis/priority/title/description and promotes to READY.
func (s *Server) handleInternalTriaged(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Analysis    string `json:"analysis"`
		Description string `json:"description"`
		Title       string `json:"title"`
		Priority    int    `json:"priority"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	slog.Info("internal API: triage result reported", "task_id", id, "has_title", req.Title != "")

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, errTaskNotFound)
		return
	}

	// Update triage results — store triage fields separately to preserve user-provided originals
	if req.Analysis != "" {
		task.TriageAnalysis = req.Analysis
	}
	if req.Description != "" {
		task.TriageDescription = req.Description
	}
	if req.Title != "" {
		task.TriageTitle = req.Title
	}
	if req.Priority > 0 && req.Priority != task.Priority {
		slog.Info("triage adjusted priority", "task_id", id, "old", task.Priority, "new", req.Priority)
		task.Priority = req.Priority
	}

	// Promote to READY
	task.Status = models.TaskReady
	task.ExecutorID = "" // Clear triager claim

	if err := s.tasks.Update(task); err != nil {
		serverError(w, "failed to update task after triage", "task_id", id, "error", err)
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
		SubtaskDependencies []models.SubtaskDependency `json:"subtask_dependencies"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
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
		serverError(w, "failed to count subtasks", "parent_id", req.ParentID, "error", err)
		return
	}
	if existingCount+len(req.Subtasks) > s.config.Subtask.MaxPerTask {
		writeError(w, http.StatusBadRequest, "maximum subtasks per parent exceeded")
		return
	}

	// Create subtasks first to generate IDs, then validate dependencies
	var created []*models.Task
	subtaskIDMap := make(map[int]string) // index -> task ID mapping

	for i, sub := range req.Subtasks {
		task := &models.Task{
			Title:       sub.Title,
			Description: sub.Description,
			Type:        parent.Type,
			Priority:    parent.Priority,
			Source:      models.TaskSourceSelf,
			ProjectID:   parent.ProjectID,
			ParentID:    parent.ID,
			Depth:       parent.Depth + 1,
			GoalID:      parent.GoalID,
		}
		if err := s.tasks.Create(task); err != nil {
			serverError(w, "failed to create subtask", "parent_id", parent.ID, "error", err)
			return
		}
		slog.Info("internal API: subtask created", "parent_id", parent.ID, "subtask_id", task.ID, "title", sub.Title)
		created = append(created, task)
		subtaskIDMap[i] = task.ID
	}

	// Validate and apply dependencies if provided
	if len(req.SubtaskDependencies) > 0 {
		// Build subtask ID list for validation
		subtaskIDs := make([]string, len(created))
		for i, t := range created {
			subtaskIDs[i] = t.ID
		}

		// Convert index-based dependencies to ID-based dependencies
		// The request format expects {dependent_id: "0", dependency_id: "1"} as indices
		idBasedDeps := make([]models.SubtaskDependency, 0, len(req.SubtaskDependencies))
		for _, dep := range req.SubtaskDependencies {
			// Try to use dependency IDs directly if they look like UUIDs
			// Otherwise treat them as indices (legacy support)
			dependentID := dep.DependentID
			dependencyID := dep.DependencyID

			// Check if these are already actual task IDs (contain dashes)
			if !strings.Contains(dependentID, "-") {
				// Treat as index
				idx := -1
				fmt.Sscanf(dependentID, "%d", &idx)
				if idx < 0 || idx >= len(subtaskIDMap) {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid dependent index: %s", dependentID))
					return
				}
				dependentID = subtaskIDMap[idx]
			}

			if !strings.Contains(dependencyID, "-") {
				// Treat as index
				idx := -1
				fmt.Sscanf(dependencyID, "%d", &idx)
				if idx < 0 || idx >= len(subtaskIDMap) {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid dependency index: %s", dependencyID))
					return
				}
				dependencyID = subtaskIDMap[idx]
			}

			idBasedDeps = append(idBasedDeps, models.SubtaskDependency{
				DependentID:  dependentID,
				DependencyID: dependencyID,
			})
		}

		// Validate DAG
		if err := models.ValidateDAG(subtaskIDs, idBasedDeps); err != nil {
			slog.Warn("DAG validation failed", "parent_id", req.ParentID, "error", err)
			writeError(w, http.StatusBadRequest, fmt.Sprintf("circular dependency detected: %v", err))
			return
		}

		// Apply dependencies to tasks
		for _, dep := range idBasedDeps {
			for _, task := range created {
				if task.ID == dep.DependentID {
					task.DependsOn = append(task.DependsOn, dep.DependencyID)
					if err := s.tasks.Update(task); err != nil {
						serverError(w, "failed to update task dependencies", "task_id", task.ID, "error", err)
						return
					}
					break
				}
			}
		}

		slog.Info("dependencies applied", "parent_id", req.ParentID, "dependency_count", len(idBasedDeps))
	}

	// Broadcast all subtask creations after dependencies are set
	for _, task := range created {
		s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})
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
		writeError(w, http.StatusBadRequest, errInvalidBody)
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

	// Validate input for security
	if err := ValidateTaskInput(req.Title, req.Description); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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
		task.Priority = 40
	}
	if task.Source == "" {
		task.Source = models.TaskSourceSystem
	}

	if err := s.tasks.Create(task); err != nil {
		serverError(w, "failed to create internal task", "error", err)
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
		writeError(w, http.StatusNotFound, errTaskNotFound)
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

	if s.mgr != nil {
		task, err := s.mgr.GetTask(taskID)
		if err != nil {
			slog.Error("failed to get task", "id", taskID, "error", err)
			model := "sonnet"
			slog.Info("internal API: model assigned (fallback)", "task_id", taskID, "model", model)
			writeJSON(w, http.StatusOK, map[string]string{"model": model})
			return
		}

		// Use NeedsOpus heuristic for model selection
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
		writeError(w, http.StatusNotFound, errProjectNotFound)
		return
	}

	writeJSON(w, http.StatusOK, project)
}
