package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"time"
)

type projectActivity struct {
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	TaskCount   int    `json:"task_count"`
}

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
		s.broadcastDeployStatus("failed", "restart failed: executable path lookup error", err.Error())
		return
	}

	// Resolve symlinks
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		slog.Error("failed to resolve symlinks", "error", err)
		s.broadcastDeployStatus("failed", "restart failed: symlink resolution error", err.Error())
		return
	}

	// Get the project root (assuming binary is in go/bin/)
	projectRoot := filepath.Join(filepath.Dir(exePath), "..", "..")
	projectRoot, err = filepath.Abs(projectRoot)
	if err != nil {
		slog.Error("failed to get absolute path", "error", err)
		s.broadcastDeployStatus("failed", "restart failed: project root resolution error", err.Error())
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
		s.broadcastDeployStatus("warning", "git pull failed during restart", string(output))
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
		s.broadcastDeployStatus("failed", "restart failed: build error", string(output))
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
		s.broadcastDeployStatus("failed", "restart failed: could not signal process", err.Error())
	}
}

func (s *Server) broadcastDeployStatus(status, message, detail string) {
	if s.ws == nil {
		return
	}

	payload := map[string]any{
		"status":  status,
		"message": message,
	}
	if detail != "" {
		payload["detail"] = detail
	}
	s.ws.Broadcast(Event{Type: EventDeployStatus, Data: payload})
}

// handleConfig handles GET /api/config
// Returns sanitized configuration, redacting sensitive fields.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if s.config == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}

	// Build a sanitized config map using reflection
	result := sanitizeStruct(reflect.ValueOf(*s.config), 0)
	writeJSON(w, http.StatusOK, result)
}

// sensitiveFieldNames are field names whose values should be redacted.
var sensitiveFieldNames = map[string]bool{
	"token": true, "password": true, "webhookurl": true,
	"tokenenv": true, "passwordenv": true, "webhookurlenv": true,
}

func sanitizeStruct(v reflect.Value, depth int) map[string]interface{} {
	if depth > 4 {
		return nil
	}
	t := v.Type()
	out := make(map[string]interface{})
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Use yaml tag name if available, else lowercase field name
		name := field.Tag.Get("yaml")
		if name == "" || name == "-" {
			name = strings.ToLower(field.Name)
		}

		fv := v.Field(i)

		// Redact sensitive fields
		if sensitiveFieldNames[strings.ToLower(field.Name)] {
			s := fv.String()
			if s != "" {
				out[name] = "***"
			} else {
				out[name] = ""
			}
			continue
		}

		switch fv.Kind() {
		case reflect.Struct:
			out[name] = sanitizeStruct(fv, depth+1)
		case reflect.Slice:
			items := make([]interface{}, fv.Len())
			for j := 0; j < fv.Len(); j++ {
				elem := fv.Index(j)
				if elem.Kind() == reflect.Struct {
					items[j] = sanitizeStruct(elem, depth+1)
				} else {
					items[j] = elem.Interface()
				}
			}
			out[name] = items
		default:
			out[name] = fv.Interface()
		}
	}
	return out
}

// handleInsights handles GET /api/insights
// Returns aggregated metrics: token usage, cost, and activities per project.
func (s *Server) handleInsights(w http.ResponseWriter, r *http.Request) {
	totalTokens, totalCost, err := s.queryInsightsTotals()
	if err != nil {
		serverError(w, "failed to aggregate usage metrics", "error", err)
		return
	}

	activities, err := s.queryProjectActivities()
	if err != nil {
		serverError(w, "failed to aggregate project activities", "error", err)
		return
	}

	response := map[string]interface{}{
		"total_tokens":       totalTokens,
		"total_cost":         totalCost,
		"project_activities": sliceOrEmpty(activities),
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) queryInsightsTotals() (int, float64, error) {
	var totalTokens int
	var totalCost float64
	err := s.db.QueryRow(`
		SELECT COALESCE(SUM(tokens_used), 0), COALESCE(SUM(cost_usd), 0)
		FROM tasks
	`).Scan(&totalTokens, &totalCost)
	return totalTokens, totalCost, err
}

func (s *Server) queryProjectActivities() ([]projectActivity, error) {
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
		return nil, err
	}
	defer rows.Close()

	var activities []projectActivity
	for rows.Next() {
		var pa projectActivity
		if err := rows.Scan(&pa.ProjectID, &pa.ProjectName, &pa.TaskCount); err != nil {
			return nil, err
		}
		activities = append(activities, pa)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return activities, nil
}

// handleOrchestratorStatus handles GET /api/orchestrator/status
// Returns orchestrator health, sub-component status, and scale state.
func (s *Server) handleOrchestratorStatus(w http.ResponseWriter, r *http.Request) {
	resp := s.buildOrchestratorStatusPayload()

	if s.scaleManager != nil {
		scaleStatus := s.scaleManager.Status()
		resp["scale_status"] = map[string]any{
			"executor_pods":       scaleStatus.GetExecutorPods(),
			"triager_pods":        scaleStatus.GetTriagerPods(),
			"researcher_pods":     scaleStatus.GetResearcherPods(),
			"max_executor_pods":   scaleStatus.GetMaxExecutorPods(),
			"max_triager_pods":    scaleStatus.GetMaxTriagerPods(),
			"max_researcher_pods": scaleStatus.GetMaxResearcherPods(),
			"queue_state":         scaleStatus.GetQueueState(),
			"last_scale_time":     scaleStatus.GetLastScaleTime(),
		}
	}

	if s.cleaner != nil {
		resp["disk_status"] = s.cleaner.CheckDiskSpace()
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) buildOrchestratorStatusPayload() map[string]any {
	resp := map[string]any{
		"running":    false,
		"uptime":     "",
		"tick_count": 0,
	}

	if s.orch != nil {
		status := s.orch.Status()
		resp["running"] = status.Running
		resp["tick_count"] = status.TickCount
		resp["rate_limited"] = status.RateLimited

		if !status.RateLimitUntil.IsZero() {
			resp["rate_limit_until"] = status.RateLimitUntil.Format(time.RFC3339)
		}

		if !status.StartedAt.IsZero() {
			resp["uptime"] = fmt.Sprintf("%s", time.Since(status.StartedAt).Round(time.Second))
			resp["started_at"] = status.StartedAt.Format(time.RFC3339)
		}

		var components []map[string]any
		for _, ch := range status.Components {
			var lastTick string
			if !ch.LastTick.IsZero() {
				lastTick = ch.LastTick.Format(time.RFC3339)
			}
			components = append(components, map[string]any{
				"name":       ch.Name,
				"healthy":    ch.Healthy,
				"last_tick":  lastTick,
				"last_error": ch.LastError,
			})
		}
		resp["sub_components"] = components
	}
	return resp
}
