package server

import (
	"database/sql"
	"net/http"

	"github.com/circle-oo/flux/internal/models"
)

// handleCreateGoal handles POST /api/goals
func (s *Server) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Priorities  []string `json:"priorities"`
		Metrics     []string `json:"metrics"`
		Source      string   `json:"source"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	goal := &models.Goal{
		Title:       req.Title,
		Description: req.Description,
		Priorities:  req.Priorities,
		Metrics:     req.Metrics,
		Source:      req.Source,
		Status:      models.GoalProposed,
	}
	if goal.Source == "" {
		goal.Source = models.GoalSourceOperator
	}

	if err := s.goals.Create(goal); err != nil {
		serverError(w, "failed to create goal", "error", err)
		return
	}

	writeJSON(w, http.StatusCreated, goal)
}

// handleListGoals handles GET /api/goals
func (s *Server) handleListGoals(w http.ResponseWriter, r *http.Request) {
	goals, err := s.goals.List()
	if err != nil {
		serverError(w, "failed to list goals", "error", err)
		return
	}
	goals = sliceOrEmpty(goals)
	writeJSON(w, http.StatusOK, map[string]interface{}{"goals": goals})
}

// handleGetCurrentGoal handles GET /api/goals/current
func (s *Server) handleGetCurrentGoal(w http.ResponseWriter, r *http.Request) {
	goal, err := s.goals.GetCurrent()
	if err != nil {
		serverError(w, "failed to get current goal", "error", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"goal": goal})
}

// handleUpdateGoal handles PATCH /api/goals/{id}
func (s *Server) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	goal, err := s.goals.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, errGoalNotFound)
		return
	}
	if err != nil {
		serverError(w, "failed to get goal", "id", id, "error", err)
		return
	}

	var req struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Priorities  []string `json:"priorities"`
		Metrics     []string `json:"metrics"`
		Status      *string  `json:"status"`
	}
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.Title != nil {
		goal.Title = *req.Title
	}
	if req.Description != nil {
		goal.Description = *req.Description
	}
	if req.Priorities != nil {
		goal.Priorities = req.Priorities
	}
	if req.Metrics != nil {
		goal.Metrics = req.Metrics
	}
	if req.Status != nil {
		goal.Status = *req.Status
	}

	if err := s.goals.Update(goal); err != nil {
		serverError(w, "failed to update goal", "id", id, "error", err)
		return
	}

	writeJSON(w, http.StatusOK, goal)
}

// handleActivateGoal handles POST /api/goals/{id}/activate
func (s *Server) handleActivateGoal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.goals.Activate(id); err != nil {
		serverError(w, "failed to activate goal", "id", id, "error", err)
		return
	}

	goal, err := s.goals.GetByID(id)
	if err != nil {
		serverError(w, "failed to get goal after activation", "id", id, "error", err)
		return
	}

	// Broadcast goal change via WebSocket
	s.ws.Broadcast(Event{Type: EventGoalChanged, Data: goal})

	writeJSON(w, http.StatusOK, goal)
}
