# Flux

**Autonomous engineering system that runs 24/7.** You set goals — Flux writes the code, runs tests, creates PRs, and merges them.

Flux orchestrates [Claude Code](https://claude.ai/claude-code) agents in isolated Git worktrees, managing the full software development lifecycle from task assignment through PR merge. Built with **Go + SQLite + React**, it ships as a **single binary** with no external dependencies.

**What makes Flux different:**
- Runs continuously without human intervention
- Intelligent priority-based task queue (P1-P100 scale)
- Automatic task triage and complexity assessment
- Smart auto-merge for simple changes, manual review for complex ones
- Built-in guardrails prevent runaway execution
- Complete usage tracking and cost visibility

## How It Works

```
You (Operator)                        Flux (Autonomous)
  │                                     │
  ├── Create goals & tasks              ├── Triager analyzes PENDING tasks
  ├── Review complex PRs                ├── Manager picks next READY task by priority
  ├── Approve new projects              ├── Orchestrator assigns task to executor pod
  └── Get Discord notifications         ├── Executor creates isolated Git worktree
                                        ├── Claude Code writes code & runs tests
                                        ├── Executor creates GitHub PR
                                        ├── Auto-merge (simple) or request review (complex)
                                        ├── Manager marks task COMPLETED
                                        └── Worktree cleaned up automatically
```

### Complete Autonomous Workflow

**Phase 1: Task Creation & Triage**
```
1. Operator creates task via Web UI
   └─ Task state: PENDING

2. Triager pod claims PENDING task
   ├─ Runs Claude Code with triage prompt (2-min timeout)
   ├─ Analyzes complexity, urgency, scope
   ├─ Assigns priority (P1-P100 scale)
   ├─ Recommends model (Opus vs Sonnet)
   └─ Task state: PENDING → READY

3. Manager enqueues READY task by priority
   └─ Lower priority number = executed first
```

**Phase 2: Task Execution**
```
4. Orchestrator selects idle executor pod
   ├─ Checks available pods (max 5 concurrent)
   ├─ Selects model based on priority & complexity
   └─ Assigns READY task to pod

5. Executor pod processes task
   ├─ Creates Git worktree: workspaces/trees/{project}--task-{id}/
   ├─ Branches from main: flux--task-{id}
   ├─ Injects task prompt + execution protocol
   ├─ Runs Claude Code CLI with guardrails
   │   • Max execution time: 30 minutes
   │   • Max turns: 30
   │   • Max diff lines: 2000
   │   • Max files changed: 20
   ├─ Claude Code writes code, tests, commits
   ├─ Task state: READY → RUNNING
   └─ Executor monitors for violations
```

**Phase 3: PR Creation & Review**
```
6. Executor creates GitHub PR
   ├─ Pushes branch to GitHub: flux--task-{id}
   ├─ Generates PR description from task summary
   ├─ Decides merge strategy:
   │   • Auto-merge: ≤3 files, <100 additions, system/maintenance work
   │   • Request review: Complex changes or guardrail violations
   └─ Records PR metadata in database

7. Manager handles PR outcome
   ├─ Auto-merged → Task state: COMPLETED
   ├─ Review requested → Operator approves/requests changes
   └─ Failed → Retry (up to 3 attempts) or mark FAILED
```

**Phase 4: Cleanup & Notification**
```
8. Executor cleanup
   ├─ Records execution log to Obsidian vault
   ├─ Removes worktree directory
   └─ Pod returns to idle state

9. Manager updates task state
   ├─ COMPLETED → ARCHIVED (after cooldown)
   └─ FAILED → ARCHIVED (after max retries)

10. Notifier sends Discord webhook
    ├─ Task completion notification
    ├─ PR link for review (if needed)
    └─ Daily summary (completed/failed/pending counts)
```

**Task lifecycle:** `PENDING → READY → RUNNING → COMPLETED → ARCHIVED`
**Retry logic:** Failed tasks retry up to 3 times before marking as permanently FAILED
**Crash recovery:** Tasks in RUNNING state on crash are automatically retried without consuming retry count

## Key Features

- **🤖 Fully Autonomous** — Set goals, Flux handles everything from code writing to PR merging
- **🔒 Isolated Execution** — Each task runs in a dedicated Git worktree with guardrails (timeout, diff size, file limits)
- **📊 Priority-Based Queue** — P1-P100 task system ensures critical work gets done first
- **🧠 Intelligent Triaging** — Automatic task analysis, priority assignment, and complexity assessment
- **🔄 Smart Auto-Merge** — Simple PRs merge automatically, complex ones request review
- **📦 Single Binary** — Frontend embedded in Go binary, no separate deployments
- **💾 Zero Configuration Database** — SQLite with automatic backups and WAL mode for reliability
- **📈 Usage Tracking** — Built-in Claude Code usage monitoring and cost visibility
- **🔧 Obsidian Integration** — Execution logs written to vault for long-term knowledge retention
- **⚡ Auto-Update** — Pulls latest changes and rebuilds automatically when running 24/7

## Tech Stack

- **Backend:** Go 1.25.6, SQLite (modernc.org/sqlite - pure Go, no CGO)
- **Frontend:** React 18, TypeScript 5.3, Tailwind CSS 3.4, Vite 5
- **Automation:** Claude Code CLI (Max 5x plan recommended)
- **Infrastructure:** Git worktrees, GitHub API, Discord webhooks

## Prerequisites

- **Go 1.22+** — `brew install go`
- **Node.js 20+** — `brew install node` (includes npm for ccusage tracking)
- **Claude Code** subscription (Max 5x plan recommended)
- **GitHub** personal access token
- **macOS** (launchd support for 24/7 operation)

## Quick Start

Get Flux running in under 5 minutes:

```bash
# 1. Clone the repository
git clone https://github.com/circle-oo/flux.git
cd flux

# 2. Set required environment variables
export FLUX_UI_PASSWORD='your-secure-password'
export GITHUB_TOKEN='ghp_your_token_here'  # Needs 'repo' scope
export DISCORD_WEBHOOK_URL='https://discord.com/api/webhooks/...'  # optional

# 3. Run setup (installs deps, builds frontend, compiles binary)
bash setup.sh
# This creates config.yaml from template, installs Go deps, builds frontend

# 4. Start Flux
bash startup-prod.sh    # Production mode (background)
# Or for development:
bash startup-dev.sh     # Dev mode (foreground, with logs)
```

**Access the Web UI:** Open **http://localhost:8080** and log in with your password.

### First Steps After Setup

1. **Create a goal** (Goals page)
   ```
   Example: "Build a personal task tracker with React and Go"
   ```

2. **Register a project** (Projects page)
   - Project name: `my-project`
   - Repository: `github.com/username/my-project`
   - Approve the project after registration

3. **Create your first task** (Tasks page)
   ```
   Title: "Initialize React + Go project structure"
   Project: my-project
   Priority: 20 (Operator task)
   Description: "Set up a basic React frontend and Go backend structure"
   ```

4. **Watch Flux work autonomously**
   - Triager analyzes the task (PENDING → READY)
   - Executor pod picks it up (READY → RUNNING)
   - Claude Code writes the code
   - PR is created on GitHub
   - Auto-merge or review request

5. **Monitor progress**
   - Dashboard: System overview, usage metrics
   - Tasks: Real-time task status updates
   - PRs: Review pending PRs
   - Logs: Live execution logs

## Configuration

Setup creates `config.yaml` from the template. Key settings:

```yaml
server:
  port: 8080                    # Web UI port
  auth:
    password_env: "FLUX_UI_PASSWORD"

database:
  path: "./data/flux.db"        # SQLite database location
  backup_dir: "./data/backups"  # Database backup directory
  backup_cron: "0 4 * * *"      # Daily backup at 4 AM
  backup_retention_days: 7      # Keep backups for 7 days

vault:
  path: "~/ObsidianVault/Flux"  # Obsidian vault for execution logs

github:
  username: "your-username"     # Edit this
  token_env: "GITHUB_TOKEN"
  auto_create_repo: true        # Auto-create GitHub repos for new projects
  default_visibility: "public"  # Default repo visibility

claude_code:
  plan: "max_5x"                # Claude Code subscription plan

ccusage:
  command: "npx ccusage@latest" # Command to collect usage stats
  collection_interval: 1h       # How often to collect usage

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
  check_interval: 5m            # How often to check for work
  scale_cooldown: 15m           # Cooldown before scaling pods

shutdown:
  pod_grace_period: 10m         # Grace period for pod shutdown
  force_kill_after: 12m         # Force kill if not stopped

cleanup:
  service_metrics_raw_days: 7   # Keep raw service metrics for 7 days
  jsonl_retention_days: 30      # Keep JSONL logs for 30 days
  usage_snapshots_days: 90      # Keep usage snapshots for 90 days
  failed_worktree_hours: 24     # Clean up failed worktrees after 24h

auto_update:
  enabled: true                 # Auto-update from git remote
  check_interval: 5m            # How often to check for updates
  branch: "main"                # Branch to track

logging:
  level: "info"                 # Log level (debug, info, warn, error)
  file: "./logs/flux.log"       # Log file location
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

- **Dashboard** — system status overview with usage metrics
- **Goals** — create and activate goals that drive all work
- **Tasks** — create, monitor, retry, and cancel tasks; view triage analysis
- **PRs** — review pending PRs, approve or request changes
- **Projects** — manage registered repositories
- **Settings** — view auto-updater status, system configuration
- **Logs** — real-time log viewer

Access is password-protected. For network-level security, run behind [Tailscale](https://tailscale.com) — session tokens have no expiry when Tailscale handles auth.

## Architecture

Flux consists of four main components working together:

```
┌─────────────────────────────────────────────────────────────────┐
│                          WEB UI (React)                         │
│                    http://localhost:8080                        │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTP/WebSocket
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                      SERVER (HTTP API)                          │
│  • Authentication & sessions                                    │
│  • External API (/api/...) for Web UI                          │
│  • Internal API (/internal/...) for executor pods              │
│  • Real-time WebSocket event streaming                         │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    MANAGER (Task Queue)                         │
│  • Priority-based scheduling (P1-P100)                         │
│  • State machine: PENDING→READY→RUNNING→COMPLETED→ARCHIVED     │
│  • Retry logic (up to 3 attempts)                              │
│  • Dependency resolution for blocked tasks                      │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                  ORCHESTRATOR (Pod Manager)                     │
│  • Spawns executor pods (up to 5 concurrent)                   │
│  • Model selection (Sonnet/Opus by priority)                   │
│  • Rate limit handling & pod scaling                            │
│  • Usage tracking & daily summaries                             │
│  • Goal advisor for strategic planning                          │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│                    EXECUTOR POD (Worker)                        │
│                                                                 │
│  1. Claims READY task from Manager                              │
│  2. Creates isolated Git worktree                               │
│  3. Runs Claude Code CLI with task prompt                       │
│  4. Enforces guardrails (timeout, diff size, file count)       │
│  5. Commits changes & creates GitHub PR                         │
│  6. Reports completion to Manager                               │
│  7. Cleans up worktree                                          │
│                                                                 │
│  ┌─────────────────────────────────────────┐                  │
│  │   Git Worktree (Isolated Workspace)     │                  │
│  │   • Dedicated branch per task           │                  │
│  │   • Full project checkout               │                  │
│  │   • Claude Code CLI execution           │                  │
│  │   • Automatic cleanup after PR          │                  │
│  └─────────────────────────────────────────┘                  │
└─────────────────────────────────────────────────────────────────┘
```

### Component Details

#### 1. Server (`internal/server`)
- HTTP API and WebSocket server for the Web UI
- Authentication and session management
- **External API** (`/api/...`) for Web UI operations
- **Internal API** (`/internal/...`) for executor pods (localhost-only endpoints)
- Real-time event streaming via WebSocket

#### 2. Manager (`internal/manager`)
- Task queue management with priority-based scheduling
- State machine for task lifecycle (PENDING → READY → RUNNING → COMPLETED → ARCHIVED)
- Retry logic for failed tasks (up to 3 attempts)
- Dependency resolution for blocked tasks

#### 3. Orchestrator (`internal/orchestrator`)
- Spawns and manages executor pods (up to 5 concurrent by default)
- Model selection (Sonnet vs Opus based on task priority and complexity)
- Rate limit handling and pod scaling
- Usage tracking and daily summary generation
- Goal advisor for strategic planning

#### 4. Executor (`internal/executor`)
- Runs Claude Code CLI in isolated Git worktrees
- Enforces guardrails (30-min timeout, 2000-line diff limit, 20-file limit)
- Handles task decomposition into subtasks
- Creates GitHub PRs with auto-merge or review requests
- Records execution logs to Obsidian vault

### Claude Code Integration

The Executor interfaces with Claude Code CLI through a sophisticated execution pipeline:

```bash
# Executor spawns Claude Code CLI with task-specific prompt
claude --yes --non-interactive \
  --model opus-4.6 \
  --max-turns 30 \
  --working-directory /path/to/worktree \
  < task_prompt.txt
```

**Key integration points:**
1. **Prompt injection** — Task description, triage analysis, and execution protocol injected as system prompt
2. **Guardrail enforcement** — Timeout, diff size, and file count limits enforced during execution
3. **Output parsing** — Executor parses Claude Code output for task summary, decomposition decisions, and error detection
4. **Worktree isolation** — Each task runs in a dedicated Git worktree to enable parallel execution without conflicts
5. **Result extraction** — Task summary, file changes, and completion status extracted from execution logs

**Data flow:** Server receives tasks → Manager queues by priority → Orchestrator assigns to executor pod → Executor creates worktree, runs Claude Code, creates PR → Manager updates task state → Server notifies Web UI via WebSocket.

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

### Setup for Contributors

```bash
# 1. Fork and clone the repository
git clone https://github.com/YOUR_USERNAME/flux.git
cd flux

# 2. Install prerequisites
brew install go node  # macOS
# Requires Go 1.22+ and Node.js 20+

# 3. Run setup script
bash setup.sh
# This installs dependencies, builds frontend, compiles binary

# 4. Set environment variables
export FLUX_UI_PASSWORD='dev-password'
export GITHUB_TOKEN='your-github-token'

# 5. Start development servers
make dev              # Backend (port 8080)
make frontend-dev     # Frontend with hot reload (port 5173)
```

### Development Workflow

```bash
# Backend development (Go)
make dev              # Starts server with live compilation
make test             # Run Go tests
make lint             # Run golangci-lint

# Frontend development (React/TypeScript)
cd frontend
npm run dev           # Hot reload on port 5173
npm run build         # Build for production
npm run lint          # ESLint check

# Full build (production binary)
make build            # Compiles frontend + embeds in Go binary
# Output: go/bin/flux (single binary with embedded UI)
```

### Development Iteration Cycle

```
1. Make Changes
   ├─ Backend: Edit Go files in go/src/internal/
   ├─ Frontend: Edit React files in frontend/src/
   └─ Config: Update config.yaml.template if needed

2. Test Locally
   ├─ Backend: make test (Go unit tests)
   ├─ Frontend: npm run lint (TypeScript checks)
   └─ Integration: Start dev servers and test in browser

3. Build & Verify
   ├─ make build (produces single binary)
   ├─ ./go/bin/flux (run production build locally)
   └─ Verify at http://localhost:8080

4. Submit PR
   ├─ Create feature branch
   ├─ Commit with clear message
   ├─ Push to your fork
   └─ Open PR with description of changes
```

**Build process details:**
- `make build` runs `npm run build` in frontend/ (produces `dist/`)
- Go's `go:embed` directive embeds `frontend/dist` into binary at compile time
- Single binary includes both backend and frontend — no separate deployment needed

### Testing Your Changes

```bash
# Unit tests
make test                           # All Go tests
go test ./go/src/internal/manager   # Specific package

# Integration testing
bash setup.sh                       # Full setup
bash startup-dev.sh                 # Start in dev mode
# Open http://localhost:8080 and test manually

# Check database state
sqlite3 data/flux.db "SELECT * FROM tasks LIMIT 5;"
```

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

**Git Worktree Lifecycle:**

```
1. Task Assignment
   ├─ Pod claims READY task from Manager via /internal/tasks/next
   └─ Marks task as RUNNING via /internal/tasks/:id/started

2. Worktree Creation
   ├─ Creates isolated worktree: workspaces/trees/{project}--task-{id}/
   ├─ Branches from main: flux--task-{id}
   └─ Full project checkout (independent of other worktrees)

3. Claude Code Execution
   ├─ Injects task prompt with description, triage, and protocol
   ├─ Runs Claude Code CLI with model, turn limit, and timeout
   ├─ Claude Code writes code, runs tests, creates commits
   └─ Executor monitors for guardrail violations

4. PR Creation
   ├─ Pushes branch to GitHub
   ├─ Creates PR with auto-generated description
   ├─ Decides: auto-merge (simple) or request review (complex)
   └─ Records PR metadata in database

5. Cleanup
   ├─ Reports completion to Manager via /internal/tasks/:id/done
   ├─ Removes worktree directory
   └─ Pod returns to idle state, ready for next task
```

**Parallel execution:** Multiple pods can work simultaneously because each has its own worktree. This enables Flux to process 5+ tasks concurrently without Git conflicts.

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

### Database Backups

Flux automatically backs up the SQLite database on a configurable schedule (default: daily at 4 AM). Backups are stored in `data/backups/` with automatic rotation based on `backup_retention_days` (default: 7 days). The backup process uses SQLite's VACUUM INTO for consistent snapshots without downtime.

### Usage Tracking

Flux tracks Claude Code usage via the `ccusage` command (runs `npx ccusage@latest` by default). Collection happens at configurable intervals (default: 1 hour). Usage data includes token consumption, API calls, and model usage, displayed in the Web UI dashboard.

### Cleanup & Retention

Automatic cleanup policies keep disk usage under control:

- **Service metrics:** Raw data retained for 7 days (hourly aggregates kept longer)
- **JSONL logs:** Execution logs kept for 30 days
- **Usage snapshots:** Detailed usage data kept for 90 days
- **Failed worktrees:** Cleaned up after 24 hours to prevent disk bloat

All retention periods are configurable in `config.yaml`.

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
| Auth | `POST /api/auth/login`, `POST /api/auth/logout` |
| Goals | `POST/GET/PATCH /api/goals`, `GET /api/goals/current`, `POST /api/goals/:id/activate` |
| Tasks | `POST/GET/PATCH/DELETE /api/tasks`, `GET /api/tasks/:id`, `POST /api/tasks/:id/cancel`, `POST /api/tasks/:id/retry`, `POST /api/tasks/:id/archive`, `GET /api/tasks/:id/subtasks` |
| Projects | `POST/GET/PATCH /api/projects`, `GET /api/projects/:id`, `POST /api/projects/:id/approve`, `POST /api/projects/:id/reject` |
| PRs | `GET /api/prs/pending`, `POST /api/prs/:id/approve`, `POST /api/prs/:id/request-changes` |
| Services | `GET /api/services`, `GET /api/alerts` |
| System | `POST /api/system/restart`, `GET /api/system/deploy/status`, `POST /api/system/deploy`, `POST /api/system/deploy/check-remote` |
| Logs | `GET /api/logs/recent` |

Internal endpoints (`/internal/...`) for executor pods (localhost only):

| Endpoint | Purpose |
|----------|---------|
| `POST /internal/tasks/next` | Executor pod requests next READY task |
| `POST /internal/tasks/next-pending` | Triager requests next PENDING task |
| `POST /internal/tasks/:id/started` | Pod marks task as started |
| `POST /internal/tasks/:id/done` | Pod reports completion |
| `POST /internal/tasks/:id/triaged` | Triager reports triage completion |
| `POST /internal/tasks` | Create new task (internal) |
| `POST /internal/subtasks` | Create subtasks during execution |
| `GET /internal/model/:task_id` | Query model assignment for task |
| `GET /internal/tasks/:id/status` | Query task status |
| `GET /internal/projects/:id` | Get project details |

WebSocket: `GET /ws/events` for real-time event streaming.

## Specifications

| Document | Audience | Description |
|----------|----------|-------------|
| [`docs/spec-human-en.md`](./docs/spec-human-en.md) | Humans | High-level system overview |
| [`docs/spec-human-ko.md`](./docs/spec-human-ko.md) | Humans | Korean translation |
| [`docs/spec-agent-en.md`](./docs/spec-agent-en.md) | AI Agents | Full implementation detail — schemas, structs, APIs |
| [`docs/spec-agent-ko.md`](./docs/spec-agent-ko.md) | AI Agents | Korean translation |

Implementation plans for each phase are in [`docs/phases/`](./docs/phases/).

## FAQ

**Q: How much does it cost to run Flux?**
A: Cost depends on Claude Code usage. Max 5x plan is recommended (~$40/month subscription). Actual API costs vary by task complexity and execution time. The Web UI dashboard shows real-time usage tracking.

**Q: Can I use Flux with private repositories?**
A: Yes! Set `github.default_visibility: "private"` in `config.yaml`. Your GitHub token needs `repo` scope for private repositories.

**Q: What happens if Flux crashes?**
A: Flux has automatic crash recovery. On restart, tasks in RUNNING state are retried without consuming retry count. Database backups are taken daily (configurable).

**Q: Can I run Flux on Linux or Windows?**
A: Currently optimized for macOS (launchd support). Linux support is planned. Windows support requires WSL2 (untested).

**Q: How do I limit Flux's activity to business hours?**
A: Modify the launchd plist to include `StartCalendarInterval` constraints, or use `orchestrator.check_interval` to control execution frequency.

**Q: Can Flux work with monorepos?**
A: Yes! Each project can point to any repository. Git worktrees handle monorepos efficiently since they share the `.git` directory.

**Q: What models does Flux support?**
A: Flux uses Claude Sonnet 4.5 by default. Opus 4.6 is used for high-priority tasks (P1-P5) and complex work. Model selection is automatic based on task triage.

**Q: How do I prevent Flux from merging PRs automatically?**
A: Set stricter auto-merge criteria in the Executor's decision logic, or disable auto-merge entirely by requiring manual approval for all PRs (modify `executor/guardrails.go`).

**Q: Can I add custom guardrails?**
A: Yes! Edit `internal/executor/guardrails.go` and add your custom checks. Common additions: file path restrictions, dependency change detection, security scan integration.

**Q: How do I integrate with Slack instead of Discord?**
A: Implement a new notifier in `internal/notifier/slack.go` following the Discord pattern. PRs welcome!

## Current Status

| Phase | Name | Status |
|-------|------|--------|
| 1 | Foundation | ✅ Complete |
| 2A | Core Pipeline | ✅ Complete |
| 2B | Pipeline Hardening | ✅ Complete |
| 3 | Orchestration | ✅ Complete |
| 4 | Knowledge & Autonomy | 🚧 In Progress |

**What works now:**
- ✅ Autonomous task execution with Claude Code
- ✅ Priority-based task queue (P1-P100)
- ✅ Automatic task triage and complexity assessment
- ✅ Git worktree isolation for parallel execution
- ✅ Smart auto-merge for simple PRs
- ✅ Guardrails (timeout, diff size, file count limits)
- ✅ Usage tracking and cost visibility
- ✅ Auto-update from git remote
- ✅ Database backups and retention policies
- ✅ Discord notifications
- ✅ Obsidian vault integration for execution logs

**Coming soon (Phase 4):**
- 🚧 Knowledge graph for inter-task learning
- 🚧 Adaptive priority adjustment based on outcomes
- 🚧 Improved goal advisor with long-term planning
- 🚧 Multi-repository project support

## Troubleshooting

### Common Issues

**"Port 8080 already in use"**
```bash
# Find and kill process using port 8080
lsof -ti:8080 | xargs kill -9
# Or change port in config.yaml
```

**"Database is locked"**
```bash
# SQLite WAL mode should prevent this, but if it occurs:
rm data/flux.db-wal data/flux.db-shm
# Restart Flux
```

**"Worktree creation failed"**
```bash
# Clean up orphaned worktrees
git worktree prune
# Remove stale worktree directories
rm -rf workspaces/trees/*
```

**"Claude Code command not found"**
```bash
# Ensure Claude Code is installed and in PATH
which claude
# If not found, install from https://claude.ai/claude-code
```

**"GitHub authentication failed"**
```bash
# Verify token has correct permissions:
# - repo (full control)
# - workflow (if needed for CI)
# Test token:
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/user
```

**Executor pod stuck in RUNNING**
```bash
# Check if Claude Code process is alive
ps aux | grep claude
# Force cleanup via Web UI: Tasks → Cancel
# Or restart Flux (crash recovery will reset state)
```

### Debugging

```bash
# Enable debug logging
# Edit config.yaml:
logging:
  level: "debug"

# View real-time logs
tail -f logs/flux.log

# Check database state
sqlite3 data/flux.db
> SELECT * FROM tasks WHERE state = 'RUNNING';
> SELECT * FROM executor_pods;

# Inspect worktree
cd workspaces/trees/{project}--task-{id}/worktree
git status
git log
```

## Contributing

Contributions are welcome! Flux is in active development. To contribute:

### Getting Started

1. **Fork the repository** and create a feature branch
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Follow existing conventions**
   - **Go:** Follow standard Go conventions (see `internal/` for patterns)
   - **React/TypeScript:** Use functional components, TypeScript strict mode
   - **Commit messages:** Clear, descriptive (e.g., "Add retry logic for failed tasks")

3. **Write tests** for new functionality
   ```bash
   # Go tests
   go test ./go/src/internal/your_package -v

   # Frontend tests (if applicable)
   cd frontend && npm run test
   ```

4. **Run linters** before submitting
   ```bash
   make lint              # Go linter
   cd frontend && npm run lint  # Frontend linter
   ```

5. **Submit a PR** with:
   - Clear description of changes
   - Screenshots (for UI changes)
   - Test results
   - Related issue number (if applicable)

### Contribution Areas

We especially welcome contributions in:

- **Executor improvements** — Better guardrails, smarter decomposition logic
- **Frontend enhancements** — UI/UX improvements, new visualizations
- **Orchestrator features** — Advanced scheduling, better model selection
- **Integrations** — Slack, Linear, Jira, other tools
- **Documentation** — Tutorials, architecture guides, API docs
- **Testing** — Integration tests, end-to-end tests

### Architecture Guidelines

- **Keep components isolated** — Each component (Server, Manager, Orchestrator, Executor) should have minimal coupling
- **Use internal APIs** — Executor pods communicate via `/internal/` endpoints only
- **Database access** — Only Manager and Server directly access SQLite; other components use APIs
- **Configuration** — Add new config options to `config.yaml.template` with sensible defaults
- **Error handling** — Log errors verbosely, return user-friendly messages via API

### Code Review Process

1. All PRs require review before merge
2. Automated checks must pass (linting, tests)
3. For major changes, expect architecture discussion
4. Maintainers will provide feedback within 48 hours

### Local Testing Checklist

Before submitting your PR:

- [ ] Backend compiles: `make build`
- [ ] Tests pass: `make test`
- [ ] Linter passes: `make lint`
- [ ] Frontend builds: `cd frontend && npm run build`
- [ ] Manual testing completed in dev environment
- [ ] Database migrations included (if schema changed)
- [ ] Config template updated (if new settings added)
- [ ] Documentation updated (README, code comments)

For major features or architectural changes, **open an issue first** to discuss the approach.

## License

TBD
