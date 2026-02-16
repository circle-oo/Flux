# Phase 3: Orchestration — "Autonomous operation"

## Goal

Flux manages itself: automatic Pod scaling based on workload, ccusage-based usage tracking with time-series graphs, dynamic rate limit wait using billing window data, and Discord daily summaries. The Operator only sets Goals, reviews complex PRs, and approves new projects.

**Extended Goal (with Insight + Obsidian)**: Flux provides comprehensive operational intelligence through analytics dashboards and integrates with Obsidian for knowledge management, enabling deep observability and research capabilities.

## Deliverable

**Core**: Auto-scaling Pods, ccusage time-series graphs in Web UI, dynamic rate limit wait, Discord daily summary, daily backups.

**Insight**: Comprehensive analytics page with time-series charts, efficiency metrics, pipeline health, failure analysis, and enhanced Dashboard.

**Obsidian**: Full Obsidian CLI integration with Knowledge page (Browse, Search, Research, Graph, Health tabs), VaultFacade with graceful fallback, and 15 Knowledge API endpoints.

## Prerequisites

- Phase 2B complete (reliable pipeline with rate limit handling, Vault recording, launchd)
- ccusage installed and working

---

## Revision Notes (from Phase 3/4 Plan Refinement)

This plan was refined after auditing the Phase 2B codebase. Key changes from the original:

1. **Task 3.1 (Orchestrator Framework)**: Significantly reworked. The current `Orchestrator` struct (`orchestrator.go:14-17`) is a Phase 2B stub with only `config` and `rateLimitHandler` fields — no tick loop, no sub-component coordination. The original plan understated the wiring work: `main.go:128-139` currently starts executors directly (not via ScaleManager), so Phase 3 must also refactor the executor lifecycle in `main.go` to be managed by the Orchestrator.

2. **Task 3.2 (ScaleManager)**: Revised to account for the existing pod registry (`server/pod_registry.go`) and the fact that executors are currently started as goroutines in `main.go:132-139`. ScaleManager must integrate with the existing `PodRegistry` for status visibility. Also removed "Researcher" pods from Phase 3 scope (deferred to Phase 4) to reduce complexity — Phase 3 ScaleManager only manages Executor pod count.

3. **Task 3.5 (Time-Series Queries)**: The `models/usage.go:28-78` already has `UsageStore` with `CreateSnapshot` and `ListSnapshots`. Revised to clarify this task adds time-range query methods, not new schema.

4. **Task 3.6 (Per-Task Usage)**: The `executor/ccusage.go` already implements per-task usage collection from Phase 2B.9. Revised to clarify this task is about ensuring async collection is production-ready and integrated, not reimplementation.

5. **Task 3.8 (JSONL Cleanup)**: Changed retention from 30 days to 7 days (matching spec) for sensitive JSONL data. Removed "secure deletion" (zero-fill) — unnecessary on macOS with APFS.

6. **Task 3.12 (Usage UI)**: Updated file paths from `web/src/` to `frontend/src/` matching actual project structure. The spec references `web/` but the actual codebase uses `frontend/`.

7. **Task 3.13 (GoalAdvisor)**: Downgraded from standalone task to optional enhancement. For Phase 3, simple heuristics suffice and can be folded into the Orchestrator tick as a lightweight check.

8. **Task 3.14 (Orchestrator API)**: The `/api/pods` endpoint already exists (`server/api_pods.go`). Revised to only add missing endpoints (`/api/orchestrator/status`, usage endpoints).

9. **Task 3.15 (Disk Space)**: Worktree manager is in `executor/worktree.go` (not `internal/worktree/manager.go` as originally stated). File paths corrected.

10. **Task 3.16 (Settings UI)**: `frontend/src/pages/Settings.tsx` already exists. Revised to enhance existing page rather than create new one.

11. **New Task 3.0 added**: Orchestrator integration in `main.go` — refactoring executor lifecycle to be managed by the Orchestrator is a prerequisite for everything else.

---

## Task Breakdown

### Task 3.0: Orchestrator Integration in main.go

**Description**: Refactor `main.go` so the Orchestrator owns the executor lifecycle instead of `main()` directly spawning executor goroutines. This is the prerequisite wiring task that makes all other Phase 3 tasks possible.

**Files to modify**:
```
cmd/flux/main.go
internal/orchestrator/orchestrator.go
```

**Implementation details**:
- Currently `main.go:128-139` creates executors directly and spawns goroutines
- Refactor: `main.go` creates the full Orchestrator, passes it the Manager, Notifier, VaultWriter, and Config
- Orchestrator.Run(ctx) replaces the direct executor goroutine spawning
- Orchestrator internally creates executors and manages their lifecycle
- This preserves the existing executor logic but moves lifecycle control to the Orchestrator
- The Orchestrator struct (`orchestrator.go:14-17`) must be expanded to hold: manager, notifier, vaultWriter, executors, and all Phase 3 sub-components

**Acceptance criteria**:
- [ ] Executors are created and managed by Orchestrator, not main.go
- [ ] Orchestrator.Run(ctx) starts executors and the tick loop
- [ ] Graceful shutdown still works (context cancellation propagates)
- [ ] Pod registration still works (executors still register with PodRegistry)
- [ ] Existing behavior preserved — no functional changes yet

**Complexity**: Medium

---

### Task 3.1: Orchestrator Tick Loop

**Description**: Implement the 5-minute tick loop that coordinates all sub-components. Build on the refactored Orchestrator from Task 3.0.

**Files to modify**:
```
internal/orchestrator/orchestrator.go
```

**Implementation details**:
- Add `Run(ctx)` method with 5-minute ticker (from `config.Orchestrator.CheckInterval`)
- `tick()` calls sub-components in order:
  1. `rateLimitHandler.CheckAndRecover()` — already implemented (`rate_limit_handler.go:77-96`)
  2. `scaleManager.Rebalance()` — Task 3.2
  3. `usageCollector.CollectIfDue()` — Task 3.4
  4. `goalAdvisor.ProposeIfNeeded()` — Task 3.13 (optional)
  5. `dailySummary.SendIfDue()` — Task 3.7
  6. `cleanupIfDue()` — Tasks 3.8, 3.9, 3.11
- Each sub-component call wrapped in `recover()` to prevent panic propagation
- Sub-components that don't exist yet should be nil-checked (progressive integration)
- Context cancellation stops the loop

**Acceptance criteria**:
- [ ] Orchestrator ticks every 5 minutes (configurable)
- [ ] All sub-components called in correct order
- [ ] Context cancellation stops cleanly
- [ ] Panic in one sub-component doesn't crash Orchestrator (recover + log)
- [ ] Nil sub-components safely skipped
- [ ] Unit tests verify panic recovery and nil-safety

**Complexity**: Medium

**Depends on**: Task 3.0

---

### Task 3.2: ScaleManager (Executor-only)

**Description**: Automatic Executor Pod count management based on workload. Phase 3 scope: Executor pods only. Researcher pod ratio logic deferred to Phase 4.

**Files to create**:
```
internal/orchestrator/scale_manager.go
```

**Implementation details**:
- `ScaleManager` struct: config, manager, maxPods, cooldown (15 min), lastScaleAt, executors []*executor.Executor, mu sync.RWMutex
- `Rebalance(running bool)`:
  - If `!running` (rate limited): stop all Executor Pods
  - If cooldown active (< 15 min since last scale): skip
  - Determine target executor count based on queue depth:
    - Urgent tasks (P1-5) or Operator tasks → maxPods
    - System tasks with moderate queue → max(1, maxPods * 0.7)
    - Queue nearly empty (1-2 tasks) → 1
    - Queue empty → 0 (save resources; Phase 4 adds Researcher pods here)
  - Start/stop executors to match target
- Executor lifecycle: each executor runs as goroutine, stop via Stop() method + context
- Integrate with existing PodRegistry via executor's self-registration (no changes needed)
- **Key difference from original plan**: No Researcher ratio logic — Phase 3 only manages Executor count. This avoids building Researcher infrastructure before it's needed.

**Acceptance criteria**:
- [ ] Pod count adjusts based on queue state
- [ ] 15-minute cooldown between scaling events
- [ ] All Pods stop during rate limit
- [ ] Pods correctly started/stopped (no leaked goroutines)
- [ ] PodRegistry reflects current pod state
- [ ] Unit tests for scaling logic

**Complexity**: High

**Depends on**: Task 3.0

---

### Task 3.3: RateLimitHandler Upgrade (Dynamic Wait)

**Description**: Upgrade from fixed 5-hour wait to dynamic wait using ccusage billing window data. Current implementation (`rate_limit_handler.go:42-73`) uses a fixed `RateLimitDuration = 5 * time.Hour`.

**Files to modify**:
```
internal/orchestrator/rate_limit_handler.go
```

**Implementation details**:
- Add `HandleRateLimitDynamic(ctx context.Context)`:
  1. Query `ccusage blocks --json` for billing window reset time
  2. Parse JSON response for next reset timestamp
  3. If reset time available: set `rateLimitUntil` to reset + 1 minute buffer
  4. If query fails: fallback to existing fixed 5-hour wait
  5. Discord notification includes expected resume time (already done in current code)
- Modify `HandleRateLimit()` to call `HandleRateLimitDynamic()` first, fall back to fixed
- `rateLimitUntil` is already used for time-based recovery (`rate_limit_handler.go:85-88`)
- Add ccusage command execution helper (shell out to `ccusage blocks --json`)

**Acceptance criteria**:
- [ ] Dynamic wait uses ccusage blocks data when available
- [ ] Fallback to 5-hour wait if ccusage fails
- [ ] Discord notification includes expected resume time
- [ ] Existing CheckAndRecover() logic unchanged
- [ ] Unit tests with mock ccusage output

**Complexity**: Medium

**Depends on**: Phase 2B (already complete)

---

### Task 3.4: UsageCollector

**Description**: Collect usage snapshots from ccusage at configured intervals and store in the `usage_snapshots` table.

**Files to create**:
```
internal/orchestrator/usage_collector.go
```

**Implementation details**:
- `UsageCollector` struct: config (CCUsageConfig), db, lastCollection time.Time
- `CollectIfDue()`: check if `collection_interval` (default 1 hour) has passed since lastCollection
- Collection sequence:
  1. Run `ccusage daily --json` → store as HOURLY snapshot
  2. Run `ccusage blocks --json` → store as BLOCKS snapshot
- Use existing `models.UsageStore.CreateSnapshot()` (`models/usage.go:41-62`) to persist
- Extract `total_tokens` and `total_cost` from ccusage JSON for quick-query fields
- Graceful degradation: log error, skip collection if ccusage unavailable
- First collection on startup (don't wait for first interval)

**Acceptance criteria**:
- [ ] Hourly collection via ccusage CLI
- [ ] Raw JSON stored in usage_snapshots table via existing UsageStore
- [ ] Summary fields (tokens, cost) extracted from JSON
- [ ] ccusage failure doesn't crash Orchestrator
- [ ] Unit tests with mock ccusage output

**Complexity**: Medium

---

### Task 3.5: Time-Series Query Methods

**Description**: Add time-range query methods to UsageStore for time-series access, powering the Usage UI.

**Files to modify**:
```
internal/models/usage.go
```

**Implementation details**:
- The existing `UsageStore` (`models/usage.go:28-78`) has `CreateSnapshot` and `ListSnapshots` (list by type)
- Add query methods:
  - `GetUsageTimeSeries(from, to time.Time, snapshotType string) ([]UsageSnapshot, error)` — range query with type filter
  - `GetDailySummary(date time.Time) (*UsageSnapshot, error)` — latest HOURLY for a given date
  - `GetMonthlyTotal() (totalTokens int, totalCost float64, err error)` — SUM aggregation for current month
  - `GetLatestBlocks() (*UsageSnapshot, error)` — most recent BLOCKS snapshot
- All queries use the existing `idx_usage_snapshots_type_time` index (`db/schema.go:112`)

**Acceptance criteria**:
- [ ] Time-series query returns correct date ranges
- [ ] Daily and monthly aggregations correct
- [ ] Queries use existing index
- [ ] Unit tests for each query method

**Complexity**: Low

**Depends on**: Task 3.4

---

### Task 3.6: Per-Task Usage Integration (Production Hardening)

**Description**: Ensure per-task usage collection (already implemented in `executor/ccusage.go`) is production-ready with async, non-blocking behavior.

**Files to modify**:
```
internal/executor/ccusage.go (if needed)
internal/executor/executor.go (if needed)
```

**Implementation details**:
- `executor/ccusage.go` already implements per-task ccusage collection from Phase 2B.9
- This task validates:
  1. Collection is truly async (fire-and-forget goroutine, not blocking task completion)
  2. Path encoding matches ccusage project identification
  3. Tokens and cost are written back to task record
  4. ccusage failures are logged but non-blocking
- If already working correctly, mark as verified-complete
- If gaps found, fix them

**Acceptance criteria**:
- [ ] Per-task usage collected after completion (async, non-blocking)
- [ ] Tokens and cost stored on task record
- [ ] Path encoding verified
- [ ] ccusage failure logged but non-blocking
- [ ] Integration test confirming non-blocking behavior

**Complexity**: Low (verification + minor fixes)

---

### Task 3.7: DailySummary

**Description**: Send Discord daily summary at a configured hour.

**Files to create**:
```
internal/orchestrator/daily_summary.go
```

**Implementation details**:
- `DailySummary` struct: config (DailySummaryHour from OrchestratorConfig), notifier, db, lastSentDate string
- `SendIfDue()`:
  - Check if current hour matches `config.Orchestrator.DailySummaryHour` (default 0 = midnight)
  - Check if `lastSentDate` differs from today's date (prevent duplicate sends)
  - If due, collect and send summary
- Summary content (queried from DB):
  - Tasks completed today (count)
  - Tasks failed (count)
  - PRs merged / pending review (from task records with pr_status)
  - Total tokens/cost for the day (from usage_snapshots)
  - Active Goal title (from GoalStore)
  - Rate limit events today (from rate_limit_events)
- Format as Discord embed-friendly message (use existing notifier.Send)
- Persist `lastSentDate` in memory only (re-sends acceptable after restart)

**Acceptance criteria**:
- [ ] Summary sent at configured hour
- [ ] Sent exactly once per day (date check)
- [ ] Includes all specified metrics
- [ ] Readable format in Discord
- [ ] Unit tests for summary generation

**Complexity**: Medium

---

### Task 3.8: JSONL Cleanup

**Description**: Delete old Claude Code JSONL files to save disk space. Usage data is preserved in DB snapshots.

**Files to create**:
```
internal/orchestrator/cleanup.go
```

**Implementation details**:
- `CleanOldJSONL(retentionDays int)`:
  - Scan `~/.claude/projects/` for `.jsonl` files
  - Delete files older than `retentionDays` (default 7 from `config.Cleanup.JSONLRetentionDays`)
  - Simple `os.Remove()` — no zero-fill needed on APFS
- Called from Orchestrator tick, daily (check date to avoid repeated work)
- Log deletions at Info level

**Acceptance criteria**:
- [ ] Old JSONL files deleted after retention period
- [ ] Retention period from config
- [ ] No deletion of recent files
- [ ] Errors logged but non-fatal

**Complexity**: Low

---

### Task 3.9: Daily Backup

**Description**: Automated daily backups of SQLite database and Obsidian Vault.

**Files to modify**:
```
internal/orchestrator/cleanup.go (add backup function)
```

**Implementation details**:
- `RunDailyBackup(dbPath, vaultPath, backupDir string)`:
  - SQLite: use `sqlite3 .backup` command for consistent copy (or `VACUUM INTO` if available)
  - Vault: `tar -czf` of entire vault directory
  - Backup naming: `flux-db-{YYYY-MM-DD}.bak`, `flux-vault-{YYYY-MM-DD}.tar.gz`
  - Retention: delete backups older than `config.Database.BackupRetentionDays` (default 7)
  - Auto-create backup directory
- Triggered from Orchestrator tick at configured hour (e.g., 4am)

**Acceptance criteria**:
- [ ] SQLite backup created consistently
- [ ] Vault tar.gz created
- [ ] Retention enforced
- [ ] Backup directory auto-created
- [ ] Errors logged but non-fatal

**Complexity**: Medium

---

### Task 3.10: Graceful Shutdown Upgrade

**Description**: Upgrade shutdown to support Phase 3's two-stage timeout (10 min grace → 12 min force kill).

**Files to modify**:
```
internal/shutdown/shutdown.go
```

**Implementation details**:
- Current implementation (`shutdown.go:21-68`) already has grace period support
- Verify existing `PodGracePeriod` and `ForceKillAfter` config values are used
- If the current implementation already supports two-stage timeout via config, mark as verified-complete
- If not, add second timer: after `ForceKillAfter`, absolute `process.Kill()` on any remaining processes
- Task cleanup: RUNNING → RETRY with crash_recovery=true (already implemented in `recovery.go:14-78`)

**Acceptance criteria**:
- [ ] Two-stage timeout implemented (configurable durations)
- [ ] Force kill at ForceKillAfter
- [ ] Tasks correctly transitioned
- [ ] No process leaks

**Complexity**: Low (likely already implemented via config)

**Depends on**: Phase 2B (already complete)

---

### Task 3.11: Data Cleanup

**Description**: Periodic cleanup of old metrics, snapshots, and failed worktrees.

**Files to modify**:
```
internal/orchestrator/cleanup.go
```

**Implementation details**:
- `RunDataCleanup(cfg config.CleanupConfig, db *sql.DB)`:
  - `service_metrics`: DELETE WHERE recorded_at < now - `ServiceMetricsRawDays` (7)
  - `usage_snapshots`: DELETE WHERE recorded_at < now - `UsageSnapshotsDays` (90)
  - Failed worktrees: call existing worktree cleanup in `executor/worktree.go` for trees older than `FailedWorktreeHours` (24)
  - Backup files: delete older than `BackupRetentionDays`
- Run daily via Orchestrator tick (date-gated like JSONL cleanup)

**Acceptance criteria**:
- [ ] Old metrics cleaned up per config
- [ ] Retention periods from config.Cleanup
- [ ] No deletion of recent data
- [ ] Cleanup runs without errors
- [ ] Unit tests for retention logic

**Complexity**: Low

---

### Task 3.12: Usage API Endpoints

**Description**: HTTP API endpoints for usage data access, powering the Usage UI.

**Files to create**:
```
internal/server/api_usage.go
```

**API endpoints**:
```
GET /api/usage/daily           — Today's usage (latest HOURLY snapshot)
GET /api/usage/monthly         — Monthly aggregation (SUM tokens/cost)
GET /api/usage/blocks          — Latest billing window (BLOCKS snapshot)
GET /api/usage/timeseries      — Time-series data (?from=&to=&type=HOURLY)
GET /api/usage/rate-limits     — Rate limit event history
```

**Implementation details**:
- Each endpoint calls the query methods from Task 3.5
- Register routes in `server.go` route setup (after existing routes, around line 174)
- Follow existing patterns: `writeJSON`, `writeError`, auth middleware
- Pagination for timeseries if result set is large

**Acceptance criteria**:
- [ ] All 5 endpoints return correct data
- [ ] Consistent with existing API patterns
- [ ] Auth middleware applied
- [ ] Unit tests for each endpoint

**Complexity**: Medium

**Depends on**: Task 3.5

---

### Task 3.13: Usage UI

**Description**: Add Usage page to Web UI with time-series display.

**Files to create**:
```
frontend/src/pages/Usage.tsx
frontend/src/stores/usageStore.ts
```

**Implementation details**:
- Create Zustand store (`usageStore.ts`) with API calls to usage endpoints
- Usage.tsx page:
  - Daily summary: today's token count and cost (from `/api/usage/daily`)
  - Monthly total: tokens and cost (from `/api/usage/monthly`)
  - Billing window status (from `/api/usage/blocks`)
  - Rate limit events list (from `/api/usage/rate-limits`)
  - Time-series table/list view for hourly snapshots (from `/api/usage/timeseries`)
- Use simple table/list display for time-series (no charting library in Phase 3 — keep dependencies minimal)
- Add to App.tsx navigation

**Acceptance criteria**:
- [ ] Usage page accessible from nav
- [ ] Daily/monthly/blocks data displayed
- [ ] Rate limit history displayed
- [ ] Time-series data viewable
- [ ] Responsive layout (Tailwind)

**Complexity**: Medium

**Depends on**: Task 3.12

---

### Task 3.14: Orchestrator API Endpoints

**Description**: HTTP API endpoints for Orchestrator status and Pod management.

**Files to create**:
```
internal/server/api_orchestrator.go
```

**Implementation details**:
- `GET /api/orchestrator/status` — Returns:
  - active_pods (count)
  - max_pods (from config)
  - rate_limited (bool)
  - rate_limit_until (timestamp, if limited)
  - last_tick (timestamp)
  - queue_depth (pending + ready task count)
- `POST /api/orchestrator/propose-goals` — Trigger GoalAdvisor manually (if implemented)
- Note: `GET /api/pods` already exists (`server/api_pods.go:11-16`) — no changes needed
- Note: `GET /api/services` and `GET /api/alerts` are Phase 1 stubs — upgrade to return real data if service monitoring is configured, otherwise keep stubs

**Acceptance criteria**:
- [ ] Orchestrator status endpoint returns scaling state
- [ ] Orchestrator struct exposes status methods for the API
- [ ] Unit tests for endpoints

**Complexity**: Low-Medium

**Depends on**: Task 3.1

---

### Task 3.15: Disk Space Monitoring

**Description**: Monitor disk space and prevent worktree creation when low on disk.

**Files to modify**:
```
internal/orchestrator/orchestrator.go (add disk check to tick)
internal/executor/worktree.go (add pre-creation check)
```

**Implementation details**:
- Add `checkDiskSpace()` to Orchestrator tick:
  - Use `syscall.Statfs` on macOS to get free space
  - WARNING at 10GB free → Discord notification (once per day)
  - CRITICAL at 2GB free → Discord critical alert
- Add disk space check in `executor/worktree.go` before `CreateWorktree()`:
  - Block worktree creation at 5GB free → return error (task goes to RETRY)
- Force cleanup at 1GB: trigger immediate JSONL cleanup + failed worktree cleanup

**Acceptance criteria**:
- [ ] Disk space checked every tick
- [ ] WARNING/CRITICAL alerts sent to Discord (not spammy)
- [ ] Worktree creation blocked when space critical
- [ ] Unit tests for threshold logic

**Complexity**: Low

---

### Task 3.16: Settings UI Enhancement + Metrics Endpoint

**Description**: Enhance existing Settings page and add a metrics endpoint.

**Files to modify**:
```
frontend/src/pages/Settings.tsx (already exists — enhance)
```

**Files to create**:
```
internal/server/api_metrics.go
```

**Implementation details**:

**api_metrics.go**:
- `GET /api/metrics`: returns JSON with:
  - active_pods (count by type)
  - queue_depth (count by status)
  - task_completion_rate (completed / total last 24h)
  - disk_usage_bytes (free/total)
  - rate_limit_status (limited, until)
  - current_goal (id, title, status)

**Settings.tsx enhancement**:
- Add sections: Pod status, System health (disk, rate limit), Queue depth
- Pull data from `/api/metrics` and `/api/orchestrator/status`
- Read-only display for Phase 3

**Acceptance criteria**:
- [ ] Metrics endpoint returns all specified data
- [ ] Settings page shows system health
- [ ] Pod status displayed
- [ ] Disk usage visible

**Complexity**: Medium

---

### Task 3.17: GoalAdvisor (Optional)

**Description**: Lightweight heuristic-based Goal proposal system. This is an optional enhancement that can be deferred without blocking Phase 3 completion.

**Files to create**:
```
internal/orchestrator/goal_advisor.go
```

**Implementation details**:
- `GoalAdvisor` struct: config, manager, notifier, lastProposalDate
- `ProposeIfNeeded()`: simple heuristics (no Claude Code):
  - Heuristic 1: No active Goal + queue empty for >1 hour → propose "Plan next development cycle"
  - Heuristic 2: Active Goal with >80% tasks completed → propose follow-up
  - Heuristic 3: No operator tasks in 48 hours → propose "Review and update goals"
- Create Goal with status=PROPOSED, source=ORCHESTRATOR
- Discord notification for proposals
- Max 1 proposal per day (prevent spam)

**Acceptance criteria**:
- [ ] Goal proposed when heuristic conditions met
- [ ] Proposed Goal requires Operator activation
- [ ] No duplicate proposals (daily limit)
- [ ] Discord notification sent

**Complexity**: Medium

---

### Task 3.18: Insight — Backend Query Methods

**Description**: Add comprehensive analytics query methods for insight data aggregation. Provides the data foundation for the Insight analytics page.

**Files to create**:
```
internal/models/insights.go
```

**Implementation details**:
- Create `InsightStore` (or extend `TaskStore`) with 5 query methods:
  - `GetSummary() (*InsightSummary, error)` — all-time and today's aggregates from tasks table (total tokens, cost, task counts by status, PR stats, project activities)
  - `GetTimeSeries(period, bucket string) ([]TimeSeriesBucket, error)` — time-bucketed metrics using `GROUP BY strftime()` for daily/hourly buckets
  - `GetEfficiency() (*EfficiencyReport, error)` — cost/tokens per task, breakdowns by type/model/priority
  - `GetPipelineHealth() (*PipelineHealth, error)` — queue depth, throughput, latency (using julianday calculations), retry stats, decomposition stats
  - `GetFailureAnalysis(limit int) (*FailureAnalysis, error)` — failure patterns, error categorization via keyword matching on error_log field
- All queries run against existing tables (tasks, usage_snapshots, rate_limit_events) — no schema changes
- Use existing indexes: `idx_tasks_status_priority`, `idx_tasks_project`
- Null-safe handling with COALESCE for empty results
- Error categorization: "build" → build_failure, "test" → test_failure, "rate limit" → rate_limit, "timeout" → timeout, else → other

**Acceptance criteria**:
- [ ] All 5 query methods return correct results
- [ ] Queries use existing indexes
- [ ] Null-safe handling (COALESCE for empty results)
- [ ] Unit tests with test database

**Complexity**: Medium

**Depends on**: Phase 2B (tasks table with all fields)

---

### Task 3.19: Insight — Backend API Endpoints

**Description**: HTTP API endpoints for comprehensive analytics data, replacing the minimal `/api/insights` endpoint.

**Files to create**:
```
internal/server/api_insights.go
```

**Files to modify**:
```
internal/server/server.go (register routes)
```

**Implementation details**:
- Implement 5 HTTP handlers:
  - `GET /api/insights/summary` — replaces current `/api/insights` with richer aggregations
  - `GET /api/insights/timeseries?period=7d&bucket=daily` — time-bucketed metrics for charting
  - `GET /api/insights/efficiency` — efficiency and resource metrics
  - `GET /api/insights/pipeline` — task pipeline health metrics
  - `GET /api/insights/failures` — failure analysis for debugging
- Keep existing `GET /api/insights` for backward compatibility (mark as deprecated)
- Register routes in `server.go` setupRoutes following existing patterns with auth middleware
- Query parameters validated (period: 24h/7d/30d/90d, bucket: hourly/daily)

**Acceptance criteria**:
- [ ] All 5 endpoints return correct JSON
- [ ] Query parameters validated
- [ ] Auth middleware applied
- [ ] Existing `/api/insights` still works (backward compatible)
- [ ] Unit tests for handlers

**Complexity**: Medium

**Depends on**: Task 3.18

---

### Task 3.20: Insight — Frontend Store & API Client

**Description**: TypeScript interfaces and Zustand store for insight data management in the Web UI.

**Files to create**:
```
frontend/src/stores/insightStore.ts
```

**Files to modify**:
```
frontend/src/lib/api.ts (add API methods)
```

**Implementation details**:
- Add TypeScript interfaces matching API response shapes (InsightSummary, TimeSeriesBucket, EfficiencyReport, PipelineHealth, FailureAnalysis)
- Create API methods for all 5 insight endpoints
- Create Zustand store with:
  - State for each data section (summary, timeseries, efficiency, pipeline, failures)
  - Fetch methods with loading/error states
  - Period/bucket selection state (24h, 7d, 30d, 90d)
  - Auto-refresh on period change

**Acceptance criteria**:
- [ ] TypeScript interfaces match API response shapes
- [ ] API methods for all 5 endpoints
- [ ] Zustand store with loading/error states
- [ ] Period selector state management

**Complexity**: Low

**Depends on**: Task 3.19

---

### Task 3.21: Insight — Frontend Insight Page

**Description**: Dedicated analytics page with comprehensive visualizations using recharts library.

**Files to create**:
```
frontend/src/pages/Insight.tsx
```

**Files to modify**:
```
frontend/src/App.tsx (add route)
frontend/src/components/Sidebar.tsx (add nav)
```

**Implementation details**:
- Install `recharts` npm package for chart visualizations
- Build Insight page with 5 sections:
  1. Summary cards row (today's stats + all-time stats with trend indicators ↑↓)
  2. Time-series chart area (task completion trend, cost trend using BarChart/LineChart)
  3. Efficiency breakdown tables (by type, model, priority)
  4. Pipeline health panel (queue depth, throughput, latency metrics)
  5. Failure analysis section (recent failures list, error categories, failure rate by project)
- Period selector buttons at top: 24h / 7d / 30d / 90d
- Add navigation link in sidebar
- Responsive layout (Tailwind, dark theme consistent with existing pages)

**Acceptance criteria**:
- [ ] Insight page renders all 5 sections
- [ ] Charts display time-series data correctly
- [ ] Period selector updates all sections
- [ ] Responsive layout (mobile-friendly)
- [ ] Consistent styling with existing pages
- [ ] Navigation link in sidebar

**Complexity**: High

**Depends on**: Task 3.20

---

### Task 3.22: Insight — Dashboard Enhancement

**Description**: Enhance existing Dashboard with compact analytics summary and link to full Insight page.

**Files to modify**:
```
frontend/src/pages/Dashboard.tsx
```

**Implementation details**:
- Replace current flat Insights section (`Dashboard.tsx:270-328`) with:
  - Compact summary cards from `/api/insights/summary` (today's completions, cost, success rate)
  - Trend indicators (↑↓) comparing today to yesterday (from timeseries data if available)
  - "View Details →" link to the Insight page
- Keep it minimal — Dashboard should be a quick glance, not a full analytics view

**Acceptance criteria**:
- [ ] Dashboard shows today's key metrics from summary endpoint
- [ ] Trend indicators for day-over-day comparison
- [ ] Link to full Insight page
- [ ] No regression in existing dashboard functionality

**Complexity**: Low

**Depends on**: Task 3.19, Task 3.21

---

### Task 3.23: Obsidian — Backend CLI Client

**Description**: Implement Obsidian CLI wrapper to replace direct filesystem I/O with Obsidian-native operations (search, frontmatter, backlinks, tags, daily notes).

**Files to create**:
```
internal/vault/client.go
internal/vault/client_test.go
```

**Implementation details**:
- `ObsidianClient` struct: vaultName, timeout (30s), retryCount (3), retryDelay (2s)
- Implement all CLI operations via `exec.Command("obsidian", ...)`:
  - Read: `obsidian read path=X`
  - Create: `obsidian create name=X content=Y silent`
  - Append: `obsidian append path=X content=Y`
  - Search: `obsidian search query=X format=json matches`
  - Properties: `obsidian property:read path=X`
  - Tags: `obsidian tags all counts`
  - Backlinks: `obsidian backlinks path=X`
  - Links: `obsidian links path=X`
  - Orphans: `obsidian orphans`
  - Unresolved: `obsidian unresolved`
  - Files: `obsidian files list folder=X`
  - Folders: `obsidian folders`
  - DailyAppend: `obsidian daily:append content=X`
  - VaultInfo: `obsidian vault`
- `IsAvailable()` method checks if CLI responds (exec timeout + parse test)
- All create operations include `silent` flag to prevent UI pop-ups
- Auto-create directories before create operations
- Mock `exec.Command` in tests to verify all CLI operations

**Acceptance criteria**:
- [ ] All CLI operations implemented with correct command syntax
- [ ] IsAvailable() detects CLI availability
- [ ] Retry logic (3 attempts, 2s delay) for transient failures
- [ ] Silent flag prevents Obsidian UI pop-ups
- [ ] Unit tests with mocked exec

**Complexity**: High

---

### Task 3.24: Obsidian — Backend Fallback Functions

**Description**: Degraded-mode implementations for when Obsidian CLI is unavailable.

**Files to create**:
```
internal/vault/fallback.go
internal/vault/fallback_test.go
```

**Implementation details**:
- `FallbackSearch(vaultPath, query, folder string, limit int)` — grep-based search on vault path
- `FallbackProperties(path string)` — YAML frontmatter parser for `---` delimited front matter
- `FallbackTags(vaultPath string)` — grep for `#tag` patterns across files
- All functions accept vaultPath as parameter (use existing Writer.path)
- Return same data structures as CLI methods where possible
- Test with real markdown files in `t.TempDir()`

**Acceptance criteria**:
- [ ] Grep-based search returns matching files
- [ ] YAML frontmatter parsing handles standard format
- [ ] Tag extraction finds all #tag patterns
- [ ] Unit tests with markdown test files

**Complexity**: Low

**Depends on**: Task 3.23

---

### Task 3.25: Obsidian — Backend Facade

**Description**: Unified Vault interface that tries Obsidian CLI first, falls back to Writer/fallback functions gracefully.

**Files to create**:
```
internal/vault/facade.go
internal/vault/facade_test.go
```

**Implementation details**:
- `VaultFacade` struct: client *ObsidianClient, writer *Writer, vaultPath string, mu sync.Mutex (for daily note serialization)
- Implements `VaultReader` interface (see obsidian plan D5 for interface definition)
- `Write(req WriteRequest)` — tries `client.Create()/Append()`, falls back to `writer.Write()`
- Read operations — tries CLI, falls back to `os.ReadFile` on failure
- Search — tries CLI, falls back to `FallbackSearch`
- Tags/Links — tries CLI, returns empty or degraded results if unavailable (no critical failures)
- DailyAppend protected by mutex to prevent concurrent writes
- Constructor: `NewFacade(vaultName, vaultPath string)` creates client and writer
- Unit tests verify CLI-first / Writer-fallback logic with CLI available and unavailable

**Acceptance criteria**:
- [ ] Write operations fall back to Writer when CLI unavailable
- [ ] Read operations fall back to file I/O
- [ ] Search falls back to grep-based search
- [ ] DailyNote mutex prevents concurrent writes
- [ ] Unit tests for fallback logic

**Complexity**: Medium

**Depends on**: Task 3.23, Task 3.24

---

### Task 3.26: Obsidian — Config & Bootstrap

**Description**: Add Obsidian vault name to config and CLI health check at bootstrap.

**Files to modify**:
```
internal/config/config.go
cmd/flux/main.go
```

**Implementation details**:
- Extend `VaultConfig` struct in `config.go:59-61`:
  ```go
  type VaultConfig struct {
      Path string `yaml:"path"`
      Name string `yaml:"name"` // NEW: Obsidian vault name for CLI
  }
  ```
- In `main.go`:
  - Replace `vault.NewWriter(cfg.Vault.Path)` with `vault.NewFacade(cfg.Vault.Name, cfg.Vault.Path)` (~line 91)
  - Pass `VaultFacade` to `server.NewServer()` and `executor.NewExecutor()`
  - Add CLI health check after bootstrap (~line 83):
    ```go
    if !vaultFacade.IsAvailable() {
        logger.Warn("Obsidian CLI unavailable, using file-based fallback",
            "features_degraded", "search, tags, backlinks, orphans, properties")
        notifier.Send(discord.LevelWarning, "Obsidian CLI unavailable. Knowledge features degraded.")
    }
    ```

**Acceptance criteria**:
- [ ] Config accepts vault.name field
- [ ] VaultFacade replaces Writer in main.go
- [ ] Health check logs warning if CLI unavailable
- [ ] Discord notification sent if degraded

**Complexity**: Low

**Depends on**: Task 3.25

---

### Task 3.27: Obsidian — Template Additions

**Description**: Add research finding and project proposal templates for future Phase 4 Researcher pods.

**Files to modify**:
```
internal/vault/templates.go
```

**Implementation details**:
- Add `ResearchFindingTemplate` — markdown template for research session results (question, findings, sources, confidence, recommendations)
- Add `ProjectProposalTemplate` — markdown template for new project proposals (goal, rationale, scope, tech stack, risks)
- Follow existing template patterns in `templates.go`

**Acceptance criteria**:
- [ ] ResearchFindingTemplate includes all key fields
- [ ] ProjectProposalTemplate includes all key fields
- [ ] Templates use consistent markdown formatting

**Complexity**: Low

---

### Task 3.28: Obsidian — Knowledge API Endpoints

**Description**: HTTP API endpoints exposing Vault data (notes, search, tags, links, health) to the Web UI.

**Files to create**:
```
internal/server/api_knowledge.go
internal/server/api_knowledge_test.go
```

**Files to modify**:
```
internal/server/server.go
```

**Implementation details**:
- Add `vault` field to `Server` struct: `vault *vault.VaultFacade`
- Implement 15 HTTP handlers (all require auth middleware):
  - `GET /api/knowledge/search?q={query}&folder={folder}&limit={n}` — full-text search
  - `GET /api/knowledge/note?path={path}` — read note content
  - `GET /api/knowledge/outline?path={path}` — note outline/headings
  - `GET /api/knowledge/tags` — all tags with counts
  - `GET /api/knowledge/tags/{tag}/files` — files with specific tag
  - `GET /api/knowledge/backlinks?path={path}` — notes linking to this note
  - `GET /api/knowledge/links?path={path}` — outgoing links from this note
  - `GET /api/knowledge/orphans` — orphaned notes (no backlinks)
  - `GET /api/knowledge/unresolved` — broken links
  - `GET /api/knowledge/files?folder={folder}` — list files in folder
  - `GET /api/knowledge/folders` — all folders
  - `GET /api/knowledge/stats` — vault statistics (note count, total size, etc.)
  - `GET /api/knowledge/daily` — today's daily note
  - `GET /api/knowledge/research/history` — research task history (from tasks table WHERE type=RESEARCH)
  - `GET /api/knowledge/research/stats` — research statistics (count by type_tag)
- Register routes in `server.go` (~line 170) following existing patterns
- Follow existing error handling: `writeJSON()`, `writeError()`
- Unit tests with mock VaultFacade

**Acceptance criteria**:
- [ ] All 15 endpoints return correct data
- [ ] Auth middleware applied
- [ ] Consistent error handling
- [ ] Unit tests for each endpoint

**Complexity**: Medium

**Depends on**: Task 3.25

---

### Task 3.29: Obsidian — Wire Facade to Server

**Description**: Connect VaultFacade to Server struct and route registration (part of Task 3.28 implementation).

**Implementation details**:
- Modify `Server` struct in `server.go` to add `vault *vault.VaultFacade` field
- Update `NewServer()` constructor to accept vault parameter
- Wire vault from `main.go` where Server is created
- Register all `/api/knowledge/*` routes in setupRoutes

**Acceptance criteria**:
- [ ] Server struct has vault field
- [ ] Routes registered correctly
- [ ] main.go passes VaultFacade to Server

**Complexity**: Low

**Depends on**: Task 3.28

---

### Task 3.30: Obsidian — Knowledge Store & Main Page

**Description**: Zustand store for Knowledge data and main Knowledge page with sub-tab routing.

**Files to create**:
```
frontend/src/stores/knowledgeStore.ts
frontend/src/pages/Knowledge.tsx
```

**Files to modify**:
```
frontend/src/App.tsx (add routes)
frontend/src/components/Layout.tsx (add nav item)
```

**Implementation details**:
- Create `knowledgeStore.ts` with:
  - State for all Knowledge sections (notes, search results, tags, links, stats, research history)
  - Fetch methods for all 15 API endpoints
  - Loading/error states
  - Selected note/folder state
- Create `Knowledge.tsx` main page with sub-tab routing (Browse, Search, Research, Graph, Health)
- Add `/knowledge` and `/knowledge/:tab` routes in `App.tsx`
- Add "Knowledge" navigation item in `Layout.tsx` sidebar

**Acceptance criteria**:
- [ ] Knowledge store has methods for all API endpoints
- [ ] Main page renders with sub-tab navigation
- [ ] Routes configured in App.tsx
- [ ] Navigation item in sidebar

**Complexity**: Medium

**Depends on**: Task 3.28

---

### Task 3.31: Obsidian — Browse Tab

**Description**: File tree navigation and note preview with markdown rendering.

**Files to create**:
```
frontend/src/pages/knowledge/Browse.tsx
```

**Implementation details**:
- Left panel: Folder tree (from `/api/knowledge/folders` and `/api/knowledge/files`)
- Right panel: Note preview with markdown rendering using existing `react-markdown` + `remark-gfm` dependencies
- Click on file → load note content via `/api/knowledge/note?path=`
- Breadcrumb navigation for folders
- Responsive layout (collapsible sidebar on mobile)

**Acceptance criteria**:
- [ ] File tree displays vault structure
- [ ] Note preview renders markdown correctly
- [ ] Navigation works (click file → load note)
- [ ] Responsive layout

**Complexity**: Medium

**Depends on**: Task 3.30

---

### Task 3.32: Obsidian — Search Tab

**Description**: Full-text search with result highlighting.

**Files to create**:
```
frontend/src/pages/knowledge/Search.tsx
```

**Implementation details**:
- Search input with query submission
- Folder filter dropdown (optional)
- Results list showing: file path, matching snippet, match count
- Click result → navigate to Browse tab with note loaded
- Search powered by `/api/knowledge/search?q=`
- Highlight search terms in snippets (simple string match)

**Acceptance criteria**:
- [ ] Search input and submission works
- [ ] Results display with snippets
- [ ] Click result navigates to note
- [ ] Folder filtering works

**Complexity**: Medium

**Depends on**: Task 3.30

---

### Task 3.33: Obsidian — Research Tab

**Description**: Research task timeline, active researcher pods, and research statistics.

**Files to create**:
```
frontend/src/pages/knowledge/Research.tsx
```

**Implementation details**:
- Timeline view of research tasks (from `/api/knowledge/research/history`)
  - Shows: title, type_tag, created_at, status, brief summary
  - Click task → view full task details (link to Tasks page)
- Active researcher pods panel (from `/api/pods` filtered by type=RESEARCHER)
- Research statistics panel (from `/api/knowledge/research/stats`)
  - Breakdown by type_tag (market_research, technical_research, user_research, etc.)
  - Total research count, success rate
- Note: Researcher pods are Phase 4, so this panel will be empty until then

**Acceptance criteria**:
- [ ] Research timeline displays all RESEARCH tasks
- [ ] Active pods panel shows researcher pods (empty for Phase 3)
- [ ] Statistics panel shows research breakdown
- [ ] Task links navigate to task details

**Complexity**: Medium

**Depends on**: Task 3.30

---

### Task 3.34: Obsidian — Graph & Health Tabs

**Description**: Tag cloud, link relationship list, orphan detection, and vault health statistics.

**Files to create**:
```
frontend/src/pages/knowledge/Graph.tsx
frontend/src/pages/knowledge/Health.tsx
```

**Implementation details**:
- **Graph.tsx**:
  - Tag cloud (from `/api/knowledge/tags`) — size based on count
  - Click tag → list files with that tag (from `/api/knowledge/tags/{tag}/files`)
  - Backlinks/Outlinks for selected note (from `/api/knowledge/backlinks` and `/api/knowledge/links`)
  - Simple list view (no d3.js graph visualization in Phase 3)
- **Health.tsx**:
  - Orphaned notes list (from `/api/knowledge/orphans`)
  - Broken links list (from `/api/knowledge/unresolved`)
  - Vault statistics (from `/api/knowledge/stats`) — note count, total size, folder count
  - "Vault health" indicator (green if no orphans/broken links, yellow/red otherwise)

**Acceptance criteria**:
- [ ] Tag cloud displays tags by count
- [ ] Click tag shows files
- [ ] Backlinks/outlinks shown for selected note
- [ ] Orphan notes listed
- [ ] Broken links listed
- [ ] Vault stats displayed

**Complexity**: Low

**Depends on**: Task 3.30

---

### Task 3.35: Obsidian — Dashboard Widget

**Description**: Add Knowledge summary widget to Dashboard page.

**Files to modify**:
```
frontend/src/pages/Dashboard.tsx
```

**Implementation details**:
- Add Knowledge card to Dashboard (similar to existing Task/Project cards)
- Display:
  - Total notes count (from `/api/knowledge/stats`)
  - Recent research count (last 7 days, from `/api/knowledge/research/history`)
  - Orphan count (from `/api/knowledge/orphans`)
  - "View Knowledge →" link to `/knowledge`
- Keep it minimal — Dashboard widget is a quick summary

**Acceptance criteria**:
- [ ] Knowledge widget displays summary stats
- [ ] Link to Knowledge page works
- [ ] Consistent styling with other Dashboard widgets

**Complexity**: Low

**Depends on**: Task 3.28, Task 3.30

---

## Recommended Implementation Order

### Core Orchestration (Original Phase 3)

```
Task 3.0  (Orchestrator integration in main.go)     — Foundation
  ├── Task 3.1  (Tick loop)                          — Core loop
  │   ├── Task 3.2  (ScaleManager)                   — Auto-scaling
  │   ├── Task 3.14 (Orchestrator API)               — Status visibility
  │   └── Task 3.15 (Disk monitoring)                — Safety
  ├── Task 3.3  (RateLimitHandler upgrade)           — Independent
  ├── Task 3.4  (UsageCollector)                     — Data collection
  │   └── Task 3.5  (Time-series queries)            — Data access
  │       └── Task 3.12 (Usage API)                  — API layer
  │           └── Task 3.13 (Usage UI)               — Frontend
  ├── Task 3.7  (DailySummary)                       — Independent
  ├── Task 3.8  (JSONL Cleanup)                      — Independent
  ├── Task 3.9  (Daily Backup)                       — Independent
  ├── Task 3.11 (Data Cleanup)                       — Independent
  └── Task 3.16 (Settings + Metrics)                 — Frontend

Task 3.6  (Per-task usage hardening)                 — Independent verification
Task 3.10 (Shutdown upgrade)                         — Independent verification
Task 3.17 (GoalAdvisor)                              — Optional
```

**Critical path**: 3.0 → 3.1 → 3.2 (gets Pods auto-managed)
**Parallel tracks** after 3.0: Usage (3.4→3.5→3.12→3.13), Maintenance (3.8, 3.9, 3.11), Daily Summary (3.7)

### Insight Analytics (New Tasks)

```
Task 3.18 (Insight query methods)                    — Foundation
  └── Task 3.19 (Insight API endpoints)              — Backend
      └── Task 3.20 (Insight store + API client)     — Frontend data layer
          ├── Task 3.21 (Insight page)               — Main analytics UI
          └── Task 3.22 (Dashboard enhancement)      — Dashboard widget
```

**Critical path**: 3.18 → 3.19 → 3.20 → 3.21
**Dependencies**: Task 3.18 depends on Phase 2B complete (tasks table with all fields)
**Can start**: After Phase 3 core is stable (or in parallel if database is ready)

### Obsidian Integration (New Tasks)

```
Phase 3a: Backend Core
Task 3.23 (Obsidian CLI client)                      — Foundation
  └── Task 3.24 (Fallback functions)                 — Degraded mode
      └── Task 3.25 (VaultFacade)                    — Unified interface
          ├── Task 3.26 (Config + bootstrap)         — Integration
          └── Task 3.27 (Template additions)         — Phase 4 prep
              │
              ▼
Phase 3b: Knowledge API
Task 3.28 (Knowledge API endpoints)                  — API layer
  └── Task 3.29 (Wire facade to server)              — Integration
      │
      ▼
Phase 3c: Knowledge Frontend UI
Task 3.30 (Knowledge store + main page)              — Frontend foundation
  ├── Task 3.31 (Browse tab)                         — File tree + preview
  ├── Task 3.32 (Search tab)                         — Full-text search
  ├── Task 3.33 (Research tab)                       — Research timeline
  ├── Task 3.34 (Graph + Health tabs)                — Tag cloud + health
  └── Task 3.35 (Dashboard widget)                   — Dashboard card
```

**Critical path**: 3.23 → 3.25 → 3.28 → 3.30 → 3.31 (minimum viable Knowledge UI)
**Parallel tracks** after 3.30: All UI tabs (3.31-3.35) can be built in parallel
**Dependencies**: Requires Obsidian app v1.12+ installed and CLI enabled
**Can start**: Independent of other Phase 3 tasks (can run in parallel)

### Summary

**Three parallel tracks** in Phase 3:
1. **Core Orchestration** (3.0-3.17) — Auto-scaling, usage tracking, maintenance
2. **Insight Analytics** (3.18-3.22) — Comprehensive analytics and observability
3. **Obsidian Integration** (3.23-3.35) — Knowledge management via Obsidian CLI

**Recommended sequence**:
- Start with Core Orchestration (3.0-3.2) to get foundation in place
- Launch Insight Analytics (3.18-3.22) in parallel once tasks table is stable
- Launch Obsidian Integration (3.23-3.35) as independent track
- Complete all three tracks before marking Phase 3 done

---

## Phase 3 Completion Checklist

### Core Orchestration
- [ ] Orchestrator runs with 5-minute tick cycle
- [ ] Pod scaling adjusts automatically based on workload (executor-only)
- [ ] Dynamic rate limit wait uses ccusage billing data (fallback to 5h)
- [ ] Hourly usage snapshots in DB
- [ ] Per-task usage tracking verified
- [ ] Daily Discord summary at configured hour
- [ ] JSONL cleanup (7-day retention for sensitive data)
- [ ] Daily backup (SQLite + Vault, 7-day retention)
- [ ] Data cleanup (metrics, snapshots)
- [ ] Usage UI with time-series data
- [ ] Orchestrator status API functional
- [ ] Disk space monitoring active
- [ ] Settings UI enhanced with system health
- [ ] (Optional) Goal proposal system active

### Insight Analytics
- [ ] Insight query methods return all analytics data
- [ ] 5 Insight API endpoints functional
- [ ] Insight page with time-series charts (using recharts)
- [ ] Dashboard shows analytics summary with trends
- [ ] Efficiency metrics (cost/tokens per task, by type/model/priority)
- [ ] Pipeline health metrics (queue depth, throughput, latency)
- [ ] Failure analysis with error categorization

### Obsidian Integration
- [ ] Obsidian CLI client wrapper functional
- [ ] VaultFacade with CLI-first, Writer fallback pattern
- [ ] CLI health check at bootstrap
- [ ] 15 Knowledge API endpoints functional
- [ ] Knowledge page with 5 sub-tabs (Browse, Search, Research, Graph, Health)
- [ ] File tree navigation and markdown preview
- [ ] Full-text search via Obsidian index
- [ ] Tag cloud and backlink/outlink views
- [ ] Orphan detection and broken link detection
- [ ] Dashboard Knowledge widget
- [ ] Research task timeline (Phase 4 prep)

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| **Core Orchestration** | | |
| Go backend | ~7 files | ~5 files |
| React frontend | ~2 files | ~2 files |
| **Insight Analytics** | | |
| Go backend | ~2 files | ~1 file |
| React frontend | ~2 files | ~3 files |
| **Obsidian Integration** | | |
| Go backend | ~9 files (+ 6 test files) | ~3 files |
| React frontend | ~8 files | ~3 files |
| **Total** | **~30 files** | **~17 files** |

**Estimated total additions**: ~5,000-6,000 lines across all three tracks
