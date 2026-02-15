package db

import (
	"database/sql"
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
	Name    string
	Type    string
	RepoURL string
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
			slog.Info("seed project already exists, skipping", "name", p.Name)
			continue
		}

		id := uuid.New().String()
		_, err = database.Exec(
			`INSERT INTO projects (id, name, type, repo_url, status) VALUES (?, ?, ?, ?, 'ACTIVE')`,
			id, p.Name, p.Type, p.RepoURL,
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
