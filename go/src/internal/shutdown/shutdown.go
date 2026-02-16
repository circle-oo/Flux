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
	"syscall"
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

// AgentCanceller can cancel running agent tasks.
type AgentCanceller interface {
	CancelTask(ctx context.Context, taskID string) error
}

// GracefulShutdown initiates a two-stage graceful shutdown of all pods.
//
// Stage 1 (Grace Period): Signal all pods to stop and wait for them to finish
// within cfg.PodGracePeriod (default 10min).
//
// Stage 2 (Force Kill): If pods still running after grace period, cancel tasks
// via agentClient, SIGTERM the Python Agent Manager, wait cfg.ForceKillAfter
// (default 2min), then SIGKILL if still running. Move all remaining tasks to RETRY.
//
// agentClient may be nil if no agent connection is available.
func GracefulShutdown(ctx context.Context, cfg *config.ShutdownConfig, pods []Pod, db *sql.DB, discord *notifier.Discord, agentClient AgentCanceller) error {
	gracePeriod := cfg.PodGracePeriod
	if gracePeriod <= 0 {
		gracePeriod = 10 * time.Minute
	}

	slog.Info("initiating graceful shutdown", "pods", len(pods), "grace_period", gracePeriod)

	if discord != nil {
		discord.Send(notifier.LevelInfo, fmt.Sprintf("Shutdown initiated: %d pods, grace period %s", len(pods), gracePeriod))
	}

	// Stage 1: Signal all pods to stop
	for i, pod := range pods {
		slog.Info("signaling pod to stop", "pod_index", i)
		pod.Stop()
	}

	// Wait for pods to finish within grace period
	graceCtx, graceCancel := context.WithTimeout(ctx, gracePeriod)
	defer graceCancel()

	if waitForPods(graceCtx, pods) {
		slog.Info("all pods stopped gracefully (stage 1)")
		return nil
	}

	slog.Warn("shutdown grace period expired, entering stage 2 force kill")
	return forceKillStage2(ctx, cfg, pods, db, discord, agentClient)
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

// forceKillStage2 implements the second stage of shutdown:
// cancel tasks via agent, SIGTERM agent manager, wait, then SIGKILL.
func forceKillStage2(ctx context.Context, cfg *config.ShutdownConfig, pods []Pod, db *sql.DB, discord *notifier.Discord, agentClient AgentCanceller) error {
	forceTimeout := cfg.ForceKillAfter
	if forceTimeout <= 0 {
		forceTimeout = 2 * time.Minute
	}

	// Cancel running tasks via agent client
	if agentClient != nil {
		for _, pod := range pods {
			if !pod.IsRunning() {
				continue
			}
			taskID := pod.CurrentTaskID()
			if taskID == "" {
				continue
			}
			slog.Info("cancelling task via agent", "task_id", taskID)
			cancelCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := agentClient.CancelTask(cancelCtx, taskID); err != nil {
				slog.Error("failed to cancel task via agent", "task_id", taskID, "error", err)
			}
			cancel()
		}
	}

	// SIGTERM the Python Agent Manager process
	if err := killAgentManager(); err != nil {
		slog.Error("failed to kill agent manager", "error", err)
	}

	// Wait for force timeout
	slog.Info("waiting for force kill timeout", "timeout", forceTimeout)
	forceCtx, forceCancel := context.WithTimeout(ctx, forceTimeout)
	defer forceCancel()

	if waitForPods(forceCtx, pods) {
		slog.Info("all pods stopped after stage 2 cancel")
		return nil
	}

	// Time's up - SIGKILL any remaining processes
	slog.Warn("force kill timeout expired, sending SIGKILL")
	if err := killOrphanedClaudeProcesses(); err != nil {
		slog.Error("failed to kill orphaned claude processes", "error", err)
	}
	return forceKillPods(pods, db, discord)
}

// waitForPods polls until all pods stop or context expires. Returns true if all stopped.
func waitForPods(ctx context.Context, pods []Pod) bool {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			allDone := true
			for _, pod := range pods {
				if pod.IsRunning() {
					allDone = false
					break
				}
			}
			if allDone {
				return true
			}
		}
	}
}

// killAgentManager finds and kills the Python Agent Manager process.
// It sends SIGTERM first, waits 30s, then SIGKILL.
func killAgentManager() error {
	slog.Info("looking for agent_manager process")

	cmd := exec.Command("pgrep", "-f", "agent_manager")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			slog.Debug("no agent_manager process found")
			return nil
		}
		return fmt.Errorf("pgrep agent_manager: %w", err)
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			continue
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}

		slog.Info("sending SIGTERM to agent_manager", "pid", pid)
		if err := process.Signal(syscall.SIGTERM); err != nil {
			slog.Warn("SIGTERM failed for agent_manager, sending SIGKILL", "pid", pid, "error", err)
			_ = process.Kill()
			continue
		}

		// Wait up to 30s for clean exit
		done := make(chan struct{})
		go func() {
			process.Wait()
			close(done)
		}()

		select {
		case <-done:
			slog.Info("agent_manager exited cleanly", "pid", pid)
		case <-time.After(30 * time.Second):
			slog.Warn("agent_manager did not exit in 30s, sending SIGKILL", "pid", pid)
			_ = process.Kill()
		}
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
