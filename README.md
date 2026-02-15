# Flux

**Autonomous Engineering System** — a 24/7 self-operating development platform that orchestrates Claude Code agents to handle coding, testing, deployment, research, and knowledge accumulation.

## What is Flux?

Flux autonomously manages the full software development lifecycle. You set a **Goal** — Flux handles everything else:

- **Executor Pods** pick up tasks, write code via Claude Code, run tests, and create PRs
- **Researcher Pods** explore technologies, audit dependencies, and propose new projects
- **Orchestrator** scales pods, tracks usage, handles rate limits, and sends daily reports
- **Manager** prioritizes tasks, enforces state machines, and manages the work queue

```
You (Operator)
  ├── Set Goals
  ├── Approve new projects
  ├── Review complex PRs
  └── Receive Discord notifications

Flux (Autonomous)
  ├── Orchestrator — pod scaling, usage tracking, daily summaries
  ├── Manager — task queue, priority, state transitions
  ├── Executor Pods — coding, testing, PRs, deployment
  ├── Researcher Pods — research, knowledge, project discovery
  ├── Vault Writer — Obsidian knowledge management
  └── Web UI — dashboard, goals, tasks, PRs, usage graphs
```

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Backend | Go 1.22+ |
| Database | SQLite (WAL mode, no CGO) |
| Frontend | React + TypeScript + Tailwind CSS + Vite |
| State | Zustand + WebSocket |
| AI Engine | Claude Code CLI (subscription) |
| Knowledge | Obsidian Vault |
| Notifications | Discord Webhooks |
| VCS | GitHub API + Git Worktrees |
| Usage | ccusage |
| Deployment | macOS launchd |

## Key Design Decisions

- **Single binary** — Go `embed` bundles the React frontend into one executable
- **No CGO** — SQLite via `modernc.org/sqlite` for easy cross-compilation
- **Git worktrees** — parallel task execution without repo locking
- **Quality over speed** — minimum Sonnet model, never Haiku. Reduce pods if budget tight, never downgrade models
- **PR-based workflow** — all code goes through PRs with auto-merge for simple changes and operator review for complex ones
- **Obsidian knowledge** — all research, decisions, and learnings persist in a structured Vault

## Implementation Phases

| Phase | Name | Status | Description |
|-------|------|--------|-------------|
| 1 | Foundation | Planned | Go project, SQLite, CRUD API, Web UI |
| 2A | Core Pipeline | Planned | Task → Claude Code → Test → PR → Merge |
| 2B | Pipeline Hardening | Planned | Rate limits, Vault recording, launchd, crash recovery |
| 3 | Orchestration | Planned | Auto-scaling, usage tracking, daily summaries |
| 4 | Knowledge & Autonomy | Planned | Researcher Pods, self-improvement |

Detailed plans: [`plan/`](./plan/)

## Project Structure

```
flux/
├── cmd/flux/              # Entry point
├── internal/
│   ├── config/            # YAML config loader
│   ├── db/                # SQLite (WAL mode)
│   ├── models/            # Goal, Task, Project, Alert, Usage
│   ├── manager/           # Task queue, priority, state machine
│   ├── orchestrator/      # Pod scaling, usage, rate limits, daily summary
│   ├── executor/          # Claude Code runner, worktrees, guardrails
│   ├── researcher/        # Autonomous research pods
│   ├── vault/             # Obsidian writer (channel-based serial)
│   ├── github/            # Repo creation, PRs, comments
│   ├── notifier/          # Discord webhooks
│   ├── server/            # HTTP API + WebSocket + auth
│   └── shutdown/          # Graceful shutdown + crash recovery
├── web/                   # React frontend (embedded)
├── plan/                  # Implementation plans
├── config.yaml            # Configuration
└── Makefile
```

## Specifications

| Document | Audience | Language |
|----------|----------|----------|
| [`spec-human-en.md`](./spec-human-en.md) | Humans | English |
| [`spec-human-ko.md`](./spec-human-ko.md) | Humans | Korean |
| [`spec-agent-en.md`](./spec-agent-en.md) | AI Agents | English |
| [`spec-agent-ko.md`](./spec-agent-ko.md) | AI Agents | Korean |

The **human specs** describe the system at a high level. The **agent specs** contain every implementation detail — database schemas, Go structs, CLI flags, API endpoints, state machine transitions — for an AI coding agent to build Flux from scratch.

## License

TBD
