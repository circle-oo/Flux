package server

import (
	"context"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
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
		// Even with no updater, fetch local commit info for display
		status := updater.Status{
			Enabled: false,
			State:   "disabled",
		}

		// Try to get local commit hash
		if localCommit := getLocalCommit(); localCommit != "" {
			status.LocalCommit = localCommit
		}

		resp["updater"] = status
	}

	writeJSON(w, http.StatusOK, resp)
}

// getLocalCommit fetches the current git HEAD commit hash.
// Returns empty string if git command fails.
func getLocalCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// handleCheckRemote fetches the remote commit hash without deploying.
func (s *Server) handleCheckRemote(w http.ResponseWriter, r *http.Request) {
	slog.Info("remote commit check requested via API")

	if s.updater != nil {
		// Use updater's built-in fetch method
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()

		if err := s.updater.FetchRemoteCommit(ctx); err != nil {
			slog.Error("remote commit check failed", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "success",
			"message": "Remote commit fetched successfully.",
		})
		return
	}

	// No updater configured, but we can still fetch remote commit manually
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	local, remote, err := fetchGitCommits(ctx)
	if err != nil {
		slog.Error("remote commit check failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	slog.Info("remote commit fetched (no updater)",
		"local", local[:8],
		"remote", remote[:8],
		"update_available", local != remote,
	)

	// Broadcast the result via WebSocket so frontend can update
	s.ws.Broadcast(Event{
		Type: "DEPLOY_STATUS",
		Data: map[string]any{
			"updater": updater.Status{
				Enabled:      false,
				State:        "disabled",
				LocalCommit:  local,
				RemoteCommit: remote,
			},
		},
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Remote commit fetched successfully.",
	})
}

// fetchGitCommits fetches both local and remote commit hashes.
// Returns (localCommit, remoteCommit, error).
func fetchGitCommits(ctx context.Context) (string, string, error) {
	// Fetch from remote
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", "main")
	if err := fetchCmd.Run(); err != nil {
		return "", "", err
	}

	// Get local HEAD
	localCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	localOut, err := localCmd.Output()
	if err != nil {
		return "", "", err
	}
	local := strings.TrimSpace(string(localOut))

	// Get remote HEAD
	remoteCmd := exec.CommandContext(ctx, "git", "rev-parse", "origin/main")
	remoteOut, err := remoteCmd.Output()
	if err != nil {
		return "", "", err
	}
	remote := strings.TrimSpace(string(remoteOut))

	return local, remote, nil
}

// SetUpdater sets the auto-updater reference so deploy endpoints can use it.
func (s *Server) SetUpdater(u *updater.Updater) {
	s.updater = u
}
