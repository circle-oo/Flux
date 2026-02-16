# Phase 3: Orchestration + Insights + Knowledge — "Autonomous operation with analytics"

## Goal

Flux manages itself: automatic agent scaling based on workload, comprehensive insights dashboard with time-series analytics, dynamic rate limit handling, daily operational summaries, and a Knowledge UI for browsing the Obsidian Vault. The Operator only sets Goals, reviews complex PRs, and approves new projects.

## Deliverable

Auto-scaling agents, Insights UI (time-series charts, agent performance, tool usage stats), Knowledge UI (vault browsing, search, tags, health), Obsidian CLI Go backend (VaultFacade), dynamic rate limit wait, Discord daily summary, daily backups.

## Prerequisites

- Phase 2C complete (protobuf-unified architecture, Python Agent SDK, Connect-RPC, obsidian-cli for agents)
- ccusage installed and working
- Obsidian app running in background (macOS LaunchAgent)
- Obsidian CLI enabled (v1.12+, Settings → CLI → ON)

---

## Task Breakdown

### Task 3.1: Orchestrator Framework

**Description**: Implement the main Orchestrator loop that coordinates all sub-components on a 5-minute tick.

**Files to create/modify**:
```
go/src/internal/orchestrator/orchestrator.go  (rewrite for new architecture)
```

**Implementation details**:
- `Orchestrator` struct: config, manager, scaleManager, insightsCollector, usageCollector, dailySummary, rateLimitHandler, goalAdvisor, agentClient, notifier
- `Run(ctx)`: 5-minute ticker loop
- `tick()` sequence:
  1. `rateLimitHandler.CheckAndRecover()`
  2. `scaleManager.Rebalance(!rateLimitHandler.IsLimited())`
  3. `insightsCollector.CollectIfDue()`
  4. `usageCollector.CollectIfDue()`
  5. `goalAdvisor.ProposeIfNeeded()`
  6. `dailySummary.SendIfDue()`
- Each sub-component call wrapped in `recover()` to prevent single-component panic from crashing the Orchestrator
- Context cancellation stops the loop gracefully
- Each sub-component is independently testable

**Acceptance criteria**:
- [ ] Orchestrator ticks every 5 minutes
- [ ] All sub-components called in correct order
- [ ] Context cancellation stops cleanly
- [ ] Panic in one sub-component doesn't crash Orchestrator (recover + log)
- [ ] Unit tests verify panic recovery behavior

**Complexity**: Medium

---

### Task 3.2: ScaleManager — Agent Process Scaling

**Description**: Automatic agent count management. Instead of Go goroutine Pods, ScaleManager now manages Python agent capacity by adjusting how many concurrent tasks the Agent Manager accepts.

**Files to create**:
```
go/src/internal/orchestrator/scale_manager.go
```

**Implementation details**:
- `ScaleManager` struct: maxAgents, cooldown (15 min), lastScaleAt, minResearch (0.2), agentClient
- New paradigm: Python Agent Manager runs a pool of concurrent agent sessions. ScaleManager controls the pool size via a configuration RPC or config file.
- `Rebalance(running bool)`:
  - If not running (rate limited): set max concurrent agents to 0
  - If cooldown active (< 15 min since last scale): skip
  - Determine agent type ratio based on queue state:
    - Urgent tasks (P1-5) → 90% dev, 10% rnd
    - Operator tasks → 80% dev, 20% rnd
    - System tasks only → 70% dev, 30% rnd
    - Queue nearly empty → 30% dev, 70% rnd
    - Queue empty → 0% dev, 100% rnd (pure research)
  - Communicate target to Python Agent Manager
- R&D protection: minimum 20% research except during incidents

**Acceptance criteria**:
- [ ] Agent counts adjust based on queue state
- [ ] Dev:R&D ratio follows spec
- [ ] 15-minute cooldown between scaling events
- [ ] R&D protection (min 20%) enforced
- [ ] All agents stop during rate limit
- [ ] Communication with Python Agent Manager works

**Complexity**: High

---

### Task 3.3: Insights Data Model + Collection

**Description**: Define the data model for insights analytics and implement background collection of metrics: task completion rates, agent performance, tool usage statistics, and duration distributions. All insight data is derived from existing tables — no new tables are needed.

**Files to create**:
```
go/src/internal/insights/collector.go
go/src/internal/insights/models.go
go/src/internal/db/insights_queries.go
```

**Implementation details**:

**models.go** — Insights data types:
- `DailyMetric`: date, tasks_completed, tasks_failed, total_duration_seconds
- `AgentPerformance`: agent_type, success_rate, avg_turns, avg_duration_seconds, tasks_count
- `ToolUsageStat`: tool_name, invocation_count, avg_duration_ms
- `InsightsSummary`: daily_metrics[], agent_performance[], tool_usage[]

**collector.go** — Background insights collection:
- `InsightsCollector` struct: db, config, lastCollection
- `CollectIfDue()`: Run every 15 minutes
- Collection logic:
  - Query completed/failed tasks in window → compute `DailyMetric`
  - Parse `TaskEvent` metadata from stored events → extract tool usage (TOOL_USE events contain tool name)
  - Aggregate by agent_type → compute `AgentPerformance` (success rate, avg turns, avg duration)
  - Store aggregated metrics in new `insights_snapshots` table
- `GetInsights(timeRange string) *InsightsSummary`: Query stored snapshots for dashboard

**insights_queries.go** — DB queries:
- `InsertInsightsSnapshot(snapshot)`
- `GetDailyMetrics(from, to) []DailyMetric`
- `GetAgentPerformance(from, to) []AgentPerformance`
- `GetToolUsage(from, to) []ToolUsageStat`

**New DB table**:
```sql
CREATE TABLE IF NOT EXISTS insights_snapshots (
    id           TEXT PRIMARY KEY,
    type         TEXT NOT NULL,           -- DAILY, AGENT_PERF, TOOL_USAGE
    data         TEXT NOT NULL,           -- JSON
    period_start DATETIME NOT NULL,
    period_end   DATETIME NOT NULL,
    recorded_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_insights_type_period ON insights_snapshots(type, period_start);
```

**SQL implementation notes** (from insight-plan):
- Time bucketing: `GROUP BY strftime('%Y-%m-%d', completed_at)` for daily, `strftime('%Y-%m-%d %H:00', completed_at)` for hourly
- Latency computation: `julianday(started_at) - julianday(created_at)` for queue wait, `julianday(completed_at) - julianday(started_at)` for execution time
- P90 calculation: `ORDER BY total_time LIMIT 1 OFFSET (count * 0.9)` subquery
- Decomposition stats derived from `parent_id` relationships in the tasks table
- All queries use existing indexes (`idx_tasks_status_priority`, `idx_tasks_project`)
- Null-safe handling with `COALESCE` for empty results

**Error categorization logic** — Failures are categorized by keyword matching on the `error_log` field (generated by Flux's executor, so patterns are predictable):
- Contains "build" → `build_failure`
- Contains "test" → `test_failure`
- Contains "rate limit" → `rate_limit`
- Contains "timeout" or "context deadline" → `timeout`
- Else → `other`

**Acceptance criteria**:
- [ ] Insights data models defined
- [ ] Background collection runs every 15 minutes
- [ ] Daily metrics correctly aggregated from task data
- [ ] Agent performance computed per agent type
- [ ] Tool usage extracted from TaskEvent metadata
- [ ] Snapshots stored in DB
- [ ] Query methods return correct time-range data
- [ ] Error categorization correctly buckets failures
- [ ] Unit tests for aggregation logic

**Complexity**: High

---

### Task 3.4: Insights API Endpoints

**Description**: Implement 5 HTTP insight API endpoints plus wire insights data into the `GetInsights` RPC already defined in `flux.proto`.

**Files to modify**:
```
go/src/internal/handler/flux_service.go    (implement GetInsights)
go/src/internal/server/api_insights.go     (new — 5 HTTP endpoints)
go/src/internal/server/server.go           (register routes)
```

**Implementation details**:

**Connect-RPC integration**:
- `GetInsights(req) → GetInsightsResponse`:
  - Parse `time_range` ("24h", "7d", "30d") → compute from/to timestamps
  - Call `insightsCollector.GetInsights(timeRange)`
  - Return: `daily_metrics[]`, `tool_usage[]`, `agent_performance[]`
- Enrich `GetDashboard` with today's insights summary:
  - Add total tasks today, completed, failed, active agents
  - Agent summaries with completion counts and avg duration

**HTTP API endpoints** (5 endpoints under `/api/insights/*`):

1. `GET /api/insights/summary` — Enhanced summary with task stats, PR stats, today's metrics, project activity breakdown
2. `GET /api/insights/timeseries?period=7d&bucket=daily` — Time-bucketed metrics for charting (periods: 24h, 7d, 30d, 90d; buckets: hourly, daily)
3. `GET /api/insights/efficiency` — Cost per task, tokens per task, breakdown by type/model/priority
4. `GET /api/insights/pipeline` — Queue depth, throughput, latency (avg + P90), retry stats, decomposition stats
5. `GET /api/insights/failures` — Failure analysis with error categorization, recent failures list, per-project failure rates

All endpoints require auth middleware. Register routes in `server.go:setupRoutes` following existing patterns.

**Backward compatibility**: The existing `GET /api/insights` endpoint is preserved as-is. New endpoints use `/api/insights/` prefix with subpaths.

See **Appendix A** for full JSON response schemas for all 5 endpoints.

**Acceptance criteria**:
- [ ] `GetInsights` RPC returns correct data for all time ranges
- [ ] `GetDashboard` includes today's stats
- [ ] All 5 HTTP endpoints return correct JSON
- [ ] Query parameters validated (period, bucket)
- [ ] Auth middleware applied to all endpoints
- [ ] Existing `/api/insights` still works (backward compatible)
- [ ] Data is properly typed (protobuf messages)
- [ ] Unit tests for handlers

**Complexity**: Medium

**Depends on**: Task 3.3

---

### Task 3.5: Insights UI — Analytics Dashboard

**Description**: Build the Insights page in the frontend with time-series charts, agent performance metrics, tool usage statistics, efficiency breakdowns, pipeline health, and failure analysis.

**Files to create/modify**:
```
frontend/src/pages/Insights.tsx             (new)
frontend/src/stores/insightStore.ts         (new)
frontend/src/components/InsightsPanel.tsx    (modify — full implementation)
frontend/src/components/charts/TimeSeriesChart.tsx    (new)
frontend/src/components/charts/AgentPerfChart.tsx     (new)
frontend/src/components/charts/ToolUsageChart.tsx     (new)
frontend/src/lib/api.ts                     (modify — add API methods)
```

**Implementation details**:

**Insight page sections** (5 sections):
1. **Summary Cards** — Today's stats (completions, cost, success rate) + all-time stats + trend indicators (up/down arrows comparing today vs yesterday)
2. **Time-Series Chart** — Tasks completed/failed over time, cost over time. Line chart with green (completed) vs red (failed). X-axis: dates, Y-axis: count. Interactive tooltips.
3. **Efficiency Table** — Breakdown by type (CODING, BUGFIX, etc.), model (opus, sonnet), and priority range (1-10, 11-40, 41-100). Shows count, success rate, avg cost, avg tokens.
4. **Pipeline Health** — Queue depth (pending/ready/running), throughput (24h/7d/30d), latency metrics (avg queue wait, avg execution, P90 total), retry and decomposition stats
5. **Failure Analysis** — Recent failures list with task details, error categorization bar chart, per-project failure rates

**Time range selector**: 24h | 7d | 30d | 90d buttons at the top. Updates all sections.

**Chart library**: `recharts` — React-native charting (14M weekly npm downloads, ~45KB gzipped). Provides BarChart, LineChart, AreaChart with tooltips, responsive containers, and animations. Preferred over pure CSS/SVG bars because the insight page benefits from real interactive charts.

**insightStore.ts** — Zustand store:
- State for each data section (summary, timeseries, efficiency, pipeline, failures)
- Fetch methods with loading/error states
- Period/bucket selection state
- Auto-refresh on period change

**Dashboard integration**:
- `InsightsPanel` component shows mini-summary on Dashboard
- Today's stats: total/completed/failed tasks, cost, success rate
- Trend indicators for day-over-day comparison
- "View Details →" link to the Insight page
- Top performing agent type
- Most used tool

**Acceptance criteria**:
- [ ] Time-series chart renders with correct data
- [ ] Agent performance chart shows all agent types
- [ ] Tool usage chart displays top tools
- [ ] Efficiency table shows breakdowns by type, model, priority
- [ ] Pipeline health panel renders queue, throughput, latency
- [ ] Failure analysis shows categorized errors and recent failures
- [ ] Time range selector updates all charts and sections
- [ ] Loading/error states handled
- [ ] Dashboard mini-summary works with trend indicators
- [ ] Responsive design
- [ ] Consistent styling with existing pages (Tailwind, dark theme)

**Complexity**: High

**Depends on**: Task 3.4

---

### Task 3.6: RateLimitHandler Upgrade — Dynamic Wait

**Description**: Upgrade from fixed 5-hour wait to dynamic wait using ccusage billing window data.

**Files to modify**:
```
go/src/internal/orchestrator/rate_limit_handler.go
```

**Implementation details**:
- `HandleRateLimitDynamic()`:
  1. Set `isLimited = true` (ScaleManager stops all agents)
  2. Query `ccusage blocks --json` for billing window reset time
  3. If reset time available: set `rateLimitUntil = resetTime + 1min`
  4. If query fails: fallback to fixed 5-hour wait
  5. Discord notification with expected resume time
  6. `CheckAndRecover()` clears limited state when `time.Now().After(rateLimitUntil)`
- Non-blocking: state-based (Pods check `IsLimited()` before requesting tasks)
- Record event in `rate_limit_events` table with billing window info

**Acceptance criteria**:
- [ ] Dynamic wait uses ccusage blocks data
- [ ] Fallback to 5-hour wait if ccusage fails
- [ ] Discord notification includes expected resume time
- [ ] Non-blocking state-based approach
- [ ] Unit tests for dynamic vs fallback paths

**Complexity**: Medium

**Depends on**: Phase 2B rate limit handler

---

### Task 3.7: UsageCollector — ccusage Integration

**Description**: Collect usage snapshots from ccusage at configured intervals, store in DB for time-series display.

**Files to create**:
```
go/src/internal/orchestrator/usage_collector.go
```

**Implementation details**:
- `UsageCollector` struct: config, db, lastCollection
- `CollectIfDue()`: check if `collection_interval` (1 hour) has passed
- Collection: run `ccusage daily --json`, `ccusage blocks --json`
- Store raw JSON in `usage_snapshots` table with type (HOURLY, BLOCKS)
- Extract summary: total_tokens, total_cost
- Per-task usage: After task completion, fire-and-forget `ccusage daily --project {encoded-path} --json`
  - Update task record: tokens_used, cost_usd (async, best-effort)
- Query methods:
  - `GetUsageTimeSeries(from, to, type) []UsageSnapshot`
  - `GetDailySummary(date) *UsageSnapshot`
  - `GetMonthlyTotal() (tokens int, cost float64)`
- Graceful degradation: log error, skip if ccusage unavailable

**Acceptance criteria**:
- [ ] Hourly collection via ccusage CLI
- [ ] Raw JSON stored in usage_snapshots
- [ ] Summary fields extracted
- [ ] Per-task usage collected async
- [ ] Time-series queries work
- [ ] ccusage failure doesn't crash Orchestrator

**Complexity**: Medium

---

### Task 3.8: DailySummary — Discord Report

**Description**: Send Discord daily summary at midnight with comprehensive operational metrics.

**Files to create**:
```
go/src/internal/orchestrator/daily_summary.go
```

**Implementation details**:
- `DailySummary` struct: config (daily_summary_hour=0), notifier, db, insights, lastSent
- `SendIfDue()`: check if current hour matches config and hasn't sent today
- Summary content:
  - Tasks completed today (count, titles)
  - Tasks failed (count, error summaries)
  - PRs merged / pending review
  - Total tokens/cost for the day
  - Active Goal progress
  - Agent performance (success rate by type)
  - Top tools used today
  - Rate limit events (if any)
- Format as Discord embed-friendly message (markdown with sections)

**Acceptance criteria**:
- [ ] Summary sent at configured hour (midnight)
- [ ] Sent exactly once per day
- [ ] Includes all specified metrics
- [ ] Readable format in Discord
- [ ] Includes insights data (agent perf, tool usage)

**Complexity**: Medium

---

### Task 3.9: GoalAdvisor

**Description**: Orchestrator sub-component that proposes new Goals based on system state.

**Files to create**:
```
go/src/internal/orchestrator/goal_advisor.go
```

**Implementation details**:
- `GoalAdvisor` struct: config, manager, notifier
- `ProposeIfNeeded()`: check conditions for proposing a new Goal
- Phase 3 heuristics (simple, not Claude-powered):
  - No active Goal and queue empty for >1 hour → propose "Plan next development cycle"
  - Active Goal with >80% of goal-related tasks completed → propose follow-up
  - No operator tasks in 48 hours → propose "Review and update goals"
- Create PROPOSED Goal (requires Operator approval)
- Discord notification for proposals
- Prevent duplicate proposals (check for existing PROPOSED goals)

**Acceptance criteria**:
- [ ] Goal proposed when no active Goal exists
- [ ] Heuristics correctly detect proposal conditions
- [ ] Proposed Goal requires Operator activation
- [ ] Discord notification sent
- [ ] No duplicate proposals
- [ ] Unit tests for heuristic logic

**Complexity**: Medium

---

### Task 3.10: Cleanup, Backup, and Maintenance

**Description**: Automated cleanup of old data, daily backups, JSONL cleanup, and disk space monitoring.

**Files to create/modify**:
```
go/src/internal/cleanup/cleanup.go          (new)
go/src/internal/cleanup/backup.go           (new)
```

**Implementation details**:

**cleanup.go**:
- `CleanOldJSONL(retentionDays int)`: Delete JSONL files older than 7 days from `~/.claude/projects/` paths. Secure deletion (zero-fill + remove).
- `RunDataCleanup(cfg)`:
  - `service_metrics`: delete older than 7 days
  - `usage_snapshots`: delete older than 90 days
  - `insights_snapshots`: delete older than 90 days
  - Backup files: delete older than 7 days
  - Failed worktrees: delete older than 24 hours
- Disk space monitoring:
  - WARNING at 10GB free → Discord
  - Block new worktree creation at 5GB free
  - CRITICAL at 2GB free → Discord
  - Force cleanup at 1GB free → delete oldest worktrees

**backup.go**:
- `RunDailyBackup(dbPath, vaultPath, backupDir)`:
  - SQLite: `.backup` command for consistent copy
  - Vault: `tar.gz` of entire vault directory
  - Naming: `flux-db-{date}.bak`, `flux-vault-{date}.tar.gz`
  - 7-day retention
- Run at 4am via Orchestrator scheduling

**Acceptance criteria**:
- [ ] JSONL cleanup with 7-day retention
- [ ] Data cleanup for metrics, snapshots, worktrees
- [ ] Disk space monitoring with alerts
- [ ] SQLite backup via .backup command
- [ ] Vault tar.gz created
- [ ] 7-day backup retention
- [ ] Disk space warnings sent to Discord

**Complexity**: Medium

---

### Task 3.11: Orchestrator API + Settings UI

**Description**: Connect-RPC endpoints for orchestrator status and Settings page in the frontend.

**Files to modify**:
```
proto/flux/v1/flux.proto                    (add Orchestrator RPCs if needed)
go/src/internal/handler/flux_service.go     (add status methods)
frontend/src/pages/Settings.tsx             (enhance)
```

**Implementation details**:

**Additional RPCs** (if not already in proto):
- `GetOrchestratorStatus` → scaling state, rate limit, agent ratio
- `GetMetrics` → queue depth, completion rate, disk usage

**Settings.tsx**:
- Current configuration (read-only)
- Agent status (active agents, types, current tasks)
- System health (disk usage, memory, rate limit status)
- Queue depth by priority
- Rate limit event history
- Daily usage summary

**Acceptance criteria**:
- [ ] Orchestrator status endpoint returns scaling state
- [ ] Agent list shows current tasks
- [ ] Settings page displays system health
- [ ] Disk usage visible with warnings
- [ ] Rate limit history displayed

**Complexity**: Medium

**Depends on**: Task 3.1

---

### Task 3.12: Graceful Shutdown Upgrade

**Description**: Upgrade shutdown to handle both Go and Python processes with two-stage timeout.

**Files to modify**:
```
go/src/internal/shutdown/shutdown.go
```

**Implementation details**:
- Two-stage timeout: `pod_grace_period` (10 min) → `force_kill_after` (12 min)
- Shutdown sequence:
  1. Stop accepting new tasks
  2. Cancel running agent tasks via gRPC `CancelAgentTask`
  3. Wait up to 10 min for agents to finish
  4. If timeout: SIGTERM to Python Agent Manager
  5. After 12 min: SIGKILL + move tasks to RETRY
  6. Close DB, drain Vault Writer
- Coordinate with Python Agent Manager graceful shutdown

**Acceptance criteria**:
- [ ] Two-stage timeout implemented
- [ ] Agent cancellation via gRPC
- [ ] Python process signaled for shutdown
- [ ] Tasks correctly transitioned to RETRY
- [ ] No process leaks

**Complexity**: Medium

**Depends on**: Phase 2C process management

---

### Task 3.13: Obsidian CLI Backend — VaultFacade

**Description**: Implement Go-side Obsidian CLI wrapper with CLI-first, Writer-fallback pattern. This enables the Knowledge API and pre-fetches vault context for agents.

**Files to create/modify**:
```
go/src/internal/vault/client.go         (new — CLI wrapper, ~250 lines)
go/src/internal/vault/facade.go         (new — CLI-first + Writer fallback, ~100 lines)
go/src/internal/vault/fallback.go       (new — degraded-mode grep/YAML, ~80 lines)
go/src/internal/config/config.go        (modify — add vault.name)
go/src/cmd/flux/main.go                 (modify — switch Writer → VaultFacade)
```

**Implementation details**:

**vault.Client** struct: wraps `obsidian` CLI via `exec.Command`
- `Read(ctx, path) (string, error)` — `obsidian read path=X`
- `Search(ctx, query, folder, limit) ([]SearchResult, error)` — `obsidian search query=X format=json matches`
- `TagsAll(ctx) (map[string]int, error)` — `obsidian tags all counts`
- `Backlinks(ctx, path) ([]string, error)` — `obsidian backlinks path=X`
- `Links(ctx, path) ([]string, error)` — `obsidian links path=X`
- `Orphans(ctx) ([]string, error)` — `obsidian orphans`
- `Unresolved(ctx) ([]string, error)` — `obsidian unresolved`
- `Create(ctx, name, content) error` — `obsidian create name=X content=Y silent`
- `DailyAppend(ctx, content) error` — `obsidian daily:append content=X`
- `ListFiles(ctx, folder) ([]string, error)` — `obsidian files list folder=X`
- `IsAvailable() bool` — check if CLI responds

**CLI command mapping**:

| Flux Operation | Obsidian CLI Command | Fallback |
|---------------|---------------------|----------|
| Read note | `obsidian read path=X` | `os.ReadFile` |
| Create note | `obsidian create name=X content=Y silent` | `Writer.Write(ModeCreate)` |
| Append to note | `obsidian append path=X content=Y` | `Writer.Write(ModeAppend)` |
| Full-text search | `obsidian search query=X format=json matches` | `grep -r` on vault path |
| Read frontmatter | `obsidian property:read path=X` | YAML frontmatter parser |
| Set property | `obsidian property:set name=K value=V path=X` | N/A (degraded) |
| List tags | `obsidian tags all counts` | `grep -r "#tag"` |
| Get backlinks | `obsidian backlinks path=X` | N/A (degraded) |
| Get outlinks | `obsidian links path=X` | N/A (degraded) |
| Find orphans | `obsidian orphans` | N/A (degraded) |
| Find broken links | `obsidian unresolved` | N/A (degraded) |
| Daily note append | `obsidian daily:append content=X` | Direct file write to date-named path |
| List files | `obsidian files list folder=X` | `os.ReadDir` |
| Vault info | `obsidian vault` | N/A (health check only) |

**vault.VaultFacade**: CLI-first, Writer fallback
- Uses `vault.Client` when Obsidian available
- Falls back to existing `vault.Writer` for writes
- Falls back to `vault.Fallback*` functions for reads (grep, YAML parse)
- DailyAppend protected by mutex to prevent concurrent writes

**VaultReader interface** (for testability — `Server` and `Executor` consume this):
```go
type VaultReader interface {
    Read(ctx context.Context, path string) (string, error)
    Search(ctx context.Context, query, folder string, limit int) ([]SearchResult, error)
    Properties(ctx context.Context, path string) (map[string]string, error)
    TagsAll(ctx context.Context) (map[string]int, error)
    FilesByTag(ctx context.Context, tag string) ([]string, error)
    Backlinks(ctx context.Context, path string) ([]string, error)
    Links(ctx context.Context, path string) ([]string, error)
    Orphans(ctx context.Context) ([]string, error)
    Unresolved(ctx context.Context) ([]string, error)
    ListFiles(ctx context.Context, folder string) ([]string, error)
    Folders(ctx context.Context) ([]string, error)
    Outline(ctx context.Context, path string) (string, error)
    VaultInfo(ctx context.Context) (string, error)
    DailyRead(ctx context.Context) (string, error)
    IsAvailable() bool
}
```

**vault.Fallback*** — degraded-mode implementations:
- `FallbackSearch(basePath, query)` — recursive grep
- `FallbackTagsAll(basePath)` — grep for `#tag` patterns

**Three-mode operation**:

| Mode | Condition | Read | Write | Search | Tags/Links |
|------|-----------|------|-------|--------|------------|
| **Full** | Obsidian app running + CLI enabled | CLI | CLI | CLI (indexed) | CLI |
| **Fallback** | Obsidian not running / CLI unavailable | File I/O | Writer (channel) | grep | grep (tags only) |
| **Degraded** | Vault path missing | Error | Error | Error | Error |

**Config change** — extend `VaultConfig` struct:
```go
type VaultConfig struct {
    Path string `yaml:"path"`
    Name string `yaml:"name"` // Obsidian vault name for CLI
}
```

```yaml
vault:
  path: ~/ObsidianVault/Flux    # existing
  name: Flux                     # NEW: Obsidian vault name for CLI commands
```

**Health check at startup** (`main.go` bootstrap):
```go
client := vault.NewClient(cfg.Vault.Name)
if !client.IsAvailable() {
    logger.Warn("Obsidian CLI not available, using file-based fallback",
        "features_degraded", "search, tags, backlinks, orphans, properties")
    discord.Send(notifier.LevelWarning, "Obsidian CLI unavailable. Knowledge features degraded.")
}
```

**Bootstrap changes** in `main.go`:
- Replace `vault.NewWriter(cfg.Vault.Path)` with `vault.NewFacade(cfg.Vault.Name, cfg.Vault.Path)` (~line 91)
- Pass `VaultFacade` to `server.NewServer()` and `executor.NewExecutor()`
- Add Obsidian CLI health check log message after bootstrap (~line 83)

**CLI gotchas and mitigations**:

| Issue | Impact | Mitigation in Go Client |
|-------|--------|------------------------|
| `create` without `silent` opens Obsidian UI | Disrupts background operation | `Client.Create()` always appends `silent` flag |
| CLI doesn't auto-create directories | Create fails for new paths | `Client.Create()` calls `os.MkdirAll()` before exec |
| `tags all` required for vault-wide tags | Without `all`, returns current-file tags | Client always adds `all` to tag queries |
| Empty result on fresh vault = index not ready | False negatives | Retry 3 times with 2s delay |
| 22.8% silent failure rate (exit 0, no output) | Silent data loss | Post-write verification: search for created note |
| `template=` parameter ignores path | Created in unexpected location | Verify location after template creation via search |

**Acceptance criteria**:
- [ ] CLI wrapper executes all Obsidian operations
- [ ] Fallback works when Obsidian not running
- [ ] VaultFacade transparently switches between CLI and fallback
- [ ] Three-mode operation (Full/Fallback/Degraded) works correctly
- [ ] Health check at startup with Discord notification on degraded mode
- [ ] CLI gotchas mitigated (silent flag, MkdirAll, retry logic)
- [ ] Unit tests with mocked exec

**Complexity**: High

---

### Task 3.14: Knowledge API

**Description**: HTTP endpoints exposing Vault data to the Web UI via VaultFacade.

**Files to create/modify**:
```
go/src/internal/server/api_knowledge.go     (new, ~300 lines)
go/src/internal/server/api_knowledge_test.go (new, ~200 lines)
go/src/internal/server/server.go            (modify — register routes, add vault field)
```

**Implementation details**:
- 15 endpoints under `/api/knowledge/*`:
  ```
  GET /api/knowledge/search?q={query}&folder={folder}&limit={n}
  GET /api/knowledge/note?path={path}
  GET /api/knowledge/outline?path={path}
  GET /api/knowledge/tags
  GET /api/knowledge/tags/{tag}/files
  GET /api/knowledge/backlinks?path={path}
  GET /api/knowledge/links?path={path}
  GET /api/knowledge/orphans
  GET /api/knowledge/unresolved
  GET /api/knowledge/files?folder={folder}
  GET /api/knowledge/folders
  GET /api/knowledge/stats
  GET /api/knowledge/daily
  GET /api/knowledge/research/history
  GET /api/knowledge/research/stats
  ```
- All require auth middleware (following existing patterns in `server.go:100-174` using `s.authMiddleware()`, `writeJSON()`, `writeError()`)
- All delegate to VaultFacade
- Research endpoints reuse task DB (type=RESEARCH) for history/stats
- `knowledgeStats` handler makes 4 parallel CLI calls — use `errgroup` for concurrency
- No new DB tables: vault data queried live via CLI (Obsidian maintains its own index), research history reuses `tasks` table with `type=RESEARCH` filter

**Data flow**:
```
┌──────────────┐     IPC      ┌──────────────────┐
│ Obsidian App  │◄────────────│  obsidian CLI     │
│ (background)  │             └────────▲──────────┘
│ • indexing    │                      │
│ • search      │             ┌────────┴──────────┐
│ • link graph  │             │  Flux Process      │
└──────┬────────┘             │  vault/            │
       ▼                      │  ├── client.go     │
~/ObsidianVault/Flux/        │  ├── facade.go     │
├── Goals/                    │  └── fallback.go   │
├── Projects/                 │  server/           │
├── Research/                 │  └── api_knowledge │
├── Tasks/                    └────────┬───────────┘
└── Templates/                         │ HTTP
                              ┌────────┴───────────┐
                              │ Web Browser         │
                              │ Knowledge UI        │
                              └────────────────────┘
```

**Acceptance criteria**:
- [ ] All 15 endpoints return correct JSON
- [ ] Auth middleware applied
- [ ] VaultFacade used for all vault operations
- [ ] Stats endpoint uses parallel CLI calls (errgroup)
- [ ] Unit tests for handlers with mock VaultFacade

**Complexity**: Medium

**Depends on**: Task 3.13

---

### Task 3.15: Knowledge Frontend UI

**Description**: Knowledge page with sub-tabs for browsing, searching, and monitoring vault health, plus a Dashboard widget.

**Files to create/modify**:
```
frontend/src/pages/Knowledge.tsx                    (new)
frontend/src/pages/knowledge/Browse.tsx             (new, ~200 lines)
frontend/src/pages/knowledge/Search.tsx             (new, ~150 lines)
frontend/src/pages/knowledge/Research.tsx           (new, ~200 lines — basic, enhanced in Phase 4)
frontend/src/pages/knowledge/Graph.tsx              (new, ~100 lines)
frontend/src/pages/knowledge/Health.tsx             (new, ~100 lines)
frontend/src/stores/knowledgeStore.ts               (new, ~150 lines)
frontend/src/App.tsx                                (modify — add routes)
frontend/src/components/Layout.tsx                  (modify — add nav)
frontend/src/pages/Dashboard.tsx                    (modify — add widget)
```

**Implementation details**:
- Knowledge page with sub-tab routing:
  ```
  /knowledge              → Browse (default)
  /knowledge/search       → Search
  /knowledge/research     → Research History
  /knowledge/graph        → Tag/Link Graph
  /knowledge/health       → Vault Health
  ```
- **Browse**: File tree by folder + note preview with markdown rendering (react-markdown already in package.json)
- **Search**: Full-text search via Knowledge API, result highlighting
- **Research**: Research task timeline (type=RESEARCH tasks), active researcher agent status
- **Graph**: Tag cloud with counts + link relationship list (no d3.js, simple list view — Obsidian's native graph is already superior; the Web UI's purpose is quick access)
- **Health**: Orphan notes, broken links, vault statistics
- **Dashboard widget**: Knowledge summary card (note count, recent research, vault health)
- Zustand store for state management
- No new npm dependencies (react-markdown v10.1.0, remark-gfm v4.0.1, zustand v4.4.7 already available)

**Acceptance criteria**:
- [ ] Knowledge page with all 5 sub-tabs
- [ ] Browse renders file tree and note preview
- [ ] Search returns results with highlighting
- [ ] Research shows task timeline
- [ ] Health shows orphans and broken links
- [ ] Dashboard widget shows vault summary
- [ ] Responsive design

**Complexity**: High

**Depends on**: Task 3.14

---

## Phase 3 Completion Checklist

- [ ] Orchestrator runs with 5-minute tick cycle
- [ ] Agent scaling adjusts automatically based on workload
- [ ] Dev:R&D ratio follows spec
- [ ] Insights: daily metrics, agent performance, tool usage collected
- [ ] Insights UI with time-series charts and analytics
- [ ] Dynamic rate limit wait uses ccusage billing data
- [ ] Hourly usage snapshots in DB
- [ ] Per-task usage tracking
- [ ] Daily Discord summary at midnight
- [ ] Goal proposal system active
- [ ] Cleanup: JSONL, metrics, backups, disk monitoring
- [ ] Settings UI and system health metrics
- [ ] Graceful shutdown handles Go + Python
- [ ] Obsidian CLI Go backend (VaultFacade) with fallback
- [ ] Knowledge API (15 endpoints) serving vault data
- [ ] Knowledge UI with Browse, Search, Research, Graph, Health tabs
- [ ] Dashboard Knowledge widget

---

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~15 files | ~7 files |
| Frontend | ~12 files | ~5 files |
| Proto | ~1 file | — |
| **Total** | **~28 files** | **~12 files** |

---

## Appendix A: Insights API Response Schemas

### A.1 `GET /api/insights/summary`

Replaces the current `/api/insights` endpoint with richer aggregations. Implementation: single SQL query with subqueries and CASE aggregation from the `tasks` table.

```json
{
  "total_tokens": 1250000,
  "total_cost": 45.23,
  "tasks": {
    "total": 150,
    "completed": 120,
    "failed": 15,
    "cancelled": 5,
    "decomposed": 10,
    "success_rate": 0.88,
    "avg_completion_time_min": 18.5
  },
  "prs": {
    "total_created": 95,
    "merged": 82,
    "open": 8,
    "closed": 5,
    "merge_rate": 0.86
  },
  "today": {
    "tasks_completed": 8,
    "tasks_failed": 1,
    "tokens_used": 45000,
    "cost_usd": 1.80
  },
  "project_activities": [
    {
      "project_id": "...",
      "project_name": "flux",
      "task_count": 80,
      "completed": 65,
      "failed": 8,
      "tokens_used": 800000,
      "cost_usd": 30.00
    }
  ]
}
```

### A.2 `GET /api/insights/timeseries?period=7d&bucket=daily`

Parameters: `period` (24h, 7d, 30d, 90d; default: 7d), `bucket` (hourly, daily; default: daily).

SQL: `GROUP BY strftime('%Y-%m-%d', completed_at)` for daily; `strftime('%Y-%m-%d %H:00', completed_at)` for hourly.

```json
{
  "period": "7d",
  "bucket": "daily",
  "data": [
    {
      "timestamp": "2026-02-10",
      "tasks_completed": 12,
      "tasks_failed": 2,
      "tokens_used": 65000,
      "cost_usd": 2.50,
      "avg_completion_time_min": 15.2,
      "prs_created": 8
    }
  ]
}
```

### A.3 `GET /api/insights/efficiency`

SQL: Aggregation queries grouped by type, model, and priority ranges. All data exists in current schema.

```json
{
  "cost_per_completed_task": 0.38,
  "tokens_per_completed_task": 10416,
  "avg_diff_lines": 85,
  "avg_files_changed": 3.2,
  "by_type": [
    {"type": "CODING", "count": 80, "success_rate": 0.90, "avg_cost": 0.42, "avg_tokens": 12000},
    {"type": "BUGFIX", "count": 25, "success_rate": 0.84, "avg_cost": 0.35, "avg_tokens": 9000}
  ],
  "by_model": [
    {"model": "opus", "count": 30, "success_rate": 0.93, "avg_cost": 1.20, "avg_tokens": 25000},
    {"model": "sonnet", "count": 120, "success_rate": 0.87, "avg_cost": 0.25, "avg_tokens": 8000}
  ],
  "by_priority": [
    {"range": "1-10", "count": 10, "success_rate": 0.90, "avg_completion_time_min": 22},
    {"range": "11-40", "count": 60, "success_rate": 0.88, "avg_completion_time_min": 17},
    {"range": "41-100", "count": 80, "success_rate": 0.85, "avg_completion_time_min": 15}
  ]
}
```

### A.4 `GET /api/insights/pipeline`

SQL: Latency via `julianday(started_at) - julianday(created_at)` (queue wait) and `julianday(completed_at) - julianday(started_at)` (execution). P90 via `ORDER BY total_time LIMIT 1 OFFSET (count*0.9)` subquery. Decomposition stats from `parent_id` relationships.

```json
{
  "queue_depth": {
    "pending": 5,
    "ready": 3,
    "running": 2
  },
  "throughput": {
    "last_24h": 12,
    "last_7d": 65,
    "last_30d": 210
  },
  "latency": {
    "avg_queue_wait_min": 8.5,
    "avg_execution_min": 15.2,
    "avg_total_min": 23.7,
    "p90_total_min": 35.0
  },
  "retry_stats": {
    "tasks_retried": 15,
    "retry_success_rate": 0.73,
    "avg_retries_before_success": 1.2
  },
  "decomposition_stats": {
    "tasks_decomposed": 10,
    "avg_subtasks": 3.5,
    "subtask_success_rate": 0.92
  }
}
```

### A.5 `GET /api/insights/failures`

Failures from `tasks WHERE status = 'FAILED'`. Error categorization via keyword matching on `error_log` (see Task 3.3 for keyword → category mapping).

```json
{
  "total_failures": 15,
  "failure_rate": 0.10,
  "recent_failures": [
    {
      "task_id": "...",
      "title": "Implement auth middleware",
      "type": "CODING",
      "project_name": "my-api",
      "error_summary": "build failed: missing import",
      "failed_at": "2026-02-15T14:30:00Z",
      "model": "sonnet",
      "retry_count": 2,
      "was_retried": true,
      "retry_succeeded": false
    }
  ],
  "by_error_category": [
    {"category": "build_failure", "count": 6},
    {"category": "test_failure", "count": 4},
    {"category": "rate_limit", "count": 3},
    {"category": "timeout", "count": 1},
    {"category": "other", "count": 1}
  ],
  "by_project": [
    {"project_id": "...", "project_name": "flux", "failure_count": 8, "failure_rate": 0.12}
  ]
}
```

---

## Appendix B: Insights Technical Decisions

### B.1 No New Tables

All insight data is derived from existing `tasks`, `usage_snapshots`, and `rate_limit_events` tables. This avoids schema migrations and keeps the system simple. SQLite's aggregation functions (`COUNT`, `SUM`, `AVG`, `GROUP BY strftime`) are sufficient for all metrics.

### B.2 Compute on Read (Not Materialized)

Insight queries run against live data on every request. With Flux's expected data volume (hundreds to low thousands of tasks), SQLite can handle these aggregations in milliseconds. Materialized views or cron-computed summaries are unnecessary complexity at this scale. If performance becomes a concern later, add a simple in-memory cache with a 60-second TTL.

### B.3 recharts Over Custom SVG

The Insight page benefits from interactive, professional charts. `recharts` is React-native (composable components), well-maintained (14M weekly npm downloads), small enough (~45KB gzipped), and provides tooltips, responsive containers, and animations out of the box. Pure CSS bars would be inadequate for time-series line charts and area charts.

### B.4 Error Categorization via Keyword Matching

Rather than structured error codes (which would require executor changes), failure categorization uses simple string matching on the `error_log` field. This works because executor error messages are generated by Flux itself (not arbitrary user input) and follow predictable patterns. See `executor/executor.go` for error generation patterns.

### B.5 Backward Compatibility

The existing `GET /api/insights` endpoint is preserved as-is. New endpoints use the `/api/insights/` prefix with subpaths. The frontend Dashboard can migrate to the new summary endpoint while the old one remains available.

---

## Appendix C: Insights Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SQLite aggregation performance on large datasets | Slow page loads | Add composite index on `(status, completed_at)` if needed; cache with 60s TTL |
| `recharts` bundle size bloat | Larger frontend build | Tree-shake; only import needed chart types |
| Time zone handling in SQLite | Incorrect daily bucketing | Use UTC throughout; SQLite `datetime()` is UTC by default |
| Empty state (new installations) | Confusing UI with all zeros | Show "Not enough data yet" placeholders when < 5 tasks exist |

---

## Appendix D: Obsidian Integration — External Dependencies

| Dependency | Version | Status | Risk |
|-----------|---------|--------|------|
| Obsidian Desktop | v1.12+ | Released 2026-02-10 | Low — stable release |
| Obsidian CLI | Built-in to 1.12 | Available | Low — official feature |
| Catalyst License | $25 one-time | Required for Early Access | Low — one-time cost |
| macOS LaunchAgent | Standard | Required for background Obsidian | Low — well-understood |
| `react-markdown` | v10.1.0 | Already in package.json | None |
| `remark-gfm` | v4.0.1 | Already in package.json | None |

No new Go or npm dependencies required for the Obsidian integration layer.

---

## Appendix E: Obsidian Integration — Test Strategy

### E.1 Unit Tests (Go)

| Component | Test File | What to Test |
|-----------|-----------|-------------|
| `vault.Client` | `client_test.go` | Mock `exec.Command` to test all CLI operations: read, create, search, tags, backlinks, etc. Test error handling for CLI failures, timeout, and empty results. |
| `vault.VaultFacade` | `facade_test.go` | Test CLI-first / Writer-fallback logic. Test with CLI available and unavailable. Test DailyNote mutex prevents concurrent writes. |
| `vault.Fallback*` | `fallback_test.go` | Test grep-based search, YAML frontmatter parsing, tag extraction on real markdown files in `t.TempDir()`. |
| `server.Knowledge` | `api_knowledge_test.go` | HTTP handler tests using `httptest.NewRecorder`. Mock VaultFacade. Test auth middleware, query parameters, error responses. |

Follow existing test patterns in `go/src/internal/vault/writer_test.go` and `go/src/internal/server/server_test.go`.

### E.2 Integration Tests (Go)

| Test | Description |
|------|-------------|
| Facade + real filesystem | Write via Facade with CLI unavailable → verify file written. Read back. |
| API → Facade → filesystem | Call Knowledge API endpoint → verify response matches actual Vault content. |
| Bootstrap health check | Start with CLI unavailable → verify warning logged, fallback mode active. |

### E.3 Edge Cases

| Edge Case | Expected Behavior |
|-----------|-------------------|
| Obsidian app not running | Client.IsAvailable() returns false → all operations use fallback |
| Obsidian CLI not installed | Same as above — exec fails, fallback mode |
| Obsidian app crashes mid-operation | CLI exec timeout (30s) → return error → Facade falls back to Writer |
| Empty vault (no notes) | Search returns empty. Tags returns empty. Files returns empty. UI shows "No notes yet." |
| Very large vault (10k+ notes) | Search uses Obsidian index (fast). File listing paginated by folder. |
| Concurrent daily note appends | VaultFacade.mu Mutex protects DailyAppend |
| Note path with special characters | URL-encode paths in API queries. Facade handles path escaping for CLI. |
| CLI silent failure (exit 0 no output) | Retry logic: 3 attempts with 2s delay. Log warning if all fail. |
| Network timeout on API endpoints | HTTP handler returns 504 with descriptive error |

### E.4 Frontend Testing

- Component tests for Knowledge sub-tabs using React Testing Library
- Zustand store tests for API call mocking and state transitions
- No E2E tests for Phase 3 (manual verification sufficient for UI)

---

## Appendix F: Obsidian Integration — Risk Mitigation

### F.1 Backward Compatibility

| Risk | Mitigation |
|------|-----------|
| Breaking existing VaultWriter consumers | VaultFacade has identical `Write()` signature. Existing `Writer` retained and used as fallback. |
| Breaking executor vault writes | Facade is a drop-in replacement — executors don't need code changes. |
| API namespace collision | All new endpoints under `/api/knowledge/*` — no overlap with existing `/api/tasks/*`, `/api/projects/*`, etc. |
| Frontend route collision | `/knowledge` and `/knowledge/*` are new routes — no overlap. |
| Config change | `vault.name` is additive — existing `vault.path` unchanged. Empty `vault.name` means CLI is skipped. |

### F.2 Data Consistency

| Risk | Mitigation |
|------|-----------|
| Stale CLI results after file write | Obsidian indexes in near real-time. The `silent` flag on create prevents UI pop-ups but indexing still happens. |
| Concurrent file modifications | VaultFacade serializes daily note writes via mutex. Other writes are append-only (task summaries, research findings). |
| CLI returns partial data during index rebuild | Retry with delay. Fallback functions provide degraded but consistent results. |

### F.3 Performance

| Concern | Mitigation |
|---------|-----------|
| CLI exec overhead (~100ms per call) | Acceptable for API queries. Knowledge API is user-facing (not hot path). |
| Large search results | `limit` parameter on search API. Default 20. |
| Tag cloud computation | Cached in Zustand store. Refreshed on page visit, not on every render. |
| Many sequential CLI calls (stats endpoint) | `knowledgeStats` handler makes 4 parallel CLI calls via `errgroup`. |

---

## Appendix G: Obsidian LaunchAgent Configuration

To ensure Obsidian runs in the background on the Mac Mini:

```xml
<!-- ~/Library/LaunchAgents/md.obsidian.app.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>md.obsidian.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>open</string>
        <string>-a</string>
        <string>Obsidian</string>
    </array>
    <key>RunAtLoad</key><true/>
</dict>
</plist>
```
