package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// TaskUsageEvent represents a single usage data point for a task.
type TaskUsageEvent struct {
	ID         int64             `json:"id"`
	TaskID     string            `json:"task_id"`
	Source     string            `json:"source"`
	Tokens     int               `json:"tokens"`
	CostUSD    float64           `json:"cost_usd"`
	Meta       map[string]string `json:"meta"`
	RecordedAt string            `json:"recorded_at"`
}

// UsageTimePoint represents an aggregated usage bucket for timeseries.
type UsageTimePoint struct {
	Time      string  `json:"time"`
	Tokens    int     `json:"tokens"`
	CostUSD   float64 `json:"cost_usd"`
	TaskCount int     `json:"task_count"`
}

// TaskUsageEventStore provides CRUD operations for task usage events.
type TaskUsageEventStore struct {
	DB *sql.DB
}

// NewTaskUsageEventStore creates a new TaskUsageEventStore.
func NewTaskUsageEventStore(db *sql.DB) *TaskUsageEventStore {
	return &TaskUsageEventStore{DB: db}
}

// Record inserts a usage event.
func (s *TaskUsageEventStore) Record(event *TaskUsageEvent) error {
	metaJSON := "{}"
	if event.Meta != nil {
		b, err := json.Marshal(event.Meta)
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}
		metaJSON = string(b)
	}
	if event.Source == "" {
		event.Source = "executor"
	}

	result, err := s.DB.Exec(
		`INSERT INTO task_usage_events (task_id, source, tokens, cost_usd, meta)
		 VALUES (?, ?, ?, ?, ?)`,
		event.TaskID, event.Source, event.Tokens, event.CostUSD, metaJSON,
	)
	if err != nil {
		return fmt.Errorf("insert usage event: %w", err)
	}

	id, _ := result.LastInsertId()
	event.ID = id
	return nil
}

// ListByTask returns all usage events for a task, ordered by time.
func (s *TaskUsageEventStore) ListByTask(taskID string) ([]*TaskUsageEvent, error) {
	rows, err := s.DB.Query(
		`SELECT id, task_id, source, tokens, cost_usd, meta, recorded_at
		 FROM task_usage_events WHERE task_id = ? ORDER BY recorded_at ASC`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("query usage events: %w", err)
	}
	defer rows.Close()

	var events []*TaskUsageEvent
	for rows.Next() {
		e, err := scanUsageEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// SumByTask returns aggregated totals for a task.
func (s *TaskUsageEventStore) SumByTask(taskID string) (tokens int, cost float64, err error) {
	err = s.DB.QueryRow(
		`SELECT COALESCE(SUM(tokens), 0), COALESCE(SUM(cost_usd), 0)
		 FROM task_usage_events WHERE task_id = ?`,
		taskID,
	).Scan(&tokens, &cost)
	if err != nil {
		return 0, 0, fmt.Errorf("sum usage: %w", err)
	}
	return tokens, cost, nil
}

// RecentTimeseries returns minute-bucketed usage for the last N minutes.
func (s *TaskUsageEventStore) RecentTimeseries(minutes int) ([]UsageTimePoint, error) {
	if minutes <= 0 {
		minutes = 60
	}

	rows, err := s.DB.Query(
		`SELECT
			strftime('%Y-%m-%dT%H:%M:00Z', recorded_at) as bucket,
			COALESCE(SUM(tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COUNT(DISTINCT task_id)
		 FROM task_usage_events
		 WHERE recorded_at >= datetime('now', ?)
		 GROUP BY bucket
		 ORDER BY bucket ASC`,
		fmt.Sprintf("-%d minutes", minutes),
	)
	if err != nil {
		return nil, fmt.Errorf("query timeseries: %w", err)
	}
	defer rows.Close()

	var points []UsageTimePoint
	for rows.Next() {
		var p UsageTimePoint
		if err := rows.Scan(&p.Time, &p.Tokens, &p.CostUSD, &p.TaskCount); err != nil {
			return nil, fmt.Errorf("scan timeseries: %w", err)
		}
		points = append(points, p)
	}
	return points, rows.Err()
}

func scanUsageEvent(rows *sql.Rows) (*TaskUsageEvent, error) {
	var e TaskUsageEvent
	var metaJSON string
	if err := rows.Scan(&e.ID, &e.TaskID, &e.Source, &e.Tokens, &e.CostUSD, &metaJSON, &e.RecordedAt); err != nil {
		return nil, fmt.Errorf("scan usage event: %w", err)
	}
	if metaJSON != "" {
		_ = json.Unmarshal([]byte(metaJSON), &e.Meta)
	}
	if e.Meta == nil {
		e.Meta = map[string]string{}
	}
	return &e, nil
}
