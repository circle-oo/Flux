package server

import (
	"net/http"
	"strconv"
	"time"

	"github.com/circle-oo/flux/internal/insights"
	"github.com/circle-oo/flux/internal/models"
)

// registerInsightsRoutes registers the detailed insights API endpoints.
func (s *Server) registerInsightsRoutes() {
	s.mux.Handle("GET /api/insights/summary", s.authMiddleware(http.HandlerFunc(s.handleInsightsSummary)))
	s.mux.Handle("GET /api/insights/timeseries", s.authMiddleware(http.HandlerFunc(s.handleInsightsTimeseries)))
	s.mux.Handle("GET /api/insights/efficiency", s.authMiddleware(http.HandlerFunc(s.handleInsightsEfficiency)))
	s.mux.Handle("GET /api/insights/pipeline", s.authMiddleware(http.HandlerFunc(s.handleInsightsPipeline)))
	s.mux.Handle("GET /api/insights/failures", s.authMiddleware(http.HandlerFunc(s.handleInsightsFailures)))
	s.mux.Handle("GET /api/insights/usage-realtime", s.authMiddleware(http.HandlerFunc(s.handleInsightsUsageRealtime)))
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

func (s *Server) handleInsightsUsageRealtime(w http.ResponseWriter, r *http.Request) {
	minutes := 60
	if m := r.URL.Query().Get("minutes"); m != "" {
		if parsed, err := strconv.Atoi(m); err == nil && parsed > 0 {
			minutes = parsed
		}
	}

	points, err := s.taskUsageEvents.RecentTimeseries(minutes)
	if err != nil {
		serverError(w, "failed to get realtime usage", "error", err)
		return
	}
	if points == nil {
		points = []models.UsageTimePoint{}
	}
	writeJSON(w, http.StatusOK, points)
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

// handleBillingInfo returns the billing mode and budget configuration.
func (s *Server) handleBillingInfo(w http.ResponseWriter, r *http.Request) {
	cc := &s.config.ClaudeCode
	plan := cc.EffectivePlan()
	isAPI := cc.IsAPIBilling()

	configuredPlan := cc.Plan
	if configuredPlan == "" {
		configuredPlan = "auto"
	}

	resp := map[string]interface{}{
		"plan":            plan,
		"configured_plan": configuredPlan,
		"is_api":          isAPI,
		"show_cost":       isAPI,
	}

	// Read cached ccusage data (refreshed in background every 2 minutes).
	if s.billingCache != nil {
		if daily := s.billingCache.Daily(); daily != nil {
			resp["ccusage_daily"] = daily
		}
		if block := s.billingCache.Block(); block != nil {
			resp["ccusage_block"] = block
		}
	}

	if isAPI {
		budget := s.config.Orchestrator.DailyCostBudget
		if budget <= 0 {
			budget = 20.0
		}
		resp["daily_cost_budget"] = budget

		var dailyCost float64
		_ = s.db.QueryRow(
			`SELECT COALESCE(SUM(cost_usd), 0) FROM task_usage_events
			 WHERE date(recorded_at) = date('now')`,
		).Scan(&dailyCost)
		resp["daily_cost_used"] = dailyCost
	} else {
		tokenBudget := s.config.Orchestrator.WindowTokenBudget
		if tokenBudget <= 0 {
			tokenBudget = 5_000_000
		}
		resp["window_token_budget"] = tokenBudget
		resp["window_hours"] = 5

		now := time.Now().UTC()
		windowHour := (now.Hour() / 5) * 5
		windowStart := time.Date(now.Year(), now.Month(), now.Day(), windowHour, 0, 0, 0, time.UTC)
		windowEnd := windowStart.Add(5 * time.Hour)

		var windowTokens int
		_ = s.db.QueryRow(
			`SELECT COALESCE(SUM(tokens), 0) FROM task_usage_events
			 WHERE recorded_at >= ?`,
			windowStart.Format(time.RFC3339),
		).Scan(&windowTokens)

		resp["window_tokens_used"] = windowTokens
		resp["window_start"] = windowStart.Format(time.RFC3339)
		resp["window_end"] = windowEnd.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, resp)
}
