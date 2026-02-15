# Flux Implementation Plans

## Overview

This directory contains detailed implementation plans for the Flux Autonomous Engineering System. Each phase builds on the previous one, progressing from a basic skeleton to a fully autonomous system.

## Phase Summary

| Phase | Name | Goal | Key Deliverable |
|-------|------|------|-----------------|
| [Phase 1](./phase1-foundation.md) | Foundation | Flux boots, Web UI CRUD | `go build` → single binary → Web UI |
| [Phase 2A](./phase2a-core-pipeline.md) | Core Pipeline | Task → PR pipeline | End-to-end execution pipeline |
| [Phase 2B](./phase2b-pipeline-hardening.md) | Pipeline Hardening | Reliable unattended ops | Rate limit, Vault, launchd |
| [Phase 3](./phase3-orchestration.md) | Orchestration | Autonomous operation | Auto-scaling, usage tracking |
| [Phase 4](./phase4-knowledge-autonomy.md) | Knowledge & Autonomy | Autonomous growth | Researcher Pods, self-improvement |

## Dependency Chain

```
Phase 1 (Foundation)
  └── Phase 2A (Core Pipeline)
        ├── Phase 2B (Pipeline Hardening)
        │     └── Phase 3 (Orchestration)
        │           └── Phase 4 (Knowledge & Autonomy)
        └── "Flux builds Flux" transition point
```

## Tech Stack

- **Language**: Go 1.22+
- **Database**: SQLite (WAL mode, `modernc.org/sqlite` — no CGO)
- **Frontend**: React 18 + TypeScript + Tailwind CSS + Vite + Zustand
- **Integrations**: Claude Code CLI, GitHub API, Discord Webhooks, ccusage
- **Knowledge**: Obsidian Vault
- **Deployment**: macOS launchd

## Conventions

- All optional TEXT fields: `''` default (never NULL)
- JSON array fields: `'[]'` default
- Quality: minimum Sonnet model (never Haiku)
- QA: tests mandatory for coding tasks
- Git: worktree-based parallel execution

## How to Use These Plans

1. Start with Phase 1 and complete all tasks sequentially within the phase
2. Each task lists its files, dependencies, and acceptance criteria
3. Do not skip phases — each builds on the previous
4. Phase 2A experiments (sandbox, rate limit, Agent Teams) inform later phases
5. "Flux builds Flux" is the Phase 2A completion milestone

## Testing Strategy

- **Unit Tests Required**: Every Medium+ complexity task must include unit tests
- **Phase 1**: Establish test infrastructure (testutil package, mock helpers)
- **Phase 2A**: Add integration tests for pipeline components
- **Phase 2B**: E2E smoke test for complete task lifecycle
- **Coverage**: Focus on business logic, error paths, and edge cases

## Security Principles

1. **Sandbox Evaluation**: Phase 2A blocker — must validate before unattended operation
2. **Authentication**: Internal API uses shared secret authentication (not just localhost check)
3. **Session Management**: Persisted in SQLite (not in-memory), with optional expiry
4. **Rate Limiting**: Login attempts limited to 5/hour
5. **Self-Improvement PRs**: NEVER auto-merged — always require human approval
6. **Input Validation**: Prompt validation on all task inputs to prevent injection

## Cross-Cutting Concerns

- **Logging**: `log/slog` configured in Phase 1, file output with rotation
- **Health Checks**: `/health` endpoint in Phase 1, `/metrics` in Phase 2B
- **Config Validation**: `Validate()` method required on Config struct
- **Thread Safety**: `sync.RWMutex` on all shared state (sessions, pods)
- **HTTP Routing**: Go 1.22+ enhanced `http.ServeMux` with `{id}` pattern syntax
- **Disk Monitoring**: Orchestrator checks free space each tick (Phase 3)

## Known Plan Changes from Review

The following adjustments were identified during code review and should be incorporated:

1. **Phase 2B**: Error recovery (2B.12) moved BEFORE launchd (2B.11) — recovery must work before automation
2. **Phase 2B**: Rate limit handler uses non-blocking state instead of `time.Sleep` for better responsiveness
3. **Phase 3**: Per-task usage (3.6) clarified as integration layer (not duplicate of 2B.9 metrics collection)
4. **Phase 3**: Missing API endpoints added (orchestrator status, pods, services, alerts)
5. **Phase 1**: `hasComplexKeywords()` function defined in Task 1.3 (task complexity detection)
6. **Phase 2A**: `ManagerClient` HTTP client defined in Task 2A.8 (Manager API interaction)
