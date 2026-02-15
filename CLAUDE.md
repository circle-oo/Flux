# Flux — Autonomous Engineering System

Flux is an autonomous engineering system that runs 24/7. The Operator sets direction via Goals; Flux handles development, QA, and PR workflows by orchestrating Claude Code agents.

## Architecture

```
Operator (Web UI)
  └─ Goals, Tasks, PR Review, Project Approval

Flux
  ├─ Server (HTTP API + WebSocket + embedded React UI)
  ├─ Manager (priority queue, state machine, dependency resolution)
  ├─ Orchestrator (model selection, rate limit handling, goal injection)
  ├─ Executor Pod (Claude Code → worktree → test → PR → merge)
  └─ Vault Writer (Obsidian knowledge recording)
```

## Directory Structure

```
flux/
  go/src/               # Go backend (module root)
    cmd/flux/            # Entrypoint
    internal/
      config/            # YAML config loader
      db/                # SQLite schema, bootstrap, queries
      executor/          # Claude Code runner, worktree, triage, prompts/
      github/            # PR creation, merge, comments, description builder
      manager/           # Task queue, priority, state transitions
      models/            # Task, Goal, Project, Alert, Usage models
      notifier/          # Discord webhook
      orchestrator/      # Model selection, rate limit handler
      server/            # HTTP routes, auth, WebSocket, log streaming
      shutdown/          # Graceful shutdown, crash recovery
      updater/           # Auto-update from git
      vault/             # Obsidian vault writer
    web/                 # Embedded frontend dist
  frontend/              # React + TypeScript + Tailwind + Vite
    src/
      components/        # Layout, Sidebar
      lib/               # API client
      pages/             # Dashboard, Goals, Tasks, TaskDetail, Projects, PRs, Logs, Login
      stores/            # Zustand stores (auth, goal, task, project, pr, log, ws)
  docs/                  # Specifications and phase plans
  deploy/                # launchd plist
```

## Build & Run

```bash
# Full build (frontend + backend)
make build               # -> go/bin/flux

# Development
make dev                 # Run backend with hot config
make frontend-dev        # Vite dev server with HMR

# Testing
make test                # Go tests
cd frontend && npm run build  # Frontend type check + build

# Individual
cd go/src && go build ./cmd/flux
cd go/src && go test ./...
cd frontend && npm run build
```

## Conventions

### Go
- All optional TEXT fields default to `''`, never NULL
- JSON array fields default to `'[]'`
- Package-level slog with structured attributes
- `component` field auto-injected by LogBroadcastHandler
- Internal API at `/internal/...` (localhost only), external at `/api/...` (auth required)
- Models in `internal/models/`, stores with `*Store` pattern

### Frontend
- Zustand for state management
- API calls through `lib/api.ts` singleton
- WebSocket events via `stores/wsStore.ts`
- Dark theme: Tailwind `slate-*` palette
- CSS utilities in `index.css` (card, btn-*, badge-*, input, label)

### Task Pipeline
```
Operator creates task → Triage (async, haiku) → READY
Executor picks up → Autopilot prompt (analyze→plan→implement→verify)
→ Build check → Test → Commit → Rebase → Push → PR → Auto-merge or review
```

### Prompt Templates
Edit `go/src/internal/executor/prompts/*.txt` to improve executor behavior.
Templates use Go `text/template` syntax and are embedded at compile time.

## Config

Copy `config.yaml.template` to `config.yaml`. Key environment variables:
- `FLUX_UI_PASSWORD` — Web UI login password
- `GITHUB_TOKEN` — GitHub API token
- `DISCORD_WEBHOOK_URL` — Discord notifications
