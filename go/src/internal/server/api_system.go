package server

import (
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// handleRestart handles the legacy restart endpoint.
// It updates the flux binary and restarts the service.
// Prefer POST /api/system/deploy for the improved deploy flow.
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	slog.Info("restart requested (legacy endpoint)")

	// If updater is available, delegate to the deploy endpoint
	if s.updater != nil {
		s.handleDeploy(w, r)
		return
	}

	// Send success response before restarting
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "restarting",
		"message": "Flux is updating and restarting...",
	})

	// Flush the response to ensure it's sent
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	go s.legacyRestart()
}

// legacyRestart performs the git pull + make build + SIGTERM restart sequence.
// Used when no auto-updater is configured.
func (s *Server) legacyRestart() {
	slog.Info("executing legacy restart sequence")

	// Get the directory of the currently running binary
	exePath, err := os.Executable()
	if err != nil {
		slog.Error("failed to get executable path", "error", err)
		return
	}

	// Resolve symlinks
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		slog.Error("failed to resolve symlinks", "error", err)
		return
	}

	// Get the project root (assuming binary is in go/bin/)
	projectRoot := filepath.Join(filepath.Dir(exePath), "..", "..")
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		slog.Error("failed to get absolute path", "error", err)
		return
	}

	slog.Info("project root detected", "path", projectRoot)

	// Pull latest changes
	slog.Info("pulling latest changes from git")
	gitCmd := exec.Command("git", "pull")
	gitCmd.Dir = projectRoot
	if output, err := gitCmd.CombinedOutput(); err != nil {
		slog.Error("git pull failed", "error", err, "output", string(output))
		if s.notifier != nil {
			s.notifier.Send("error", "Restart failed: git pull error")
		}
		// Continue anyway - the update might not be necessary
	} else {
		slog.Info("git pull completed", "output", string(output))
	}

	// Rebuild the binary
	slog.Info("rebuilding flux binary")
	buildCmd := exec.Command("make", "build")
	buildCmd.Dir = projectRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		slog.Error("build failed", "error", err, "output", string(output))
		if s.notifier != nil {
			s.notifier.Send("error", "Restart failed: build error")
		}
		return
	}
	slog.Info("build completed successfully")

	// Send notification
	if s.notifier != nil {
		s.notifier.Send("info", "Flux updated successfully, restarting...")
	}

	// Restart the process
	slog.Info("restarting process", "pid", os.Getpid())

	// Send SIGTERM to self to trigger graceful shutdown
	// The process manager (launchd, systemd, or manual restart) should restart it
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		slog.Error("failed to send SIGTERM", "error", err)
	}
}

// handleInsights handles GET /api/insights
// Returns aggregated metrics: token usage, cost, and activities per project.
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	// Aggregate token usage and cost
	var totalTokens int
	var totalCost float64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(tokens_used), 0), COALESCE(SUM(cost_usd), 0)
		FROM tasks
	`).Scan(&totalTokens, &totalCost)
	if err != nil {
		serverError(w, "failed to aggregate usage metrics", "error", err)
		return
	}

	// Get activities per project
	type ProjectActivity struct {
		ProjectID   string `json:"project_id"`
		ProjectName string `json:"project_name"`
		TaskCount   int    `json:"task_count"`
	}
	rows, err := s.db.Query(`
		SELECT
			t.project_id,
			COALESCE(p.name, 'Unknown') as project_name,
			COUNT(*) as task_count
		FROM tasks t
		LEFT JOIN projects p ON t.project_id = p.id
		WHERE t.project_id != ''
		GROUP BY t.project_id
		ORDER BY task_count DESC
	`)
	if err != nil {
		serverError(w, "failed to query project activities", "error", err)
		return
	}
	defer rows.Close()

	var activities []ProjectActivity
	for rows.Next() {
		var pa ProjectActivity
		if err := rows.Scan(&pa.ProjectID, &pa.ProjectName, &pa.TaskCount); err != nil {
			serverError(w, "failed to scan project activity", "error", err)
			return
		}
		activities = append(activities, pa)
	}
	if err := rows.Err(); err != nil {
		serverError(w, "failed to iterate project activities", "error", err)
		return
	}

	// If no activities found, return empty array instead of null
	if activities == nil {
		activities = []ProjectActivity{}
	}

	response := map[string]interface{}{
		"total_tokens":       totalTokens,
		"total_cost":         totalCost,
		"project_activities": activities,
	}

	writeJSON(w, http.StatusOK, response)
}
