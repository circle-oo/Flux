package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	content := `
server:
  port: 9090
  auth:
    enabled: false
database:
  path: /tmp/test.db
vault:
  path: /tmp/vault
orchestrator:
  max_total_pods: 2
executor:
  max_turns: 10
subtask:
  max_depth: 1
  max_per_task: 5
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090, got %d", cfg.Server.Port)
	}
	if cfg.Database.Path != "/tmp/test.db" {
		t.Errorf("expected database path /tmp/test.db, got %s", cfg.Database.Path)
	}
	if cfg.Vault.Path != "/tmp/vault" {
		t.Errorf("expected vault path /tmp/vault, got %s", cfg.Vault.Path)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_ExpandEnv(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	t.Setenv("TEST_DB_PATH", "/custom/db.sqlite")

	content := `
server:
  port: 8080
database:
  path: $TEST_DB_PATH
vault:
  path: /tmp/vault
orchestrator:
  max_total_pods: 1
executor:
  max_turns: 5
subtask:
  max_depth: 1
  max_per_task: 3
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Database.Path != "/custom/db.sqlite" {
		t.Errorf("expected /custom/db.sqlite, got %s", cfg.Database.Path)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")

	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_Valid(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
		},
		Database: DatabaseConfig{
			Path: "/tmp/test.db",
		},
		Vault: VaultConfig{
			Path: "/tmp/vault",
		},
		Orchestrator: OrchestratorConfig{
			MaxTotalPods: 2,
		},
		Executor: ExecutorConfig{
			MaxTurns: 10,
		},
		Subtask: SubtaskConfig{
			MaxDepth:   1,
			MaxPerTask: 5,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid, got error: %v", err)
	}
}

func TestValidate_InvalidPort(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{Port: 0},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for port 0")
	}

	cfg.Server.Port = 70000
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for port 70000")
	}
}

func TestValidate_MissingDatabasePath(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing database path")
	}
}

func TestValidate_MissingVaultPath(t *testing.T) {
	cfg := &Config{
		Server:   ServerConfig{Port: 8080},
		Database: DatabaseConfig{Path: "/tmp/test.db"},
		Vault:    VaultConfig{},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for missing vault path")
	}
}

func TestValidate_AuthEnabledNoPassword(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 8080,
			Auth: AuthConfig{Enabled: true, Password: ""},
		},
		Database: DatabaseConfig{Path: "/tmp/test.db"},
		Vault:    VaultConfig{Path: "/tmp/vault"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for auth enabled without password")
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/documents", filepath.Join(home, "documents")},
		{"~", home},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := expandHome(tt.input)
			if got != tt.expected {
				t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
