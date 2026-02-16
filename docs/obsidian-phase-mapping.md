# Obsidian Integration Phase Mapping

> Maps obsidian-integration-plan.md sections 3a-3d to flux phase structure (2B, 3, 4)

---

## Executive Summary

The Obsidian CLI integration spans **three existing phases** without requiring new intermediate phases:

- **Phase 2B enhancement** (2.c): Backend Core (CLI Client + Facade) - prerequisite infrastructure
- **Phase 3 enhancement** (new tasks): Knowledge API + Frontend UI - fits orchestration goals
- **Phase 4 as-is**: Executor/Agent Integration - already planned under Task 4.3 and 4.7

**Rationale**: The integration builds incrementally on existing phase goals. Phase 2B establishes reliable vault access, Phase 3 adds visibility/monitoring (knowledge UI fits this), and Phase 4 enables autonomous research (requires the full knowledge system).

---

## Section 3a: Backend Core (CLI Client + Facade)

### Phase Assignment: **Phase 2B Enhancement (new phase 2.c)**

**Tasks from obsidian-integration-plan.md §4**:
- 3a.1: `vault/client.go` — Obsidian CLI wrapper (~250 lines)
- 3a.2: `vault/fallback.go` — Degraded mode grep/YAML (~80 lines)
- 3a.3: `vault/facade.go` — CLI-first, Writer fallback (~100 lines)
- 3a.4: Config: Add `vault.name` to VaultConfig
- 3a.5: Bootstrap: CLI health check in main.go
- 3a.6: Tests: client_test, facade_test, fallback_test (~380 lines total)

### Affected Components

#### Go Backend
| File | Change Type | Impact |
|------|-------------|--------|
| `go/src/internal/vault/client.go` | NEW | CLI wrapper with exec(), all Obsidian operations |
| `go/src/internal/vault/facade.go` | NEW | VaultFacade interface, CLI-first + Writer fallback |
| `go/src/internal/vault/fallback.go` | NEW | Grep search, YAML frontmatter parsing |
| `go/src/internal/vault/writer.go` | MODIFY | No code changes — retained as fallback |
| `go/src/internal/vault/client_test.go` | NEW | Mock exec tests |
| `go/src/internal/vault/facade_test.go` | NEW | Fallback logic tests |
| `go/src/internal/vault/fallback_test.go` | NEW | Degraded mode tests |
| `go/src/internal/config/config.go` | MODIFY | Add `Name string` to VaultConfig (line ~60) |
| `go/src/cmd/flux/main.go` | MODIFY | Replace Writer with VaultFacade (~lines 91, 134), add CLI health check (~line 83) |

#### External Dependencies
- Obsidian app running (LaunchAgent, already handled operationally)
- Obsidian CLI enabled (Settings → CLI → ON)
- No new Go modules required

### Rationale for Phase 2B (2.c)

**Why not Phase 2B as-is?**
- Phase 2B Task 2B.7 implements the basic `vault/writer.go` (file-based)
- The CLI integration is a **prerequisite** for Phase 3/4 knowledge features
- Without it, Phase 3 Knowledge API would have no real search/tag/link capabilities

**Why not Phase 3?**
- Phase 3's goal is "autonomous operation" (scaling, usage tracking, daily summaries)
- The vault backend is **infrastructure**, not an autonomous feature
- Phase 3 tasks should *use* the enhanced vault system, not *build* it

**Why new 2.c instead of expanding 2B.7?**
- Clean separation: 2B.7 = basic writer, 2.c = CLI integration
- Allows incremental rollout: Phase 2B can complete without Obsidian CLI dependency
- Provides fallback: if CLI integration is delayed, Phase 2B's file-based writer still works

**Dependencies**:
- Must complete **before** Phase 3 Knowledge API (3b)
- Builds on Phase 2B Task 2B.7 (existing Writer as fallback)

---

## Section 3b: Knowledge API

### Phase Assignment: **Phase 3 (new tasks 3.18-3.19)**

**Tasks from obsidian-integration-plan.md §4**:
- 3b.1: `server/api_knowledge.go` — 15 HTTP endpoints (~300 lines)
- 3b.2: Wire VaultFacade into Server struct, register routes
- 3b.3: Tests: HTTP handler tests with mock VaultFacade (~200 lines)

### Affected Components

#### Go Backend
| File | Change Type | Impact |
|------|-------------|--------|
| `go/src/internal/server/api_knowledge.go` | NEW | 15 endpoints for vault data access |
| `go/src/internal/server/api_knowledge_test.go` | NEW | HTTP handler tests |
| `go/src/internal/server/server.go` | MODIFY | Add `vault *vault.VaultFacade` field (~line 20), register routes (~line 170) |

#### API Endpoints (all under `/api/knowledge/*`)
```
GET  /api/knowledge/search          — Full-text search via Obsidian index
GET  /api/knowledge/note            — Note content + properties + links
GET  /api/knowledge/outline         — Note heading structure
GET  /api/knowledge/tags            — Tag cloud (all tags + counts)
GET  /api/knowledge/tags/:tag/files — Files with specific tag
GET  /api/knowledge/backlinks       — Incoming links to note
GET  /api/knowledge/links           — Outgoing links from note
GET  /api/knowledge/orphans         — Notes with no incoming links
GET  /api/knowledge/unresolved      — Broken links
GET  /api/knowledge/files           — File list by folder
GET  /api/knowledge/folders         — Vault folder structure
GET  /api/knowledge/stats           — Vault statistics
GET  /api/knowledge/daily           — Daily note content
GET  /api/knowledge/research/history — Research task history (type=RESEARCH)
GET  /api/knowledge/research/stats  — Research type distribution
```

### Rationale for Phase 3

**Why Phase 3?**
- Phase 3 goal: "Flux manages itself" with **visibility into system state**
- Knowledge API provides **visibility into the knowledge base** (vault statistics, research history)
- Fits alongside existing Phase 3 visibility features:
  - Task 3.12-3.13: Usage API + UI (usage visibility)
  - Task 3.14: Orchestrator API (orchestrator visibility)
  - Task 3.16: Settings + Metrics (system health visibility)
- All are read-only monitoring/visibility endpoints

**Why not Phase 4?**
- Phase 4 is about **autonomous action** (Researchers write to vault)
- This API is about **reading** the vault for visibility
- Researchers (Phase 4) will *benefit from* the API, but don't require it to function

**Dependencies**:
- Requires Phase 2.c complete (VaultFacade must exist)
- Prerequisite for Phase 3 Knowledge UI (3c)

**Suggested task numbers**: 3.18 (api_knowledge.go), 3.19 (tests)

---

## Section 3c: Knowledge Frontend UI

### Phase Assignment: **Phase 3 (new tasks 3.20-3.27)**

**Tasks from obsidian-integration-plan.md §4**:
- 3c.1: `knowledgeStore.ts` — Zustand store (~150 lines)
- 3c.2: `Knowledge.tsx` + `Browse.tsx` — File tree + preview (~250 lines)
- 3c.3: `Search.tsx` — Full-text search (~150 lines)
- 3c.4: `Research.tsx` — Timeline + active pods (~200 lines)
- 3c.5: `Graph.tsx` — Tag cloud + link list (~100 lines)
- 3c.6: `Health.tsx` — Orphans, broken links (~100 lines)
- 3c.7: Routing + Navigation — App.tsx, Layout.tsx (~10 lines total)
- 3c.8: Dashboard widget — Knowledge stats card (~50 lines)

### Affected Components

#### React Frontend
| File | Change Type | Impact |
|------|-------------|--------|
| `frontend/src/pages/Knowledge.tsx` | NEW | Main page with sub-tab routing |
| `frontend/src/pages/knowledge/Browse.tsx` | NEW | File tree + markdown preview |
| `frontend/src/pages/knowledge/Search.tsx` | NEW | Search UI with highlighting |
| `frontend/src/pages/knowledge/Research.tsx` | NEW | Research timeline + pod status |
| `frontend/src/pages/knowledge/Graph.tsx` | NEW | Tag cloud + link relationships |
| `frontend/src/pages/knowledge/Health.tsx` | NEW | Vault health metrics |
| `frontend/src/stores/knowledgeStore.ts` | NEW | State management for knowledge data |
| `frontend/src/App.tsx` | MODIFY | Add `/knowledge` routes (~5 lines) |
| `frontend/src/components/Layout.tsx` | MODIFY | Add "Knowledge" nav item (~3 lines) |
| `frontend/src/pages/Dashboard.tsx` | MODIFY | Add Knowledge summary widget (~50 lines) |

#### Frontend Dependencies
- **Already available**: `react-markdown` v10.1.0, `remark-gfm` v4.0.1, `zustand` v4.4.7
- **No new npm dependencies required**

### UI Structure
```
/knowledge              → Browse (default tab)
/knowledge/search       → Search
/knowledge/research     → Research History
/knowledge/graph        → Tag/Link Graph
/knowledge/health       → Vault Health
```

### Rationale for Phase 3

**Why Phase 3?**
- Phase 3 theme: **visibility and monitoring**
- Knowledge UI provides:
  - Vault health monitoring (orphans, broken links) — monitoring
  - Research timeline visibility — monitoring
  - Tag/link graph — system state visibility
- Complements existing Phase 3 UI:
  - Task 3.13: Usage page (usage monitoring)
  - Task 3.16: Settings enhancement (system health monitoring)
- All are read-only visibility features

**Why not Phase 4?**
- Phase 4 Research UI (Task 4.7) was originally standalone
- **Consolidation**: obsidian-integration-plan.md §5.5 already integrates Research.tsx as a Knowledge sub-tab
- Phase 3 builds the **infrastructure** (Knowledge page), Phase 4 just enhances Research.tsx with autonomous research data

**Relationship to Phase 4 Task 4.7**:
- Phase 3: Build Knowledge.tsx + Browse/Search/Research/Graph/Health tabs
- Phase 4 Task 4.7: Enhance Research.tsx with autonomous research features (proposal triggers, researcher pod integration)
- The Research tab *exists* in Phase 3 but shows manual research tasks only
- Phase 4 adds autonomous research and project proposals to the existing UI

**Dependencies**:
- Requires Phase 3 Knowledge API (3b) complete
- Research.tsx uses `/api/pods` (Phase 1), `/api/tasks?type=RESEARCH` (Phase 2A)

**Suggested task numbers**:
- 3.20: knowledgeStore.ts
- 3.21: Knowledge.tsx + Browse.tsx
- 3.22: Search.tsx
- 3.23: Research.tsx (basic version)
- 3.24: Graph.tsx
- 3.25: Health.tsx
- 3.26: Routing + Navigation (App.tsx, Layout.tsx)
- 3.27: Dashboard widget

---

## Section 3d: Executor/Agent Integration

### Phase Assignment: **Phase 4 (existing Task 4.1-4.2 enhancement)**

**Tasks from obsidian-integration-plan.md §4**:
- 3d.1: Executor prompt — Add Obsidian CLI usage to CLAUDE.md template
- 3d.2: Researcher prompt — Add research-specific Obsidian CLI patterns

### Affected Components

#### Go Backend (Prompt Generation)
| File | Change Type | Impact |
|------|-------------|--------|
| `go/src/internal/executor/executor.go` | MODIFY | Enhance CLAUDE.md template generation with Obsidian CLI instructions |
| `go/src/internal/researcher/researcher.go` | MODIFY | Add research-specific CLI patterns to workspace CLAUDE.md |

#### Prompt Content (examples)
**Executor additions** (obsidian-integration-plan.md §9):
```markdown
## Obsidian Vault (Knowledge Base)

Read context before working:
  obsidian read path="Projects/{project}/_index.md"
  obsidian search query="{keyword}" path="Projects/{project}" format=json matches

Save results after completion:
  obsidian create name="Projects/{project}/learnings/{slug}" content="..." silent
  obsidian daily:append content="- ✅ {task}: {summary}"
  obsidian property:set name=status value=completed path="Tasks/{task}.md"

⚠️ Always use `silent` flag with `create`. Use `mkdir -p` before create if directory doesn't exist.
```

**Researcher additions**:
```markdown
Check existing research:
  obsidian search query="{topic}" format=json matches
  obsidian tags all counts

Save research findings:
  obsidian create name="Research/{category}/{slug}" content="..." silent

Log to daily note:
  obsidian daily:append content="- 🔬 Research: {topic}"
```

### Rationale for Phase 4

**Why Phase 4?**
- Executor prompt changes are **low-impact** additions (5-10 lines of prompt text)
- Researcher prompt integration directly supports Task 4.1 (Researcher Pod) and 4.2 (Autonomous Research)
- These prompts enable **autonomous knowledge accumulation**, which is Phase 4's core theme
- Executors *can* use Obsidian CLI earlier (Phase 2.c provides the CLI), but the **value** comes in Phase 4 when:
  - Researchers actively write findings
  - Executors read project context for complex tasks
  - Knowledge base becomes actionable for autonomous work

**Why not Phase 2.c or 3?**
- Phase 2.c: Too early — no research system exists yet, executors don't need vault reads for simple tasks
- Phase 3: Possible, but no compelling use case until Researchers exist
- Phase 4: Natural fit — Researchers (4.1) immediately benefit from CLI usage patterns

**Dependencies**:
- Requires Phase 2.c complete (VaultFacade must be available)
- Tightly coupled with Phase 4 Task 4.1 (Researcher Pod implementation)
- Enhances Phase 4 Task 4.2 (Autonomous Research) by providing CLI usage patterns

**Integration with existing Phase 4 tasks**:
- Task 4.1: Researcher Pod implementation includes `workspace.go` which writes CLAUDE.md — 3d.2 adds Obsidian section
- Task 4.3: Vault Writer Upgrade already adds research templates — 3d.1/3d.2 add prompt instructions for using them
- No new task numbers needed — these are enhancements to 4.1 and 4.3

---

## Intermediate Phases Analysis

### Question: Are new phases 2.c or 2.d needed?

**Answer: Yes, one new phase (2.c) is recommended**

#### Proposed Phase 2.c: Obsidian CLI Integration

**Goal**: Upgrade vault backend from file-only to Obsidian CLI-based with graceful fallback.

**Deliverable**: VaultFacade with CLI-first access, supporting search, tags, backlinks, properties.

**Tasks**:
- 2.c.1: Obsidian CLI wrapper (`vault/client.go`)
- 2.c.2: Fallback implementations (`vault/fallback.go`)
- 2.c.3: VaultFacade with CLI-first logic (`vault/facade.go`)
- 2.c.4: Config + bootstrap integration
- 2.c.5: Tests (client_test, facade_test, fallback_test)

**Placement**: Between Phase 2B and Phase 3

**Rationale**:
- Clean separation of concerns: vault infrastructure vs. orchestration features
- Allows Phase 2B to complete without Obsidian dependency (file-based writer still works)
- Provides stable foundation for Phase 3 Knowledge API
- Incremental risk: can roll back to Phase 2B writer if CLI integration issues arise

**Prerequisites**:
- Phase 2B Task 2B.7 complete (Writer exists as fallback)

**Enables**:
- Phase 3 Knowledge API (tasks 3.18-3.19)
- Phase 4 autonomous research with full vault capabilities

#### Phase 2.d: Not needed

No additional intermediate phase required. The progression is clean:
- **2B** → vault file writes (basic recording)
- **2.c** → vault CLI access (search, tags, links)
- **3** → knowledge API + UI (visibility)
- **4** → autonomous research (action)

---

## Dependency Graph

```
Phase 2B (existing)
  └── Task 2B.7: Vault Writer (file-based)
         │
         ▼
Phase 2.c (NEW)
  ├── 2.c.1: vault/client.go (Obsidian CLI wrapper)
  ├── 2.c.2: vault/fallback.go (degraded mode)
  ├── 2.c.3: vault/facade.go (CLI-first + Writer fallback)
  ├── 2.c.4: Config + bootstrap (vault.name, health check)
  └── 2.c.5: Tests (client_test, facade_test, fallback_test)
         │
         ▼
Phase 3 (existing + NEW tasks)
  ├── 3.18: server/api_knowledge.go (Knowledge API - 15 endpoints)
  ├── 3.19: api_knowledge_test.go (tests)
  │      │
  │      ▼
  ├── 3.20: frontend/stores/knowledgeStore.ts
  ├── 3.21: frontend/pages/Knowledge.tsx + Browse.tsx
  ├── 3.22: frontend/pages/knowledge/Search.tsx
  ├── 3.23: frontend/pages/knowledge/Research.tsx (basic)
  ├── 3.24: frontend/pages/knowledge/Graph.tsx
  ├── 3.25: frontend/pages/knowledge/Health.tsx
  ├── 3.26: Routing + Navigation (App.tsx, Layout.tsx)
  └── 3.27: Dashboard.tsx widget
         │
         ▼
Phase 4 (existing, with enhancements)
  ├── Task 4.1: Researcher Pod
  │      └── Enhancement: Add Obsidian CLI section to workspace CLAUDE.md (3d.2)
  ├── Task 4.2: Autonomous Research
  ├── Task 4.3: Vault Writer Upgrade (research templates)
  │      └── Enhancement: Add Obsidian CLI section to executor CLAUDE.md (3d.1)
  └── Task 4.7: Research UI enhancement
         └── Builds on Phase 3 Research.tsx with autonomous research data
```

---

## Critical Path

```
2B.7 → 2.c.1 → 2.c.3 → 3.18 → 3.20 → 3.21
                               ↓
                            3.23 (Research tab - basic)
                               ↓
                            4.1 + 4.2 (Researchers + autonomous research)
                               ↓
                            4.7 (Research tab - enhanced)
```

**Minimum viable Knowledge UI**: 2B.7 → 2.c → 3.18-3.21 (Browse tab functional)

**Full Knowledge system**: Add 3.22-3.27 (all tabs + dashboard widget)

**Autonomous research**: Requires Phase 4 Tasks 4.1, 4.2, 4.3

---

## File Count Summary

### Phase 2.c (NEW)
| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go vault | 6 files | 3 files |
| **Total** | **6** | **3** |

**New**: client.go, facade.go, fallback.go, client_test.go, facade_test.go, fallback_test.go
**Modified**: config.go, main.go, writer.go (comment only)

### Phase 3 Enhancements
| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go server | 2 files | 1 file |
| React pages | 7 files | 3 files |
| **Total** | **9** | **4** |

**New**: api_knowledge.go, api_knowledge_test.go, Knowledge.tsx, Browse.tsx, Search.tsx, Research.tsx, Graph.tsx, Health.tsx, knowledgeStore.ts
**Modified**: server.go, App.tsx, Layout.tsx, Dashboard.tsx

### Phase 4 Enhancements
| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | 0 files | 2 files |
| **Total** | **0** | **2** |

**Modified**: executor/executor.go (CLAUDE.md template), researcher/researcher.go (CLAUDE.md template)

### Grand Total (Obsidian Integration)
- **New files**: 15
- **Modified files**: 9
- **Total lines (estimate)**: ~2,260 lines of new code

---

## Risk Analysis

### Phase 2.c Risks

| Risk | Mitigation |
|------|-----------|
| Obsidian app not running | VaultFacade falls back to Writer (file-based). Health check warns on startup. |
| CLI unavailable | Fallback functions provide degraded search/tag capabilities via grep. |
| CLI breaks between versions | Pin Obsidian version requirement (1.12+). Test fallback regularly. |
| File write conflicts | VaultFacade uses existing Writer's channel-based serialization. |

### Phase 3 Risks

| Risk | Mitigation |
|------|-----------|
| Knowledge API performance | Obsidian CLI cached by app. Add query limits (default 20 results). |
| Large vault slowdown | Paginate file listings by folder. Lazy-load note content. |
| React state bloat | Zustand store clears old data on navigation. No infinite scrolling in Phase 3. |

### Phase 4 Risks

| Risk | Mitigation |
|------|-----------|
| Prompt too long with CLI examples | Keep examples concise (5-10 lines). Full docs in vault, not prompt. |
| Researchers overwrite each other | Per-pod workspaces prevent conflicts (4.1 design). |
| CLI silent failures | Retry logic with verification (obsidian-integration-plan.md §11). |

---

## Recommendation

### Proposed Phase Structure

1. **Complete Phase 2B as-is** (existing Task 2B.7 provides file-based fallback)
2. **Add Phase 2.c** (Obsidian CLI Integration) — 6 new files, 3 modified
3. **Enhance Phase 3** with Knowledge API + UI — 9 new files, 4 modified
4. **Enhance Phase 4** with Obsidian prompt sections — 0 new files, 2 modified

### Justification

- **Incremental risk**: Each phase can roll back to previous state
- **Clear dependencies**: 2B → 2.c → 3 → 4 (no parallel requirements)
- **Consistent with phase themes**:
  - 2B/2.c: Infrastructure hardening
  - 3: Visibility and monitoring
  - 4: Autonomous action
- **No new phase 2.d needed**: Single intermediate phase (2.c) is sufficient

### Implementation Order

1. Phase 2.c (foundation): ~1-2 tasks (can be broken down further if needed)
2. Phase 3.18-3.19 (API): 1-2 tasks
3. Phase 3.20-3.27 (UI): 5-8 tasks (can be parallelized by sub-tab)
4. Phase 4 enhancements: Folded into existing 4.1, 4.3 (no new tasks)

---

## Acceptance Criteria Mapping

From obsidian-integration-plan.md §13:

| Criterion | Mapped Phase | Task(s) |
|-----------|--------------|---------|
| Plan identifies all Obsidian integration points | **Phase 2.c** | All tasks (client.go, facade.go, fallback.go) |
| Plan specifies affected Flux components | **This document** | Component tables in each section |
| Plan includes architecture diagram showing data flow | **obsidian-integration-plan.md §2.2** | (Diagram already exists in original plan) |
| Plan outlines implementation phases with dependencies | **This document** | Dependency Graph section |
| Plan identifies external dependencies | **Phase 2.c** | Obsidian app (LaunchAgent), Obsidian CLI (Settings) |
| Plan includes test strategy | **Phase 2.c** | 2.c.5 (client_test, facade_test, fallback_test) |
| Plan documents risk mitigation | **This document** | Risk Analysis section |

---

## Open Questions

1. **Timeline**: Should Phase 2.c be a single large task or decomposed into 2-3 subtasks?
   - **Recommendation**: Single task if files are tightly coupled, otherwise split into: (a) client.go, (b) facade.go + fallback.go + tests

2. **Phase 3 UI parallelization**: Can Research.tsx, Graph.tsx, Health.tsx be built in parallel?
   - **Answer**: Yes — all depend on knowledgeStore.ts (3.20) but are otherwise independent

3. **Phase 4 prompt changes**: Should these be separate tasks or enhancements to existing 4.1/4.3?
   - **Recommendation**: Enhancements to existing tasks (5-10 lines of prompt text each)

4. **Vault templates**: Should Phase 4 Task 4.3 (Vault Writer Upgrade) also add Obsidian-specific frontmatter?
   - **Deferred**: obsidian-integration-plan.md §3.1 shows property support via CLI. Use default templates first, add frontmatter in post-Phase 4 if needed.
