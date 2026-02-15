package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCreateVaultDirs(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "vault")

	if err := CreateVaultDirs(vaultPath); err != nil {
		t.Fatalf("CreateVaultDirs: %v", err)
	}

	// Check key directories exist
	expectedDirs := []string{
		"Goals", "Goals/completed", "Goals/proposals",
		"Projects",
		"Research", "Research/Industry", "Research/Tools", "Research/Ideas",
		"Tasks", "Tasks/completed",
		"Services", "Services/alerts",
		"Templates",
	}
	for _, d := range expectedDirs {
		p := filepath.Join(vaultPath, d)
		info, err := os.Stat(p)
		if err != nil {
			t.Errorf("directory %s not created: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected directory, got file: %s", d)
		}
	}

	// Check key files exist
	expectedFiles := []string{
		"Goals/_current.md",
		"Research/_history.md",
		"Templates/project.md",
	}
	for _, f := range expectedFiles {
		p := filepath.Join(vaultPath, f)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("file %s not created: %v", f, err)
		}
	}

	// Idempotent: run again
	if err := CreateVaultDirs(vaultPath); err != nil {
		t.Fatalf("CreateVaultDirs (idempotent): %v", err)
	}
}

func TestRegisterSeedProjects(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()
	CreateSchema(database)

	projects := []SeedProject{
		{Name: "flux", Type: "REPO", RepoURL: "https://github.com/circle-oo/flux"},
	}

	if err := RegisterSeedProjects(database, projects); err != nil {
		t.Fatalf("RegisterSeedProjects: %v", err)
	}

	// Verify project exists
	var name string
	err = database.QueryRow("SELECT name FROM projects WHERE name = 'flux'").Scan(&name)
	if err != nil {
		t.Fatalf("query seed project: %v", err)
	}
	if name != "flux" {
		t.Errorf("expected name=flux, got %s", name)
	}

	// Idempotent: run again
	if err := RegisterSeedProjects(database, projects); err != nil {
		t.Fatalf("RegisterSeedProjects (idempotent): %v", err)
	}

	// Still only one
	var count int
	database.QueryRow("SELECT COUNT(*) FROM projects WHERE name = 'flux'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 flux project, got %d", count)
	}
}

func TestBootstrap(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	vaultPath := filepath.Join(dir, "vault")

	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	projects := []SeedProject{
		{Name: "flux", Type: "REPO", RepoURL: "https://github.com/circle-oo/flux"},
	}

	if err := Bootstrap(database, vaultPath, projects); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Verify tables
	var name string
	err = database.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='goals'").Scan(&name)
	if err != nil {
		t.Error("goals table not found")
	}

	// Verify vault
	if _, err := os.Stat(filepath.Join(vaultPath, "Goals")); err != nil {
		t.Error("Goals vault dir not found")
	}

	// Verify seed project
	err = database.QueryRow("SELECT name FROM projects WHERE name='flux'").Scan(&name)
	if err != nil {
		t.Error("seed project not found")
	}
}
