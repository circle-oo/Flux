package server

import (
	"net/http"

	"github.com/circle-oo/flux/internal/insights"
)

// registerInsightsRoutes registers the detailed insights API endpoints.
func (s *Server) registerInsightsRoutes() {
	s.mux.Handle("GET /api/insights/summary", s.authMiddleware(http.HandlerFunc(s.handleInsightsSummary)))
	s.mux.Handle("GET /api/insights/timeseries", s.authMiddleware(http.HandlerFunc(s.handleInsightsTimeseries)))
	s.mux.Handle("GET /api/insights/efficiency", s.authMiddleware(http.HandlerFunc(s.handleInsightsEfficiency)))
	s.mux.Handle("GET /api/insights/pipeline", s.authMiddleware(http.HandlerFunc(s.handleInsightsPipeline)))
	s.mux.Handle("GET /api/insights/failures", s.authMiddleware(http.HandlerFunc(s.handleInsightsFailures)))
}

// initInsights initializes the insights collector from the server's database.
func (s *Server) initInsights() {
	s.insights = insights.NewCollector(s.db)
}

func (s *Server) handleInsightsSummary(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	summary, err := s.insights.GetSummary(period)
	if err != nil {
		serverError(w, "failed to get insights summary", "error", err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) handleInsightsTimeseries(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	data, err := s.insights.GetTimeseries(period)
	if err != nil {
		serverError(w, "failed to get insights timeseries", "error", err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleInsightsEfficiency(w http.ResponseWriter, r *http.Request) {
	data, err := s.insights.GetEfficiency()
	if err != nil {
		serverError(w, "failed to get insights efficiency", "error", err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleInsightsPipeline(w http.ResponseWriter, r *http.Request) {
	data, err := s.insights.GetPipelineHealth()
	if err != nil {
		serverError(w, "failed to get pipeline health", "error", err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

func (s *Server) handleInsightsFailures(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}

	data, err := s.insights.GetFailures(period)
	if err != nil {
		serverError(w, "failed to get failure analysis", "error", err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}
