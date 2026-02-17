package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/circle-oo/flux/internal/models"
)

// handleInternalNextTask handles POST /internal/tasks/next
// Pod requests next task from Manager.
func (s *Server) handleInternalNextTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PodID   string `json:"pod_id"`
		PodType string `json:"pod_type"`
	}
	// Accept empty body gracefully (backwards compat with Phase 1 stub tests)
	if err := readOptionalJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
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

// taskDoneFields holds the optional fields reported when a task completes.
type taskDoneFields struct {
	Result       string
	ErrorLog     string
	TokensUsed   int
	CostUSD      float64
	ExecutorID   string
	Model        string
	BranchName   string
	DiffLines    int
	FilesChanged int
	TestPassed   *bool
	PRUrl        string
	PRStatus     string
}

// updateTaskFields applies non-zero reported fields to a task.
func updateTaskFields(task *models.Task, f taskDoneFields) {
	if f.Result != "" {
		task.Result = f.Result
	}
	if f.ErrorLog != "" {
		task.ErrorLog = f.ErrorLog
	}
	if f.TokensUsed > 0 {
		task.TokensUsed = f.TokensUsed
	}
	if f.CostUSD > 0 {
		task.CostUSD = f.CostUSD
	}
	if f.ExecutorID != "" {
		task.ExecutorID = f.ExecutorID
	}
	if f.Model != "" {
		task.Model = f.Model
	}
	if f.BranchName != "" {
		task.BranchName = f.BranchName
	}
	if f.DiffLines != 0 {
		task.DiffLines = f.DiffLines
	}
	if f.FilesChanged != 0 {
		task.FilesChanged = f.FilesChanged
	}
	if f.TestPassed != nil {
		task.TestPassed = f.TestPassed
	}
	if f.PRUrl != "" {
		task.PRUrl = f.PRUrl
	}
	if f.PRStatus != "" {
		task.PRStatus = f.PRStatus
	}
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

	// Apply all reported fields to the task
	updateTaskFields(task, taskDoneFields{
		Result:       req.Result,
		ErrorLog:     req.ErrorLog,
		TokensUsed:   req.TokensUsed,
		CostUSD:      req.CostUSD,
		ExecutorID:   req.ExecutorID,
		Model:        req.Model,
		BranchName:   req.BranchName,
		DiffLines:    req.DiffLines,
		FilesChanged: req.FilesChanged,
		TestPassed:   req.TestPassed,
		PRUrl:        req.PRUrl,
		PRStatus:     req.PRStatus,
	})

	// Record final usage event from ccusage reconciliation
	if req.TokensUsed > 0 || req.CostUSD > 0 {
		if err := s.taskUsageEvents.Record(&models.TaskUsageEvent{
			TaskID:  id,
			Source:  "ccusage",
			Tokens:  req.TokensUsed,
			CostUSD: req.CostUSD,
		}); err != nil {
			slog.Warn("failed to record usage event", "task_id", id, "error", err)
		}
	}

	// Reconcile: aggregate real totals from usage events onto the task
	// This ensures tokens_used/cost_usd on the task row always reflect
	// the sum of all tracked usage events (executor streaming + ccusage).
	if totalTokens, totalCost, err := s.taskUsageEvents.SumByTask(id); err == nil && (totalTokens > 0 || totalCost > 0) {
		task.TokensUsed = totalTokens
		task.CostUSD = totalCost
	}

	if err := s.tasks.Update(task); err != nil {
		serverError(w, "failed to update task", "id", id, "error", err)
		return
	}

	// Auto-detect billing mode from first task completion.
	// If cost was reported (cost_usd > 0), it's API billing. Otherwise, it's a plan.
	s.config.ClaudeCode.DetectBilling(task.CostUSD > 0 || req.CostUSD > 0)

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
	if err := readOptionalJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
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
			Title       string   `json:"title"`
			Description string   `json:"description"`
			DependsOn   []string `json:"depends_on"`
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

	// Create all subtasks atomically within a transaction so partial failures
	// don't leave orphaned subtasks in the database.
	tx, err := s.db.Begin()
	if err != nil {
		serverError(w, "failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback()

	var created []*models.Task

	for _, sub := range req.Subtasks {
		task := &models.Task{
			Title:       sub.Title,
			Description: sub.Description,
			Priority:    parent.Priority,
			Source:      models.TaskSourceSelf,
			ProjectID:   parent.ProjectID,
			ParentID:    parent.ID,
			Depth:       parent.Depth + 1,
			GoalID:      parent.GoalID,
			DependsOn:   sub.DependsOn,
		}
		if err := s.tasks.CreateTx(tx, task); err != nil {
			serverError(w, "failed to create subtask", "parent_id", parent.ID, "error", err)
			return
		}
		slog.Info("internal API: subtask created", "parent_id", parent.ID, "subtask_id", task.ID, "title", sub.Title)
		created = append(created, task)
	}

	// Commit the subtask creation transaction before applying dependencies.
	// This ensures all subtasks are created atomically; dependency updates
	// happen outside the transaction (recoverable if they fail).
	if err := tx.Commit(); err != nil {
		serverError(w, "failed to commit subtask transaction", "parent_id", req.ParentID, "error", err)
		return
	}

	// Validate and apply dependencies if provided
	if len(req.SubtaskDependencies) > 0 {
		idByIndex := buildSubtaskIndex(created)
		idBasedDeps, err := resolveSubtaskDependencies(req.SubtaskDependencies, idByIndex)
		if err != nil {
			s.cleanupCreatedSubtasks(created, req.ParentID)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate DAG
		if err := models.ValidateDAG(taskIDs(created), idBasedDeps); err != nil {
			s.cleanupCreatedSubtasks(created, req.ParentID)
			slog.Warn("DAG validation failed", "parent_id", req.ParentID, "error", err)
			writeError(w, http.StatusBadRequest, fmt.Sprintf("circular dependency detected: %v", err))
			return
		}

		if err := s.applySubtaskDependencies(created, idBasedDeps); err != nil {
			s.cleanupCreatedSubtasks(created, req.ParentID)
			serverError(w, "failed to update task dependencies", "parent_id", req.ParentID, "error", err)
			return
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

	// Validate input for security
	if err := ValidateTaskInput(req.Title, req.Description); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	task := &models.Task{
		Title:       req.Title,
		Description: req.Description,
		Priority:    req.Priority,
		Source:      req.Source,
		ProjectID:   req.ProjectID,
		GoalID:      req.GoalID,
		BranchName:  req.BranchName,
		Tags:        req.Tags,
	}
	applyTaskDefaults(task, models.TaskSourceSystem)

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
			slog.Info("internal API: model assigned (fallback)", "task_id", taskID, "model", models.DefaultModel)
			writeJSON(w, http.StatusOK, map[string]string{"model": models.DefaultModel})
			return
		}

		model := task.SelectModel()
		slog.Info("internal API: model assigned (heuristic)", "task_id", taskID, "model", model)
		writeJSON(w, http.StatusOK, map[string]string{"model": model})
		return
	}

	// No manager: fallback
	slog.Info("internal API: model assigned (default)", "task_id", taskID, "model", models.DefaultModel)
	writeJSON(w, http.StatusOK, map[string]string{"model": models.DefaultModel})
}

// handleInternalTaskUsage handles POST /internal/tasks/{id}/usage
// Pod reports incremental usage data during execution.
func (s *Server) handleInternalTaskUsage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Tokens  int               `json:"tokens"`
		CostUSD float64           `json:"cost_usd"`
		Source  string            `json:"source"`
		Meta    map[string]string `json:"meta"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Source == "" {
		req.Source = "executor"
	}

	event := &models.TaskUsageEvent{
		TaskID:  id,
		Source:  req.Source,
		Tokens:  req.Tokens,
		CostUSD: req.CostUSD,
		Meta:    req.Meta,
	}

	if err := s.taskUsageEvents.Record(event); err != nil {
		serverError(w, "failed to record usage event", "task_id", id, "error", err)
		return
	}

	// Get running totals for the broadcast
	totalTokens, totalCost, _ := s.taskUsageEvents.SumByTask(id)

	slog.Debug("internal API: usage event recorded", "task_id", id, "tokens", req.Tokens, "cost_usd", req.CostUSD, "source", req.Source)

	s.ws.Broadcast(Event{Type: EventUsageUpdate, Data: map[string]interface{}{
		"task_id":      id,
		"tokens":       req.Tokens,
		"cost_usd":     req.CostUSD,
		"total_tokens": totalTokens,
		"total_cost":   totalCost,
	}})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

func buildSubtaskIndex(tasks []*models.Task) map[int]string {
	index := make(map[int]string, len(tasks))
	for i, task := range tasks {
		index[i] = task.ID
	}
	return index
}

func taskIDs(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for i, task := range tasks {
		ids[i] = task.ID
	}
	return ids
}

func resolveSubtaskDependencies(raw []models.SubtaskDependency, idByIndex map[int]string) ([]models.SubtaskDependency, error) {
	resolved := make([]models.SubtaskDependency, 0, len(raw))
	for _, dep := range raw {
		dependentID, err := resolveDependencyRef(dep.DependentID, "dependent", idByIndex)
		if err != nil {
			return nil, err
		}
		dependencyID, err := resolveDependencyRef(dep.DependencyID, "dependency", idByIndex)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, models.SubtaskDependency{
			DependentID:  dependentID,
			DependencyID: dependencyID,
		})
	}
	return resolved, nil
}

func resolveDependencyRef(ref, label string, idByIndex map[int]string) (string, error) {
	// If the value already looks like a task ID, use it as-is.
	if strings.Contains(ref, "-") {
		return ref, nil
	}
	idx, err := strconv.Atoi(ref)
	if err != nil {
		return "", fmt.Errorf("invalid %s index: %s", label, ref)
	}
	id, ok := idByIndex[idx]
	if !ok {
		return "", fmt.Errorf("invalid %s index: %s", label, ref)
	}
	return id, nil
}

func (s *Server) applySubtaskDependencies(created []*models.Task, deps []models.SubtaskDependency) error {
	depsByTask := make(map[string][]string)
	for _, dep := range deps {
		depsByTask[dep.DependentID] = append(depsByTask[dep.DependentID], dep.DependencyID)
	}

	for _, task := range created {
		dependencies := depsByTask[task.ID]
		if len(dependencies) == 0 {
			continue
		}
		task.DependsOn = append(task.DependsOn, dependencies...)
		if err := s.tasks.Update(task); err != nil {
			return fmt.Errorf("task %s: %w", task.ID, err)
		}
	}
	return nil
}

func (s *Server) cleanupCreatedSubtasks(created []*models.Task, parentID string) {
	for _, task := range created {
		if err := s.tasks.Delete(task.ID); err != nil {
			slog.Warn("failed to cleanup subtask after dependency error",
				"parent_id", parentID, "subtask_id", task.ID, "error", err)
		}
	}
}
