package manager

import (
	"fmt"
	"log/slog"

	"github.com/circle-oo/flux/internal/models"
)

// GetCurrentGoalID returns the current goal ID or empty string if none.
func GetCurrentGoalID(goalStore *models.GoalStore) string {
	goal, err := goalStore.GetCurrent()
	if err != nil || goal == nil {
		slog.Debug("get current goal id", "goal_id", "")
		return ""
	}
	slog.Debug("get current goal id", "goal_id", goal.ID)
	return goal.ID
}

// CountByStatus returns the number of tasks with the given status.
func (m *Manager) CountByStatus(status string) (int, error) {
	var count int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE status = ?`, status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by status: %w", err)
	}
	slog.Debug("count by status", "status", status, "count", count)
	return count, nil
}

// CountBySource returns the number of tasks with the given source.
func (m *Manager) CountBySource(source string) (int, error) {
	var count int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM tasks WHERE source = ?`, source).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by source: %w", err)
	}
	slog.Debug("count by source", "source", source, "count", count)
	return count, nil
}

// CountByPriority returns the number of tasks with priority in the range [minP, maxP].
func (m *Manager) CountByPriority(minP, maxP int) (int, error) {
	var count int
	err := m.db.QueryRow(
		`SELECT COUNT(*) FROM tasks WHERE priority >= ? AND priority <= ?`,
		minP, maxP,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count by priority: %w", err)
	}
	slog.Debug("count by priority", "min_priority", minP, "max_priority", maxP, "count", count)
	return count, nil
}

// ListByPRStatus returns all tasks with the given PR status.
func (m *Manager) ListByPRStatus(prStatus string) ([]*models.Task, error) {
	query := models.TaskSelectSQL + `
		WHERE pr_status = ?
		ORDER BY priority ASC, created_at ASC`

	rows, err := m.db.Query(query, prStatus)
	if err != nil {
		return nil, fmt.Errorf("query by pr_status: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := models.ScanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	slog.Debug("list by pr status", "pr_status", prStatus, "count", len(tasks))
	return tasks, rows.Err()
}

