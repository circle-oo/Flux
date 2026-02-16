package shutdown

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/notifier"
)

// Pod represents an executor pod that can be gracefully stopped.
type Pod interface {
	Stop() // Signal graceful stop
	IsRunning() bool
	CurrentTaskID() string
}

// GracefulShutdown initiates a graceful shutdown of all pods.
// It stops new task assignment, waits for pods to finish within the grace period,
// and force-kills any remaining pods by moving their tasks to RETRY status.
// The caller must pass a context with the desired timeout (e.g., context.WithTimeout).
func GracefulShutdown(ctx context.Context, cfg *config.ShutdownConfig, pods []Pod, db *sql.DB, discord *notifier.Discord) error {
	slog.Info("initiating graceful shutdown", "pods", len(pods), "grace_period", cfg.PodGracePeriod)

	// Signal all pods to stop
	for i, pod := range pods {
		slog.Info("signaling pod to stop", "pod_index", i)
		pod.Stop()
	}

	// Wait for pods to finish, polling every 500ms until context deadline.
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("shutdown grace period expired, forcing pod kill")
			if err := killOrphanedClaudeProcesses(); err != nil {
				slog.Error("failed to kill orphaned claude processes", "error", err)
			}
			return forceKillPods(pods, db, discord)

		case <-ticker.C:
			allDone := true
			runningCount := 0
			for _, pod := range pods {
				if pod.IsRunning() {
					allDone = false
					runningCount++
				}
			}

			if allDone {
				slog.Info("all pods stopped gracefully")
				return nil
			}

			slog.Debug("shutdown: waiting for pods", "running", runningCount, "total", len(pods))
		}
	}
}

// forceKillPods moves all running pods' current tasks to RETRY status with crash_recovery=true.
func forceKillPods(pods []Pod, db *sql.DB, discord *notifier.Discord) error {
	slog.Warn("force killing running pods", "count", len(pods))

	var recoveredTasks []string

	for i, pod := range pods {
		if !pod.IsRunning() {
			continue
		}

		taskID := pod.CurrentTaskID()
		if taskID == "" {
			slog.Warn("force killing pod with no task", "pod_index", i)
			continue
		}

		// Move task to RETRY with crash_recovery=true
		_, err := db.Exec(`
			UPDATE tasks
			SET status='RETRY', crash_recovery=TRUE, updated_at=CURRENT_TIMESTAMP
			WHERE id=? AND status='RUNNING'
		`, taskID)

		if err != nil {
			slog.Error("failed to recover task during force kill", "task_id", taskID, "error", err)
			continue
		}

		recoveredTasks = append(recoveredTasks, taskID)
		slog.Info("force killed pod and recovered task", "pod_index", i, "task_id", taskID)
	}

	if len(recoveredTasks) > 0 {
		msg := fmt.Sprintf("Force killed %d pods during shutdown. Tasks moved to RETRY: %v",
			len(recoveredTasks), recoveredTasks)
		if discord != nil {
			discord.Send(notifier.LevelWarning, msg)
		}
	}

	return nil
}

// killOrphanedClaudeProcesses finds and kills any orphaned Claude Code CLI processes.
// This ensures no zombie processes are left running after shutdown.
func killOrphanedClaudeProcesses() error {
	slog.Info("checking for orphaned claude processes")

	// Find all "claude" processes (pgrep is available on macOS and Linux)
	cmd := exec.Command("pgrep", "-f", "^claude")
	output, err := cmd.Output()
	if err != nil {
		// Exit code 1 means no processes found, which is fine
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			slog.Debug("no orphaned claude processes found")
			return nil
		}
		return fmt.Errorf("failed to search for claude processes: %w", err)
	}

	// Parse PIDs
	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(pids) == 0 || (len(pids) == 1 && pids[0] == "") {
		slog.Debug("no orphaned claude processes found")
		return nil
	}

	// Get our own PID to avoid killing ourselves
	myPID := os.Getpid()

	// Kill each process with SIGTERM first, then SIGKILL
	var killedCount int
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			continue
		}

		// Skip our own process
		if pid == myPID {
			continue
		}

		// Try SIGTERM first
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		slog.Info("terminating orphaned claude process", "pid", pid)
		if err := process.Signal(os.Interrupt); err != nil {
			// If SIGTERM fails, try SIGKILL
			slog.Warn("SIGTERM failed, sending SIGKILL", "pid", pid, "error", err)
			_ = process.Kill()
		}

		killedCount++
	}

	if killedCount > 0 {
		slog.Warn("killed orphaned claude processes", "count", killedCount)
	}

	return nil
}

// CleanupIncompleteWorktrees removes worktree directories for tasks that were interrupted.
// This prevents disk space leaks from incomplete task executions.
func CleanupIncompleteWorktrees(workspaceBase string, db *sql.DB) error {
	slog.Info("cleaning up incomplete worktrees", "workspace_base", workspaceBase)

	// Find all RUNNING or RETRY tasks
	rows, err := db.Query(`
		SELECT t.id, p.name
		FROM tasks t
		JOIN projects p ON t.project_id = p.id
		WHERE t.status IN ('RUNNING', 'RETRY')
		  AND t.branch_name IS NOT NULL
		  AND t.branch_name != ''
	`)
	if err != nil {
		return fmt.Errorf("query incomplete tasks: %w", err)
	}
	defer rows.Close()

	var cleanedCount int
	for rows.Next() {
		var taskID, projectName string
		if err := rows.Scan(&taskID, &projectName); err != nil {
			slog.Error("failed to scan task", "error", err)
			continue
		}

		// Construct task directory path: {workspace_base}/trees/{project}--task-{taskID}/
		taskShortID := taskID
		if len(taskID) > 8 {
			taskShortID = taskID[:8]
		}
		taskBaseDir := fmt.Sprintf("%s/trees/%s--task-%s", workspaceBase, projectName, taskShortID)

		// Check if directory exists
		if _, err := os.Stat(taskBaseDir); os.IsNotExist(err) {
			continue
		}

		// Remove the task directory
		slog.Info("removing incomplete worktree", "task_id", taskID, "path", taskBaseDir)
		if err := os.RemoveAll(taskBaseDir); err != nil {
			slog.Error("failed to remove worktree directory", "task_id", taskID, "path", taskBaseDir, "error", err)
			continue
		}

		cleanedCount++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tasks: %w", err)
	}

	if cleanedCount > 0 {
		slog.Info("cleaned up incomplete worktrees", "count", cleanedCount)
	} else {
		slog.Debug("no incomplete worktrees to clean up")
	}

	return nil
}
