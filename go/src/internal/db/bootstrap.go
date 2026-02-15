package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// VaultDirs defines the Obsidian Vault directory structure to create during bootstrap.
var VaultDirs = []string{
	"Goals",
	"Goals/completed",
	"Goals/proposals",
	"Projects",
	"Research",
	"Research/Industry",
	"Research/Tools",
	"Research/Ideas",
	"Tasks",
	"Tasks/completed",
	"Services",
	"Services/alerts",
	"Templates",
}

// VaultFiles defines initial files to create in the Vault.
var VaultFiles = map[string]string{
	"Goals/_current.md":          "# Current Goal\n\nNo active goal set.\n",
	"Research/_history.md":       "# Research History\n\n",
	"Templates/project.md":       "# {{project_name}}\n\n## Overview\n\n## Architecture\n\n## Decisions\n\n",
	"Templates/research.md":      "# Research: {{topic}}\n\n## Summary\n\n## Findings\n\n## Recommendations\n\n",
	"Templates/task-summary.md":  "# Task: {{title}}\n\n## Result\n\n## Changes\n\n## Learnings\n\n",
	"Templates/decision.md":      "# Decision: {{title}}\n\n## Context\n\n## Decision\n\n## Consequences\n\n",
}

// SeedProject represents a project to register during bootstrap.
type SeedProject struct {
	Name        string
	Type        string
	RepoURL     string
	Description string
	TechStack   []string
}

// CreateVaultDirs creates the Obsidian Vault directory structure.
func CreateVaultDirs(vaultPath string) error {
	for _, dir := range VaultDirs {
		fullPath := filepath.Join(vaultPath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("create vault directory %s: %w", dir, err)
		}
	}

	// Create initial files if they don't exist
	for relPath, content := range VaultFiles {
		fullPath := filepath.Join(vaultPath, relPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				return fmt.Errorf("create vault file %s: %w", relPath, err)
			}
		}
	}

	return nil
}

// RegisterSeedProjects inserts seed projects into the database if they don't already exist.
func RegisterSeedProjects(database *sql.DB, projects []SeedProject) error {
	for _, p := range projects {
		exists, err := exists(database, "projects", "name = ?", p.Name)
		if err != nil {
			return fmt.Errorf("check seed project %s: %w", p.Name, err)
		}
		if exists {
			// Update description and tech_stack if they changed in config
			techStackJSON := "[]"
			if len(p.TechStack) > 0 {
				if b, err := json.Marshal(p.TechStack); err == nil {
					techStackJSON = string(b)
				}
			}
			if p.Description != "" || len(p.TechStack) > 0 {
				_, _ = database.Exec(
					`UPDATE projects SET description = ?, tech_stack = ?, updated_at = CURRENT_TIMESTAMP WHERE name = ? AND (description = '' OR description IS NULL)`,
					p.Description, techStackJSON, p.Name,
				)
			}
			slog.Info("seed project already exists, updated metadata", "name", p.Name)
			continue
		}

		id := uuid.New().String()
		techStackJSON := "[]"
		if len(p.TechStack) > 0 {
			if b, err := json.Marshal(p.TechStack); err == nil {
				techStackJSON = string(b)
			}
		}
		_, err = database.Exec(
			`INSERT INTO projects (id, name, type, repo_url, description, tech_stack, status) VALUES (?, ?, ?, ?, ?, ?, 'ACTIVE')`,
			id, p.Name, p.Type, p.RepoURL, p.Description, techStackJSON,
		)
		if err != nil {
			return fmt.Errorf("insert seed project %s: %w", p.Name, err)
		}
		slog.Info("registered seed project", "name", p.Name, "id", id)
	}
	return nil
}

// Bootstrap runs the full bootstrap sequence: create schema, vault dirs, and seed projects.
func Bootstrap(database *sql.DB, vaultPath string, projects []SeedProject) error {
	if err := CreateSchema(database); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	if err := CreateVaultDirs(vaultPath); err != nil {
		return fmt.Errorf("create vault dirs: %w", err)
	}

	if err := RegisterSeedProjects(database, projects); err != nil {
		return fmt.Errorf("register seed projects: %w", err)
	}

	return nil
}
