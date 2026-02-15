package shutdown

import (
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/circle-oo/flux/internal/notifier"
)

// RecoverFromCrash finds all tasks that were interrupted by a crash (status='RUNNING')
// and moves them to RETRY status with crash_recovery=true.
// This should be called at startup before any pods begin work.
func RecoverFromCrash(db *sql.DB, discord *notifier.Discord) error {
	slog.Info("checking for tasks interrupted by crash")

	// Find all RUNNING tasks (interrupted by crash)
	rows, err := db.Query(`
		SELECT id, title
		FROM tasks
		WHERE status='RUNNING'
	`)
	if err != nil {
		return fmt.Errorf("query running tasks: %w", err)
	}
	defer rows.Close()

	var recoveredTasks []struct {
		ID    string
		Title string
	}

	for rows.Next() {
		var task struct {
			ID    string
			Title string
		}
		if err := rows.Scan(&task.ID, &task.Title); err != nil {
			return fmt.Errorf("scan task: %w", err)
		}
		recoveredTasks = append(recoveredTasks, task)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tasks: %w", err)
	}

	// No interrupted tasks found
	if len(recoveredTasks) == 0 {
		slog.Info("no crash recovery needed")
		return nil
	}

	// Move all interrupted tasks to RETRY with crash_recovery=true
	result, err := db.Exec(`
		UPDATE tasks
		SET status='RETRY', crash_recovery=TRUE, updated_at=CURRENT_TIMESTAMP
		WHERE status='RUNNING'
	`)
	if err != nil {
		return fmt.Errorf("update crashed tasks: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	slog.Warn("recovered from crash", "tasks_recovered", rowsAffected)

	// Send Discord notification
	msg := fmt.Sprintf("Recovered from crash. %d tasks moved to RETRY.", rowsAffected)
	for _, task := range recoveredTasks {
		slog.Info("recovered task", "task_id", task.ID, "title", task.Title)
	}

	if discord != nil {
		discord.Send(notifier.LevelWarning, msg)
	}

	return nil
}
