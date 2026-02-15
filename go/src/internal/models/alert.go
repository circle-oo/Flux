package models

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// Alert severity constants
const (
	AlertSeverityCritical = "CRITICAL"
	AlertSeverityHigh     = "HIGH"
	AlertSeverityMedium   = "MEDIUM"
	AlertSeverityLow      = "LOW"
)

// Alert type constants
const (
	AlertTypeHealthCheck = "HEALTH_CHECK"
	AlertTypeLatency     = "LATENCY"
	AlertTypeErrorRate   = "ERROR_RATE"
)

// Alert status constants
const (
	AlertActive   = "ACTIVE"
	AlertResolved = "RESOLVED"
)

// Alert represents a service alert.
type Alert struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	Severity    string `json:"severity"`
	Type        string `json:"type"`
	Message     string `json:"message"`
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ResolvedAt  string `json:"resolved_at"`
}

// AlertStore provides CRUD operations for alerts.
type AlertStore struct {
	DB *sql.DB
}

// NewAlertStore creates a new AlertStore.
func NewAlertStore(db *sql.DB) *AlertStore {
	return &AlertStore{DB: db}
}

// Create inserts a new alert.
func (s *AlertStore) Create(a *Alert) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	if a.Status == "" {
		a.Status = AlertActive
	}

	_, err := s.DB.Exec(
		`INSERT INTO alerts (id, service_name, severity, type, message, task_id, status, resolved_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ServiceName, a.Severity, a.Type, a.Message, a.TaskID, a.Status, a.ResolvedAt,
	)
	if err != nil {
		return fmt.Errorf("insert alert: %w", err)
	}
	return nil
}

// GetByID retrieves an alert by its ID.
func (s *AlertStore) GetByID(id string) (*Alert, error) {
	row := s.DB.QueryRow(alertSelectSQL+" WHERE id = ?", id)
	return scanAlert(row)
}

// List retrieves all alerts, optionally filtered by status.
func (s *AlertStore) List(status string) ([]*Alert, error) {
	query := alertSelectSQL
	var args []interface{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()

	var alerts []*Alert
	for rows.Next() {
		a, err := scanAlertRow(rows)
		if err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// Update modifies an existing alert.
func (s *AlertStore) Update(a *Alert) error {
	_, err := s.DB.Exec(
		`UPDATE alerts SET service_name = ?, severity = ?, type = ?, message = ?,
		 task_id = ?, status = ?, resolved_at = ? WHERE id = ?`,
		a.ServiceName, a.Severity, a.Type, a.Message, a.TaskID, a.Status, a.ResolvedAt, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	return nil
}

// Delete removes an alert by its ID.
func (s *AlertStore) Delete(id string) error {
	_, err := s.DB.Exec(`DELETE FROM alerts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}
	return nil
}

const alertSelectSQL = `SELECT id, service_name, severity, type, message, task_id, status, created_at, resolved_at
	FROM alerts`

func scanAlert(row *sql.Row) (*Alert, error) {
	var a Alert
	err := row.Scan(
		&a.ID, &a.ServiceName, &a.Severity, &a.Type, &a.Message,
		&a.TaskID, &a.Status, &a.CreatedAt, &a.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func scanAlertRow(rows *sql.Rows) (*Alert, error) {
	var a Alert
	err := rows.Scan(
		&a.ID, &a.ServiceName, &a.Severity, &a.Type, &a.Message,
		&a.TaskID, &a.Status, &a.CreatedAt, &a.ResolvedAt,
	)
	if err != nil {
		return nil, err
	}
	return &a, nil
}
