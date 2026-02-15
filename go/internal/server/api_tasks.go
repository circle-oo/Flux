package server

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/circle-oo/flux/internal/models"
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
	if err := readJSON(r, &req); err != nil {
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Broadcast task update via WebSocket
	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusCreated, task)
}

// handleListTasks handles GET /api/tasks
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	filter := models.ListFilter{
		Status:    q.Get("status"),
		ProjectID: q.Get("project_id"),
		Page:      page,
		Limit:     limit,
	}

	tasks, err := s.tasks.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
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
	if err := readJSON(r, &req); err != nil {
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}

// handleDeleteTask handles DELETE /api/tasks/{id}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Delete(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleCancelTask handles POST /api/tasks/{id}/cancel
func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.tasks.Cancel(id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	task, err := s.tasks.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.ws.Broadcast(Event{Type: EventTaskUpdated, Data: task})

	writeJSON(w, http.StatusOK, task)
}
