package manager

import (
	"encoding/json"
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
	query := `SELECT id, title, description, type, status, priority, source,
		project_id, parent_id, depth, alert_id, goal_id, depends_on, tags, prompt,
		result, error_log, executor_id, model, branch_name, pr_url, pr_status,
		diff_lines, files_changed, triage_analysis, plan, test_passed, retry_count,
		crash_recovery, tokens_used, cost_usd, created_at, updated_at, started_at,
		completed_at
		FROM tasks
		WHERE pr_status = ?
		ORDER BY priority ASC, created_at ASC`

	rows, err := m.db.Query(query, prStatus)
	if err != nil {
		return nil, fmt.Errorf("query by pr_status: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		t, err := scanTaskFromRows(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	slog.Debug("list by pr status", "pr_status", prStatus, "count", len(tasks))
	return tasks, rows.Err()
}

// scanTaskFromRows is a helper to scan a task from sql.Rows.
func scanTaskFromRows(rows interface{ Scan(...interface{}) error }) (*models.Task, error) {
	var t models.Task
	var dependsOnJSON, tagsJSON string
	err := rows.Scan(
		&t.ID, &t.Title, &t.Description, &t.Type, &t.Status, &t.Priority, &t.Source,
		&t.ProjectID, &t.ParentID, &t.Depth, &t.AlertID, &t.GoalID,
		&dependsOnJSON, &tagsJSON, &t.Prompt,
		&t.Result, &t.ErrorLog, &t.ExecutorID, &t.Model, &t.BranchName,
		&t.PRUrl, &t.PRStatus, &t.DiffLines, &t.FilesChanged,
		&t.TriageAnalysis, &t.Plan, &t.TestPassed,
		&t.RetryCount, &t.CrashRecovery, &t.TokensUsed, &t.CostUSD,
		&t.CreatedAt, &t.UpdatedAt, &t.StartedAt, &t.CompletedAt,
	)
	if err != nil {
		return nil, err
	}

	// Parse JSON fields
	if err := parseJSONField(dependsOnJSON, &t.DependsOn); err != nil {
		t.DependsOn = []string{}
	}
	if err := parseJSONField(tagsJSON, &t.Tags); err != nil {
		t.Tags = []string{}
	}

	return &t, nil
}

// parseJSONField is a helper to parse JSON strings into Go types.
func parseJSONField(jsonStr string, dest interface{}) error {
	if jsonStr == "" || jsonStr == "null" {
		return fmt.Errorf("empty or null JSON")
	}
	return json.Unmarshal([]byte(jsonStr), dest)
}
