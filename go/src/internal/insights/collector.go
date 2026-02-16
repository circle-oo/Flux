package insights

import (
	"database/sql"
	"fmt"
	"strings"
)

// Collector queries the tasks table for aggregated insights.
type Collector struct {
	db *sql.DB
}

// NewCollector creates a new Collector backed by the given database.
func NewCollector(db *sql.DB) *Collector {
	return &Collector{db: db}
}

// periodFilter returns a SQL WHERE clause fragment for the given period.
func periodFilter(period string) string {
	switch period {
	case "24h":
		return "created_at >= datetime('now', '-1 day')"
	case "7d":
		return "created_at >= datetime('now', '-7 days')"
	case "30d":
		return "created_at >= datetime('now', '-30 days')"
	default:
		return "created_at >= datetime('now', '-7 days')"
	}
}

// GetSummary returns overview metrics for the given period.
func (c *Collector) GetSummary(period string) (*InsightsSummary, error) {
	pf := periodFilter(period)

	var s InsightsSummary
	err := c.db.QueryRow(fmt.Sprintf(`
		SELECT
			COUNT(*) as total_tasks,
			COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0) as completed,
			COALESCE(SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END), 0) as failed,
			COALESCE(SUM(tokens_used), 0) as total_tokens,
			COALESCE(SUM(cost_usd), 0) as total_cost,
			COALESCE(AVG(
				CASE WHEN completed_at != '' AND started_at != ''
				THEN (julianday(completed_at) - julianday(started_at)) * 1440
				ELSE NULL END
			), 0) as avg_latency_min
		FROM tasks WHERE %s
	`, pf)).Scan(
		&s.TotalTasks, &s.CompletedTasks, &s.FailedTasks,
		&s.TotalTokens, &s.TotalCost, &s.AvgLatencyMin,
	)
	if err != nil {
		return nil, fmt.Errorf("query summary: %w", err)
	}

	if s.TotalTasks > 0 {
		s.SuccessRate = float64(s.CompletedTasks) / float64(s.TotalTasks) * 100
	}

	err = c.db.QueryRow(fmt.Sprintf(`
		SELECT COUNT(DISTINCT project_id) FROM tasks WHERE project_id != '' AND %s
	`, pf)).Scan(&s.ActiveProjects)
	if err != nil {
		return nil, fmt.Errorf("query active projects: %w", err)
	}

	return &s, nil
}

// GetTimeseries returns daily task metrics for the given period.
func (c *Collector) GetTimeseries(period string) ([]DailyMetric, error) {
	pf := periodFilter(period)

	rows, err := c.db.Query(fmt.Sprintf(`
		SELECT
			strftime('%%Y-%%m-%%d', created_at) as date,
			COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0) as completed,
			COALESCE(SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END), 0) as failed,
			COUNT(*) as created,
			COALESCE(SUM(tokens_used), 0) as tokens,
			COALESCE(SUM(cost_usd), 0) as cost,
			COALESCE(AVG(
				CASE WHEN completed_at != '' AND started_at != ''
				THEN (julianday(completed_at) - julianday(started_at)) * 1440
				ELSE NULL END
			), 0) as avg_latency
		FROM tasks
		WHERE %s
		GROUP BY strftime('%%Y-%%m-%%d', created_at)
		ORDER BY date ASC
	`, pf))
	if err != nil {
		return nil, fmt.Errorf("query timeseries: %w", err)
	}
	defer rows.Close()

	var metrics []DailyMetric
	for rows.Next() {
		var m DailyMetric
		if err := rows.Scan(&m.Date, &m.TasksCompleted, &m.TasksFailed,
			&m.TasksCreated, &m.TotalTokens, &m.TotalCost, &m.AvgLatencyMinutes); err != nil {
			return nil, fmt.Errorf("scan timeseries row: %w", err)
		}
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate timeseries: %w", err)
	}

	if metrics == nil {
		metrics = []DailyMetric{}
	}
	return metrics, nil
}

// GetEfficiency returns per-executor performance stats.
func (c *Collector) GetEfficiency() ([]AgentPerformance, error) {
	rows, err := c.db.Query(`
		SELECT
			COALESCE(NULLIF(executor_id, ''), 'unassigned') as executor_id,
			COALESCE(SUM(CASE WHEN status = 'COMPLETED' THEN 1 ELSE 0 END), 0) as completed,
			COALESCE(SUM(CASE WHEN status = 'FAILED' THEN 1 ELSE 0 END), 0) as failed,
			COALESCE(AVG(
				CASE WHEN completed_at != '' AND started_at != ''
				THEN (julianday(completed_at) - julianday(started_at)) * 1440
				ELSE NULL END
			), 0) as avg_duration
		FROM tasks
		WHERE executor_id != ''
		GROUP BY executor_id
		ORDER BY completed DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query efficiency: %w", err)
	}
	defer rows.Close()

	var agents []AgentPerformance
	for rows.Next() {
		var a AgentPerformance
		if err := rows.Scan(&a.ExecutorID, &a.TasksCompleted, &a.TasksFailed, &a.AvgDurationMin); err != nil {
			return nil, fmt.Errorf("scan efficiency row: %w", err)
		}
		total := a.TasksCompleted + a.TasksFailed
		if total > 0 {
			a.SuccessRate = float64(a.TasksCompleted) / float64(total) * 100
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate efficiency: %w", err)
	}

	if agents == nil {
		agents = []AgentPerformance{}
	}
	return agents, nil
}

// GetPipelineHealth returns task count by status.
func (c *Collector) GetPipelineHealth() ([]PipelineHealth, error) {
	rows, err := c.db.Query(`
		SELECT status, COUNT(*) as count
		FROM tasks
		GROUP BY status
		ORDER BY count DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("query pipeline health: %w", err)
	}
	defer rows.Close()

	var health []PipelineHealth
	for rows.Next() {
		var h PipelineHealth
		if err := rows.Scan(&h.Status, &h.Count); err != nil {
			return nil, fmt.Errorf("scan pipeline row: %w", err)
		}
		health = append(health, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pipeline: %w", err)
	}

	if health == nil {
		health = []PipelineHealth{}
	}
	return health, nil
}

// errorCategories maps keywords in error_log to category names.
var errorCategories = map[string]string{
	"rate_limit":     "Rate Limit",
	"rate limit":     "Rate Limit",
	"timeout":        "Timeout",
	"timed out":      "Timeout",
	"build_failed":   "Build Failed",
	"build failed":   "Build Failed",
	"compile":        "Build Failed",
	"test_failed":    "Test Failed",
	"test failed":    "Test Failed",
	"tests failed":   "Test Failed",
	"git":            "Git Error",
	"merge conflict":  "Git Error",
	"decomposition":  "Decomposition",
	"decompose":      "Decomposition",
}

// GetFailures returns categorized error analysis for the given period.
func (c *Collector) GetFailures(period string) ([]FailureAnalysis, error) {
	pf := periodFilter(period)

	rows, err := c.db.Query(fmt.Sprintf(`
		SELECT error_log
		FROM tasks
		WHERE status = 'FAILED' AND error_log != '' AND %s
	`, pf))
	if err != nil {
		return nil, fmt.Errorf("query failures: %w", err)
	}
	defer rows.Close()

	categoryCount := make(map[string]int)
	categoryExamples := make(map[string][]string)

	for rows.Next() {
		var errorLog string
		if err := rows.Scan(&errorLog); err != nil {
			return nil, fmt.Errorf("scan failure row: %w", err)
		}

		lower := strings.ToLower(errorLog)
		categorized := false
		for keyword, category := range errorCategories {
			if strings.Contains(lower, keyword) {
				categoryCount[category]++
				if len(categoryExamples[category]) < 3 {
					truncated := errorLog
					if len(truncated) > 200 {
						truncated = truncated[:200] + "..."
					}
					categoryExamples[category] = append(categoryExamples[category], truncated)
				}
				categorized = true
				break
			}
		}
		if !categorized {
			categoryCount["Other"]++
			if len(categoryExamples["Other"]) < 3 {
				truncated := errorLog
				if len(truncated) > 200 {
					truncated = truncated[:200] + "..."
				}
				categoryExamples["Other"] = append(categoryExamples["Other"], truncated)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate failures: %w", err)
	}

	var failures []FailureAnalysis
	for category, count := range categoryCount {
		examples := categoryExamples[category]
		if examples == nil {
			examples = []string{}
		}
		failures = append(failures, FailureAnalysis{
			Category: category,
			Count:    count,
			Examples: examples,
		})
	}

	if failures == nil {
		failures = []FailureAnalysis{}
	}
	return failures, nil
}

// GetModelUsage returns token usage grouped by model for the given period.
func (c *Collector) GetModelUsage(period string) ([]ToolUsageStat, error) {
	pf := periodFilter(period)

	rows, err := c.db.Query(fmt.Sprintf(`
		SELECT
			COALESCE(NULLIF(model, ''), 'unknown') as model,
			COUNT(*) as task_count,
			COALESCE(SUM(tokens_used), 0) as tokens_used
		FROM tasks
		WHERE %s
		GROUP BY model
		ORDER BY tokens_used DESC
	`, pf))
	if err != nil {
		return nil, fmt.Errorf("query model usage: %w", err)
	}
	defer rows.Close()

	var stats []ToolUsageStat
	for rows.Next() {
		var s ToolUsageStat
		if err := rows.Scan(&s.Model, &s.TaskCount, &s.TokensUsed); err != nil {
			return nil, fmt.Errorf("scan model usage row: %w", err)
		}
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model usage: %w", err)
	}

	if stats == nil {
		stats = []ToolUsageStat{}
	}
	return stats, nil
}
