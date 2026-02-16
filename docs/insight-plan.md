# Insight: Comprehensive Analytics & Observability for Flux

## Problem Statement

Flux's current "insights" feature (`server/api_system.go:109-173`) is a minimal aggregation endpoint that returns only three metrics: total tokens, total cost, and task count per project. The dashboard (`frontend/src/pages/Dashboard.tsx:270-328`) renders these as static numbers with no time context, trends, or actionable intelligence.

For an autonomous engineering system running 24/7, the operator needs deep observability to answer questions like:
- Is the system getting more efficient over time?
- Which projects consume the most resources relative to output?
- What's the success rate, and is it improving or degrading?
- When do failures cluster, and what causes them?
- How is the task pipeline performing (throughput, latency, bottlenecks)?

**Insight** transforms Flux from "black box with counters" into a system with rich operational intelligence.

---

## Scope

| Layer | Affected | Changes |
|-------|----------|---------|
| Go backend | Yes | New API endpoints, new query methods, new aggregation logic |
| React frontend | Yes | New Insight page, charts/visualizations, enhanced dashboard |
| SQLite database | No | Existing schema is sufficient (tasks, usage_snapshots, rate_limit_events) |
| Executor/Orchestrator | No | No changes to execution pipeline |

---

## Design

### Architecture

```
┌─────────────────────────────────────────────────────┐
│                  Frontend (React)                     │
│                                                       │
│  Dashboard (enhanced)    Insight Page (new)           │
│  - Sparklines            - Time-series charts          │
│  - Trend indicators      - Efficiency metrics          │
│  - Quick stats           - Project comparisons         │
│                          - Failure analysis             │
│                          - Pipeline health              │
└──────────────────────┬────────────────────────────────┘
                       │ HTTP API
┌──────────────────────┴────────────────────────────────┐
│                  Backend (Go)                          │
│                                                       │
│  GET /api/insights/summary     (enhanced)             │
│  GET /api/insights/timeseries  (new)                  │
│  GET /api/insights/efficiency  (new)                  │
│  GET /api/insights/pipeline    (new)                  │
│  GET /api/insights/failures    (new)                  │
│                                                       │
│  internal/server/api_insights.go (new file)           │
│  internal/models/insights.go     (new query methods)  │
└──────────────────────┬────────────────────────────────┘
                       │ SQL queries
┌──────────────────────┴────────────────────────────────┐
│            SQLite (existing tables)                    │
│  tasks, usage_snapshots, rate_limit_events, projects  │
└───────────────────────────────────────────────────────┘
```

### API Design

#### 1. `GET /api/insights/summary`

Replaces the current `/api/insights` endpoint with richer aggregations.

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

**Implementation**: Single SQL query with subqueries and CASE aggregation from the `tasks` table. No new tables needed.

#### 2. `GET /api/insights/timeseries?period=7d&bucket=daily`

Returns time-bucketed metrics for charting.

Parameters:
- `period`: `24h`, `7d`, `30d`, `90d` (default: `7d`)
- `bucket`: `hourly`, `daily` (default: `daily`)

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

**Implementation**: `GROUP BY strftime('%Y-%m-%d', completed_at)` on `tasks` table. For hourly buckets, use `strftime('%Y-%m-%d %H:00', completed_at)`. SQLite's date functions are sufficient.

#### 3. `GET /api/insights/efficiency`

Efficiency and resource metrics for optimization.

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

**Implementation**: Aggregation queries on `tasks` table grouped by type, model, and priority ranges. All data already exists in the current schema.

#### 4. `GET /api/insights/pipeline`

Task pipeline health metrics — throughput and bottleneck detection.

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

**Implementation**: Latency computed as `julianday(started_at) - julianday(created_at)` (queue wait) and `julianday(completed_at) - julianday(started_at)` (execution time). P90 via `ORDER BY ... LIMIT 1 OFFSET (count*0.9)` subquery. Decomposition stats from `parent_id` relationships.

#### 5. `GET /api/insights/failures`

Failure analysis for debugging systemic issues.

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

**Implementation**: Failures queried from `tasks WHERE status = 'FAILED'`. Error categorization via simple keyword matching on `error_log` field (e.g., "build failed" → build_failure, "test failed" → test_failure, "rate limit" → rate_limit, "timeout" → timeout).

---

### Frontend Design

#### New: Insight Page (`frontend/src/pages/Insight.tsx`)

A dedicated analytics page with multiple sections:

1. **Summary Cards** — Today's stats, all-time stats, trend indicators (↑↓)
2. **Time-Series Chart** — Tasks completed/failed over time, cost over time (using a lightweight chart library like `recharts` or pure SVG/CSS bars)
3. **Efficiency Table** — Breakdown by type, model, and priority range
4. **Pipeline Health** — Queue depth, throughput numbers, latency metrics
5. **Failure Analysis** — Recent failures list with error categorization, failure rate by project

#### Enhanced: Dashboard (`frontend/src/pages/Dashboard.tsx`)

Minimal additions to existing dashboard:
- Replace current flat "Insights" section with summary cards from `/api/insights/summary`
- Add small sparkline or trend indicator for today vs. yesterday

#### New: Insight Store (`frontend/src/stores/insightStore.ts`)

Zustand store for insight data fetching and caching.

#### Frontend Dependencies

For charts, evaluate two options:
- **Option A (Recommended)**: `recharts` — React-native charting library, widely used, ~45KB gzipped. Provides BarChart, LineChart, AreaChart out of the box.
- **Option B**: Pure CSS/SVG bars — Zero dependencies, simpler, but limited to bar charts and manual scaling.

Decision: Use `recharts` — the insight page benefits from real interactive charts, and the dependency is well-maintained with a small footprint.

---

## Task Breakdown

### Task 1: Backend — Insight Query Methods
**Files to create**: `go/src/internal/models/insights.go`
**Files to modify**: None

Add query methods to a new `InsightStore` (or extend `TaskStore`):
- `GetSummary() (*InsightSummary, error)` — aggregates from tasks table
- `GetTimeSeries(period, bucket string) ([]TimeSeriesBucket, error)` — time-bucketed metrics
- `GetEfficiency() (*EfficiencyReport, error)` — per-type, per-model, per-priority breakdowns
- `GetPipelineHealth() (*PipelineHealth, error)` — queue depth, throughput, latency
- `GetFailureAnalysis(limit int) (*FailureAnalysis, error)` — failure patterns

All queries run against existing tables. No schema changes.

**Acceptance criteria**:
- [ ] All 5 query methods return correct results
- [ ] Queries use existing indexes (`idx_tasks_status_priority`, `idx_tasks_project`)
- [ ] Null-safe handling (COALESCE for empty results)
- [ ] Unit tests with test database

**Complexity**: Medium

---

### Task 2: Backend — Insight API Endpoints
**Files to create**: `go/src/internal/server/api_insights.go`
**Files to modify**: `go/src/internal/server/server.go` (add routes)

Implement 5 HTTP handlers:
- `handleInsightSummary` — GET /api/insights/summary
- `handleInsightTimeSeries` — GET /api/insights/timeseries
- `handleInsightEfficiency` — GET /api/insights/efficiency
- `handleInsightPipeline` — GET /api/insights/pipeline
- `handleInsightFailures` — GET /api/insights/failures

Deprecate (but keep) the existing `GET /api/insights` for backward compatibility.

Register routes in `server.go:93` (`setupRoutes`) following existing patterns with auth middleware.

**Acceptance criteria**:
- [ ] All 5 endpoints return correct JSON
- [ ] Query parameters validated (period, bucket)
- [ ] Auth middleware applied
- [ ] Existing `/api/insights` still works (backward compatible)
- [ ] Unit tests for handlers

**Depends on**: Task 1
**Complexity**: Medium

---

### Task 3: Frontend — Insight Store & API Client
**Files to create**: `frontend/src/stores/insightStore.ts`
**Files to modify**: `frontend/src/lib/api.ts` (add API methods)

Add TypeScript interfaces and API methods for all 5 insight endpoints.
Create Zustand store with:
- State for each data section (summary, timeseries, efficiency, pipeline, failures)
- Fetch methods with loading states
- Period/bucket selection state
- Auto-refresh on period change

**Acceptance criteria**:
- [ ] TypeScript interfaces match API response shapes
- [ ] API methods for all 5 endpoints
- [ ] Zustand store with loading/error states
- [ ] Period selector state management

**Depends on**: Task 2
**Complexity**: Low

---

### Task 4: Frontend — Insight Page
**Files to create**: `frontend/src/pages/Insight.tsx`
**Files to modify**: `frontend/src/App.tsx` (add route), `frontend/src/components/Sidebar.tsx` (add nav)

Build the Insight page with sections:
1. Summary cards row (today's stats + all-time)
2. Time-series chart area (task completion trend, cost trend)
3. Efficiency breakdown tables (by type, model, priority)
4. Pipeline health panel (queue, throughput, latency)
5. Failure analysis section (recent failures, error categories)

Install `recharts` for chart visualizations.

Period selector: 24h / 7d / 30d / 90d buttons at the top.

**Acceptance criteria**:
- [ ] Insight page renders all 5 sections
- [ ] Charts display time-series data correctly
- [ ] Period selector updates all sections
- [ ] Responsive layout (mobile-friendly)
- [ ] Consistent styling with existing pages (Tailwind, dark theme)
- [ ] Navigation link in sidebar

**Depends on**: Task 3
**Complexity**: High (largest frontend task)

---

### Task 5: Dashboard Enhancement
**Files to modify**: `frontend/src/pages/Dashboard.tsx`

Replace the current flat Insights section (`Dashboard.tsx:270-328`) with:
- Compact summary cards from `/api/insights/summary` (today's completions, cost, success rate)
- Trend indicators (↑↓) comparing today to yesterday (if timeseries data available)
- "View Details →" link to the Insight page

Keep it minimal — the Dashboard should be a quick glance, not a full analytics view.

**Acceptance criteria**:
- [ ] Dashboard shows today's key metrics from summary endpoint
- [ ] Trend indicators for day-over-day comparison
- [ ] Link to full Insight page
- [ ] No regression in existing dashboard functionality

**Depends on**: Task 2, Task 4
**Complexity**: Low

---

## Implementation Order

```
Task 1 (Query Methods)       ← Foundation, no dependencies
  └── Task 2 (API Endpoints) ← Backend complete
      └── Task 3 (Store + API Client) ← Frontend data layer
          └── Task 4 (Insight Page)    ← Main deliverable
          └── Task 5 (Dashboard)       ← Enhancement
```

Tasks 4 and 5 can run in parallel once Task 3 is done.

**Critical path**: Task 1 → Task 2 → Task 3 → Task 4

---

## Technical Decisions

### 1. No New Tables
All insight data is derived from existing `tasks`, `usage_snapshots`, and `rate_limit_events` tables. This avoids schema migrations and keeps the system simple. SQLite's aggregation functions (`COUNT`, `SUM`, `AVG`, `GROUP BY strftime`) are sufficient for all metrics.

### 2. Compute on Read (Not Materialized)
Insight queries run against live data on every request. With Flux's expected data volume (hundreds to low thousands of tasks), SQLite can handle these aggregations in milliseconds. Materialized views or cron-computed summaries are unnecessary complexity at this scale.

If performance becomes a concern later, add a simple in-memory cache with a 60-second TTL.

### 3. recharts Over Custom SVG
The Insight page benefits from interactive, professional charts. `recharts` is:
- React-native (composable components)
- Well-maintained (14M weekly npm downloads)
- Small enough (~45KB gzipped)
- Provides tooltips, responsive containers, and animations out of the box

The alternative (pure CSS bars) would be adequate for simple metrics but inadequate for time-series line charts and area charts.

### 4. Error Categorization via Keyword Matching
Rather than structured error codes (which would require executor changes), failure categorization uses simple string matching on the `error_log` field:
- Contains "build" → `build_failure`
- Contains "test" → `test_failure`
- Contains "rate limit" → `rate_limit`
- Contains "timeout" or "context deadline" → `timeout`
- Else → `other`

This works because executor error messages are generated by Flux itself (not arbitrary user input) and follow predictable patterns. See `executor/executor.go` for error generation patterns.

### 5. Backward Compatibility
The existing `GET /api/insights` endpoint is preserved as-is. The new endpoints use the `/api/insights/` prefix (with subpaths). The frontend Dashboard can migrate to the new summary endpoint while the old one remains available.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| SQLite aggregation performance on large datasets | Slow page loads | Add composite index on `(status, completed_at)` if needed; cache with 60s TTL |
| `recharts` bundle size bloat | Larger frontend build | Tree-shake; only import needed chart types |
| Time zone handling in SQLite | Incorrect daily bucketing | Use UTC throughout; SQLite `datetime()` is UTC by default |
| Empty state (new installations) | Confusing UI with all zeros | Show "Not enough data yet" placeholders when < 5 tasks exist |

---

## Success Metrics

1. **Operator can answer "how is the system performing?" in < 10 seconds** — Summary cards provide instant overview
2. **Trends are visible** — Time-series charts show completion rates and costs over 7/30/90 day windows
3. **Failures are diagnosable** — Error categorization and recent failure list enable quick triage
4. **Resource efficiency is measurable** — Cost per task, tokens per task, breakdown by model and type
5. **Pipeline bottlenecks are detectable** — Queue wait times, execution times, retry rates

---

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | 2 | 1 |
| React frontend | 2 | 3 |
| **Total** | **4** | **4** |

**Estimated diff**: ~800-1200 lines across all tasks.
