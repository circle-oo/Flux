package models

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// Project status constants
const (
	ProjectProposed = "PROPOSED"
	ProjectActive   = "ACTIVE"
	ProjectArchived = "ARCHIVED"
	ProjectRejected = "REJECTED"
)

// Project type constants
const (
	ProjectTypeRepo    = "REPO"
	ProjectTypeService = "SERVICE"
	ProjectTypeLibrary = "LIBRARY"
	ProjectTypeTool    = "TOOL"
)

// Project represents a managed project.
type Project struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	RepoURL     string   `json:"repo_url"`
	Description string   `json:"description"`
	VaultPath   string   `json:"vault_path"`
	Status      string   `json:"status"`
	TechStack   []string `json:"tech_stack"`
	Inspiration string   `json:"inspiration"`
	GoalID      string   `json:"goal_id"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ProjectStore provides CRUD operations for projects.
type ProjectStore struct {
	DB *sql.DB
}

// NewProjectStore creates a new ProjectStore.
func NewProjectStore(db *sql.DB) *ProjectStore {
	return &ProjectStore{DB: db}
}

// Create inserts a new project.
func (s *ProjectStore) Create(p *Project) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.Status == "" {
		p.Status = ProjectProposed
	}
	if p.TechStack == nil {
		p.TechStack = []string{}
	}

	techStackJSON, err := marshalStringSlice("tech_stack", p.TechStack)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`INSERT INTO projects (id, name, type, repo_url, description, vault_path,
		 status, tech_stack, inspiration, goal_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Type, p.RepoURL, p.Description, p.VaultPath,
		p.Status, techStackJSON, p.Inspiration, p.GoalID,
	)
	if err != nil {
		return fmt.Errorf("insert project: %w", err)
	}
	return nil
}

// GetByID retrieves a project by its ID.
func (s *ProjectStore) GetByID(id string) (*Project, error) {
	row := s.DB.QueryRow(projectSelectSQL+" WHERE id = ?", id)
	return scanProject(row)
}

// List retrieves all projects, optionally filtered by status.
func (s *ProjectStore) List(status string) ([]*Project, error) {
	query := projectSelectSQL
	var args []interface{}

	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}

	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// Update modifies an existing project.
func (s *ProjectStore) Update(p *Project) error {
	techStackJSON, err := marshalStringSlice("tech_stack", p.TechStack)
	if err != nil {
		return err
	}

	_, err = s.DB.Exec(
		`UPDATE projects SET name = ?, type = ?, repo_url = ?, description = ?,
		 vault_path = ?, status = ?, tech_stack = ?, inspiration = ?, goal_id = ?,
		 updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		p.Name, p.Type, p.RepoURL, p.Description,
		p.VaultPath, p.Status, techStackJSON, p.Inspiration, p.GoalID, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

// Delete removes a project by its ID.
func (s *ProjectStore) Delete(id string) error {
	_, err := s.DB.Exec(`DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	return nil
}

// Approve transitions a project from PROPOSED to ACTIVE.
func (s *ProjectStore) Approve(id string) error {
	result, err := s.DB.Exec(
		`UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?`,
		ProjectActive, id, ProjectProposed,
	)
	if err != nil {
		return fmt.Errorf("approve project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project not found or not in PROPOSED status: %s", id)
	}
	return nil
}

// Reject transitions a project from PROPOSED to REJECTED.
func (s *ProjectStore) Reject(id string) error {
	result, err := s.DB.Exec(
		`UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND status = ?`,
		ProjectRejected, id, ProjectProposed,
	)
	if err != nil {
		return fmt.Errorf("reject project: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("project not found or not in PROPOSED status: %s", id)
	}
	return nil
}

const projectSelectSQL = `SELECT id, name, type, repo_url, description, vault_path,
	status, tech_stack, inspiration, goal_id, created_at, updated_at
	FROM projects`

// scanProject scans a project from any source that implements Scan (works with
// both *sql.Row and *sql.Rows). This is the single source of truth for
// the scan field order — it must match projectSelectSQL.
func scanProject(scanner interface{ Scan(...interface{}) error }) (*Project, error) {
	var p Project
	var techStackJSON string
	err := scanner.Scan(
		&p.ID, &p.Name, &p.Type, &p.RepoURL, &p.Description, &p.VaultPath,
		&p.Status, &techStackJSON, &p.Inspiration, &p.GoalID,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.TechStack = unmarshalStringSlice("tech_stack", techStackJSON)
	return &p, nil
}

// ListByName retrieves all projects matching partial name, used for dedup checks.
func (s *ProjectStore) ListByName(name string) ([]*Project, error) {
	rows, err := s.DB.Query(projectSelectSQL+" WHERE name = ?", name)
	if err != nil {
		return nil, fmt.Errorf("query projects by name: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

