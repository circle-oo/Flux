# Flux — Autonomous Engineering System

## Overview

Flux is an **autonomous engineering system** running 24/7. The Operator (user) sets direction; Flux autonomously handles development, QA, DevOps, and R&D by orchestrating Claude Code agents.

**Core capabilities:** coding, testing, deployment, service monitoring, bug detection/fixing, technical research, knowledge accumulation, and new project discovery.

All knowledge and documentation is managed in an Obsidian Vault.

## System Architecture

```
Operator (User)
  ├── Direction: Set Goals, discuss strategy with Orchestrator
  ├── Approval: Approve/reject new projects
  ├── Review: PRs (Approve / Request Changes + GitHub comments)
  ├── Feedback: Create tasks ("backseat driving")
  └── Notifications: Discord

Flux
  ├── Orchestrator: Pod count/model decisions, scaling, daily summary
  │     ├── ScaleManager: Pod count/ratio, cooldown management
  │     ├── UsageCollector: ccusage time-series collection, DB snapshots
  │     ├── DailySummary: Midnight Discord summary
  │     ├── RateLimitHandler: Detect → stop → dynamic wait → resume
  │     └── GoalAdvisor: Goal proposals (PROPOSED → Operator selection)
  ├── Manager: Task queue, priority assignment, state management
  ├── Executor Pods: Coding, testing, PRs, deployment
  ├── Researcher Pods: Research, knowledge accumulation, project discovery
  ├── Vault Writer: Serialized Obsidian writes
  ├── Notifier: Discord notifications
  └── Obsidian Vault: Knowledge repository
```

## Core Philosophy

- **Goal-centric**: The current Goal drives all decisions.
- **Maximum autonomy**: Operator sets direction only. Flux handles the rest.
- **Quality first**: No Haiku. Reduce pods if budget is tight, never downgrade models. Minimum Sonnet.
- **QA mandatory**: Tests must pass before commit. Write tests if none exist. PR-based workflow.
- **Knowledge accumulation**: Research → knowledge → work → new knowledge → repeat.
- **Budget-aware**: ccusage tracking + rate limit detection.
- **Simple design**: Go + SQLite + Obsidian + filesystem.

## Tech Stack

Go is chosen because the Operator's primary language is Go (enabling self-improvement), it has excellent process orchestration (goroutine + exec.Command), produces single binaries (easy launchd registration), supports Web UI embedding (`embed` package for React), and works with SQLite without CGO (`modernc.org/sqlite`).

## Conventions

All optional TEXT fields use empty string (`''`) as default, never NULL. Queries use `WHERE field != ''` consistently. JSON array fields use `'[]'` as default.

---

## State Machines

**Tasks:** `PENDING → READY → RUNNING → COMPLETED/FAILED → ARCHIVED`. FAILED can RETRY (max 3). Cancellation transitions to FAILED with `error_log: "cancelled by operator"`.

**Projects:** `PROPOSED → ACTIVE → ARCHIVED` (or `REJECTED`).

**Goals:** `PROPOSED → ACTIVE → COMPLETED / SUPERSEDED`.

**PRs:** `OPEN → APPROVED → MERGED` (or `CHANGES_REQUESTED → fix → OPEN`).

---

## Internal Communication

Pods communicate with Manager via internal HTTP API (`/internal/...`). Currently localhost calls within the same process, but the interface allows future process separation without API changes.

```
POST /internal/tasks/next        "Next task"
POST /internal/tasks/:id/done    "Completion report"
POST /internal/subtasks          "Create subtask"
GET  /internal/model/:task_id    "Model decision query"
```

External Web UI API (`/api/...`) and internal Pod API (`/internal/...`) are separated by prefix. `/internal/...` endpoints are never exposed externally.

---

## Integrations

### GitHub

Auto-create repos, PR creation/merge, single-fetch comment retrieval. No polling.

### Claude Code (Subscription)

Uses Claude Code CLI in non-interactive mode (`-p` flag). Key flags: `--max-turns 30`, `--output-format json`, `--append-system-prompt` (Goal injection), `--dangerously-skip-permissions` (or native sandbox after Phase 2A evaluation), `--model sonnet/opus`.

### Discord

Webhook-based notifications for: service failures (CRITICAL/HIGH), project proposals, plan limits, Goal proposals, PR review requests, task failures, auth expiry, daily summary.

---

## Goal System

One ACTIVE Goal at a time. All Pods read the Goal before work and pass it to Claude Code via `--append-system-prompt`. Operator-set Goals become ACTIVE immediately. Orchestrator(GoalAdvisor) proposals go through `PROPOSED → Operator approval`. Goals stored in `Goals/_current.md`.

---

## Usage Tracking (ccusage)

Fully delegated to [ccusage](https://github.com/ryoppippi/ccusage). Flux never parses JSONL directly.

Each task runs in a separate Git worktree → Claude Code recognizes the worktree path as a project → JSONL stored per-project → ccusage queries by `--project`. Flux pre-computes the ccusage project name by encoding the absolute path (replacing `/` and `.` with `-`).

UsageCollector snapshots hourly (tokens, costs, billing windows) into the DB. 30-day-old JSONL originals are deleted; ccusage snapshots in DB preserve history.

---

## Rate Limit Handling

Two-stage detection: (1) exit code 429, (2) stderr pattern matching (`rate limit`, `too many requests`, `429`, `capacity`, `try again`). Phase 2A experiments determine which patterns actually fire.

**Phase 2B**: Basic response — detect → stop all Pods → fixed 5-hour wait → resume.

**Phase 3**: Dynamic wait — `ccusage blocks --json` determines exact reset time, with 5-hour fallback if query fails.

No attempt to "estimate" limits. Simple stop/resume approach.

---

## Model Selection

Default Sonnet. Opus only when: no recent rate limit AND complex task (priority ≤5, operator complex work, initial-design, goal-strategy). Budget tight → reduce pods, never downgrade model.

---

## QA & Branch Strategy

### Testing

Coding tasks require passing tests. If none exist, write them. RESEARCH/DOCUMENT exempt. Max 3 retry attempts on failure.

### Git Worktree

Parallel work via bare repos + per-task worktrees. No repo locking needed.

**Cleanup policy:**
- COMPLETED + PR MERGED → delete immediately
- COMPLETED + PR pending review → preserve until merge/rejection
- FAILED → preserve 24 hours for debugging
- CHANGES_REQUESTED → reuse existing worktree

**External change detection**: Post-execution verification scans for changes outside the worktree directory. Detection → FAILED + Discord alert.

---

## PR Review

**Auto-merge conditions**: System/self tasks, maintenance, low-priority bugfixes, small PRs (≤3 files, <100 additions). Guardrail violations (>2000 diff lines or >20 files) force Operator review.

**Operator review flow**: PR appears in Web UI + Discord notification → Approve (Flux merges, worktree deleted) or Request Changes (Operator writes GitHub comments → clicks "Request Changes" in Web UI → Flux fetches comments once → creates fix task P:6 → Executor fixes in existing worktree → pushes to same PR).

No GitHub API polling. Operator triggers at their preferred timing.

---

## Orchestration

### Orchestrator Structure

Orchestrator coordinates five sub-components: ScaleManager (pod count/ratio), UsageCollector (ccusage snapshots), DailySummary (midnight Discord report), RateLimitHandler (detect → stop → dynamic wait → resume), GoalAdvisor (Goal proposals).

### Orchestrator vs Manager

**Orchestrator — "How much"**: Pod counts, model selection, rate limit response, usage collection, daily summary, Goal proposals.

**Manager — "What"**: Task queue (Priority Queue), task assignment (pop highest priority), subtask creation API (depth 1, max 5), state transitions, Goal boost, dependency checks.

### Pod Scaling

Normal operation: run up to `max_total_pods`. Rate limited: stop all, wait for billing window reset, resume.

**Executor:Researcher ratio**: 9:1 (urgent operator work) → 8:2 (normal operator work) → 7:3 (system tasks only) → 3:7 (queue nearly empty) → 0:10 (queue empty) → 10:0 (service incident).

R&D protection: minimum 20% research except during emergencies. Scaling cooldown: 15 minutes minimum between changes.

---

## Core Components

### Executor Pod

Full pipeline: request task → get model → load Goal + knowledge → worktree → Claude Code (-p, guardrails) → post-execution verification → QA → commit → diff check → PR → auto-merge or Operator review → ccusage tracking → worktree cleanup.

**Guardrails**: 30-min timeout, 10MB output limit, 30 max-turns, 2000-line diff / 20-file limit.

**Subtask decomposition**: Claude Code autonomously decides via prompt instruction. If task is too large, outputs decomposition plan as JSON → Executor parses → creates subtasks via Manager API. Depth limited to 1, max 5 per task.

### Researcher Pod

Autonomous judgment on research type and scheduling. Types include: github-scan, dependency-check, industry-research, project-ideas, opensource-scan, self-improve, library-audit, service-review, goal-research. Each uses independent workspace to prevent parallel conflicts.

### Manager

Priority levels: Service incidents (P:1-5) > Operator tasks/PR comments (P:6-20) > Maintenance (P:21-40) > Improvements (P:41-60) > Research-derived (P:61-80) > New projects (P:81-100). Goal-related tasks get boosted within their tier.

### Vault Writer

Single goroutine processes write requests from a channel sequentially. No file locking needed. Obsidian writes are ms-level, so no bottleneck.

### Obsidian Vault

```
~/ObsidianVault/Flux/
├── Goals/ (_current.md, completed/, proposals/)
├── Projects/{name}/ (_index.md, architecture.md, decisions/, learnings/)
├── Research/ (Industry/, Tools/, Ideas/, _history.md)
├── Tasks/completed/
└── Templates/
```

---

## Autonomous Operations

| Action | Autonomous | Operator Approval |
|--------|:---:|:---:|
| Goal proposal | ✅ | |
| **Goal activation** | | ✅ |
| Task creation/coding/commit | ✅ | |
| Test writing | ✅ | |
| Simple PR auto-merge | ✅ | |
| Complex PR merge | | ✅ |
| **Guardrail-exceeded PR** | | ✅ |
| PR comment resolution | ✅ | |
| Deployment | ✅ | |
| **New project** | | ✅ |
| Flux self-improvement + PR | ✅ | |
| **Flux restart** | ⏰ idle only | |
| Research/documentation | ✅ | |
| **Plan changes** | propose only | ✅ |

---

## Bootstrap

DB not found → create SQLite + schema (WAL) → Obsidian Vault structure → Notifier start → register Flux as first project → Claude Code auth check (fail → Discord alert) → Smoke Test → ccusage availability check (warning only) → .claude/settings.json → Research workspaces → Web UI start → Discord: "Flux initialized. Please set a Goal." → Operator sets Goal → Pods start (2-3).

---

## Self-Improvement

Discover improvement → worktree → tag `flux-safe-{ts}` → modify + full test suite → pass: PR + auto-merge (restart at idle) / fail: `git revert` + FAILED. **DB schema changes excluded** from self-improvement scope.

---

## Error Recovery

Mac reboot → launchd → flux → WAL SQLite recovery → RUNNING tasks → RETRY (`crash_recovery=true`, retry_count not incremented) → Notifier + Vault Writer start → Pods start → Discord alert.

**Crash recovery vs execution failure**: Crash-recovered RETRY tasks are flagged `crash_recovery=true` and don't consume retry_count. Only normal execution failures consume retries.

Daily backup: 4am, SQLite `.backup` + Vault `tar.gz`. 7-day retention.

---

## Graceful Shutdown

SIGTERM → stop new task assignment → signal running Pods "finish current task and stop" → 10-min wait → remaining Pods SIGKILL + tasks RETRY (crash_recovery=true) → DB flush, Vault Writer drain → exit. Phase 3 adds 12-min force kill refinement.

---

## Web UI

Go `embed` with React. Tailscale access. Password auth (bcrypt hash → session token → cookie, **no expiry** — Tailscale handles network-level auth, explicit logout only).

Pages: Dashboard, Goals, Tasks, PRs, Projects, Research, Usage, Settings.

Tech: React + TypeScript + Tailwind CSS + Vite, WebSocket, Zustand.

---

## Database (SQLite)

WAL mode. Tables: `goals`, `projects`, `tasks` (with `crash_recovery` boolean), `alerts`, `usage_snapshots`, `rate_limit_events`, `service_metrics`. Indexed on status+priority, project, PR status, parent task, usage type+time, service+time.

---

## API

External (`/api/...`): Goals CRUD + activate + orchestrator proposals, Tasks CRUD + cancel, PRs pending + approve + request-changes, Projects CRUD + approve/reject, Services + alerts, Usage (daily/monthly/blocks/timeseries/rate-limits), Orchestrator status + pods.

Internal (`/internal/...`): tasks/next, tasks/:id/done, subtasks, model/:task_id.

WebSocket: `/ws/events` event stream.

---

## Implementation Phases

### Phase 1: Foundation — "Skeleton"

**Goal**: Flux boots, Web UI manages Goals/Tasks/Projects via CRUD.

**Deliverable**: `go build` → single binary → Web UI CRUD.

Items: Go project init + config loader, SQLite schema + bootstrap, Goal/Task/Project CRUD API, internal API framework, Discord Notifier, GitHub client (repo creation only), Web UI (Dashboard, Goals, Tasks, Projects).

### Phase 2A: Core Pipeline — "A task becomes a PR"

**Goal**: Operator registers task → Executor codes → PR created → auto/manual merge.

**Deliverable**: Task → Claude Code → test → PR → merge pipeline.

Items: Claude Code CLI integration, JSON response parsing strategy, **Sandbox evaluation** (native sandbox compatibility test), Smoke Test, Git worktree management, GitHub PR client, Manager basic implementation, Executor Pod + guardrails, post-execution verification, QA, PR + auto-merge + Operator review, Web UI PRs page, rate limit detection **experiment**, **Claude Code Agent Teams compatibility experiment** (including OMC agent definition patterns).

**Phase 2A completion = "Flux builds Flux" transition point.**

### Phase 2B: Pipeline Hardening — "Runs reliably"

**Goal**: Safety nets, usage tracking, knowledge recording, unattended operation.

**Deliverable**: Rate limit basic response, model selection, Goal prompting, Vault recording, launchd.

Items: Rate limit detection implementation, basic rate limit response (fixed 5h wait), model selection, Goal prompt injection, subtask decomposition, Manager enhancement, Vault Writer, minimal Vault recording, ccusage project name mapping, minimal Graceful Shutdown, launchd plist registration, basic error recovery.

### Phase 3: Orchestration — "Autonomous operation"

**Goal**: Automatic Pod management, usage tracking, daily reporting.

**Deliverable**: Auto-scaling, ccusage time-series graphs, dynamic rate limit wait, Discord daily summary.

Items: Orchestrator framework + sub-components, ScaleManager, RateLimitHandler upgrade (dynamic wait), UsageCollector, time-series snapshots, per-task usage, DailySummary, JSONL cleanup, daily backup, Graceful Shutdown upgrade, data cleanup, Usage UI.

### Phase 4: Knowledge & Autonomy — "Autonomous growth"

**Goal**: Researcher autonomously researches, accumulates knowledge, Flux self-improves.

**Deliverable**: Researcher Pods, systematic Obsidian knowledge, project proposals, Flux code self-modification.

Items: Researcher Pod + workspace, autonomous research scheduling, Vault Writer upgrade, new project proposals, deployment automation, self-improvement (flux-safe + revert), Research UI.

---

## Future Work

### From Original Design

Monitor HTTP healthchecks, incident reports, Services UI, multiple Goals + ranking, REFINING state, Issue system, limit learning, gradual Pod scaling, detailed daily summary, log analysis monitoring, process/resource monitoring, watchdog process, Obsidian dashboard auto-refresh, service_metrics aggregation, JSONL gzip archiving, notification severity filters, plan change recommendations, library analysis, research quality verification, DB schema migration, PR table separation, Rate Limit 3rd-tier detection.

### From Competitive Analysis

Execution environment isolation (Sandbox), code review feedback learning, external event triggers (GitHub webhooks, Discord bot commands), CI/CD integration, multi-repo tasks, session persistence / context sharing, mobile accessibility.

### From oh-my-claudecode (OMC) Analysis

Agent definition patterns (`.claude/agents/` per task type), CLAUDE.md auto-refresh (3-tier notepad structure), native Agent Teams utilization (sub-agent parallel execution within single Executor).

**Note**: OMC is an interactive-session orchestrator and should NOT be used as a Flux plugin (dual orchestration conflict). Only ideas are borrowed.
