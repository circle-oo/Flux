package server

import (
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// handleRestart handles the restart endpoint.
// It updates the flux binary and restarts the service.
func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	slog.Info("restart requested")

	// Send success response before restarting
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "restarting",
		"message": "Flux is updating and restarting...",
	})

	// Flush the response to ensure it's sent
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Trigger restart in a goroutine to allow response to be sent
	go func() {
		slog.Info("executing restart sequence")

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
			s.notifier.Send("error", "Restart failed: build error")
			return
		}
		slog.Info("build completed successfully")

		// Send notification
		s.notifier.Send("info", "Flux updated successfully, restarting...")

		// Restart the process
		slog.Info("restarting process", "pid", os.Getpid())

		// Send SIGTERM to self to trigger graceful shutdown
		// The process manager (launchd, systemd, or manual restart) should restart it
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			slog.Error("failed to send SIGTERM", "error", err)
		}
	}()
}
