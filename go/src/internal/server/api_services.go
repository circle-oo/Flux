package server

import (
	"net/http"
)

// handleListServices handles GET /api/services
// Phase 1 stub: returns empty list.
func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"services": []interface{}{}})
}

// handleListAlerts handles GET /api/alerts
// Phase 1 stub: returns empty list.
func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{"alerts": []interface{}{}})
}
