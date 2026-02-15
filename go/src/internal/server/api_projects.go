package server

import (
	"database/sql"
	"net/http"

	"github.com/circle-oo/flux/internal/models"
)

// handleCreateProject handles POST /api/projects
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		RepoURL     string   `json:"repo_url"`
		Description string   `json:"description"`
		TechStack   []string `json:"tech_stack"`
		Inspiration string   `json:"inspiration"`
		GoalID      string   `json:"goal_id"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	project := &models.Project{
		Name:        req.Name,
		Type:        req.Type,
		RepoURL:     req.RepoURL,
		Description: req.Description,
		TechStack:   req.TechStack,
		Inspiration: req.Inspiration,
		GoalID:      req.GoalID,
		Status:      models.ProjectProposed,
	}

	if err := s.projects.Create(project); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, project)
}

// handleListProjects handles GET /api/projects
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")

	projects, err := s.projects.List(status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []*models.Project{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"projects": projects})
}

// handleGetProject handles GET /api/projects/{id}
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	project, err := s.projects.GetByID(id)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// handleApproveProject handles POST /api/projects/{id}/approve
func (s *Server) handleApproveProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.projects.Approve(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := s.projects.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, project)
}

// handleRejectProject handles POST /api/projects/{id}/reject
func (s *Server) handleRejectProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.projects.Reject(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	project, err := s.projects.GetByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, project)
}
