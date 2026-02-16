package server

import (
	"net/http"
	"path/filepath"
	"syscall"
)

// handleDiskUsage returns disk space information for GET /api/system/disk.
func (s *Server) handleDiskUsage(w http.ResponseWriter, r *http.Request) {
	var stat syscall.Statfs_t

	path := s.config.Database.Path
	if path == "" {
		path = "/"
	}

	if err := syscall.Statfs(filepath.Dir(path), &stat); err != nil {
		serverError(w, "failed to check disk space", "error", err)
		return
	}

	available := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	used := total - available

	level := "ok"
	switch {
	case available < 1*1024*1024*1024: // 1 GB
		level = "force"
	case available < 2*1024*1024*1024: // 2 GB
		level = "critical"
	case available < 5*1024*1024*1024: // 5 GB
		level = "block"
	case available < 10*1024*1024*1024: // 10 GB
		level = "warning"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"available_bytes": available,
		"total_bytes":     total,
		"used_bytes":      used,
		"level":           level,
		"available_gb":    float64(available) / (1024 * 1024 * 1024),
		"total_gb":        float64(total) / (1024 * 1024 * 1024),
		"used_pct":        float64(used) / float64(total) * 100,
	})
}

// handleSystemHealth returns an enhanced health check with orchestrator info
// for GET /api/system/health.
func (s *Server) handleSystemHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]any{
		"status":       "ok",
		"version":      version,
		"auth_enabled": s.config.Server.Auth.Enabled,
	}

	// Add orchestrator status if available
	if s.orch != nil {
		orchStatus := s.orch.Status()
		health["orchestrator_running"] = orchStatus.Running
		health["orchestrator_tick_count"] = orchStatus.TickCount
		health["rate_limited"] = orchStatus.RateLimited

		componentCount := len(orchStatus.Components)
		healthyCount := 0
		for _, c := range orchStatus.Components {
			if c.Healthy {
				healthyCount++
			}
		}
		health["components_total"] = componentCount
		health["components_healthy"] = healthyCount
	}

	// Add pod count
	if s.podRegistry != nil {
		pods := s.podRegistry.List()
		health["pod_count"] = len(pods)
	}

	writeJSON(w, http.StatusOK, health)
}
