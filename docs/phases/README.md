# Flux Implementation Plans

## Overview

This directory contains detailed implementation plans for the Flux Autonomous Engineering System. Each phase builds on the previous one, progressing from a basic skeleton to a fully autonomous system.

**Note on Obsidian Integration**: The Obsidian CLI integration (documented in `docs/obsidian-integration-plan.md` and `docs/obsidian-plan.md`) is incorporated into Phase 4 (Knowledge & Autonomy). No intermediate phases are needed between Phase 2B and Phase 3. See `phase-structure-analysis.md` for detailed rationale.

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

## Obsidian Integration Analysis (2026-02-16)

A comprehensive analysis was performed to determine if intermediate phases (2.c, 2.d) are needed to accommodate Obsidian CLI integration before Phase 3. **Conclusion**: No intermediate phases required. The existing phase structure optimally accommodates Obsidian integration:
- **Phase 2B**: Optional basic CLI client (~3 files) can be added or deferred
- **Phase 3**: Orchestration proceeds independently (no Obsidian dependencies)
- **Phase 4**: Full Knowledge UI and Researcher Pod integration (~15 files)

See `phase-structure-analysis.md` for detailed rationale and dependency analysis.

## Phase 3/4 Refinement (Post-Phase 2B Audit)

Both Phase 3 and Phase 4 plans were comprehensively revised after auditing the completed Phase 2B codebase. Key architectural decisions:

### Phase 3 Changes
- **New Task 3.0**: Orchestrator integration in `main.go` added as prerequisite — the current `main.go` spawns executors directly, so the Orchestrator must take over lifecycle management before any orchestration logic can work
- **ScaleManager (3.2)**: Scoped to Executor-only for Phase 3. Researcher pod ratio deferred to Phase 4 to reduce Phase 3 complexity
- **GoalAdvisor (3.13→3.17)**: Downgraded to optional. Simple heuristics don't justify being on the critical path
- **File paths corrected**: Frontend is `frontend/src/` not `web/src/`; worktree manager is in `executor/worktree.go` not `internal/worktree/`
- **JSONL retention**: Changed from 30 days to 7 days per spec for sensitive data
- **Existing infrastructure identified**: `UsageStore`, `PodRegistry`, `RateLimitHandler.CheckAndRecover()`, Settings page — all already exist and should be extended, not rebuilt
- **Explicit dependency graph** and **recommended implementation order** with critical path analysis added

### Phase 4 Changes
- **New Task 4.0**: ScaleManager Executor:Researcher ratio upgrade — bridges Phase 3's Executor-only ScaleManager to the full spec
- **Researcher Pod (4.1)**: Restructured to mirror Executor patterns (`executor/executor.go`) and reuse `claudecli`, `apiclient`, PodRegistry infrastructure
- **Autonomous Research (4.2)**: Simplified from complex weighted-priority to simple ordered-priority. Claude Code handles intelligent decision-making; Go code just provides category and context
- **Vault Writer (4.3)**: Reduced scope — only adds research/proposal templates, defers decision records
- **Deployment (4.5)**: Narrowed — auto-updater already handles most deployment; this task adds `make deploy` convenience wrapper
- **Self-Improvement (4.6)**: File paths updated to `go/src/internal/` (actual structure), `main.go` added to immutable list
- **Research UI (4.7)**: Simplified — reuses task filtering (`type=RESEARCH`) instead of building separate API
- **Explicit dependency graph** and **parallel track identification** added
