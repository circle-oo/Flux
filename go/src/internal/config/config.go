package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Vault         VaultConfig         `yaml:"vault"`
	GitHub        GitHubConfig        `yaml:"github"`
	ClaudeCode    ClaudeCodeConfig    `yaml:"claude_code"`
	CCUsage       CCUsageConfig       `yaml:"ccusage"`
	Orchestrator  OrchestratorConfig  `yaml:"orchestrator"`
	Executor      ExecutorConfig      `yaml:"executor"`
	Triager       TriagerConfig       `yaml:"triager"`
	Subtask       SubtaskConfig       `yaml:"subtask"`
	Shutdown      ShutdownConfig      `yaml:"shutdown"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Services      []ServiceConfig     `yaml:"services"`
	Cleanup       CleanupConfig       `yaml:"cleanup"`
	AutoUpdate    AutoUpdateConfig    `yaml:"auto_update"`
	Projects      []ProjectSeed       `yaml:"projects"`
	Logging       LoggingConfig       `yaml:"logging"`
}

type TriagerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Model    string `yaml:"model"`
	MaxTurns int    `yaml:"max_turns"`
}

type ServerConfig struct {
	Host string     `yaml:"host"` // bind address: "0.0.0.0" (all interfaces), "127.0.0.1" (localhost only), or Tailscale IP
	Port int        `yaml:"port"`
	Auth AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
	Enabled       bool   `yaml:"enabled"`
	PasswordEnv   string `yaml:"password_env"`
	Password      string `yaml:"-"` // resolved from env
	SessionExpiry string `yaml:"session_expiry"`
}

type DatabaseConfig struct {
	Path                string `yaml:"path"`
	BackupDir           string `yaml:"backup_dir"`
	BackupCron          string `yaml:"backup_cron"`
	BackupRetentionDays int    `yaml:"backup_retention_days"`
}

type VaultConfig struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type GitHubConfig struct {
	Username          string `yaml:"username"`
	TokenEnv          string `yaml:"token_env"`
	Token             string `yaml:"-"` // resolved from env
	AutoCreateRepo    bool   `yaml:"auto_create_repo"`
	DefaultVisibility string `yaml:"default_visibility"`
}

type ClaudeCodeConfig struct {
	Plan string `yaml:"plan"`
}

type CCUsageConfig struct {
	Command            string        `yaml:"command"`
	CollectionInterval time.Duration `yaml:"collection_interval"`
}

type OrchestratorConfig struct {
	CheckInterval    time.Duration `yaml:"check_interval"`
	ScaleCooldown    time.Duration `yaml:"scale_cooldown"`
	MaxTotalPods     int           `yaml:"max_total_pods"`
	MinResearchRatio float64       `yaml:"min_research_ratio"`
	WorkspaceBase    string        `yaml:"workspace_base"`
	DailySummaryHour int           `yaml:"daily_summary_hour"`
	Models           ModelsConfig  `yaml:"models"`
}

type ModelsConfig struct {
	Opus   string `yaml:"opus"`
	Sonnet string `yaml:"sonnet"`
}

type ExecutorConfig struct {
	MaxExecutionTime       time.Duration `yaml:"max_execution_time"`
	MaxOutputSize          int64         `yaml:"max_output_size"`
	MaxTurns               int           `yaml:"max_turns"`
	MaxDiffLines           int           `yaml:"max_diff_lines"`
	MaxFilesChanged        int           `yaml:"max_files_changed"`
	RegistrationMaxRetries int           `yaml:"registration_max_retries"`
	RegistrationInitialDelay time.Duration `yaml:"registration_initial_delay"`
}

type SubtaskConfig struct {
	MaxDepth   int `yaml:"max_depth"`
	MaxPerTask int `yaml:"max_per_task"`
}

type ShutdownConfig struct {
	PodGracePeriod time.Duration `yaml:"pod_grace_period"`
	ForceKillAfter time.Duration `yaml:"force_kill_after"`
}

type NotificationsConfig struct {
	Discord DiscordConfig `yaml:"discord"`
}

type DiscordConfig struct {
	WebhookURLEnv string `yaml:"webhook_url_env"`
	WebhookURL    string `yaml:"-"` // resolved from env
}

type ServiceConfig struct {
	Name          string        `yaml:"name"`
	Endpoint      string        `yaml:"endpoint"`
	ProjectID     string        `yaml:"project_id"`
	CheckInterval time.Duration `yaml:"check_interval"`
}

type CleanupConfig struct {
	ServiceMetricsRawDays int `yaml:"service_metrics_raw_days"`
	JSONLRetentionDays    int `yaml:"jsonl_retention_days"`
	UsageSnapshotsDays    int `yaml:"usage_snapshots_days"`
	FailedWorktreeHours   int `yaml:"failed_worktree_hours"`
}

type AutoUpdateConfig struct {
	Enabled       bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"check_interval"`
	Branch        string        `yaml:"branch"`
}

type ProjectSeed struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	RepoURL     string   `yaml:"repo_url"`
	Description string   `yaml:"description"`
	TechStack   []string `yaml:"tech_stack"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// Load reads a YAML config file, resolves environment variables, and expands paths.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Resolve environment variables for _env fields
	if cfg.GitHub.TokenEnv != "" {
		cfg.GitHub.Token = os.Getenv(cfg.GitHub.TokenEnv)
	}
	if cfg.Notifications.Discord.WebhookURLEnv != "" {
		cfg.Notifications.Discord.WebhookURL = os.Getenv(cfg.Notifications.Discord.WebhookURLEnv)
	}
	if cfg.Server.Auth.PasswordEnv != "" {
		cfg.Server.Auth.Password = os.Getenv(cfg.Server.Auth.PasswordEnv)
	}

	// Expand ~ in vault path
	cfg.Vault.Path = expandHome(cfg.Vault.Path)

	return &cfg, nil
}

// Validate checks that the config has valid values.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", c.Server.Port)
	}
	if c.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}
	if c.Vault.Path == "" {
		return fmt.Errorf("vault.path is required")
	}
	if c.Server.Auth.Enabled && c.Server.Auth.Password == "" {
		return fmt.Errorf("server.auth.password_env must resolve to a non-empty value when auth is enabled")
	}
	if c.Orchestrator.MaxTotalPods < 1 {
		return fmt.Errorf("orchestrator.max_total_pods must be >= 1")
	}
	if c.Executor.MaxTurns < 1 {
		return fmt.Errorf("executor.max_turns must be >= 1")
	}
	if c.Subtask.MaxDepth < 0 {
		return fmt.Errorf("subtask.max_depth must be >= 0")
	}
	if c.Subtask.MaxPerTask < 1 {
		return fmt.Errorf("subtask.max_per_task must be >= 1")
	}
	return nil
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	return path
}
