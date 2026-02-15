package models

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Goal status constants
const (
	GoalProposed   = "PROPOSED"
	GoalActive     = "ACTIVE"
	GoalCompleted  = "COMPLETED"
	GoalSuperseded = "SUPERSEDED"
)

// Goal source constants
const (
	GoalSourceOperator     = "OPERATOR"
	GoalSourceOrchestrator = "ORCHESTRATOR"
)

// Goal represents a high-level objective for the system.
type Goal struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Priorities  []string `json:"priorities"`
	Metrics     []string `json:"metrics"`
	Status      string   `json:"status"`
	Source      string   `json:"source"`
	CreatedAt   string   `json:"created_at"`
	ActiveSince string   `json:"active_since"`
}

// GoalStore provides CRUD operations for goals.
type GoalStore struct {
	DB *sql.DB
}

// NewGoalStore creates a new GoalStore.
func NewGoalStore(db *sql.DB) *GoalStore {
	return &GoalStore{DB: db}
}

// Create inserts a new goal.
func (s *GoalStore) Create(g *Goal) error {
	if g.ID == "" {
		g.ID = uuid.New().String()
	}
	if g.Status == "" {
		g.Status = GoalProposed
	}
	if g.Source == "" {
		g.Source = GoalSourceOperator
	}
	if g.Priorities == nil {
		g.Priorities = []string{}
	}
	if g.Metrics == nil {
		g.Metrics = []string{}
	}

	prioritiesJSON, err := marshalStringSlice("priorities", g.Priorities)
	if err != nil {
		return err
	}
	metricsJSON, err := marshalStringSlice("metrics", g.Metrics)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`INSERT INTO goals (id, title, description, priorities, metrics, status, source, active_since)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.Title, g.Description, prioritiesJSON, metricsJSON,
		g.Status, g.Source, g.ActiveSince,
	)
	if err != nil {
		return fmt.Errorf("insert goal: %w", err)
	}
	return nil
}

// GetByID retrieves a goal by its ID.
func (s *GoalStore) GetByID(id string) (*Goal, error) {
	row := s.DB.QueryRow(goalSelectSQL+" WHERE id = ?", id)
	return scanGoal(row)
}

// GetCurrent retrieves the current ACTIVE goal (at most one).
func (s *GoalStore) GetCurrent() (*Goal, error) {
	row := s.DB.QueryRow(goalSelectSQL+" WHERE status = ? LIMIT 1", GoalActive)
	g, err := scanGoal(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

// List retrieves all goals ordered by created_at descending.
func (s *GoalStore) List() ([]*Goal, error) {
	rows, err := s.DB.Query(goalSelectSQL + " ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query goals: %w", err)
	}
	defer rows.Close()

	var goals []*Goal
	for rows.Next() {
		g, err := scanGoal(rows)
		if err != nil {
			return nil, err
		}
		goals = append(goals, g)
	}
	return goals, rows.Err()
}

// Update modifies an existing goal.
func (s *GoalStore) Update(g *Goal) error {
	prioritiesJSON, err := marshalStringSlice("priorities", g.Priorities)
	if err != nil {
		return err
	}
	metricsJSON, err := marshalStringSlice("metrics", g.Metrics)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`UPDATE goals SET title = ?, description = ?, priorities = ?, metrics = ?,
		 status = ?, source = ?, active_since = ? WHERE id = ?`,
		g.Title, g.Description, prioritiesJSON, metricsJSON,
		g.Status, g.Source, g.ActiveSince, g.ID,
	)
	if err != nil {
		return fmt.Errorf("update goal: %w", err)
	}
	return nil
}

// Delete removes a goal by its ID.
func (s *GoalStore) Delete(id string) error {
	_, err := s.DB.Exec(`DELETE FROM goals WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete goal: %w", err)
	}
	return nil
}

// Activate sets a goal to ACTIVE, superseding any currently active goal.
// Enforces the single-ACTIVE constraint.
func (s *GoalStore) Activate(id string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Supersede any currently active goal
	_, err = tx.Exec(
		`UPDATE goals SET status = ? WHERE status = ?`,
		GoalSuperseded, GoalActive,
	)
	if err != nil {
		return fmt.Errorf("supersede active goal: %w", err)
	}

	// Activate the target goal
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := tx.Exec(
		`UPDATE goals SET status = ?, active_since = ? WHERE id = ?`,
		GoalActive, now, id,
	)
	if err != nil {
		return fmt.Errorf("activate goal: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("goal not found: %s", id)
	}

	return tx.Commit()
}

const goalSelectSQL = `SELECT id, title, description, priorities, metrics, status, source, created_at, active_since
	FROM goals`

// scanGoal scans a goal from any source that implements Scan (works with
// both *sql.Row and *sql.Rows). This is the single source of truth for
// the scan field order — it must match goalSelectSQL.
func scanGoal(scanner interface{ Scan(...interface{}) error }) (*Goal, error) {
	var g Goal
	var prioritiesJSON, metricsJSON string
	err := scanner.Scan(
		&g.ID, &g.Title, &g.Description, &prioritiesJSON, &metricsJSON,
		&g.Status, &g.Source, &g.CreatedAt, &g.ActiveSince,
	)
	if err != nil {
		return nil, err
	}
	g.Priorities = unmarshalStringSlice("priorities", prioritiesJSON)
	g.Metrics = unmarshalStringSlice("metrics", metricsJSON)
	return &g, nil
}
