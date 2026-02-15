# Flux — Autonomous Engineering System: Implementation Specification

> **Audience**: Claude Code Agent / AI coding agent implementing this system.
> **Purpose**: This document contains every detail needed to implement Flux from scratch. Follow it literally. When in doubt, prefer the simpler approach.

---

## TABLE OF CONTENTS

1. [Project Setup](#1-project-setup)
2. [Directory Structure](#2-directory-structure)
3. [Configuration](#3-configuration)
4. [Database Schema](#4-database-schema)
5. [Core Types & Models](#5-core-types--models)
6. [State Machines](#6-state-machines)
7. [Internal Communication](#7-internal-communication)
8. [Integrations](#8-integrations)
9. [Goal System](#9-goal-system)
10. [Usage Tracking](#10-usage-tracking)
11. [Rate Limit Handling](#11-rate-limit-handling)
12. [Model Selection](#12-model-selection)
13. [QA & Branch Strategy](#13-qa--branch-strategy)
14. [PR Review](#14-pr-review)
15. [Orchestration](#15-orchestration)
16. [Core Components](#16-core-components)
17. [Web UI](#17-web-ui)
18. [API Reference](#18-api-reference)
19. [Bootstrap Sequence](#19-bootstrap-sequence)
20. [Self-Improvement](#20-self-improvement)
21. [Error Recovery](#21-error-recovery)
22. [Graceful Shutdown](#22-graceful-shutdown)
23. [Implementation Phases](#23-implementation-phases)
24. [Conventions & Rules](#24-conventions--rules)
25. [Future Work](#25-future-work)

---

## 1. PROJECT SETUP

### 1.1 Prerequisites

```bash
# macOS
# Go 1.22+
go version  # must be >= 1.22

# Node.js 20+ (for Claude Code CLI and Web UI build)
node --version  # must be >= 20

# Claude Code CLI (subscription-based, max_5x plan)
claude --version
claude login  # one-time authentication

# Git
git --version

# ccusage (optional, for usage tracking)
npx ccusage@latest --version
```

### 1.2 Go Module Initialization

```bash
mkdir -p ~/workspaces/flux
cd ~/workspaces/flux
go mod init github.com/circle-oo/flux
```

### 1.3 Key Dependencies

```bash
go get modernc.org/sqlite          # SQLite without CGO
go get gopkg.in/yaml.v3            # YAML config parsing
go get github.com/google/uuid      # UUID generation
go get nhooyr.io/websocket         # WebSocket support
go get golang.org/x/crypto/bcrypt  # Password hashing
```

### 1.4 Build Command

```bash
# Single binary with embedded Web UI
go build -o flux ./cmd/flux
```

---

## 2. DIRECTORY STRUCTURE

### 2.1 Go Project Structure

```
flux/
├── cmd/
│   └── flux/
│       └── main.go                    # Entry point
├── internal/
│   ├── config/
│   │   └── config.go                  # YAML config loader → struct
│   ├── db/
│   │   ├── db.go                      # SQLite connection (WAL mode)
│   │   ├── schema.go                  # Schema creation + migration
│   │   └── queries.go                 # Shared query helpers
│   ├── models/
│   │   ├── goal.go                    # Goal struct + CRUD
│   │   ├── project.go                 # Project struct + CRUD
│   │   ├── task.go                    # Task struct + CRUD + Priority Queue
│   │   ├── alert.go                   # Alert struct + CRUD
│   │   └── usage.go                   # UsageSnapshot, RateLimitEvent structs
│   ├── manager/
│   │   ├── manager.go                 # Task queue, assignment, state transitions
│   │   └── priority.go                # Priority Queue with Goal boost
│   ├── orchestrator/
│   │   ├── orchestrator.go            # Main orchestrator loop (5-min tick)
│   │   ├── scale_manager.go           # Pod count/ratio, cooldown
│   │   ├── usage_collector.go         # ccusage time-series collection
│   │   ├── daily_summary.go           # Discord daily summary
│   │   ├── rate_limit_handler.go      # Detection → stop → dynamic wait → resume
│   │   └── goal_advisor.go            # Goal proposals
│   ├── executor/
│   │   ├── executor.go                # Executor Pod main loop
│   │   ├── claude_code.go             # Claude Code CLI wrapper (-p mode)
│   │   ├── guardrails.go              # Timeout, output size, diff limits
│   │   ├── subtask.go                 # Subtask decomposition
│   │   └── worktree.go                # Git worktree management
│   ├── researcher/
│   │   ├── researcher.go              # Researcher Pod main loop
│   │   └── types.go                   # Research type definitions
│   ├── vault/
│   │   ├── writer.go                  # Single-goroutine channel-based writer
│   │   └── templates.go               # Obsidian markdown templates
│   ├── github/
│   │   ├── client.go                  # GitHub API client
│   │   ├── repo.go                    # Repo creation
│   │   └── pr.go                      # PR create/merge/comment fetch
│   ├── notifier/
│   │   └── discord.go                 # Discord webhook notifications
│   ├── server/
│   │   ├── server.go                  # HTTP server setup
│   │   ├── auth.go                    # Password auth (bcrypt + session token)
│   │   ├── api_goals.go               # /api/goals handlers
│   │   ├── api_tasks.go               # /api/tasks handlers
│   │   ├── api_projects.go            # /api/projects handlers
│   │   ├── api_prs.go                 # /api/prs handlers
│   │   ├── api_usage.go               # /api/usage handlers
│   │   ├── api_orchestrator.go        # /api/orchestrator handlers
│   │   ├── internal_api.go            # /internal/ handlers (Pod communication)
│   │   └── websocket.go               # /ws/events handler
│   └── shutdown/
│       └── shutdown.go                # Graceful shutdown logic
├── web/                               # React frontend
│   ├── src/
│   │   ├── App.tsx
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Goals.tsx
│   │   │   ├── Tasks.tsx
│   │   │   ├── Projects.tsx
│   │   │   ├── PRs.tsx
│   │   │   ├── Research.tsx
│   │   │   ├── Usage.tsx
│   │   │   └── Settings.tsx
│   │   ├── components/
│   │   ├── stores/                    # Zustand stores
│   │   └── lib/
│   ├── package.json
│   ├── vite.config.ts
│   └── tailwind.config.js
├── data/                              # Runtime data (gitignored)
│   ├── flux.db                        # SQLite database
│   └── backups/                       # Daily backups
├── workspaces/                        # Git worktrees (gitignored)
│   ├── repos/                         # Bare repos
│   └── trees/                         # Per-task worktrees
├── logs/                              # Log files (gitignored)
├── config.yaml                        # Configuration file
├── go.mod
├── go.sum
└── Makefile
```

### 2.2 Obsidian Vault Structure

Created during bootstrap at `~/ObsidianVault/Flux/`:

```
~/ObsidianVault/Flux/
├── Goals/
│   ├── _current.md                    # Current active Goal
│   ├── completed/                     # Completed Goals
│   └── proposals/                     # Proposed Goals
├── Projects/
│   └── {project-name}/
│       ├── _index.md                  # Project overview
│       ├── architecture.md            # Architecture decisions
│       ├── decisions/                 # Decision records
│       └── learnings/                 # Lessons learned
├── Research/
│   ├── Industry/                      # Industry research
│   ├── Tools/                         # Tool evaluations
│   ├── Ideas/                         # Project ideas
│   └── _history.md                    # Research log
├── Tasks/
│   └── completed/                     # Completed task summaries
├── Services/
│   └── alerts/                        # Service alert records
└── Templates/
    ├── project.md                     # Project template
    ├── research.md                    # Research template
    ├── task-summary.md                # Task completion summary
    └── decision.md                    # Decision record template
```

### 2.3 Workspace Structure

```
workspaces/
├── repos/                             # Bare Git repositories
│   └── flux.git                       # git clone --bare
├── trees/                             # Per-task worktrees
│   ├── flux--task-abc123/             # Branch: task/abc123
│   └── flux--task-def456/             # Branch: task/def456
└── research/                          # Researcher workspaces
    ├── researcher-01/
    │   ├── workspace/                 # CLAUDE.md, .claude/settings.json
    │   └── output/                    # Temporary → move to Obsidian
    └── researcher-02/
        ├── workspace/
        └── output/
```

---

## 3. CONFIGURATION

### 3.1 config.yaml (Complete Template)

```yaml
server:
  port: 8080
  auth:
    enabled: true
    password_env: "FLUX_UI_PASSWORD"     # Plain text → Flux compares bcrypt at boot
    session_expiry: "none"               # Tailscale-dependent, explicit logout only

database:
  path: "./data/flux.db"
  backup_dir: "./data/backups"
  backup_cron: "0 4 * * *"              # Daily 4am
  backup_retention_days: 7

vault:
  path: "~/ObsidianVault/Flux"

github:
  username: "circle-oo"
  token_env: "GITHUB_TOKEN"
  auto_create_repo: true
  default_visibility: "public"

claude_code:
  plan: "max_5x"                         # pro, max_5x, max_20x

ccusage:
  command: "npx ccusage@latest"
  collection_interval: 1h

orchestrator:
  check_interval: 5m
  scale_cooldown: 15m
  max_total_pods: 5
  min_research_ratio: 0.2
  workspace_base: "./workspaces"
  daily_summary_hour: 0                  # Midnight

  models:
    opus: "opus"                         # Claude Code alias (always latest)
    sonnet: "sonnet"

executor:
  max_execution_time: 30m
  max_output_size: 10485760              # 10MB
  max_turns: 30                          # Claude Code --max-turns
  max_diff_lines: 2000
  max_files_changed: 20

subtask:
  max_depth: 1
  max_per_task: 5

shutdown:
  pod_grace_period: 10m
  force_kill_after: 12m

notifications:
  discord:
    webhook_url_env: "DISCORD_WEBHOOK_URL"

services: []

cleanup:
  service_metrics_raw_days: 7
  jsonl_retention_days: 30
  usage_snapshots_days: 90
  failed_worktree_hours: 24

projects:
  - name: "flux"
    type: "REPO"
    repo_url: "https://github.com/circle-oo/flux"

logging:
  level: "info"
  file: "./logs/flux.log"
```

### 3.2 Config Go Struct

```go
package config

type Config struct {
    Server       ServerConfig       `yaml:"server"`
    Database     DatabaseConfig     `yaml:"database"`
    Vault        VaultConfig        `yaml:"vault"`
    GitHub       GitHubConfig       `yaml:"github"`
    ClaudeCode   ClaudeCodeConfig   `yaml:"claude_code"`
    CCUsage      CCUsageConfig      `yaml:"ccusage"`
    Orchestrator OrchestratorConfig `yaml:"orchestrator"`
    Executor     ExecutorConfig     `yaml:"executor"`
    Subtask      SubtaskConfig      `yaml:"subtask"`
    Shutdown     ShutdownConfig     `yaml:"shutdown"`
    Notifications NotificationsConfig `yaml:"notifications"`
    Services     []ServiceConfig    `yaml:"services"`
    Cleanup      CleanupConfig      `yaml:"cleanup"`
    Projects     []ProjectSeed      `yaml:"projects"`
    Logging      LoggingConfig      `yaml:"logging"`
}

type ServerConfig struct {
    Port int        `yaml:"port"`
    Auth AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
    Enabled     bool   `yaml:"enabled"`
    PasswordEnv string `yaml:"password_env"`
    SessionExpiry string `yaml:"session_expiry"`
}

type DatabaseConfig struct {
    Path              string `yaml:"path"`
    BackupDir         string `yaml:"backup_dir"`
    BackupCron        string `yaml:"backup_cron"`
    BackupRetentionDays int  `yaml:"backup_retention_days"`
}

type VaultConfig struct {
    Path string `yaml:"path"`
}

type GitHubConfig struct {
    Username          string `yaml:"username"`
    TokenEnv          string `yaml:"token_env"`
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
    MaxExecutionTime time.Duration `yaml:"max_execution_time"`
    MaxOutputSize    int64         `yaml:"max_output_size"`
    MaxTurns         int           `yaml:"max_turns"`
    MaxDiffLines     int           `yaml:"max_diff_lines"`
    MaxFilesChanged  int           `yaml:"max_files_changed"`
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

type ProjectSeed struct {
    Name    string `yaml:"name"`
    Type    string `yaml:"type"`
    RepoURL string `yaml:"repo_url"`
}

type LoggingConfig struct {
    Level string `yaml:"level"`
    File  string `yaml:"file"`
}
```

### 3.3 Config Loading

```go
func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("read config: %w", err)
    }

    // Expand ~ in paths
    data = []byte(os.ExpandEnv(string(data)))

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("parse config: %w", err)
    }

    // Resolve environment variables for _env fields
    cfg.GitHub.Token = os.Getenv(cfg.GitHub.TokenEnv)
    cfg.Notifications.Discord.WebhookURL = os.Getenv(cfg.Notifications.Discord.WebhookURLEnv)
    cfg.Server.Auth.Password = os.Getenv(cfg.Server.Auth.PasswordEnv)

    // Expand ~ in vault path
    if strings.HasPrefix(cfg.Vault.Path, "~") {
        home, _ := os.UserHomeDir()
        cfg.Vault.Path = filepath.Join(home, cfg.Vault.Path[1:])
    }

    return &cfg, nil
}
```

---

## 4. DATABASE SCHEMA

### 4.1 Complete SQL Schema

```sql
-- Convention: All optional TEXT fields use empty string ('') default.
-- Never use NULL. Query with WHERE field != ''.
-- JSON array fields use '[]' default.

PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS goals (
    id           TEXT PRIMARY KEY,          -- UUID v4
    title        TEXT NOT NULL,
    description  TEXT DEFAULT '',
    priorities   TEXT DEFAULT '[]',         -- JSON array of strings
    metrics      TEXT DEFAULT '[]',         -- JSON array of strings
    status       TEXT NOT NULL DEFAULT 'PROPOSED',  -- PROPOSED, ACTIVE, COMPLETED, SUPERSEDED
    source       TEXT NOT NULL DEFAULT 'ORCHESTRATOR', -- OPERATOR, ORCHESTRATOR
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    active_since DATETIME DEFAULT ''
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,           -- UUID v4
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL,              -- REPO, SERVICE, LIBRARY, TOOL
    repo_url    TEXT DEFAULT '',
    description TEXT DEFAULT '',
    vault_path  TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'ACTIVE', -- PROPOSED, ACTIVE, ARCHIVED, REJECTED
    tech_stack  TEXT DEFAULT '[]',          -- JSON array of strings
    inspiration TEXT DEFAULT '',
    goal_id     TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
    id              TEXT PRIMARY KEY,       -- UUID v4
    title           TEXT NOT NULL,
    description     TEXT DEFAULT '',
    type            TEXT NOT NULL,          -- CODING, RESEARCH, DOCUMENT, MAINTENANCE, DEPLOY, BUGFIX, PLANNING
    status          TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, READY, RUNNING, COMPLETED, FAILED, RETRY, ARCHIVED
    priority        INTEGER NOT NULL DEFAULT 50,     -- 1=highest, 100=lowest
    source          TEXT NOT NULL DEFAULT 'SYSTEM',  -- OPERATOR, RESEARCHER, SELF, SYSTEM
    project_id      TEXT DEFAULT '',
    parent_id       TEXT DEFAULT '',        -- Parent task ID for subtasks
    depth           INTEGER NOT NULL DEFAULT 0,      -- 0: root, 1: subtask, 2+: Manager rejects
    alert_id        TEXT DEFAULT '',
    goal_id         TEXT DEFAULT '',
    depends_on      TEXT DEFAULT '[]',      -- JSON array of task IDs
    tags            TEXT DEFAULT '[]',      -- JSON array of strings
    prompt          TEXT DEFAULT '',        -- Prompt sent to Claude Code
    result          TEXT DEFAULT '',        -- Claude Code output summary
    error_log       TEXT DEFAULT '',        -- Failure reason (stderr, test logs, exit code, "cancelled by operator")
    executor_id     TEXT DEFAULT '',        -- Pod ID that executed this task
    model           TEXT DEFAULT 'sonnet',  -- sonnet, opus
    branch_name     TEXT DEFAULT '',
    pr_url          TEXT DEFAULT '',
    pr_status       TEXT DEFAULT '',        -- OPEN, APPROVED, CHANGES_REQUESTED, MERGED
    diff_lines      INTEGER DEFAULT 0,
    files_changed   INTEGER DEFAULT 0,
    test_passed     BOOLEAN DEFAULT NULL,
    retry_count     INTEGER DEFAULT 0,
    crash_recovery  BOOLEAN DEFAULT FALSE,  -- TRUE = crash recovery RETRY (doesn't consume retry_count)
    tokens_used     INTEGER DEFAULT 0,
    cost_usd        REAL DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME DEFAULT '',
    completed_at    DATETIME DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_pr_status ON tasks(pr_status);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);

CREATE TABLE IF NOT EXISTS alerts (
    id           TEXT PRIMARY KEY,          -- UUID v4
    service_name TEXT NOT NULL,
    severity     TEXT NOT NULL,             -- CRITICAL, HIGH, MEDIUM, LOW
    type         TEXT NOT NULL,             -- HEALTH_CHECK, LATENCY, ERROR_RATE
    message      TEXT DEFAULT '',
    task_id      TEXT DEFAULT '',           -- Auto-created fix task
    status       TEXT NOT NULL DEFAULT 'ACTIVE', -- ACTIVE, RESOLVED
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at  DATETIME DEFAULT ''
);

CREATE TABLE IF NOT EXISTS usage_snapshots (
    id            TEXT PRIMARY KEY,         -- UUID v4
    type          TEXT NOT NULL,            -- HOURLY, BLOCKS, DAILY_SUMMARY
    data          TEXT NOT NULL,            -- Raw ccusage --json output
    total_tokens  INTEGER DEFAULT 0,       -- Quick query summary field
    total_cost    REAL DEFAULT 0,           -- Quick query summary field (USD)
    recorded_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_snapshots_type_time ON usage_snapshots(type, recorded_at);

CREATE TABLE IF NOT EXISTS rate_limit_events (
    id          TEXT PRIMARY KEY,           -- UUID v4
    tokens_used INTEGER DEFAULT 0,
    active_pods INTEGER DEFAULT 0,
    occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS service_metrics (
    id           TEXT PRIMARY KEY,          -- UUID v4
    service_name TEXT NOT NULL,
    latency_ms   INTEGER DEFAULT 0,
    status_code  INTEGER DEFAULT 0,
    is_healthy   BOOLEAN DEFAULT TRUE,
    recorded_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_service_time ON service_metrics(service_name, recorded_at);
```

### 4.2 Database Connection

```go
package db

import (
    "database/sql"
    _ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }

    // WAL mode for concurrent reads
    if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
        return nil, err
    }
    if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
        return nil, err
    }
    if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
        return nil, err
    }
    if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
        return nil, err
    }

    return db, nil
}
```

---

## 5. CORE TYPES & MODELS

### 5.1 Goal

```go
package models

type Goal struct {
    ID          string    `json:"id" db:"id"`
    Title       string    `json:"title" db:"title"`
    Description string    `json:"description" db:"description"`
    Priorities  []string  `json:"priorities"`    // stored as JSON text
    Metrics     []string  `json:"metrics"`       // stored as JSON text
    Status      string    `json:"status" db:"status"`
    Source      string    `json:"source" db:"source"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    ActiveSince string    `json:"active_since" db:"active_since"`
}

// Status constants
const (
    GoalProposed   = "PROPOSED"
    GoalActive     = "ACTIVE"
    GoalCompleted  = "COMPLETED"
    GoalSuperseded = "SUPERSEDED"
)

// Source constants
const (
    GoalSourceOperator     = "OPERATOR"
    GoalSourceOrchestrator = "ORCHESTRATOR"
)
```

### 5.2 Task

```go
type Task struct {
    ID            string  `json:"id" db:"id"`
    Title         string  `json:"title" db:"title"`
    Description   string  `json:"description" db:"description"`
    Type          string  `json:"type" db:"type"`
    Status        string  `json:"status" db:"status"`
    Priority      int     `json:"priority" db:"priority"`
    Source        string  `json:"source" db:"source"`
    ProjectID     string  `json:"project_id" db:"project_id"`
    ParentID      string  `json:"parent_id" db:"parent_id"`
    Depth         int     `json:"depth" db:"depth"`
    AlertID       string  `json:"alert_id" db:"alert_id"`
    GoalID        string  `json:"goal_id" db:"goal_id"`
    DependsOn     []string `json:"depends_on"`       // JSON array
    Tags          []string `json:"tags"`              // JSON array
    Prompt        string  `json:"prompt" db:"prompt"`
    Result        string  `json:"result" db:"result"`
    ErrorLog      string  `json:"error_log" db:"error_log"`
    ExecutorID    string  `json:"executor_id" db:"executor_id"`
    Model         string  `json:"model" db:"model"`
    BranchName    string  `json:"branch_name" db:"branch_name"`
    PRUrl         string  `json:"pr_url" db:"pr_url"`
    PRStatus      string  `json:"pr_status" db:"pr_status"`
    DiffLines     int     `json:"diff_lines" db:"diff_lines"`
    FilesChanged  int     `json:"files_changed" db:"files_changed"`
    TestPassed    *bool   `json:"test_passed" db:"test_passed"`
    RetryCount    int     `json:"retry_count" db:"retry_count"`
    CrashRecovery bool   `json:"crash_recovery" db:"crash_recovery"`
    TokensUsed    int     `json:"tokens_used" db:"tokens_used"`
    CostUSD       float64 `json:"cost_usd" db:"cost_usd"`
    CreatedAt     string  `json:"created_at" db:"created_at"`
    UpdatedAt     string  `json:"updated_at" db:"updated_at"`
    StartedAt     string  `json:"started_at" db:"started_at"`
    CompletedAt   string  `json:"completed_at" db:"completed_at"`
}

// Task type constants
const (
    TaskTypeCoding      = "CODING"
    TaskTypeResearch    = "RESEARCH"
    TaskTypeDocument    = "DOCUMENT"
    TaskTypeMaintenance = "MAINTENANCE"
    TaskTypeDeploy      = "DEPLOY"
    TaskTypeBugfix      = "BUGFIX"
    TaskTypePlanning    = "PLANNING"
)

// Task status constants
const (
    TaskPending   = "PENDING"
    TaskReady     = "READY"
    TaskRunning   = "RUNNING"
    TaskCompleted = "COMPLETED"
    TaskFailed    = "FAILED"
    TaskRetry     = "RETRY"
    TaskArchived  = "ARCHIVED"
)

// Task source constants
const (
    TaskSourceOperator   = "OPERATOR"
    TaskSourceResearcher = "RESEARCHER"
    TaskSourceSelf       = "SELF"
    TaskSourceSystem     = "SYSTEM"
)

// Priority ranges
// P:1-5     Service incidents
// P:6-20    Operator tasks / PR comments
// P:21-40   Maintenance
// P:41-60   Improvements
// P:61-80   Research-derived
// P:81-100  New projects

func (t *Task) NeedsOpus() bool {
    if t.Priority <= 5 { return true }
    if t.Source == TaskSourceOperator && t.hasComplexKeywords() { return true }
    if t.hasTag("initial-design") { return true }
    if t.hasTag("goal-strategy") { return true }
    return false
}

func (t *Task) RequiresTest() bool {
    return t.Type == TaskTypeCoding || t.Type == TaskTypeBugfix || t.Type == TaskTypeMaintenance
}
```

### 5.3 Project

```go
type Project struct {
    ID          string `json:"id" db:"id"`
    Name        string `json:"name" db:"name"`
    Type        string `json:"type" db:"type"`
    RepoURL     string `json:"repo_url" db:"repo_url"`
    Description string `json:"description" db:"description"`
    VaultPath   string `json:"vault_path" db:"vault_path"`
    Status      string `json:"status" db:"status"`
    TechStack   []string `json:"tech_stack"`
    Inspiration string `json:"inspiration" db:"inspiration"`
    GoalID      string `json:"goal_id" db:"goal_id"`
    CreatedAt   string `json:"created_at" db:"created_at"`
    UpdatedAt   string `json:"updated_at" db:"updated_at"`
}

const (
    ProjectProposed = "PROPOSED"
    ProjectActive   = "ACTIVE"
    ProjectArchived = "ARCHIVED"
    ProjectRejected = "REJECTED"
)
```

---

## 6. STATE MACHINES

### 6.1 Task State Transitions

```
Valid transitions (enforce in Manager):

PENDING → READY        (Operator input goes directly to READY)
READY → RUNNING        (Pod picks up task)
RUNNING → COMPLETED    (Success)
RUNNING → FAILED       (Failure, cancellation, guardrail violation)
FAILED → RETRY         (retry_count < 3 AND NOT cancelled)
RETRY → RUNNING        (Pod retries)
COMPLETED → ARCHIVED   (Cleanup)
FAILED → ARCHIVED      (Cleanup)
```

```go
var validTransitions = map[string][]string{
    TaskPending:   {TaskReady},
    TaskReady:     {TaskRunning},
    TaskRunning:   {TaskCompleted, TaskFailed},
    TaskFailed:    {TaskRetry, TaskArchived},
    TaskRetry:     {TaskRunning},
    TaskCompleted: {TaskArchived},
}

func (m *Manager) TransitionTask(taskID, newStatus string) error {
    task, err := m.GetTask(taskID)
    if err != nil { return err }

    allowed := validTransitions[task.Status]
    if !contains(allowed, newStatus) {
        return fmt.Errorf("invalid transition: %s → %s", task.Status, newStatus)
    }

    // Special handling for cancellation
    // Cancel sets status to FAILED with error_log = "cancelled by operator"
    // Cancel on RUNNING task: send stop signal to Pod first

    // Special handling for RETRY
    if newStatus == TaskRetry {
        if task.RetryCount >= 3 {
            return fmt.Errorf("max retries reached")
        }
        if task.ErrorLog == "cancelled by operator" {
            return fmt.Errorf("cancelled tasks cannot retry")
        }
        // Only increment retry_count if NOT crash recovery
        if !task.CrashRecovery {
            task.RetryCount++
        }
        task.CrashRecovery = false // Reset flag
    }

    return m.updateTaskStatus(taskID, newStatus)
}
```

### 6.2 Project State Transitions

```
PROPOSED → ACTIVE      (Operator approves)
PROPOSED → REJECTED    (Operator rejects)
ACTIVE → ARCHIVED      (Project completed/abandoned)
```

### 6.3 Goal State Transitions

```
PROPOSED → ACTIVE      (Operator activates — only one ACTIVE at a time)
ACTIVE → COMPLETED     (Goal achieved)
ACTIVE → SUPERSEDED    (New Goal replaces it)
```

### 6.4 PR State Transitions

```
(none) → OPEN           (PR created)
OPEN → APPROVED         (Operator approves in Web UI)
APPROVED → MERGED       (Flux merges)
OPEN → CHANGES_REQUESTED (Operator requests changes in Web UI)
CHANGES_REQUESTED → OPEN (Fix task completes, pushes to same PR)
```

---

## 7. INTERNAL COMMUNICATION

### 7.1 Internal API Endpoints

These endpoints are for Pod ↔ Manager/Orchestrator communication only. They MUST NOT be exposed to the Web UI or external clients.

```go
// internal_api.go

// POST /internal/tasks/next
// Pod requests next task from Manager
// Request: { "pod_id": "executor-01", "pod_type": "executor" }
// Response: { "task": { ... } } or { "task": null } if queue empty
func (s *Server) handleInternalNextTask(w http.ResponseWriter, r *http.Request) {
    var req struct {
        PodID   string `json:"pod_id"`
        PodType string `json:"pod_type"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    task := s.manager.PopNextTask(req.PodType)
    if task == nil {
        json.NewEncoder(w).Encode(map[string]interface{}{"task": nil})
        return
    }

    s.manager.TransitionTask(task.ID, TaskRunning)
    task.ExecutorID = req.PodID
    json.NewEncoder(w).Encode(map[string]interface{}{"task": task})
}

// POST /internal/tasks/:id/done
// Pod reports task completion
// Request: { "status": "COMPLETED", "result": "...", "tokens_used": 1234, "cost_usd": 0.05 }
func (s *Server) handleInternalTaskDone(w http.ResponseWriter, r *http.Request) { ... }

// POST /internal/subtasks
// Executor creates subtasks via Manager
// Request: { "parent_id": "task-123", "subtasks": [{ "title": "...", "description": "..." }, ...] }
// Manager enforces: depth ≤ 1, max 5 per parent, inherits priority + goal_id
func (s *Server) handleInternalCreateSubtasks(w http.ResponseWriter, r *http.Request) { ... }

// GET /internal/model/:task_id
// Pod queries which model to use for a task
// Response: { "model": "sonnet" } or { "model": "opus" }
func (s *Server) handleInternalGetModel(w http.ResponseWriter, r *http.Request) { ... }
```

### 7.2 Routing Setup

```go
func (s *Server) setupRoutes() {
    mux := http.NewServeMux()

    // External API (requires auth)
    mux.Handle("/api/", s.authMiddleware(s.apiRouter()))

    // Internal API (no auth, localhost only)
    mux.Handle("/internal/", s.localhostOnly(s.internalRouter()))

    // WebSocket
    mux.Handle("/ws/", s.authMiddleware(s.wsHandler()))

    // Static files (embedded React)
    mux.Handle("/", http.FileServer(http.FS(webFS)))
}

func (s *Server) localhostOnly(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        host, _, _ := net.SplitHostPort(r.RemoteAddr)
        if host != "127.0.0.1" && host != "::1" {
            http.Error(w, "forbidden", http.StatusForbidden)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## 8. INTEGRATIONS

### 8.1 Claude Code CLI Wrapper

```go
package executor

type ClaudeCodeRunner struct {
    config *config.ExecutorConfig
}

type ClaudeCodeResult struct {
    ExitCode    int
    Stdout      string
    Stderr      string
    Duration    time.Duration
    TokensUsed  int
    CostUSD     float64
    SessionID   string
}

func (r *ClaudeCodeRunner) Run(ctx context.Context, opts ClaudeCodeOpts) (*ClaudeCodeResult, error) {
    args := []string{
        "-p", opts.Prompt,
        "--cwd", opts.WorkDir,
        "--model", opts.Model,
        "--max-turns", strconv.Itoa(r.config.MaxTurns),
        "--output-format", "json",
        "--dangerously-skip-permissions",  // or sandbox mode after Phase 2A evaluation
    }

    if opts.SystemPrompt != "" {
        args = append(args, "--append-system-prompt", opts.SystemPrompt)
    }

    cmd := exec.CommandContext(ctx, "claude", args...)

    // Capture stdout and stderr separately
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    // Apply guardrails
    timeoutCtx, cancel := context.WithTimeout(ctx, r.config.MaxExecutionTime)
    defer cancel()

    start := time.Now()
    err := cmd.Run()
    duration := time.Since(start)

    exitCode := 0
    if exitErr, ok := err.(*exec.ExitError); ok {
        exitCode = exitErr.ExitCode()
    }

    result := &ClaudeCodeResult{
        ExitCode: exitCode,
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
        Duration: duration,
    }

    // Parse JSON output for metadata (Phase 2A: determine exact fields)
    // Minimum: extract result text, session_id if available
    // Unknown fields are ignored

    // Check output size guardrail
    if int64(len(stdout.Bytes())) > r.config.MaxOutputSize {
        return result, fmt.Errorf("output overflow: %d bytes > %d limit",
            len(stdout.Bytes()), r.config.MaxOutputSize)
    }

    return result, err
}

type ClaudeCodeOpts struct {
    Prompt       string
    WorkDir      string
    Model        string  // "sonnet" or "opus"
    SystemPrompt string  // Goal + context injected here
}
```

### 8.2 GitHub Client

```go
package github

type Client struct {
    token    string
    username string
    http     *http.Client
}

func NewClient(token, username string) *Client {
    return &Client{
        token:    token,
        username: username,
        http:     &http.Client{Timeout: 30 * time.Second},
    }
}

// CreateRepo creates a new GitHub repository
func (c *Client) CreateRepo(name string, private bool) (string, error) { ... }

// CreatePR creates a pull request
// Returns PR URL
func (c *Client) CreatePR(owner, repo, head, base, title, body string) (string, error) { ... }

// MergePR merges a pull request
func (c *Client) MergePR(owner, repo string, prNumber int) error { ... }

// FetchPRComments fetches comments from a PR (single fetch, no polling)
func (c *Client) FetchPRComments(owner, repo string, prNumber int) ([]Comment, error) { ... }
```

### 8.3 Discord Notifier

```go
package notifier

type Discord struct {
    webhookURL string
    http       *http.Client
}

type NotificationLevel string

const (
    LevelInfo     NotificationLevel = "INFO"
    LevelWarning  NotificationLevel = "WARNING"
    LevelCritical NotificationLevel = "CRITICAL"
)

func (d *Discord) Send(level NotificationLevel, message string) error {
    payload := map[string]string{
        "content": fmt.Sprintf("[%s] %s", level, message),
    }
    body, _ := json.Marshal(payload)

    resp, err := d.http.Post(d.webhookURL, "application/json", bytes.NewReader(body))
    if err != nil { return err }
    defer resp.Body.Close()

    if resp.StatusCode >= 400 {
        return fmt.Errorf("discord webhook failed: %d", resp.StatusCode)
    }
    return nil
}

// Notification events:
// - Service failure (CRITICAL/HIGH)
// - Project proposal
// - Plan limit
// - Goal proposal
// - PR review request
// - Task failure
// - Auth expiry
// - Daily summary
```

### 8.4 Vault Writer

```go
package vault

type WriteMode int

const (
    ModeCreate  WriteMode = iota  // Create new file (error if exists)
    ModeAppend                     // Append to existing file
    ModeReplace                    // Replace file content
)

type WriteRequest struct {
    Path    string     // Relative path within vault (e.g., "Tasks/completed/task-abc.md")
    Content string     // Markdown content
    Mode    WriteMode
    Done    chan error  // Completion notification
}

type Writer struct {
    basePath string
    requests chan WriteRequest
}

func NewWriter(basePath string) *Writer {
    w := &Writer{
        basePath: basePath,
        requests: make(chan WriteRequest, 100),
    }
    go w.run()
    return w
}

func (w *Writer) run() {
    for req := range w.requests {
        err := w.atomicWrite(req)
        req.Done <- err
    }
}

func (w *Writer) atomicWrite(req WriteRequest) error {
    fullPath := filepath.Join(w.basePath, req.Path)

    // Ensure directory exists
    dir := filepath.Dir(fullPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }

    switch req.Mode {
    case ModeCreate:
        if _, err := os.Stat(fullPath); err == nil {
            return fmt.Errorf("file already exists: %s", req.Path)
        }
        return os.WriteFile(fullPath, []byte(req.Content), 0644)
    case ModeAppend:
        f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
        if err != nil { return err }
        defer f.Close()
        _, err = f.WriteString(req.Content)
        return err
    case ModeReplace:
        return os.WriteFile(fullPath, []byte(req.Content), 0644)
    }
    return fmt.Errorf("unknown write mode: %d", req.Mode)
}

// Write sends a write request and waits for completion
func (w *Writer) Write(path, content string, mode WriteMode) error {
    done := make(chan error, 1)
    w.requests <- WriteRequest{
        Path:    path,
        Content: content,
        Mode:    mode,
        Done:    done,
    }
    return <-done
}

// Close drains the channel and stops the writer
func (w *Writer) Close() {
    close(w.requests)
}
```

---

## 9. GOAL SYSTEM

### 9.1 Goal Loading for Pods

Every Pod reads the current Goal before starting work and passes it to Claude Code:

```go
func (e *Executor) buildSystemPrompt(task *Task) string {
    goal := e.manager.GetCurrentGoal()
    if goal == nil {
        return ""
    }

    return fmt.Sprintf(`Current Goal: %s
Description: %s
Priorities: %s
Metrics: %s

All your work should align with this Goal.`,
        goal.Title,
        goal.Description,
        strings.Join(goal.Priorities, ", "),
        strings.Join(goal.Metrics, ", "),
    )
}
```

### 9.2 Goal Storage in Vault

When a Goal becomes ACTIVE, write to `Goals/_current.md`:

```markdown
# Current Goal: {title}

**Status**: ACTIVE
**Since**: {active_since}
**Source**: {OPERATOR|ORCHESTRATOR}

## Description
{description}

## Priorities
1. {priority_1}
2. {priority_2}

## Metrics
- {metric_1}
- {metric_2}
```

---

## 10. USAGE TRACKING

### 10.1 ccusage Project Name Encoding

```go
func encodeCCProjectName(absolutePath string) string {
    encoded := strings.ReplaceAll(absolutePath, "/", "-")
    encoded = strings.ReplaceAll(encoded, ".", "-")
    return encoded
}

// Example:
// Input:  /home/user/workspaces/trees/flux--task-abc123
// Output: -home-user-workspaces-trees-flux--task-abc123
```

### 10.2 Per-Task Usage Collection

After task completion:

```go
func (e *Executor) collectTaskUsage(task *Task, worktreePath string) {
    projectName := encodeCCProjectName(worktreePath)
    cmd := exec.Command("sh", "-c",
        fmt.Sprintf("%s daily --project %q --json", e.config.CCUsage.Command, projectName))

    output, err := cmd.Output()
    if err != nil {
        log.Printf("ccusage failed for task %s: %v", task.ID, err)
        return  // Graceful degradation: don't block task completion
    }

    // Parse JSON output, extract tokens and cost
    var usage struct {
        TotalTokens int     `json:"total_tokens"`
        TotalCost   float64 `json:"total_cost"`
    }
    json.Unmarshal(output, &usage)

    task.TokensUsed = usage.TotalTokens
    task.CostUSD = usage.TotalCost
}
```

### 10.3 JSONL Cleanup

```go
func (c *Cleaner) CleanOldJSONL() {
    cutoff := time.Now().AddDate(0, 0, -c.config.Cleanup.JSONLRetentionDays)

    // Check both possible paths (Claude Code v1.0.30 changed location)
    paths := []string{
        filepath.Join(os.Getenv("HOME"), ".claude", "projects"),
        filepath.Join(os.Getenv("HOME"), ".config", "claude", "projects"),
    }

    for _, basePath := range paths {
        entries, err := os.ReadDir(basePath)
        if err != nil { continue }

        for _, entry := range entries {
            if !entry.IsDir() { continue }
            projectDir := filepath.Join(basePath, entry.Name())

            files, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
            for _, f := range files {
                info, err := os.Stat(f)
                if err != nil { continue }
                if info.ModTime().Before(cutoff) {
                    os.Remove(f)
                }
            }
        }
    }
}
```

---

## 11. RATE LIMIT HANDLING

### 11.1 Two-Stage Detection

```go
package orchestrator

var rateLimitPatterns = []string{
    "rate limit",
    "too many requests",
    "429",
    "capacity",
    "try again",
}

type RateLimitDetector struct{}

func (d *RateLimitDetector) Detect(exitCode int, stderr string) bool {
    // Stage 1: Exit code
    if exitCode == 429 {
        return true
    }

    // Stage 2: Stderr pattern matching
    lower := strings.ToLower(stderr)
    for _, pattern := range rateLimitPatterns {
        if strings.Contains(lower, pattern) {
            return true
        }
    }

    return false
}
```

### 11.2 Rate Limit Handler

**Phase 2B (basic)**: Fixed 5-hour wait.

```go
func (rlh *RateLimitHandler) HandleRateLimit() {
    rlh.stopAllPods()
    rlh.notifier.Send(LevelWarning, "Rate limit detected. Stopping all pods. Waiting 5 hours.")
    rlh.recordEvent()
    time.Sleep(5 * time.Hour)
    rlh.resumePods()
}
```

**Phase 3 (dynamic)**: ccusage blocks-based wait.

```go
func (rlh *RateLimitHandler) HandleRateLimitDynamic() {
    rlh.stopAllPods()

    blocks, err := rlh.usageCollector.GetBlocks()
    if err == nil {
        if resetAt, ok := blocks.NextResetTime(); ok {
            waitDuration := time.Until(resetAt) + 1*time.Minute
            rlh.notifier.Send(LevelWarning,
                fmt.Sprintf("Rate limit. Waiting until %s (%v)", resetAt, waitDuration))
            time.Sleep(waitDuration)
            rlh.resumePods()
            return
        }
    }

    // Fallback: 5 hours
    rlh.notifier.Send(LevelWarning, "Rate limit. Cannot determine reset time. Waiting 5 hours.")
    time.Sleep(5 * time.Hour)
    rlh.resumePods()
}
```

---

## 12. MODEL SELECTION

```go
func (o *Orchestrator) SelectModel(task *Task) string {
    if o.rateLimitHandler.RecentlyLimited() {
        return o.config.Orchestrator.Models.Sonnet
    }
    if task.NeedsOpus() {
        return o.config.Orchestrator.Models.Opus
    }
    return o.config.Orchestrator.Models.Sonnet
}
```

---

## 13. QA & BRANCH STRATEGY

### 13.1 Git Worktree Management

```go
package executor

type WorktreeManager struct {
    reposDir string  // workspaces/repos/
    treesDir string  // workspaces/trees/
}

// EnsureBareRepo clones bare repo if not exists
func (wm *WorktreeManager) EnsureBareRepo(project *Project) error {
    bareDir := filepath.Join(wm.reposDir, project.Name+".git")
    if _, err := os.Stat(bareDir); err == nil {
        // Fetch latest
        cmd := exec.Command("git", "-C", bareDir, "fetch", "--all")
        return cmd.Run()
    }
    // Clone bare
    cmd := exec.Command("git", "clone", "--bare", project.RepoURL, bareDir)
    return cmd.Run()
}

// CreateWorktree creates a worktree for a task
func (wm *WorktreeManager) CreateWorktree(project *Project, task *Task) (string, error) {
    bareDir := filepath.Join(wm.reposDir, project.Name+".git")
    branchName := fmt.Sprintf("task/%s", task.ID[:8])
    worktreePath := filepath.Join(wm.treesDir,
        fmt.Sprintf("%s--task-%s", project.Name, task.ID[:8]))

    // Create worktree with new branch from main
    cmd := exec.Command("git", "-C", bareDir,
        "worktree", "add", "-b", branchName, worktreePath, "main")
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("create worktree: %w", err)
    }

    // Set up .claude/settings.json in worktree
    wm.setupClaudeSettings(worktreePath)

    return worktreePath, nil
}

// CleanupWorktree removes a worktree
func (wm *WorktreeManager) CleanupWorktree(project *Project, worktreePath string) error {
    bareDir := filepath.Join(wm.reposDir, project.Name+".git")
    cmd := exec.Command("git", "-C", bareDir, "worktree", "remove", "--force", worktreePath)
    return cmd.Run()
}

// setupClaudeSettings creates .claude/settings.json with permission bypass
func (wm *WorktreeManager) setupClaudeSettings(worktreePath string) error {
    settingsDir := filepath.Join(worktreePath, ".claude")
    os.MkdirAll(settingsDir, 0755)

    settings := map[string]interface{}{
        "permissions": map[string]interface{}{
            "allow": []string{
                "Bash(*)", "Read(*)", "Write(*)", "Edit(*)",
                "Grep(*)", "Glob(*)", "TodoRead(*)", "TodoWrite(*)",
            },
        },
    }
    data, _ := json.MarshalIndent(settings, "", "  ")
    return os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644)
}
```

### 13.2 Worktree Cleanup Policy

```go
func (wm *WorktreeManager) RunCleanup(tasks []*Task) {
    for _, task := range tasks {
        worktreePath := wm.worktreePathForTask(task)
        if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
            continue
        }

        switch {
        case task.Status == TaskCompleted && task.PRStatus == "MERGED":
            // Immediately delete
            wm.CleanupWorktree(task.Project, worktreePath)

        case task.Status == TaskCompleted && task.PRStatus == "OPEN":
            // Preserve until merge/rejection (PR review pending)
            continue

        case task.Status == TaskFailed:
            // Preserve for 24 hours for debugging
            if time.Since(task.CompletedAt) > 24*time.Hour {
                wm.CleanupWorktree(task.Project, worktreePath)
            }

        case task.PRStatus == "CHANGES_REQUESTED":
            // Preserve for fix task to reuse
            continue
        }
    }
}
```

### 13.3 Post-Execution Verification

```go
func (e *Executor) verifyWorktreeIntegrity(worktreePath string) error {
    // Check for modifications outside worktree
    // Strategy: snapshot key directories before execution, compare after

    home, _ := os.UserHomeDir()
    checkPaths := []string{
        filepath.Join(home, ".ssh"),
        filepath.Join(home, ".aws"),
        filepath.Join(home, ".gitconfig"),
        filepath.Join(home, ".zshrc"),
        filepath.Join(home, ".bashrc"),
    }

    for _, p := range checkPaths {
        info, err := os.Stat(p)
        if err != nil { continue }

        // Compare modification time with execution start
        if info.ModTime().After(e.executionStartTime) {
            return fmt.Errorf("external modification detected: %s modified during execution", p)
        }
    }

    return nil
}
```

---

## 14. PR REVIEW

### 14.1 Auto-Merge Decision

```go
func ShouldAutoMerge(task *Task, diffLines, filesChanged int) bool {
    // Guardrail override: always require Operator review
    if diffLines > 2000 || filesChanged > 20 {
        return false
    }

    if task.Source == TaskSourceSelf || task.Source == TaskSourceSystem {
        return true
    }
    if task.Type == TaskTypeMaintenance {
        return true
    }
    if task.Type == TaskTypeBugfix && task.Priority <= 10 {
        return true
    }
    if filesChanged <= 3 && diffLines < 100 {
        return true
    }
    return false  // Operator review required
}
```

### 14.2 Request Changes Flow

```go
func (s *Server) handleRequestChanges(w http.ResponseWriter, r *http.Request) {
    taskID := chi.URLParam(r, "task_id")
    task, _ := s.manager.GetTask(taskID)

    // 1. Fetch GitHub PR comments (single fetch, no polling)
    prNumber := extractPRNumber(task.PRUrl)
    comments, _ := s.github.FetchPRComments(
        s.config.GitHub.Username, task.ProjectName, prNumber)

    // 2. Build comment summary
    commentText := formatComments(comments)

    // 3. Create fix task with P:6 priority
    fixTask := &Task{
        Title:       fmt.Sprintf("PR fix: %s", task.Title),
        Description: fmt.Sprintf("Fix based on PR review comments:\n\n%s", commentText),
        Type:        task.Type,
        Status:      TaskReady,
        Priority:    6,
        Source:      TaskSourceOperator,
        ProjectID:   task.ProjectID,
        GoalID:      task.GoalID,
        BranchName:  task.BranchName,  // Same branch
        PRUrl:       task.PRUrl,       // Same PR
    }
    s.manager.CreateTask(fixTask)

    // 4. Update original task PR status
    task.PRStatus = "CHANGES_REQUESTED"
    s.manager.UpdateTask(task)

    // 5. Discord notification
    s.notifier.Send(LevelInfo,
        fmt.Sprintf("PR changes requested for '%s'. Fix task created.", task.Title))
}
```

---

## 15. ORCHESTRATION

### 15.1 Orchestrator Main Loop

```go
package orchestrator

type Orchestrator struct {
    config           *config.Config
    manager          *manager.Manager
    scaleManager     *ScaleManager
    usageCollector   *UsageCollector
    dailySummary     *DailySummary
    rateLimitHandler *RateLimitHandler
    goalAdvisor      *GoalAdvisor
    notifier         *notifier.Discord
}

func (o *Orchestrator) Run(ctx context.Context) {
    ticker := time.NewTicker(o.config.Orchestrator.CheckInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            o.tick()
        }
    }
}

func (o *Orchestrator) tick() {
    o.rateLimitHandler.CheckAndRecover()
    o.scaleManager.Rebalance(!o.rateLimitHandler.IsLimited())
    o.usageCollector.CollectIfDue()
    o.goalAdvisor.ProposeIfNeeded()
    o.dailySummary.SendIfDue()
}
```

### 15.2 Scale Manager

```go
type ScaleManager struct {
    maxPods     int
    cooldown    time.Duration
    lastScaleAt time.Time
    minResearch float64
    pods        []*Pod
}

func (sm *ScaleManager) Rebalance(running bool) {
    if !running {
        sm.stopAllPods()
        return
    }

    if time.Since(sm.lastScaleAt) < sm.cooldown {
        return  // Cooldown active
    }

    ratio := sm.determineRatio()
    executorCount := int(float64(sm.maxPods) * ratio.executor)
    researcherCount := sm.maxPods - executorCount

    sm.adjustPods("executor", executorCount)
    sm.adjustPods("researcher", researcherCount)
    sm.lastScaleAt = time.Now()
}

type Ratio struct {
    executor   float64
    researcher float64
}

func (sm *ScaleManager) determineRatio() Ratio {
    operatorTasks := sm.manager.CountBySource(TaskSourceOperator)
    urgentTasks := sm.manager.CountByPriority(1, 5)
    totalPending := sm.manager.CountByStatus(TaskReady)

    switch {
    case urgentTasks > 0:
        return Ratio{0.9, 0.1}   // 9:1
    case operatorTasks > 0:
        return Ratio{0.8, 0.2}   // 8:2
    case totalPending > 5:
        return Ratio{0.7, 0.3}   // 7:3
    case totalPending > 0:
        return Ratio{0.3, 0.7}   // 3:7
    default:
        return Ratio{0.0, 1.0}   // 0:10
    }
}
```

---

## 16. CORE COMPONENTS

### 16.1 Executor Pod Main Loop

```go
package executor

type Executor struct {
    id       string  // e.g., "executor-01"
    config   *config.Config
    claude   *ClaudeCodeRunner
    worktree *WorktreeManager
    manager  *ManagerClient  // HTTP client for /internal/ API
    vault    *vault.Writer
    notifier *notifier.Discord
    stopCh   chan struct{}
}

func (e *Executor) Run(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case <-e.stopCh:
            return
        default:
            e.executeOnce(ctx)
            time.Sleep(5 * time.Second)  // Brief pause between tasks
        }
    }
}

func (e *Executor) executeOnce(ctx context.Context) {
    // 1. Request next task
    task := e.manager.NextTask(e.id, "executor")
    if task == nil {
        time.Sleep(30 * time.Second)  // No work available
        return
    }

    // 2. Get model decision
    model := e.manager.GetModel(task.ID)

    // 3. Get current Goal for system prompt
    systemPrompt := e.buildSystemPrompt(task)

    // 4. Create/reuse worktree
    project, _ := e.manager.GetProject(task.ProjectID)
    var worktreePath string
    if task.BranchName != "" {
        // Reuse existing worktree (CHANGES_REQUESTED fix)
        worktreePath = e.worktree.FindByBranch(project, task.BranchName)
    } else {
        worktreePath, _ = e.worktree.CreateWorktree(project, task)
    }

    // 5. Build prompt
    prompt := e.buildPrompt(task)

    // 6. Execute Claude Code with guardrails
    e.executionStartTime = time.Now()
    result, err := e.claude.Run(ctx, ClaudeCodeOpts{
        Prompt:       prompt,
        WorkDir:      worktreePath,
        Model:        model,
        SystemPrompt: systemPrompt,
    })

    // 7. Check for rate limit
    if e.rateLimitDetector.Detect(result.ExitCode, result.Stderr) {
        e.manager.ReportRateLimit()
        e.manager.ReportTaskDone(task.ID, TaskRetry, "", "rate limit detected")
        return
    }

    // 8. Post-execution verification
    if err := e.verifyWorktreeIntegrity(worktreePath); err != nil {
        e.manager.ReportTaskDone(task.ID, TaskFailed, "", err.Error())
        e.notifier.Send(LevelCritical,
            fmt.Sprintf("Worktree integrity violation: %s", err))
        return
    }

    // 9. Check for subtask decomposition response
    if decomposition := e.parseDecomposition(result.Stdout); decomposition != nil {
        e.manager.CreateSubtasks(task.ID, decomposition.Subtasks)
        e.manager.ReportTaskDone(task.ID, TaskCompleted, "decomposed into subtasks", "")
        return
    }

    // 10. QA (if applicable)
    if task.RequiresTest() {
        if !e.runTests(worktreePath) {
            e.manager.ReportTaskDone(task.ID, TaskFailed, "", "tests failed")
            return
        }
    }

    // 11. Commit + diff check
    diffLines, filesChanged := e.commitAndGetDiff(worktreePath, task)
    task.DiffLines = diffLines
    task.FilesChanged = filesChanged

    // 12. Create PR
    prURL := e.createPR(project, task, worktreePath)
    task.PRUrl = prURL
    task.PRStatus = "OPEN"

    // 13. Auto-merge decision
    if ShouldAutoMerge(task, diffLines, filesChanged) {
        e.github.MergePR(...)
        task.PRStatus = "MERGED"
        e.worktree.CleanupWorktree(project, worktreePath)
    } else {
        e.notifier.Send(LevelInfo,
            fmt.Sprintf("PR ready for review: %s - %s", task.Title, prURL))
    }

    // 14. Collect usage
    e.collectTaskUsage(task, worktreePath)

    // 15. Report completion
    e.manager.ReportTaskDone(task.ID, TaskCompleted, result.Stdout, "")

    // 16. Minimal Vault recording (Phase 2B)
    e.vault.Write(
        fmt.Sprintf("Tasks/completed/%s.md", task.ID[:8]),
        e.formatTaskSummary(task, result),
        vault.ModeCreate,
    )
}
```

### 16.2 Subtask Decomposition Prompt

Included in every Executor prompt:

```
If this task is too large to complete in a single session,
DO NOT write code. Instead, output only a decomposition plan as JSON:
{"decompose": true, "subtasks": [{"title": "...", "description": "..."}, ...]}
Maximum 5 subtasks. Each should be independently completable.
```

---

## 17. WEB UI

### 17.1 Tech Stack

- React 18+ with TypeScript
- Tailwind CSS for styling
- Vite for build
- Zustand for state management
- WebSocket for real-time updates
- Go `embed` package to embed built frontend

### 17.2 Embedding in Go

```go
package server

import "embed"

//go:embed web/dist/*
var webFS embed.FS

func (s *Server) setupStaticFiles() {
    distFS, _ := fs.Sub(webFS, "web/dist")
    s.mux.Handle("/", http.FileServer(http.FS(distFS)))
}
```

### 17.3 Authentication

```go
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Password string `json:"password"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Compare with bcrypt hash
    hashedPassword := bcryptHash(s.config.Server.Auth.Password)
    if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // Generate session token (no expiry)
    token := uuid.New().String()
    s.sessions[token] = true

    // Set cookie
    http.SetCookie(w, &http.Cookie{
        Name:     "flux_session",
        Value:    token,
        Path:     "/",
        HttpOnly: true,
        SameSite: http.SameSiteStrictMode,
    })

    json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

---

## 18. API REFERENCE

### 18.1 External API

```
# Goals
POST   /api/goals                        Create/propose goal
GET    /api/goals                        List goals
GET    /api/goals/current                Current active goal
PATCH  /api/goals/:id                    Update goal
POST   /api/goals/:id/activate           Activate goal (only one ACTIVE)
POST   /api/orchestrator/propose-goals   Request Orchestrator proposals

# Tasks
POST   /api/tasks                        Create task
GET    /api/tasks                        List tasks (?status=&project_id=&page=&limit=)
GET    /api/tasks/:id                    Task detail
PATCH  /api/tasks/:id                    Update task
DELETE /api/tasks/:id                    Delete task
POST   /api/tasks/:id/cancel             Cancel → FAILED + error_log "cancelled by operator"

# PR Review
GET    /api/prs/pending                  List pending PRs
POST   /api/prs/:task_id/approve         Approve → merge → worktree cleanup
POST   /api/prs/:task_id/request-changes Fetch comments → create fix task

# Projects
POST   /api/projects                     Register project
GET    /api/projects                     List projects
GET    /api/projects/:id                 Project detail
POST   /api/projects/:id/approve         Approve → create GitHub repo
POST   /api/projects/:id/reject          Reject

# Services
GET    /api/services                     Service status
GET    /api/alerts                       Alert list

# Usage
GET    /api/usage/daily                  Daily token/cost
GET    /api/usage/monthly                Monthly aggregation
GET    /api/usage/blocks                 Billing window
GET    /api/usage/timeseries             Time-series data (?from=&to=&type=HOURLY)
GET    /api/usage/rate-limits            Rate limit history

# Orchestrator
GET    /api/orchestrator/status          Scaling status
GET    /api/pods                         Pod status

# WebSocket
WS     /ws/events                        Real-time event stream
```

### 18.2 Internal API

```
POST   /internal/tasks/next              Pod requests next task
POST   /internal/tasks/:id/done          Pod reports completion
POST   /internal/subtasks                Create subtasks
GET    /internal/model/:task_id          Model decision query
```

---

## 19. BOOTSTRAP SEQUENCE

```go
func Bootstrap(cfg *config.Config) error {
    // 1. Database
    db, err := openOrCreateDB(cfg.Database.Path)
    if err != nil { return err }
    if err := createSchema(db); err != nil { return err }

    // 2. Obsidian Vault structure
    createVaultDirs(cfg.Vault.Path)

    // 3. Notifier
    notifier := notifier.NewDiscord(cfg.Notifications.Discord.WebhookURL)

    // 4. Vault Writer
    vaultWriter := vault.NewWriter(cfg.Vault.Path)

    // 5. Register Flux as first project
    registerSeedProjects(db, cfg.Projects)

    // 6. Claude Code auth check
    if err := checkClaudeAuth(); err != nil {
        notifier.Send(LevelCritical, "Claude Code auth failed: "+err.Error())
        return err
    }

    // 7. Claude Code Smoke Test
    if err := claudeCodeSmokeTest(); err != nil {
        notifier.Send(LevelCritical, "Claude Code smoke test failed: "+err.Error())
        return err  // Don't start pods
    }

    // 8. ccusage check (warning only, not required)
    if err := checkCCUsage(cfg.CCUsage.Command); err != nil {
        notifier.Send(LevelWarning, "ccusage not available: "+err.Error())
    }

    // 9. .claude/settings.json (created per-worktree, not global)

    // 10. Research workspaces
    createResearchWorkspaces(cfg)

    // 11. Web UI start (in main.go, after bootstrap)

    // 12. Discord notification
    notifier.Send(LevelInfo, "Flux initialized. Please set a Goal.")

    return nil
}

func claudeCodeSmokeTest() error {
    cmd := exec.Command("claude", "-p", "respond with exactly: SMOKE_TEST_OK",
        "--max-turns", "1", "--output-format", "json")
    output, err := cmd.Output()
    if err != nil {
        return fmt.Errorf("claude -p failed: %w", err)
    }
    if !strings.Contains(string(output), "SMOKE_TEST_OK") {
        return fmt.Errorf("unexpected smoke test output")
    }
    return nil
}
```

---

## 20. SELF-IMPROVEMENT

```go
func (e *Executor) executeSelfImprovement(task *Task) error {
    // 1. Create worktree from flux repo
    worktreePath, _ := e.worktree.CreateWorktree(fluxProject, task)

    // 2. Create safety tag
    safeTag := fmt.Sprintf("flux-safe-%d", time.Now().Unix())
    exec.Command("git", "-C", worktreePath, "tag", safeTag).Run()

    // 3. Execute Claude Code for modification
    result, err := e.claude.Run(ctx, ClaudeCodeOpts{...})

    // 4. Run full test suite
    testCmd := exec.Command("go", "test", "./...", "-v")
    testCmd.Dir = worktreePath
    if testErr := testCmd.Run(); testErr != nil {
        // 5a. FAIL: revert
        exec.Command("git", "-C", worktreePath, "checkout", safeTag).Run()
        return fmt.Errorf("self-improvement tests failed: %w", testErr)
    }

    // 5b. PASS: create PR
    // Restart only at idle (not immediate)
    return nil
}
```

**Restriction**: DB schema changes are EXCLUDED from self-improvement. If schema changes are needed, create a PROPOSED task for Operator approval.

---

## 21. ERROR RECOVERY

```go
func RecoverFromCrash(db *sql.DB, notifier *notifier.Discord) {
    // Find RUNNING tasks (interrupted by crash)
    rows, _ := db.Query("SELECT id FROM tasks WHERE status = 'RUNNING'")
    var taskIDs []string
    for rows.Next() {
        var id string
        rows.Scan(&id)
        taskIDs = append(taskIDs, id)
    }

    for _, id := range taskIDs {
        // Transition to RETRY with crash_recovery flag
        db.Exec(`UPDATE tasks SET
            status = 'RETRY',
            crash_recovery = TRUE,
            updated_at = CURRENT_TIMESTAMP
        WHERE id = ?`, id)
    }

    if len(taskIDs) > 0 {
        notifier.Send(LevelWarning,
            fmt.Sprintf("Recovered from crash. %d tasks moved to RETRY.", len(taskIDs)))
    }
}
```

---

## 22. GRACEFUL SHUTDOWN

```go
package shutdown

func GracefulShutdown(ctx context.Context, cfg *config.ShutdownConfig,
    pods []*Pod, db *sql.DB, vaultWriter *vault.Writer) {

    // 1. Stop new task assignment
    // (Manager stops popping from queue)

    // 2. Signal running pods to finish current task
    for _, pod := range pods {
        pod.SignalStop()
    }

    // 3. Wait for grace period
    timer := time.NewTimer(cfg.PodGracePeriod)
    select {
    case <-allPodsDone(pods):
        timer.Stop()
    case <-timer.C:
        // 4. Force kill remaining pods
        for _, pod := range pods {
            if pod.IsRunning() {
                pod.ForceKill()
                // Move task to RETRY with crash_recovery
                db.Exec(`UPDATE tasks SET
                    status = 'RETRY',
                    crash_recovery = TRUE
                WHERE id = ? AND status = 'RUNNING'`, pod.CurrentTaskID())
            }
        }
    }

    // 5. DB flush
    db.Close()

    // 6. Vault Writer drain
    vaultWriter.Close()
}
```

---

## 23. IMPLEMENTATION PHASES

### Phase 1: Foundation — "Skeleton"

**Goal**: Flux boots, Web UI manages Goals/Tasks/Projects.
**Deliverable**: `go build` → single binary → Web UI CRUD.
**Operator manual work**: Everything. No autonomous execution.

| # | Item | Details |
|---|------|---------|
| 1 | Go project init + config loader | `go mod init`, YAML→struct, env var resolution, ~ expansion |
| 2 | SQLite schema + bootstrap | WAL mode, all tables, Vault directory structure, seed projects |
| 3 | Goal/Task/Project CRUD API | `/api/goals`, `/api/tasks`, `/api/projects` with full CRUD |
| 4 | Internal API framework | `/internal/...` endpoints (stub implementations, localhost-only middleware) |
| 5 | Discord Notifier | Webhook-based, used across all phases |
| 6 | GitHub client (repo creation only) | `CreateRepo()` only. PR methods stubbed for Phase 2A. |
| 7 | Web UI | React+Vite+Tailwind+Go embed. Auth (bcrypt+cookie, no expiry). Dashboard, Goals, Tasks, Projects pages. WebSocket stub. |

### Phase 2A: Core Pipeline — "A task becomes a PR"

**Goal**: Task → Claude Code → test → PR → merge.
**Deliverable**: Complete execution pipeline.
**Operator manual work**: Task registration, PR review, manual rate limit response.

| # | Item | Details |
|---|------|---------|
| 1 | Claude Code CLI integration | `-p`, `--max-turns`, `--output-format json`, `--append-system-prompt`, stderr/stdout capture |
| 2 | JSON response parsing | Test actual response, determine minimum fields to extract |
| 3 | **Sandbox evaluation** | Test Claude Code native sandbox (`/sandbox`). Check filesystem/network isolation compatibility with Flux workflow. Decision: keep `--dangerously-skip-permissions` or switch to sandbox. |
| 4 | Smoke Test | Add to bootstrap. `claude -p "SMOKE_TEST_OK"` verification. |
| 5 | Git worktree management | Bare repo + per-task worktree + cleanup policy (PR pending preserve, FAILED 24h, merged immediate) |
| 6 | GitHub PR client | `CreatePR()`, `MergePR()`, `FetchPRComments()` |
| 7 | Manager basic | Priority Queue pop → internal API Pod assignment → state transitions |
| 8 | Executor Pod + guardrails | 30min timeout, 10MB output, 30 max-turns, 2000-line diff, 20 files |
| 9 | Post-execution verification | Worktree external change detection (scope adjusted by sandbox evaluation) |
| 10 | QA | Test detection → run → write if missing → max 3 retries |
| 11 | PR + auto-merge + Operator review | Auto-merge conditions, Request Changes flow (comment fetch → fix task → existing worktree) |
| 12 | Web UI: PRs page | PR list, Approve/Request Changes buttons, GitHub link |
| 13 | Rate limit detection **experiment** | Intentionally trigger rate limit. Observe exit code and stderr patterns. Document findings. |
| 14 | **Agent Teams experiment** | Test `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` in `-p` mode. Test `.claude/agents/` definition files. |

**Phase 2A completion = "Flux builds Flux" transition point.**

### Phase 2B: Pipeline Hardening — "Runs reliably"

**Goal**: Safety nets, tracking, unattended operation.
**Deliverable**: Rate limit response, model selection, Vault recording, launchd.
**Operator manual work**: Task registration, PR review, Pod count adjustment.

| # | Item | Details |
|---|------|---------|
| 1 | Rate limit detection impl | Apply Phase 2A experiment results. Finalize patterns. |
| 2 | Rate limit basic response | Detect → stop all Pods → fixed 5h wait → resume |
| 3 | Model selection | Sonnet default, Opus conditional (NeedsOpus() logic) |
| 4 | Goal prompt injection | `--append-system-prompt` with current Goal |
| 5 | Subtask decomposition | Claude Code autonomous judgment, Manager API, depth 1, max 5 |
| 6 | Manager enhancement | Goal boost, dependency check (depends_on) |
| 7 | Vault Writer | Single-goroutine channel-based writer |
| 8 | Minimal Vault recording | Task completion → summary markdown → Obsidian |
| 9 | ccusage project name mapping | Verify absolute path encoding = ccusage `--project` match |
| 10 | Minimal Graceful Shutdown | SIGTERM → 10min Pod wait → SIGKILL → task RETRY(crash_recovery) |
| 11 | launchd plist registration | KeepAlive for crash/reboot auto-restart |
| 12 | Basic error recovery | Mac reboot → launchd → RUNNING→RETRY → Pod restart |

### Phase 3: Orchestration — "Autonomous operation"

**Goal**: Auto Pod management, usage tracking, daily reporting.
**Deliverable**: Auto-scaling, ccusage graphs, dynamic rate limit, daily summary.
**Operator manual work**: Goal setting, complex PR review, new project approval only.

| # | Item |
|---|------|
| 1 | Orchestrator framework + sub-components |
| 2 | ScaleManager (Pod count, Executor:Researcher ratio, 15min cooldown) |
| 3 | RateLimitHandler upgrade (ccusage blocks dynamic wait, 5h fallback) |
| 4 | UsageCollector (ccusage daily/blocks/instances --json) |
| 5 | Time-series snapshots (hourly → usage_snapshots DB) |
| 6 | Per-task usage (`ccusage --project {encoded-path}`) |
| 7 | DailySummary (Discord) |
| 8 | JSONL cleanup (30-day deletion) |
| 9 | Daily backup (SQLite .backup + Vault tar.gz, 7-day retention) |
| 10 | Graceful Shutdown upgrade (12min force kill) |
| 11 | Data cleanup (metrics, snapshots, backup retention) |
| 12 | Usage UI (time-series graphs) |

### Phase 4: Knowledge & Autonomy — "Autonomous growth"

**Goal**: Researcher researches, knowledge accumulates, Flux self-improves.
**Deliverable**: Researcher Pods, Obsidian knowledge, project proposals, self-modification.

| # | Item |
|---|------|
| 1 | Researcher Pod + workspace (per-Pod workspace, `--add-dir`, MCP) |
| 2 | Autonomous research scheduling |
| 3 | Vault Writer upgrade (all Pods → channel → Obsidian) |
| 4 | New project proposals → Operator approval → GitHub repo |
| 5 | Deployment automation (deploy.sh / Makefile deploy) |
| 6 | Self-improvement (flux-safe tag + revert, schema changes excluded) |
| 7 | Research UI |

### launchd plist (Phase 2B)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.circle-oo.flux</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/flux</string>
        <string>--config</string>
        <string>$FLUX_DIR/config.yaml</string>       <!-- Replace $FLUX_DIR with actual path -->
    </array>
    <key>WorkingDirectory</key>
    <string>$FLUX_DIR</string>                       <!-- Replace $FLUX_DIR with actual path -->
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$FLUX_DIR/logs/flux-stdout.log</string>  <!-- Replace $FLUX_DIR with actual path -->
    <key>StandardErrorPath</key>
    <string>$FLUX_DIR/logs/flux-stderr.log</string>  <!-- Replace $FLUX_DIR with actual path -->
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
        <key>HOME</key>
        <string>$HOME</string>                       <!-- Replace $HOME with actual home path -->
    </dict>
</dict>
</plist>
```

Install: `cp com.circle-oo.flux.plist ~/Library/LaunchAgents/ && launchctl load ~/Library/LaunchAgents/com.circle-oo.flux.plist`

---

## 24. CONVENTIONS & RULES

1. **NULL convention**: All optional TEXT fields use `''` (empty string). Never NULL. Query with `!= ''`.
2. **Quality first**: No Haiku. Reduce pods if budget tight, never downgrade models. Minimum Sonnet.
3. **Opus conditional**: No recent rate limit + complex task. Otherwise Sonnet.
4. **QA mandatory**: Coding tasks require tests. Write if none exist. RESEARCH/DOCUMENT exempt.
5. **Git worktree**: Parallel work. No repo locking. PR pending → preserve worktree. FAILED → 24h preserve.
6. **Rate limit**: 2-stage detection. Phase 2B: fixed 5h wait. Phase 3: ccusage blocks dynamic wait.
7. **PR review**: Simple → auto-merge. Complex/guardrail exceeded → Operator. Changes → manual trigger, comment fetch, fix in existing worktree.
8. **Usage tracking**: Delegated to ccusage. Per-task via worktree path encoding. Hourly snapshots to DB.
9. **Sandbox**: Phase 2A evaluates Claude Code native sandbox. Decision affects `--dangerously-skip-permissions` usage.
10. **Tool permissions**: `.claude/settings.json` bypass (if not using sandbox).
11. **Claude Code auth**: Discord alert on expiry.
12. **Claude Code JSON parsing**: Determined in Phase 2A after observing actual responses.
13. **macOS**: launchd with KeepAlive (Phase 2B).
14. **Self-improvement**: PR OK. Restart at idle only. **Schema changes excluded**.
15. **Goal propagation**: Read `Goals/_current.md` → inject via `--append-system-prompt`.
16. **Obsidian**: Vault Writer channel-based serial writes. Phase 2B starts minimal recording.
17. **R&D protection**: Minimum 20% research except emergencies.
18. **Graceful Shutdown**: Phase 2B basic (10min grace → SIGKILL). Phase 3 upgrade (12min force kill). `crash_recovery` flag distinguishes crash RETRY from execution failure RETRY.
19. **Guardrails**: 30min timeout, 10MB output, 30 max-turns, 2000-line diff, 20 files. Post-execution external change detection.
20. **Subtasks**: Claude Code autonomous judgment → Manager internal API. Depth 1, max 5. No direct DB access.
21. **Researcher workspace**: Per-Pod independent workspace. Parallel conflict prevention.
22. **Pod communication**: Internal HTTP API (`/internal/...`). Never exposed externally.
23. **Sessions**: No expiry. Tailscale network auth. Explicit logout only.
24. **Task cancellation**: No CANCELLED state. → FAILED + error_log "cancelled by operator".
25. **Orchestrator structure**: ScaleManager, UsageCollector, DailySummary, RateLimitHandler, GoalAdvisor.

---

## 25. FUTURE WORK

### 25.1 Original Design Deferrals

Monitor HTTP healthchecks, incident reports, Services UI, multiple Goals+Rank, REFINING state, Issue system, limit learning, gradual Pod scaling, detailed daily summary, log analysis, process/resource monitoring, watchdog, Obsidian dashboard refresh, metrics aggregation, JSONL gzip archiving, notification severity filters, plan recommendations, library analysis, research quality verification, DB schema migration, PR table separation, Rate Limit 3rd-tier detection.

### 25.2 Competitive Analysis Gaps

Execution environment isolation (Sandbox), code review feedback learning, external event triggers (GitHub webhooks, Discord bot), CI/CD integration, multi-repo tasks, session persistence/context sharing, mobile accessibility.

### 25.3 oh-my-claudecode (OMC) Ideas

Agent definition patterns (`.claude/agents/` per task type), CLAUDE.md auto-refresh (3-tier notepad), native Agent Teams utilization (sub-agent parallel execution).

**IMPORTANT**: OMC is an interactive-session orchestrator. Do NOT install it as a Flux plugin. Only borrow ideas. Flux orchestrates OUTSIDE Claude Code; OMC orchestrates INSIDE. Combining them creates dual orchestration conflicts.
