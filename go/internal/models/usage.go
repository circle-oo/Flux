package models

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// UsageSnapshot type constants
const (
	UsageTypeHourly       = "HOURLY"
	UsageTypeBlocks       = "BLOCKS"
	UsageTypeDailySummary = "DAILY_SUMMARY"
)

// UsageSnapshot represents a usage data snapshot.
type UsageSnapshot struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`
	Data        string  `json:"data"`
	TotalTokens int     `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
	RecordedAt  string  `json:"recorded_at"`
}

// UsageStore provides CRUD operations for usage snapshots.
type UsageStore struct {
	DB *sql.DB
}

// NewUsageStore creates a new UsageStore.
func NewUsageStore(db *sql.DB) *UsageStore {
	return &UsageStore{DB: db}
}

// CreateSnapshot inserts a new usage snapshot.
func (s *UsageStore) CreateSnapshot(u *UsageSnapshot) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	_, err := s.DB.Exec(
		`INSERT INTO usage_snapshots (id, type, data, total_tokens, total_cost) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Type, u.Data, u.TotalTokens, u.TotalCost,
	)
	if err != nil {
		return fmt.Errorf("insert usage snapshot: %w", err)
	}
	return nil
}

// ListSnapshots retrieves usage snapshots by type.
func (s *UsageStore) ListSnapshots(snapshotType string) ([]*UsageSnapshot, error) {
	query := `SELECT id, type, data, total_tokens, total_cost, recorded_at FROM usage_snapshots`
	var args []interface{}

	if snapshotType != "" {
		query += " WHERE type = ?"
		args = append(args, snapshotType)
	}
	query += " ORDER BY recorded_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []*UsageSnapshot
	for rows.Next() {
		var u UsageSnapshot
		if err := rows.Scan(&u.ID, &u.Type, &u.Data, &u.TotalTokens, &u.TotalCost, &u.RecordedAt); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, &u)
	}
	return snapshots, rows.Err()
}

// RateLimitEvent represents a rate limit event.
type RateLimitEvent struct {
	ID         string `json:"id"`
	TokensUsed int    `json:"tokens_used"`
	ActivePods int    `json:"active_pods"`
	OccurredAt string `json:"occurred_at"`
}

// CreateRateLimitEvent inserts a new rate limit event.
func (s *UsageStore) CreateRateLimitEvent(e *RateLimitEvent) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	_, err := s.DB.Exec(
		`INSERT INTO rate_limit_events (id, tokens_used, active_pods) VALUES (?, ?, ?)`,
		e.ID, e.TokensUsed, e.ActivePods,
	)
	if err != nil {
		return fmt.Errorf("insert rate limit event: %w", err)
	}
	return nil
}

// ListRateLimitEvents retrieves rate limit events.
func (s *UsageStore) ListRateLimitEvents() ([]*RateLimitEvent, error) {
	rows, err := s.DB.Query(
		`SELECT id, tokens_used, active_pods, occurred_at FROM rate_limit_events ORDER BY occurred_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query rate limit events: %w", err)
	}
	defer rows.Close()

	var events []*RateLimitEvent
	for rows.Next() {
		var e RateLimitEvent
		if err := rows.Scan(&e.ID, &e.TokensUsed, &e.ActivePods, &e.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, &e)
	}
	return events, rows.Err()
}

// ServiceMetric represents a service health metric.
type ServiceMetric struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	LatencyMs   int    `json:"latency_ms"`
	StatusCode  int    `json:"status_code"`
	IsHealthy   bool   `json:"is_healthy"`
	RecordedAt  string `json:"recorded_at"`
}

// CreateServiceMetric inserts a new service metric.
func (s *UsageStore) CreateServiceMetric(m *ServiceMetric) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	_, err := s.DB.Exec(
		`INSERT INTO service_metrics (id, service_name, latency_ms, status_code, is_healthy)
		 VALUES (?, ?, ?, ?, ?)`,
		m.ID, m.ServiceName, m.LatencyMs, m.StatusCode, m.IsHealthy,
	)
	if err != nil {
		return fmt.Errorf("insert service metric: %w", err)
	}
	return nil
}

// ListServiceMetrics retrieves service metrics by name.
func (s *UsageStore) ListServiceMetrics(serviceName string) ([]*ServiceMetric, error) {
	query := `SELECT id, service_name, latency_ms, status_code, is_healthy, recorded_at FROM service_metrics`
	var args []interface{}

	if serviceName != "" {
		query += " WHERE service_name = ?"
		args = append(args, serviceName)
	}
	query += " ORDER BY recorded_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query service metrics: %w", err)
	}
	defer rows.Close()

	var metrics []*ServiceMetric
	for rows.Next() {
		var m ServiceMetric
		if err := rows.Scan(&m.ID, &m.ServiceName, &m.LatencyMs, &m.StatusCode, &m.IsHealthy, &m.RecordedAt); err != nil {
			return nil, err
		}
		metrics = append(metrics, &m)
	}
	return metrics, rows.Err()
}
