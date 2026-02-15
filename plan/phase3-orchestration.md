# Phase 3: Orchestration — "Autonomous operation"

## Goal

Flux manages itself: automatic Pod scaling based on workload, ccusage-based usage tracking with time-series graphs, dynamic rate limit wait using billing window data, and Discord daily summaries. The Operator only sets Goals, reviews complex PRs, and approves new projects.

## Deliverable

Auto-scaling Pods, ccusage time-series graphs in Web UI, dynamic rate limit wait, Discord daily summary, daily backups.

## Prerequisites

- Phase 2B complete (reliable pipeline with rate limit handling, Vault recording, launchd)
- ccusage installed and working

---

## Task Breakdown

### Task 3.1: Orchestrator Framework

**Description**: Implement the main Orchestrator loop that coordinates all sub-components on a 5-minute tick.

**Files to create/modify**:
```
internal/orchestrator/orchestrator.go
```

**Implementation details**:
- `Orchestrator` struct: config, manager, scaleManager, usageCollector, dailySummary, rateLimitHandler, goalAdvisor, notifier
- `Run(ctx)`: 5-minute ticker loop
- `tick()` sequence:
  1. `rateLimitHandler.CheckAndRecover()`
  2. `scaleManager.Rebalance(!rateLimitHandler.IsLimited())`
  3. `usageCollector.CollectIfDue()`
  4. `goalAdvisor.ProposeIfNeeded()`
  5. `dailySummary.SendIfDue()`
- Context cancellation stops the loop
- Each sub-component is independently testable

**Acceptance criteria**:
- [ ] Orchestrator ticks every 5 minutes
- [ ] All sub-components called in correct order
- [ ] Context cancellation stops cleanly
- [ ] Panic in one sub-component doesn't crash Orchestrator (recover + log)

**Complexity**: Medium

---

### Task 3.2: ScaleManager

**Description**: Automatic Pod count management with Executor:Researcher ratio based on workload.

**Files to create**:
```
internal/orchestrator/scale_manager.go
```

**Implementation details**:
- `ScaleManager` struct: maxPods, cooldown (15 min), lastScaleAt, minResearch (0.2), pods list
- `Rebalance(running bool)`:
  - If not running (rate limited): stop all Pods
  - If cooldown active (< 15 min since last scale): skip
  - Determine ratio based on queue state:
    - Urgent tasks (P1-5) → 9:1 Executor:Researcher
    - Operator tasks → 8:2
    - System tasks only → 7:3
    - Queue nearly empty → 3:7
    - Queue empty → 0:10 (pure research)
    - Service incident → 10:0
  - Adjust Pod counts to match target ratio
  - R&D protection: minimum 20% research except during incidents
- `adjustPods(type, count)`: start/stop Pods to reach target count
- Pod lifecycle: start goroutine → run executor/researcher loop → stop via channel

**Acceptance criteria**:
- [ ] Pod counts adjust based on queue state
- [ ] Executor:Researcher ratio follows spec
- [ ] 15-minute cooldown between scaling events
- [ ] R&D protection (min 20%) enforced
- [ ] All Pods stop during rate limit
- [ ] Pods correctly started/stopped (no leaked goroutines)

**Complexity**: High

---

### Task 3.3: RateLimitHandler Upgrade (Dynamic Wait)

**Description**: Upgrade from fixed 5-hour wait to dynamic wait using ccusage billing window data.

**Files to modify**:
```
internal/orchestrator/rate_limit_handler.go
```

**Implementation details**:
- `HandleRateLimitDynamic()`:
  1. Stop all Pods
  2. Query `ccusage blocks --json` for billing window reset time
  3. If reset time available: wait until reset + 1 minute buffer
  4. If query fails: fallback to fixed 5-hour wait
  5. Discord notification with expected resume time
  6. Resume Pods after wait

**Acceptance criteria**:
- [ ] Dynamic wait uses ccusage blocks data
- [ ] Fallback to 5-hour wait if ccusage fails
- [ ] Discord notification includes expected resume time
- [ ] Wait is cancellable via context

**Complexity**: Medium

**Depends on**: Phase 2B Task 2B.2

---

### Task 3.4: UsageCollector

**Description**: Collect usage snapshots from ccusage at configured intervals.

**Files to create**:
```
internal/orchestrator/usage_collector.go
```

**Implementation details**:
- `UsageCollector` struct: config, db, lastCollection
- `CollectIfDue()`: check if `collection_interval` (1 hour) has passed
- Collection: run `ccusage daily --json`, `ccusage blocks --json`
- Store raw JSON output in `usage_snapshots` table with type (HOURLY, BLOCKS)
- Extract summary fields: total_tokens, total_cost
- Graceful degradation: log error, skip collection if ccusage unavailable

**Acceptance criteria**:
- [ ] Hourly collection via ccusage CLI
- [ ] Raw JSON stored in usage_snapshots table
- [ ] Summary fields (tokens, cost) extracted
- [ ] ccusage failure doesn't crash Orchestrator

**Complexity**: Medium

---

### Task 3.5: Time-Series Snapshots

**Description**: Store hourly usage snapshots for historical analysis and graphing.

**Files to modify**:
```
internal/orchestrator/usage_collector.go
internal/models/usage.go
```

**Implementation details**:
- Each hourly collection creates a HOURLY snapshot in usage_snapshots
- Schema already supports this (type, data, total_tokens, total_cost, recorded_at)
- Add query methods:
  - `GetUsageTimeSeries(from, to, type) []UsageSnapshot`
  - `GetDailySummary(date) *UsageSnapshot`
  - `GetMonthlyTotal() (tokens int, cost float64)`

**Acceptance criteria**:
- [ ] Hourly snapshots accumulate in DB
- [ ] Time-series query returns correct date ranges
- [ ] Daily and monthly aggregations correct

**Complexity**: Low-Medium

**Depends on**: Task 3.4

---

### Task 3.6: Per-Task Usage Tracking

**Description**: Track ccusage per-task using worktree path encoding.

**Files to modify**:
```
internal/executor/executor.go
```

**Implementation details**:
- After task completion: `ccusage daily --project {encoded-worktree-path} --json`
- Update task record: tokens_used, cost_usd
- Path encoding: absolute path with `/` and `.` replaced by `-`
- Graceful degradation: don't block task completion

**Acceptance criteria**:
- [ ] Per-task usage collected after completion
- [ ] Tokens and cost stored on task record
- [ ] Path encoding matches ccusage project identification
- [ ] ccusage failure logged but non-blocking

**Complexity**: Low

---

### Task 3.7: DailySummary

**Description**: Send Discord daily summary at midnight.

**Files to create**:
```
internal/orchestrator/daily_summary.go
```

**Implementation details**:
- `DailySummary` struct: config (daily_summary_hour=0), notifier, db, lastSent
- `SendIfDue()`: check if current hour matches config and hasn't sent today
- Summary content:
  - Tasks completed today (count, list)
  - Tasks failed (count, issues)
  - PRs merged / pending review
  - Total tokens/cost for the day
  - Active Goal progress
  - Current Pod configuration
  - Rate limit events (if any)
- Format as Discord embed-friendly message

**Acceptance criteria**:
- [ ] Summary sent at configured hour (midnight)
- [ ] Sent exactly once per day
- [ ] Includes all specified metrics
- [ ] Readable format in Discord

**Complexity**: Medium

---

### Task 3.8: JSONL Cleanup

**Description**: Delete old Claude Code JSONL files (>30 days) since usage data is preserved in DB snapshots.

**Files to create**:
```
internal/cleanup/cleanup.go
```

**Implementation details**:
- `CleanOldJSONL(retentionDays int)`:
  - Scan `~/.claude/projects/` and `~/.config/claude/projects/` (both possible paths)
  - Find `.jsonl` files older than retention period
  - Delete them
- Run daily (triggered by Orchestrator or daily summary)

**Acceptance criteria**:
- [ ] Old JSONL files deleted
- [ ] Both possible paths checked
- [ ] Retention period configurable
- [ ] No deletion of recent files

**Complexity**: Low

---

### Task 3.9: Daily Backup

**Description**: Automated daily backups of SQLite database and Obsidian Vault.

**Files to create/modify**:
```
internal/cleanup/backup.go
```

**Implementation details**:
- `RunDailyBackup(dbPath, vaultPath, backupDir string)`:
  - SQLite: use `.backup` command for consistent copy
  - Vault: `tar.gz` of entire vault directory
  - Backup naming: `flux-db-{date}.bak`, `flux-vault-{date}.tar.gz`
  - Retention: delete backups older than 7 days
- Run at configured time (4am via Orchestrator scheduling)

**Acceptance criteria**:
- [ ] SQLite backup via .backup command
- [ ] Vault tar.gz created
- [ ] 7-day retention enforced
- [ ] Backup runs at configured time
- [ ] Backup directory auto-created

**Complexity**: Medium

---

### Task 3.10: Graceful Shutdown Upgrade

**Description**: Upgrade shutdown to use Phase 3's 12-minute force kill (from Phase 2B's 10-minute).

**Files to modify**:
```
internal/shutdown/shutdown.go
```

**Implementation details**:
- Two-stage timeout: `pod_grace_period` (10 min) → `force_kill_after` (12 min)
- After 10 min: send SIGKILL to remaining Pods
- After 12 min: absolute force kill (process.Kill())
- Task cleanup: RUNNING → RETRY with crash_recovery=true

**Acceptance criteria**:
- [ ] Two-stage timeout implemented
- [ ] Force kill at 12 minutes
- [ ] Tasks correctly transitioned
- [ ] No process leaks

**Complexity**: Low (incremental from Phase 2B)

**Depends on**: Phase 2B Task 2B.10

---

### Task 3.11: Data Cleanup

**Description**: Periodic cleanup of old metrics, snapshots, and backup files.

**Files to modify**:
```
internal/cleanup/cleanup.go
```

**Implementation details**:
- `RunDataCleanup(cfg CleanupConfig)`:
  - `service_metrics`: delete raw data older than 7 days
  - `usage_snapshots`: delete older than 90 days
  - Backup files: delete older than 7 days
  - Failed worktrees: delete older than 24 hours
- Run daily via Orchestrator

**Acceptance criteria**:
- [ ] Old metrics cleaned up
- [ ] Retention periods match config
- [ ] No deletion of recent data
- [ ] Cleanup runs without errors

**Complexity**: Low

---

### Task 3.12: Usage UI

**Description**: Add Usage page to Web UI with time-series charts.

**Files to create**:
```
web/src/pages/Usage.tsx
web/src/stores/usageStore.ts
web/src/components/charts/TimeSeriesChart.tsx
```

**Files to modify**:
```
internal/server/api_usage.go (new file)
```

**API endpoints**:
```
GET /api/usage/daily           — Daily token/cost
GET /api/usage/monthly         — Monthly aggregation
GET /api/usage/blocks          — Billing window
GET /api/usage/timeseries      — Time-series data (?from=&to=&type=HOURLY)
GET /api/usage/rate-limits     — Rate limit event history
```

**UI features**:
- Time-series chart: tokens and cost over time (selectable range)
- Daily summary: today's usage
- Monthly total
- Rate limit events timeline
- Billing window status

**Acceptance criteria**:
- [ ] Usage API returns correct data
- [ ] Time-series chart renders
- [ ] Date range selection works
- [ ] Rate limit history displayed
- [ ] Monthly/daily aggregations correct

**Complexity**: High

---

### Task 3.13: GoalAdvisor

**Description**: Orchestrator sub-component that proposes new Goals based on system state.

**Files to create**:
```
internal/orchestrator/goal_advisor.go
```

**Implementation details**:
- `GoalAdvisor` struct: config, manager, notifier
- `ProposeIfNeeded()`: check conditions for proposing a new Goal
  - No ACTIVE Goal → propose based on project state and research findings
  - ACTIVE Goal near completion → propose follow-up
- Create PROPOSED Goal (requires Operator approval to activate)
- Discord notification for proposals
- Phase 3 basic: simple heuristic-based proposals
- Phase 4: Claude Code-powered intelligent proposals

**Acceptance criteria**:
- [ ] Goal proposed when no active Goal exists
- [ ] Proposed Goal requires Operator activation
- [ ] Discord notification sent
- [ ] No duplicate proposals

**Complexity**: Medium

---

## Phase 3 Completion Checklist

- [ ] Orchestrator runs with 5-minute tick cycle
- [ ] Pod scaling adjusts automatically based on workload
- [ ] Executor:Researcher ratio follows spec
- [ ] Dynamic rate limit wait uses ccusage billing data
- [ ] Hourly usage snapshots in DB
- [ ] Per-task usage tracking
- [ ] Daily Discord summary at midnight
- [ ] JSONL cleanup (30-day retention)
- [ ] Daily backup (SQLite + Vault, 7-day retention)
- [ ] Data cleanup (metrics, snapshots)
- [ ] Usage UI with time-series graphs
- [ ] Goal proposal system active

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~8 files | ~5 files |
| React frontend | ~3 files | ~2 files |
| **Total** | **~11 files** | **~7 files** |
