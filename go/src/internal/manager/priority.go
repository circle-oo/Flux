package manager

import (
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
	return m.tasks.CountByStatus(status)
}

// CountBySource returns the number of tasks with the given source.
func (m *Manager) CountBySource(source string) (int, error) {
	return m.tasks.CountBySource(source)
}

// CountByPriority returns the number of tasks with priority in the range [minP, maxP].
func (m *Manager) CountByPriority(minP, maxP int) (int, error) {
	return m.tasks.CountByPriority(minP, maxP)
}

// ListByPRStatus returns all tasks with the given PR status.
func (m *Manager) ListByPRStatus(prStatus string) ([]*models.Task, error) {
	return m.tasks.ListByPRStatus(prStatus)
}

