# Obsidian Integration Plan for Flux

> Comprehensive plan for integrating Obsidian CLI (1.12+) into the Flux autonomous engineering system.
> This plan supersedes and extends `docs/obsidian-plan.md` (Korean) with implementation details, test strategy, and risk mitigation.

---

## 1. Executive Summary

### What We're Building

Three layers of Obsidian integration:

1. **Go Backend — Obsidian CLI Client**: A wrapper around the official `obsidian` CLI (v1.12+) that replaces direct filesystem I/O with Obsidian-native operations (search, frontmatter, backlinks, tags, daily notes). The existing `vault/writer.go` channel-based writer is retained as a fallback.

2. **Knowledge Backend API**: New HTTP endpoints under `/api/knowledge/*` that expose Vault data (notes, search, tags, links, health) to the Web UI.

3. **Knowledge Frontend UI**: A new "Knowledge" page with sub-tabs (Browse, Search, Research, Graph, Health) plus a Dashboard widget.

### Why

The current `vault/writer.go` (`go/src/internal/vault/writer.go:28-83`) provides file-write-only access to the Obsidian Vault. By integrating with the official CLI, Flux gains:
- Full-text search via Obsidian's index (not grep)
- Frontmatter/property management
- Backlink and link graph traversal
- Orphan note and broken link detection
- Daily note management
- Tag aggregation

This enables the Knowledge UI and makes the Researcher Pod (Phase 4) significantly more effective.

### Scope Boundaries

- **In scope**: CLI wrapper, Facade pattern, Knowledge API, Knowledge UI, Dashboard widget, bootstrap health check, executor/researcher prompt additions
- **Out of scope**: Obsidian plugin development, Obsidian Sync, mobile access, d3.js graph visualization (deferred)
- **No new DB tables**: Vault data is queried in real-time via CLI. Research history reuses the existing `tasks` table (`type=RESEARCH`).

---

## 2. Integration Points

### 2.1 Obsidian CLI Access

| Aspect | Detail |
|--------|--------|
| **Access method** | Local CLI binary (`obsidian`) communicating with Obsidian app via IPC |
| **Obsidian version** | v1.12+ (released 2026-02-10) |
| **Requires** | Obsidian app running in background (macOS LaunchAgent) |
| **License** | Catalyst ($25 one-time, Early Access — will become free) |
| **CLI activation** | Settings → Command line interface → ON |

### 2.2 Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│  Mac Mini                                                    │
│                                                               │
│  ┌──────────────┐     IPC      ┌──────────────────┐         │
│  │ Obsidian App  │◄────────────│  obsidian CLI     │         │
│  │ (background)  │             └────────▲──────────┘         │
│  │ • indexing    │                      │                     │
│  │ • search      │             ┌────────┴──────────┐         │
│  │ • link graph  │             │  Flux Process      │         │
│  └──────┬────────┘             │                    │         │
│         ▼                      │  vault/            │         │
│  ~/ObsidianVault/Flux/        │  ├── client.go     │ ← NEW  │
│  ├── Goals/                    │  ├── facade.go     │ ← NEW  │
│  ├── Projects/                 │  ├── fallback.go   │ ← NEW  │
│  ├── Research/                 │  ├── writer.go     │ EXISTS  │
│  ├── Tasks/                    │  └── templates.go  │ EXISTS  │
│  └── Templates/                │                    │         │
│                                │  server/           │         │
│  ┌──────────────┐              │  └── api_knowledge │ ← NEW  │
│  │ Web Browser   │◄─────HTTP──│                    │         │
│  │ Knowledge UI  │             │  frontend/         │         │
│  └──────────────┘              │  └── Knowledge.*   │ ← NEW  │
│                                └────────────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 CLI Command Mapping

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

---

## 3. Affected Flux Components

### 3.1 Go Backend — `go/src/internal/vault/`

**Current state** (2 files):
- `writer.go` — Channel-based async writer with `WriteMode` (Create/Append/Replace). Used by executors via `main.go:91-92,134`.
- `templates.go` — Markdown templates for task summaries, decisions, project indexes.

**New files**:

| File | Purpose | Complexity |
|------|---------|------------|
| `client.go` | Obsidian CLI wrapper — `exec()` base, all CLI operations | High |
| `facade.go` | `VaultFacade` — CLI-first, Writer fallback. Replaces direct `Writer` usage | Medium |
| `fallback.go` | Degraded-mode implementations (grep search, YAML frontmatter parse) | Low |
| `client_test.go` | Unit tests for Client (mock exec) | Medium |
| `facade_test.go` | Unit tests for Facade fallback logic | Medium |
| `fallback_test.go` | Unit tests for fallback functions | Low |

**Modified files**:

| File | Change | Impact |
|------|--------|--------|
| `writer.go` | No code changes — retained as-is for fallback | None |
| `templates.go` | Add research finding + project proposal templates (Phase 4 prep) | Low |

### 3.2 Go Backend — `go/src/internal/server/`

**New files**:

| File | Purpose |
|------|---------|
| `api_knowledge.go` | 15 Knowledge API endpoints (search, note, tags, links, health, etc.) |
| `api_knowledge_test.go` | HTTP handler tests |

**Modified files**:

| File | Change |
|------|--------|
| `server.go` | Register `/api/knowledge/*` routes (~10 lines, around line 170) |
| `server.go` | Add `vault *vault.VaultFacade` field to `Server` struct |

**API endpoint summary** (all require auth middleware):

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

These follow the existing pattern in `server.go:100-174` using `s.authMiddleware()`, `writeJSON()`, `writeError()`.

### 3.3 Go Backend — `go/src/cmd/flux/main.go`

**Changes** (minimal):
- Replace `vault.NewWriter(cfg.Vault.Path)` with `vault.NewFacade(cfg.Vault.Name, cfg.Vault.Path)` (~line 91)
- Pass `VaultFacade` to `server.NewServer()` and `executor.NewExecutor()`
- Add Obsidian CLI health check log message after bootstrap (~line 83)

### 3.4 Go Backend — `go/src/internal/config/config.go`

**Changes**:
- Extend `VaultConfig` struct (currently `config.go:59-61`) to add `Name` field:
  ```go
  type VaultConfig struct {
      Path string `yaml:"path"`
      Name string `yaml:"name"` // Obsidian vault name for CLI
  }
  ```

### 3.5 SQLite Schema — No Changes

**Decision**: No new database tables. Rationale:
- Vault data is queried in real-time via Obsidian CLI (Obsidian maintains its own index)
- Research history reuses existing `tasks` table with `type=RESEARCH` filter
- Research stats use `GROUP BY` on `type_tag` column in tasks table
- Caching Vault metadata in SQLite would create staleness issues

### 3.6 React/TypeScript Frontend

**New files**:

| File | Purpose |
|------|---------|
| `frontend/src/pages/Knowledge.tsx` | Knowledge page with sub-tab routing |
| `frontend/src/pages/knowledge/Browse.tsx` | File tree + note preview with markdown rendering |
| `frontend/src/pages/knowledge/Search.tsx` | Full-text search with result highlighting |
| `frontend/src/pages/knowledge/Research.tsx` | Research timeline + active researcher pods |
| `frontend/src/pages/knowledge/Graph.tsx` | Tag cloud + link relationship list |
| `frontend/src/pages/knowledge/Health.tsx` | Orphan notes, broken links, vault stats |
| `frontend/src/stores/knowledgeStore.ts` | Zustand store for all Knowledge state |

**Modified files**:

| File | Change |
|------|--------|
| `frontend/src/App.tsx` | Add `/knowledge` and nested routes (~5 lines) |
| `frontend/src/components/Layout.tsx` | Add "Knowledge" nav item |
| `frontend/src/pages/Dashboard.tsx` | Add Knowledge summary widget card |

**Frontend dependencies** (already available):
- `react-markdown` (v10.1.0) — already in `package.json` for note rendering
- `remark-gfm` (v4.0.1) — already in `package.json` for GFM support
- `zustand` (v4.4.7) — already used for all stores
- No new npm dependencies required

### 3.7 Executor Behavior

**Changes to executor prompt** (`CLAUDE.md` injected into worktrees):
- Add Obsidian CLI usage instructions for reading project context before starting work
- Add instructions for writing learnings to Vault after task completion
- Add daily note append for task completion logging

**No Go code changes** in `executor/executor.go` — prompt changes only. The executor already uses the VaultWriter via its constructor (`main.go:134`); switching to VaultFacade is transparent.

### 3.8 Claude Code CLI Integration

**No new CLI flags needed**. The `obsidian` CLI is invoked:
1. By the Go backend's `vault.Client` (for API queries)
2. By Claude Code agents directly (via CLAUDE.md prompt instructions in worktrees)

Both are local executions on the same machine where Obsidian app runs.

---

## 4. Implementation Phases

### Phase 3a: Backend Core (CLI Client + Facade)

**Goal**: Replace direct file I/O with Obsidian CLI wrapper, with graceful fallback.

| Task | Description | Files | Complexity | Est. Effort |
|------|-------------|-------|------------|-------------|
| 3a.1 | `vault/client.go` — CLI wrapper with `exec()`, all read/search/tag/link methods | `client.go` | High | 1 task |
| 3a.2 | `vault/fallback.go` — Degraded-mode: grep search, YAML frontmatter, tag grep | `fallback.go` | Low | 1 task |
| 3a.3 | `vault/facade.go` — VaultFacade: CLI-first, Writer fallback, DailyNote mutex | `facade.go` | Medium | 1 task |
| 3a.4 | Config: Add `vault.name` to VaultConfig | `config.go` | Low | Part of 3a.3 |
| 3a.5 | Bootstrap: Add CLI health check, log warning if unavailable | `main.go` | Low | Part of 3a.3 |
| 3a.6 | Tests: client_test, facade_test, fallback_test | `*_test.go` | Medium | 1 task |

**Dependencies**: None (builds on existing vault package)

**Critical path**: 3a.1 → 3a.3 → 3a.5

### Phase 3b: Knowledge API

**Goal**: Expose Vault data via HTTP API for the Web UI.

| Task | Description | Files | Complexity | Est. Effort |
|------|-------------|-------|------------|-------------|
| 3b.1 | `api_knowledge.go` — All 15 endpoints | `api_knowledge.go` | Medium | 1 task |
| 3b.2 | Wire VaultFacade into Server struct, register routes | `server.go` | Low | Part of 3b.1 |
| 3b.3 | Tests: handler tests with mock VaultFacade | `api_knowledge_test.go` | Medium | 1 task |

**Dependencies**: Phase 3a complete

### Phase 3c: Knowledge Frontend UI

**Goal**: Knowledge page with all sub-tabs and Dashboard widget.

| Task | Description | Files | Complexity | Est. Effort |
|------|-------------|-------|------------|-------------|
| 3c.1 | `knowledgeStore.ts` — Zustand store with API calls | `knowledgeStore.ts` | Medium | 1 task |
| 3c.2 | `Knowledge.tsx` + `Browse.tsx` — File tree, note preview, markdown rendering | `Knowledge.tsx`, `Browse.tsx` | Medium | 1 task |
| 3c.3 | `Search.tsx` — Full-text search with highlighting | `Search.tsx` | Medium | 1 task |
| 3c.4 | `Research.tsx` — Timeline + active pods + type distribution | `Research.tsx` | Medium | 1 task |
| 3c.5 | `Graph.tsx` — Tag cloud + link list | `Graph.tsx` | Low | 1 task |
| 3c.6 | `Health.tsx` — Orphans, broken links, stats | `Health.tsx` | Low | 1 task |
| 3c.7 | Routing + Navigation — App.tsx routes, Layout.tsx nav item | `App.tsx`, `Layout.tsx` | Low | Part of 3c.2 |
| 3c.8 | Dashboard widget — Knowledge stats card | `Dashboard.tsx` | Low | 1 task |

**Dependencies**: Phase 3b complete (API must exist for frontend to call)

### Phase 3d: Executor/Agent Integration

**Goal**: Executor and Researcher pods use Obsidian CLI for context and recording.

| Task | Description | Files | Complexity | Est. Effort |
|------|-------------|-------|------------|-------------|
| 3d.1 | Executor prompt: Add Obsidian CLI usage to CLAUDE.md template | Prompt templates | Low | 1 task |
| 3d.2 | Researcher prompt: Add research-specific Obsidian CLI patterns | Prompt templates | Low | Part of 3d.1 |

**Dependencies**: Phase 3a complete (CLI must be working)

### Dependency Graph

```
Phase 3a: Backend Core
  3a.1 (Client) ──┬──→ 3a.3 (Facade) ──→ 3a.5 (Bootstrap)
  3a.2 (Fallback) ─┘         │
  3a.6 (Tests) ◄─────────────┘
                              │
                              ▼
Phase 3b: Knowledge API
  3b.1 (Endpoints) ──→ 3b.3 (Tests)
                              │
                              ▼
Phase 3c: Frontend UI
  3c.1 (Store) ──┬──→ 3c.2 (Browse)
                 ├──→ 3c.3 (Search)
                 ├──→ 3c.4 (Research)
                 ├──→ 3c.5 (Graph)
                 └──→ 3c.6 (Health)
  3c.8 (Dashboard) ← independent of sub-tabs
                              │
                              ▼
Phase 3d: Agent Integration
  3d.1 (Executor prompts)  ← independent track
```

### Critical Path

```
3a.1 → 3a.3 → 3b.1 → 3c.1 → 3c.2 (minimum viable Knowledge UI)
```

---

## 5. External Dependencies

| Dependency | Version | Status | Risk |
|-----------|---------|--------|------|
| Obsidian Desktop | v1.12+ | Released 2026-02-10 | Low — stable release |
| Obsidian CLI | Built-in to 1.12 | Available | Low — official feature |
| Catalyst License | $25 one-time | Required for Early Access | Low — one-time cost |
| macOS LaunchAgent | Standard | Required for background Obsidian | Low — well-understood |
| `react-markdown` | v10.1.0 | Already in package.json | None |
| `remark-gfm` | v4.0.1 | Already in package.json | None |

**No new Go or npm dependencies required**.

---

## 6. Test Strategy

### 6.1 Unit Tests (Go)

| Component | Test File | What to Test |
|-----------|-----------|-------------|
| `vault.Client` | `client_test.go` | Mock `exec.Command` to test all CLI operations: read, create, search, tags, backlinks, etc. Test error handling for CLI failures, timeout, and empty results. |
| `vault.VaultFacade` | `facade_test.go` | Test CLI-first / Writer-fallback logic. Test with CLI available and unavailable. Test DailyNote mutex prevents concurrent writes. |
| `vault.Fallback*` | `fallback_test.go` | Test grep-based search, YAML frontmatter parsing, tag extraction on real markdown files in `t.TempDir()`. |
| `server.Knowledge` | `api_knowledge_test.go` | HTTP handler tests using `httptest.NewRecorder`. Mock VaultFacade. Test auth middleware, query parameters, error responses. |

**Pattern**: Follow existing test patterns in `go/src/internal/vault/writer_test.go` and `go/src/internal/server/server_test.go`.

### 6.2 Integration Tests (Go)

| Test | Description |
|------|-------------|
| Facade + real filesystem | Write via Facade with CLI unavailable → verify file written. Read back. |
| API → Facade → filesystem | Call Knowledge API endpoint → verify response matches actual Vault content. |
| Bootstrap health check | Start with CLI unavailable → verify warning logged, fallback mode active. |

### 6.3 Edge Cases

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

### 6.4 Frontend Testing

- Component tests for Knowledge sub-tabs using React Testing Library
- Zustand store tests for API call mocking and state transitions
- No E2E tests for Phase 3 (manual verification sufficient for UI)

---

## 7. Risk Mitigation

### 7.1 Backward Compatibility

| Risk | Mitigation |
|------|-----------|
| Breaking existing VaultWriter consumers | VaultFacade has identical `Write()` signature. Existing `Writer` retained and used as fallback. |
| Breaking executor vault writes | Facade is a drop-in replacement — executors don't need code changes. |
| API namespace collision | All new endpoints under `/api/knowledge/*` — no overlap with existing `/api/tasks/*`, `/api/projects/*`, etc. |
| Frontend route collision | `/knowledge` and `/knowledge/*` are new routes — no overlap. |
| Config change | `vault.name` is additive — existing `vault.path` unchanged. Empty `vault.name` means CLI is skipped. |

### 7.2 Obsidian Unavailability

The system operates in three modes:

| Mode | Condition | Read | Write | Search | Tags/Links |
|------|-----------|------|-------|--------|------------|
| **Full** | Obsidian app running + CLI enabled | CLI | CLI | CLI (indexed) | CLI |
| **Fallback** | Obsidian not running / CLI unavailable | File I/O | Writer (channel) | grep | grep (tags only) |
| **Degraded** | Vault path missing | Error | Error | Error | Error |

**Health check at startup** (`main.go` bootstrap):
```go
client := vault.NewClient(cfg.Vault.Name)
if !client.IsAvailable() {
    logger.Warn("Obsidian CLI not available, using file-based fallback",
        "features_degraded", "search, tags, backlinks, orphans, properties")
    discord.Send(notifier.LevelWarning, "Obsidian CLI unavailable. Knowledge features degraded.")
}
```

### 7.3 Data Consistency

| Risk | Mitigation |
|------|-----------|
| Stale CLI results after file write | Obsidian indexes in near real-time. The `silent` flag on create prevents UI pop-ups but indexing still happens. |
| Concurrent file modifications | VaultFacade serializes daily note writes via mutex. Other writes are append-only (task summaries, research findings). |
| CLI returns partial data during index rebuild | Retry with delay. Fallback functions provide degraded but consistent results. |

### 7.4 Performance

| Concern | Mitigation |
|---------|-----------|
| CLI exec overhead (~100ms per call) | Acceptable for API queries. Knowledge API is user-facing (not hot path). |
| Large search results | `limit` parameter on search API. Default 20. |
| Tag cloud computation | Cached in Zustand store. Refreshed on page visit, not on every render. |
| Many sequential CLI calls (stats endpoint) | `knowledgeStats` handler makes 4 parallel CLI calls. Consider `errgroup`. |

---

## 8. Configuration Changes

### config.yaml additions

```yaml
vault:
  path: ~/ObsidianVault/Flux    # existing
  name: Flux                     # NEW: Obsidian vault name for CLI commands
```

### LaunchAgent for Obsidian (operational, not code)

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

---

## 9. File Inventory

### New Files (~15)

| Category | File | Lines (est.) |
|----------|------|-------------|
| Go vault | `go/src/internal/vault/client.go` | ~250 |
| Go vault | `go/src/internal/vault/facade.go` | ~100 |
| Go vault | `go/src/internal/vault/fallback.go` | ~80 |
| Go vault | `go/src/internal/vault/client_test.go` | ~200 |
| Go vault | `go/src/internal/vault/facade_test.go` | ~100 |
| Go vault | `go/src/internal/vault/fallback_test.go` | ~80 |
| Go server | `go/src/internal/server/api_knowledge.go` | ~300 |
| Go server | `go/src/internal/server/api_knowledge_test.go` | ~200 |
| React page | `frontend/src/pages/Knowledge.tsx` | ~50 |
| React page | `frontend/src/pages/knowledge/Browse.tsx` | ~200 |
| React page | `frontend/src/pages/knowledge/Search.tsx` | ~150 |
| React page | `frontend/src/pages/knowledge/Research.tsx` | ~200 |
| React page | `frontend/src/pages/knowledge/Graph.tsx` | ~100 |
| React page | `frontend/src/pages/knowledge/Health.tsx` | ~100 |
| React store | `frontend/src/stores/knowledgeStore.ts` | ~150 |

### Modified Files (~5)

| File | Change |
|------|--------|
| `go/src/internal/config/config.go` | Add `Name` to `VaultConfig` |
| `go/src/cmd/flux/main.go` | Switch `Writer` → `VaultFacade`, add health check |
| `go/src/internal/server/server.go` | Add vault field, register knowledge routes |
| `frontend/src/App.tsx` | Add Knowledge routes |
| `frontend/src/pages/Dashboard.tsx` | Add Knowledge widget |

**Total**: ~15 new files, ~5 modified files, ~2,260 estimated lines of new code.

---

## 10. Design Decisions

### D1: CLI Wrapper + Existing Writer Coexistence

Keep `vault/writer.go` as-is. Introduce `VaultFacade` that tries CLI first, falls back to Writer. This ensures task completion recording continues even if Obsidian app crashes.

### D2: No New Database Tables

Vault data is queried live via CLI. This avoids cache staleness and schema complexity. Obsidian already maintains its own high-performance index. Research history reuses the `tasks` table (`type=RESEARCH`).

### D3: Research UI as Knowledge Sub-Tab

Originally planned as a standalone page (Phase 4 Task 4.7). Consolidated into Knowledge page because research findings, vault browsing, and search are all aspects of the same knowledge system.

### D4: Graph Visualization Deferred

Phase 3 uses a simple tag cloud + link list. Full d3.js graph visualization is deferred because (a) Obsidian's native graph view is already superior, and (b) the Web UI's purpose is quick access, not graph exploration.

### D5: Interface-Based Facade for Testability

`VaultFacade` will implement an interface that `Server` and `Executor` consume. This allows mock injection for testing without requiring a running Obsidian instance.

```go
// VaultReader provides read access to the Obsidian vault.
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

---

## 11. CLI Gotchas and Mitigations

| Issue | Impact | Mitigation in Go Client |
|-------|--------|------------------------|
| `create` without `silent` opens Obsidian UI | Disrupts background operation | `Client.Create()` always appends `silent` flag |
| CLI doesn't auto-create directories | Create fails for new paths | `Client.Create()` calls `os.MkdirAll()` before exec |
| `tags all` required for vault-wide tags | Without `all`, returns current-file tags | Client always adds `all` to tag queries |
| Empty result on fresh vault = index not ready | False negatives | Retry 3 times with 2s delay |
| 22.8% silent failure rate (exit 0, no output) | Silent data loss | Post-write verification: search for created note |
| `template=` parameter ignores path | Created in unexpected location | Verify location after template creation via search |

---

## 12. Relationship to Phase Plans

### Phase 2B Impact

| Original Task | Change |
|--------------|--------|
| 2B.7 (VaultWriter) | **Retained** — writer.go unchanged, used as fallback |
| 2B.8 (Minimal Vault Recording) | **Unchanged** — recording works through VaultFacade transparently |
| **New**: 2B.7a | Obsidian app LaunchAgent setup |
| **New**: 2B.7b | `vault/client.go` implementation |
| **New**: 2B.7c | `vault/fallback.go` implementation |
| **New**: 2B.8a | Bootstrap CLI health check |

### Phase 4 Impact

| Original Task | Change |
|--------------|--------|
| 4.3 (Vault Writer Upgrade) | **Redesigned** → `VaultFacade` (CLI + Writer) |
| 4.7 (Research UI) | **Redesigned** → Knowledge page sub-tab (Research.tsx) |
| **New**: 4.7a-h | Full Knowledge UI breakdown (Browse, Search, Research, Graph, Health, Dashboard widget) |

---

## 13. Acceptance Criteria Mapping

| Criterion | Section |
|-----------|---------|
| Plan identifies all Obsidian integration points | §2 (Integration Points), §2.3 (CLI Command Mapping) |
| Plan specifies affected Flux components | §3 (all subsections: vault, server, main.go, config, schema, frontend, executor, CLI) |
| Plan includes architecture diagram showing data flow | §2.2 (Data Flow diagram) |
| Plan outlines implementation phases with dependencies | §4 (Phases 3a-3d with dependency graph and critical path) |
| Plan identifies external dependencies | §5 (External Dependencies table) |
| Plan includes test strategy | §6 (Unit, Integration, Edge Cases, Frontend) |
| Plan documents risk mitigation | §7 (Backward Compatibility, Unavailability, Data Consistency, Performance) |
