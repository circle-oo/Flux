# Flux

Autonomous engineering system that runs 24/7. You set goals — Flux writes the code, runs tests, creates PRs, and merges them.

Flux orchestrates [Claude Code](https://claude.ai/claude-code) agents in isolated Git worktrees, managing the full lifecycle from task assignment through PR merge. Built with Go + SQLite + React, it ships as a single binary.

## How It Works

```
You (Operator)                        Flux (Autonomous)
  │                                     │
  ├── Create goals & tasks              ├── Manager picks next task by priority
  ├── Review complex PRs                ├── Executor Pod spins up a Git worktree
  ├── Approve new projects              ├── Claude Code writes & tests code
  └── Get Discord notifications         ├── PR created on GitHub
                                        ├── Auto-merge (simple) or request review (complex)
                                        └── Worktree cleaned up after merge
```

**Task lifecycle:** `PENDING → READY → RUNNING → COMPLETED → ARCHIVED` (failed tasks retry up to 3 times).

## Prerequisites

- **Go 1.22+** — `brew install go`
- **Node.js 20+** — `brew install node`
- **Claude Code** subscription (Max 5x plan recommended)
- **GitHub** personal access token
- **macOS** (launchd support for 24/7 operation)

## Quick Start

```bash
# 1. Clone
git clone https://github.com/circle-oo/flux.git
cd flux

# 2. Set environment variables
export FLUX_UI_PASSWORD='your-password'
export GITHUB_TOKEN='your-github-token'
export DISCORD_WEBHOOK_URL='your-webhook-url'  # optional

# 3. Run setup (installs deps, builds frontend, compiles binary)
bash setup.sh

# 4. Start Flux
bash start-up.sh
```

Open **http://localhost:8080** and log in with your password.

## Configuration

Setup creates `config.yaml` from the template. Key settings:

```yaml
server:
  port: 8080                    # Web UI port
  auth:
    password_env: "FLUX_UI_PASSWORD"

github:
  username: "your-username"     # Edit this
  token_env: "GITHUB_TOKEN"

vault:
  path: "~/ObsidianVault/Flux"  # Obsidian vault for execution logs

executor:
  max_execution_time: 30m       # Per-task timeout
  max_turns: 30                 # Claude Code turn limit
  max_diff_lines: 2000          # Guardrail: max diff size
  max_files_changed: 20         # Guardrail: max files per PR

triager:
  enabled: true                 # Enable automatic task triage
  max_execution_time: 2m        # Triage timeout per task

subtask:
  max_depth: 1                  # Maximum subtask nesting level
  max_per_task: 5               # Maximum subtasks per parent task

orchestrator:
  max_total_pods: 5             # Concurrent executor pods

auto_update:
  enabled: true                 # Auto-update from git remote
  check_interval: 5m            # How often to check for updates
  branch: "main"                # Branch to track
```

See [`config.yaml.template`](./config.yaml.template) for all options.

### Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `FLUX_UI_PASSWORD` | Yes | Web UI login password |
| `GITHUB_TOKEN` | Yes | GitHub personal access token for PR creation |
| `DISCORD_WEBHOOK_URL` | No | Discord webhook for notifications |

## Web UI

The dashboard at `http://localhost:8080` provides:

- **Dashboard** — system status overview
- **Goals** — create and activate goals that drive all work
- **Tasks** — create, monitor, retry, and cancel tasks
- **PRs** — review pending PRs, approve or request changes
- **Projects** — manage registered repositories
- **Logs** — real-time log viewer

Access is password-protected. For network-level security, run behind [Tailscale](https://tailscale.com) — session tokens have no expiry when Tailscale handles auth.

## Project Structure

```
flux/
├── go/src/                     # Go backend
│   ├── cmd/flux/               # Entry point
│   ├── web/                    # Frontend embedding (go:embed)
│   └── internal/
│       ├── config/             # YAML config loader
│       ├── db/                 # SQLite (WAL mode, no CGO)
│       ├── models/             # Goal, Task, Project, Alert, Usage
│       ├── manager/            # Task queue, priority, state machine
│       ├── orchestrator/       # Model selection, rate limiting
│       ├── executor/           # Claude Code runner, worktrees, guardrails
│       ├── triager/            # Task analysis and priority assignment
│       ├── updater/            # Auto-update from git remote
│       ├── vault/              # Obsidian vault writer for execution logs
│       ├── github/             # GitHub API client (repos, PRs)
│       ├── notifier/           # Discord webhook notifications
│       ├── server/             # HTTP API + WebSocket + auth
│       ├── shutdown/           # Graceful shutdown + crash recovery
│       └── testutil/           # Test utilities
├── frontend/                   # React + TypeScript frontend (Vite)
├── docs/                       # Specifications and phase plans
│   ├── spec-agent-en.md        # Agent-readable spec
│   ├── spec-human-en.md        # Human-readable spec
│   └── phases/                 # Implementation phase documents
├── deploy/                     # launchd plist for 24/7 operation
├── data/                       # SQLite database (created at runtime)
├── workspaces/                 # Git worktrees for parallel execution (created at runtime)
├── logs/                       # Application logs (created at runtime)
├── config.yaml                 # Configuration (created from template)
├── Makefile                    # Build targets
├── setup.sh                    # One-time setup
└── start-up.sh                 # Start the server
```

## Development

```bash
# Run backend in dev mode (recompiles on start)
make dev

# Run frontend dev server (hot reload on :5173)
make frontend-dev

# Build everything (frontend + embed + Go binary)
make build

# Run tests
make test

# Lint
make lint
```

The `make build` target compiles the React frontend, embeds it into the Go binary via `go:embed`, and produces a single executable at `go/bin/flux`.

## Key Concepts

### Goals

One active goal at a time. The goal drives all autonomous decisions — every executor pod reads the current goal before starting work. Set goals through the Web UI or API.

### Tasks

Work units with priority levels (P:1–100). Lower number = higher priority:

| Range | Category | Examples |
|-------|----------|----------|
| 1–5 | Service incidents | Production outage fixes |
| 6–20 | Operator tasks | Feature requests, PR comment fixes |
| 21–40 | Maintenance | Dependency updates, tech debt |
| 41–60 | Improvements | Refactoring, performance |
| 61–80 | Research-derived | Improvements from research findings |
| 81–100 | New projects | Exploratory work |

### Executor Pods

Each pod runs an isolated pipeline: claim task → create worktree → run Claude Code → test → commit → create PR → report result. Guardrails prevent runaway execution (30-min timeout, 2000-line diff limit, 20-file limit).

### Model Selection

Default model is Sonnet. Opus is used for complex tasks (priority ≤5, initial design, goal strategy) when no recent rate limit has occurred. Philosophy: reduce pods if budget is tight, never downgrade models.

### Auto-Merge vs Review

Simple PRs (system tasks, maintenance, ≤3 files, <100 additions) auto-merge. Complex PRs and guardrail violations require operator review via the Web UI.

### Triager

The Triager is a standalone component that automatically analyzes PENDING tasks before execution. It runs Claude Code with a specialized triage prompt to:

- Assign priority (1-100 scale) based on task complexity and urgency
- Recommend model (Opus vs Sonnet) for execution
- Generate detailed analysis and rewrite task descriptions for clarity
- Promote tasks from PENDING → READY state

Enable triaging in `config.yaml` with `triager.enabled: true`. Triage runs with a 2-minute timeout and max 1 turn.

### Subtasks

Tasks can spawn subtasks during execution for decomposition of complex work. Configuration limits:

- `subtask.max_depth: 1` — subtasks cannot spawn their own subtasks
- `subtask.max_per_task: 5` — maximum 5 subtasks per parent task

Subtasks inherit the parent's project but can have their own priority and description.

### Vault Integration

Flux writes execution logs and task details to an Obsidian vault for long-term knowledge retention. Configure the vault path with `vault.path` in `config.yaml` (supports `~` expansion). The vault writer operates asynchronously to avoid blocking executor pods.

### Auto-Updater

When enabled, Flux polls the git remote every 5 minutes (configurable). On detecting new commits:

1. Pulls latest changes from the tracked branch
2. Runs `make build` to rebuild the binary
3. Sends SIGTERM to itself (launchd restarts it automatically)

The updater tracks update count, last check time, and local/remote commit hashes. View status in the Web UI Settings page.

## Running 24/7

Install as a macOS launchd service for unattended operation:

```bash
bash deploy/install-launchd.sh
```

Flux handles crash recovery automatically — on restart, tasks that were running are retried without consuming retry count.

## API

External endpoints (`/api/...`) for the Web UI:

| Resource | Endpoints |
|----------|-----------|
| Goals | `POST/GET/PATCH /api/goals`, `POST /api/goals/:id/activate` |
| Tasks | `POST/GET/PATCH/DELETE /api/tasks`, `POST /api/tasks/:id/cancel`, `POST /api/tasks/:id/retry` |
| Projects | `POST/GET/PATCH /api/projects`, `POST /api/projects/:id/approve` |
| PRs | `GET /api/prs/pending`, `POST /api/prs/:id/approve`, `POST /api/prs/:id/request-changes` |
| Logs | `GET /api/logs/recent` |

Internal endpoints (`/internal/...`) for executor pods (localhost only):

| Endpoint | Purpose |
|----------|---------|
| `POST /internal/tasks/next` | Pod requests next task |
| `POST /internal/tasks/:id/done` | Pod reports completion |
| `POST /internal/subtasks` | Create subtasks |
| `GET /internal/model/:task_id` | Query model assignment |

WebSocket: `GET /ws/events` for real-time event streaming.

## Specifications

| Document | Audience | Description |
|----------|----------|-------------|
| [`docs/spec-human-en.md`](./docs/spec-human-en.md) | Humans | High-level system overview |
| [`docs/spec-human-ko.md`](./docs/spec-human-ko.md) | Humans | Korean translation |
| [`docs/spec-agent-en.md`](./docs/spec-agent-en.md) | AI Agents | Full implementation detail — schemas, structs, APIs |
| [`docs/spec-agent-ko.md`](./docs/spec-agent-ko.md) | AI Agents | Korean translation |

Implementation plans for each phase are in [`docs/phases/`](./docs/phases/).

## Current Status

| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation | Complete |
| 2A | Core Pipeline | Complete |
| 2B | Pipeline Hardening | Complete |
| 3 | Orchestration | Planned |
| 4 | Knowledge & Autonomy | Planned |

Flux can autonomously pick up tasks, write code, run tests, create PRs, and merge them. Phase 3 will add auto-scaling, usage tracking, and daily summaries.

## License

TBD
