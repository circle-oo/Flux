package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/circle-oo/flux/internal/updater"
)

// handleDeployStatus returns the current deploy/auto-update status.
func (s *Server) handleDeployStatus(w http.ResponseWriter, r *http.Request) {
	resp := map[string]any{
		"version": version,
	}

	if s.updater != nil {
		status := s.updater.Status()
		resp["updater"] = status
	} else {
		resp["updater"] = updater.Status{
			Enabled: false,
			State:   "disabled",
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleDeploy triggers a manual deploy (git pull + build + restart).
func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	slog.Info("manual deploy requested via API")

	if s.updater == nil {
		// No updater configured — fall back to the legacy restart behavior
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "restarting",
			"message": "No auto-updater configured. Falling back to restart.",
		})

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		go s.legacyRestart()
		return
	}

	// Send response before triggering (the process may exit)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "deploying",
		"message": "Deploy triggered. Pulling latest changes, rebuilding, and restarting...",
	})

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := s.updater.TriggerUpdate(ctx); err != nil {
			slog.Error("manual deploy failed", "error", err)
			s.notifier.Send("error", "Manual deploy failed: "+err.Error())
		}
	}()
}

// SetUpdater sets the auto-updater reference so deploy endpoints can use it.
func (s *Server) SetUpdater(u *updater.Updater) {
	s.updater = u
}
