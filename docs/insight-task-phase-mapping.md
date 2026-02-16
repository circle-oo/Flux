# Insight Task Phase Mapping

## Overview

This document maps the 5 tasks from `insight-plan.md` to the appropriate implementation phases (2B, 3, or 4) based on prerequisites, complexity, and dependencies. The Insight feature adds comprehensive analytics and observability to Flux.

---

## Summary Table

| Task | Title | Phase | Justification |
|------|-------|-------|---------------|
| Task 1 | Backend — Insight Query Methods | **Phase 2B** | Foundation queries, no dependencies, extends existing models |
| Task 2 | Backend — Insight API Endpoints | **Phase 2B** | Builds on Task 1, completes backend foundation |
| Task 3 | Frontend — Insight Store & API Client | **Phase 3** | Requires Phase 2B backend complete, enables Phase 3 UI work |
| Task 4 | Frontend — Insight Page | **Phase 3** | Main deliverable, requires usage data from Phase 3 orchestration |
| Task 5 | Dashboard Enhancement | **Phase 3** | Enhancement to existing dashboard, requires Task 2 + Task 4 |

---

## Detailed Task Analysis

### Task 1: Backend — Insight Query Methods
**Phase Assignment**: **Phase 2B** (Pipeline Hardening)

#### Affected Modules
- **Go Backend**: `internal/models/insights.go` (new file)
- **Database**: No schema changes (queries existing tables: `tasks`, `usage_snapshots`, `rate_limit_events`)

#### Files to Create/Modify
- Create: `go/src/internal/models/insights.go`
- Query methods: `GetSummary()`, `GetTimeSeries()`, `GetEfficiency()`, `GetPipelineHealth()`, `GetFailureAnalysis()`

#### Prerequisites
- Existing tables: `tasks`, `usage_snapshots`, `rate_limit_events`, `projects`
- Existing indexes: `idx_tasks_status_priority`, `idx_tasks_project`
- **All prerequisites met** — Phase 2B has these

#### Dependencies
- None (foundation task)

#### Phase Placement Justification
**Phase 2B is appropriate because:**
1. **No external dependencies** — all required tables exist after Phase 2A
2. **Foundation for observability** — these queries enable all downstream insight work
3. **Low complexity** — pure SQL aggregation logic, no API or UI work
4. **Aligns with Phase 2B goals** — "Pipeline Hardening" includes making the system observable and reliable
5. **Natural grouping** — Phase 2B already includes usage tracking experiments (Task 2B.9), making this a logical extension

#### Estimated Complexity
- Medium (5 query methods with aggregations and time-bucketing)
- ~200-300 lines of Go code
- SQLite aggregation testing required

---

### Task 2: Backend — Insight API Endpoints
**Phase Assignment**: **Phase 2B** (Pipeline Hardening)

#### Affected Modules
- **Go Backend**:
  - `internal/server/api_insights.go` (new file)
  - `internal/server/server.go` (route registration)

#### Files to Create/Modify
- Create: `go/src/internal/server/api_insights.go`
- Modify: `go/src/internal/server/server.go` (add 5 routes in `setupRoutes`)

#### API Endpoints
```
GET /api/insights/summary      — Enhanced summary with rich aggregations
GET /api/insights/timeseries   — Time-bucketed metrics for charting
GET /api/insights/efficiency   — Per-type, per-model, per-priority breakdowns
GET /api/insights/pipeline     — Queue depth, throughput, latency
GET /api/insights/failures     — Failure analysis and categorization
```

#### Prerequisites
- Task 1 complete (query methods exist)
- Existing `internal/server/server.go` route setup patterns
- Auth middleware (from Phase 1/2A)

#### Dependencies
- **Blocks**: Task 1 (must have query methods)
- **Blocked by**: None

#### Phase Placement Justification
**Phase 2B is appropriate because:**
1. **Natural continuation** — completes the backend foundation started in Task 1
2. **No Phase 3 dependencies** — doesn't require Orchestrator, ScaleManager, or UsageCollector
3. **Enables testing** — backend can be tested independently before UI work begins
4. **Backward compatible** — preserves existing `/api/insights` endpoint
5. **Phase boundary** — completing Tasks 1+2 in Phase 2B creates a clean API surface for Phase 3 frontend work
6. **Aligns with hardening** — exposing operational metrics via API is a hardening/observability task

#### Estimated Complexity
- Medium (5 HTTP handlers with parameter validation)
- ~300-400 lines of Go code
- Handler unit tests required

---

### Task 3: Frontend — Insight Store & API Client
**Phase Assignment**: **Phase 3** (Orchestration)

#### Affected Modules
- **React Frontend**:
  - `frontend/src/stores/insightStore.ts` (new Zustand store)
  - `frontend/src/lib/api.ts` (add API methods)

#### Files to Create/Modify
- Create: `frontend/src/stores/insightStore.ts`
- Modify: `frontend/src/lib/api.ts`

#### Store Functionality
- TypeScript interfaces for all 5 insight endpoints
- State management for: summary, timeseries, efficiency, pipeline, failures
- Loading/error states
- Period/bucket selection (24h/7d/30d/90d, hourly/daily)
- Auto-refresh on period change

#### Prerequisites
- Task 2 complete (API endpoints exist)
- Existing Zustand patterns (from Phase 1)
- TypeScript project setup

#### Dependencies
- **Blocks**: Task 4, Task 5 (UI pages need this store)
- **Blocked by**: Task 2

#### Phase Placement Justification
**Phase 3 is appropriate because:**
1. **Phase boundary separation** — Phase 2B focuses on backend hardening; Phase 3 adds autonomous operation and monitoring
2. **Timing with usage data** — Phase 3 adds `UsageCollector` (Task 3.4), which populates the time-series data that Insight displays
3. **Orchestrator context** — Insight page shows pipeline health, which is managed by Phase 3 Orchestrator
4. **Natural frontend grouping** — Phase 3 adds multiple frontend pages (Usage UI, Settings enhancement, Insight page)
5. **Data availability** — Phase 3's hourly usage collection provides meaningful time-series data to display

#### Estimated Complexity
- Low (~150-200 lines of TypeScript)
- Standard Zustand patterns, straightforward API client work

---

### Task 4: Frontend — Insight Page
**Phase Assignment**: **Phase 3** (Orchestration)

#### Affected Modules
- **React Frontend**:
  - `frontend/src/pages/Insight.tsx` (new page)
  - `frontend/src/App.tsx` (add route)
  - `frontend/src/components/Sidebar.tsx` (add navigation link)

#### Files to Create/Modify
- Create: `frontend/src/pages/Insight.tsx`
- Modify: `frontend/src/App.tsx`, `frontend/src/components/Sidebar.tsx`

#### Page Sections
1. **Summary Cards** — Today's stats, all-time stats, trend indicators
2. **Time-Series Chart** — Task completion and cost trends (using `recharts`)
3. **Efficiency Table** — Breakdowns by type, model, priority
4. **Pipeline Health** — Queue depth, throughput, latency metrics
5. **Failure Analysis** — Recent failures, error categorization

#### Dependencies Required
- `recharts` npm package (~45KB gzipped)

#### Prerequisites
- Task 3 complete (store and API client exist)
- Phase 3 usage data collection active (provides meaningful data)
- Existing Tailwind styling patterns

#### Dependencies
- **Blocks**: Task 5 (Dashboard needs Insight page link)
- **Blocked by**: Task 3
- **Soft dependency**: Phase 3 Task 3.4 (UsageCollector provides time-series data)

#### Phase Placement Justification
**Phase 3 is appropriate because:**
1. **Main deliverable of Insight feature** — the primary user-facing component
2. **Requires live operational data** — Phase 3's UsageCollector (3.4), ScaleManager (3.2), and Orchestrator (3.1) provide the data that makes this page useful
3. **Complements Phase 3 Usage UI** — both Insight and Usage pages show system health/metrics
4. **Autonomous operation visibility** — Phase 3's goal is "autonomous operation"; Insight page provides visibility into that autonomy
5. **Largest frontend task** — Phase 3 already has significant frontend work (Usage UI 3.13, Settings 3.16), grouping Insight here balances the workload
6. **Time-series data** — Without Phase 3's hourly collection, time-series charts would be empty or misleading

#### Estimated Complexity
- High (largest task: 5 sections, charts, responsive layout)
- ~400-600 lines of TypeScript/TSX
- Recharts integration and responsive design

---

### Task 5: Dashboard Enhancement
**Phase Assignment**: **Phase 3** (Orchestration)

#### Affected Modules
- **React Frontend**: `frontend/src/pages/Dashboard.tsx`

#### Files to Create/Modify
- Modify: `frontend/src/pages/Dashboard.tsx` (lines 270-328, replace flat Insights section)

#### Enhancement Details
- Replace existing flat Insights section with:
  - Compact summary cards from `/api/insights/summary`
  - Today's completions, cost, success rate
  - Trend indicators (↑↓) for day-over-day comparison
  - "View Details →" link to full Insight page

#### Prerequisites
- Task 2 complete (summary endpoint exists)
- Task 4 complete (Insight page link target exists)
- Existing Dashboard.tsx structure

#### Dependencies
- **Blocks**: None (terminal task)
- **Blocked by**: Task 2, Task 4

#### Phase Placement Justification
**Phase 3 is appropriate because:**
1. **Depends on Insight page** — needs Task 4 complete for the "View Details" link
2. **Enhancement, not foundation** — upgrades existing Dashboard rather than building new infrastructure
3. **Can run in parallel with Task 4** — both modify frontend after Task 3 complete
4. **Low priority** — Dashboard already has basic insights from Phase 1; this is a polish task
5. **Completes Insight feature** — natural endpoint after main Insight page is built
6. **Minimal work** — small modification to existing component

#### Estimated Complexity
- Low (~50-100 lines modified)
- Simple component update with existing patterns

---

## Phase Grouping Rationale

### Phase 2B: Backend Foundation (Tasks 1-2)
**Theme**: "Pipeline Hardening — Make system observable"

- **Focus**: Backend query methods and API endpoints
- **No UI work** — keeps Phase 2B backend-focused
- **Clean API boundary** — Phase 2B completes the backend, Phase 3 consumes it
- **Testable independently** — backend can be tested without UI

**Phase 2B Goals Alignment**:
- Adds observability (insight into system behavior)
- Enables monitoring (failure analysis, pipeline health)
- Supports reliability (understanding what's happening)

### Phase 3: Frontend & Integration (Tasks 3-5)
**Theme**: "Orchestration — Autonomous operation with visibility"

- **Focus**: Frontend components that display operational data
- **Requires Phase 3 data** — UsageCollector provides time-series data
- **Complements Phase 3 UI work** — groups with Usage UI (3.13), Settings (3.16)
- **Visibility into autonomy** — Insight page shows how autonomous operation is performing

**Phase 3 Goals Alignment**:
- Autonomous operation visibility (Insight page shows system health)
- Usage tracking integration (time-series display)
- Operator oversight (dashboard enhancements)

---

## Implementation Order Within Phases

### Phase 2B Order
```
Task 1 (Query Methods)       ← First (foundation, no dependencies)
  └── Task 2 (API Endpoints) ← Second (completes backend)
```

**Rationale**: Task 1 must complete before Task 2. Both should be done together in Phase 2B to create a clean, testable API surface.

### Phase 3 Order
```
[Phase 2B complete]
  └── Task 3 (Store + API Client) ← First frontend task
      ├── Task 4 (Insight Page)    ← Can start after Task 3
      └── Task 5 (Dashboard)       ← Can start after Task 3 + Task 4
```

**Rationale**:
- Task 3 must complete first (provides data layer)
- Task 4 and Task 5 both depend on Task 3
- Task 5 also depends on Task 4 (needs Insight page for link)
- **Parallelization opportunity**: After Task 3, Task 4 and Task 5 can be developed concurrently, with Task 5 waiting for Task 4's route to merge

---

## Risk & Dependencies Summary

### Critical Path
```
Task 1 → Task 2 → Task 3 → Task 4 → Task 5
```

### Phase Boundaries
- **Phase 2B completion**: Tasks 1-2 complete
- **Phase 3 start dependency**: Phase 2B complete
- **Phase 3 completion**: Tasks 3-5 complete

### Key Risks

#### Phase 2B Risks
1. **SQLite aggregation performance** — Large datasets could slow queries
   - **Mitigation**: Add composite index `(status, completed_at)` if needed; 60s TTL cache
2. **Time zone handling** — Incorrect daily bucketing
   - **Mitigation**: Use UTC throughout; SQLite `datetime()` defaults to UTC

#### Phase 3 Risks
1. **Empty state** — New installations have no time-series data
   - **Mitigation**: Show "Not enough data yet" placeholders when < 5 tasks exist
2. **Recharts bundle size** — Adds ~45KB gzipped to frontend
   - **Mitigation**: Tree-shake imports; only import needed chart types
3. **Timing with usage data** — Insight page less useful without Phase 3 usage collection
   - **Mitigation**: Ensure Task 3.4 (UsageCollector) runs before Insight page development

---

## File Count by Phase

### Phase 2B (Backend)
| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | 1 | 1 |
| **Total** | **1** | **1** |

- New: `internal/models/insights.go`, `internal/server/api_insights.go`
- Modified: `internal/server/server.go`

### Phase 3 (Frontend)
| Category | New Files | Modified Files |
|----------|-----------|----------------|
| React frontend | 2 | 3 |
| **Total** | **2** | **3** |

- New: `frontend/src/stores/insightStore.ts`, `frontend/src/pages/Insight.tsx`
- Modified: `frontend/src/lib/api.ts`, `frontend/src/App.tsx`, `frontend/src/components/Sidebar.tsx`, `frontend/src/pages/Dashboard.tsx`

### Combined Total
| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | 1 | 1 |
| React frontend | 2 | 3 |
| **Total** | **3** | **4** |

**Estimated total diff**: ~800-1200 lines (matches insight-plan.md estimate)

---

## SQLite Schema Analysis

### Existing Tables Used (No Changes Required)
1. **`tasks`** — Primary data source
   - Used for: summary, timeseries, efficiency, pipeline health, failures
   - Indexes: `idx_tasks_status_priority`, `idx_tasks_project`
2. **`usage_snapshots`** — Usage tracking data
   - Used for: timeseries (token/cost trends)
   - Index: `idx_usage_snapshots_type_time`
3. **`rate_limit_events`** — Rate limit history
   - Used for: failure analysis (rate limit categorization)
4. **`projects`** — Project metadata
   - Used for: project activity summary

### No Schema Migrations Required
- All Insight queries run against existing tables
- No new columns needed
- No new tables needed
- Existing indexes sufficient for query performance

---

## Success Criteria by Phase

### Phase 2B Success Criteria
- [ ] All 5 query methods return correct results
- [ ] Queries use existing indexes
- [ ] Null-safe handling (COALESCE for empty results)
- [ ] All 5 API endpoints return correct JSON
- [ ] Query parameters validated (period, bucket)
- [ ] Auth middleware applied
- [ ] Existing `/api/insights` still works (backward compatible)
- [ ] Unit tests for query methods and handlers

### Phase 3 Success Criteria
- [ ] TypeScript interfaces match API response shapes
- [ ] Zustand store with loading/error states
- [ ] Period selector state management
- [ ] Insight page renders all 5 sections
- [ ] Charts display time-series data correctly
- [ ] Period selector updates all sections
- [ ] Responsive layout (mobile-friendly)
- [ ] Consistent styling with existing pages (Tailwind, dark theme)
- [ ] Navigation link in sidebar
- [ ] Dashboard shows today's key metrics from summary endpoint
- [ ] Trend indicators for day-over-day comparison
- [ ] Link to full Insight page
- [ ] No regression in existing dashboard functionality

---

## Conclusion

**Recommended Phase Mapping:**
- **Phase 2B**: Tasks 1-2 (Backend foundation)
- **Phase 3**: Tasks 3-5 (Frontend integration)

**Rationale**: This mapping creates a clean separation of concerns, aligns with existing phase goals, ensures data availability when needed, and groups related work together for efficient implementation.

**Critical Path Duration**: 5 tasks sequential, estimated 2-3 weeks total (1 week Phase 2B backend, 1-2 weeks Phase 3 frontend)

**Parallelization Opportunity**: After Task 3 (Store), Tasks 4 and 5 can be developed concurrently, reducing Phase 3 duration.
