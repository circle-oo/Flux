package shutdown

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
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
func GracefulShutdown(ctx context.Context, cfg *config.ShutdownConfig, pods []Pod, db *sql.DB, discord *notifier.Discord) error {
	slog.Info("initiating graceful shutdown", "pods", len(pods), "grace_period", cfg.PodGracePeriod)

	// Signal all pods to stop
	for i, pod := range pods {
		slog.Info("signaling pod to stop", "pod_index", i)
		pod.Stop()
	}

	// Wait for pods to finish within grace period
	timer := time.NewTimer(cfg.PodGracePeriod)
	defer timer.Stop()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("shutdown context canceled, forcing immediate kill")
			return forceKillPods(pods, db, discord)

		case <-timer.C:
			slog.Warn("grace period expired, forcing pod kill")
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

			slog.Debug("waiting for pods to stop", "running", runningCount, "total", len(pods))
		}
	}
}

// forceKillPods moves all running pods' current tasks to RETRY status with crash_recovery=true.
func forceKillPods(pods []Pod, db *sql.DB, discord *notifier.Discord) error {
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
