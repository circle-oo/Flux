package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// handleListPods returns the list of all registered executor pods.
func (s *Server) handleListPods(w http.ResponseWriter, r *http.Request) {
	pods := s.podRegistry.List()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pods": pods,
	})
}

// handlePodRegister registers a new pod or updates its heartbeat.
// Internal endpoint called by executors on startup.
func (s *Server) handlePodRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string    `json:"id"`
		StartedAt time.Time `json:"started_at"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "pod id is required")
		return
	}

	s.podRegistry.Register(req.ID, req.StartedAt)
	slog.Debug("pod registered", "id", req.ID)

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "registered",
	})
}

// handlePodStatus updates a pod's current task status.
// Internal endpoint called by executors when starting/finishing tasks.
func (s *Server) handlePodStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string `json:"id"`
		Status    string `json:"status"` // "idle" or "busy"
		TaskID    string `json:"task_id"`
		TaskTitle string `json:"task_title"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidBody)
		return
	}

	if req.ID == "" {
		writeError(w, http.StatusBadRequest, "pod id is required")
		return
	}

	if req.Status != "idle" && req.Status != "busy" {
		writeError(w, http.StatusBadRequest, "status must be 'idle' or 'busy'")
		return
	}

	s.podRegistry.UpdateStatus(req.ID, req.Status, req.TaskID, req.TaskTitle)

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "updated",
	})
}
