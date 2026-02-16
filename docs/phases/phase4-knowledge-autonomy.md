# Phase 4: Knowledge & Autonomy — "Autonomous growth"

## Goal

Flux becomes a fully autonomous system that researches technologies, accumulates knowledge in Obsidian, proposes new projects, and improves its own code. The Operator only sets strategic direction (Goals), approves new projects, and reviews complex PRs.

## Deliverable

Researcher Pods running autonomous research, systematic Obsidian knowledge base, project proposals with Operator approval, Flux self-improvement with safety mechanisms.

## Prerequisites

- Phase 3 complete (autonomous Pod scaling, usage tracking, daily summaries)
- Obsidian Vault structure established (Phase 1)
- Vault Writer working (Phase 2B)

---

## Revision Notes (from Phase 3/4 Plan Refinement)

This plan was refined after auditing the Phase 2B codebase and the revised Phase 3 plan. Key changes:

1. **Task 4.1 (Researcher Pod)**: Restructured. The original plan assumed a `researcher/` package parallel to `executor/`. After reviewing the executor implementation (`executor/executor.go:38-71`), the Researcher should share the same Pod lifecycle patterns (Run loop, Stop channel, PodRegistry integration). The revised plan reuses executor infrastructure where possible instead of building parallel systems. Workspace paths corrected: actual workspace base is `./workspaces/` (`config.Orchestrator.WorkspaceBase`), not hardcoded.

2. **Task 4.2 (Autonomous Research)**: Simplified. The original weighted-priority system is over-engineered for an initial implementation. Revised to use a simple ordered-priority list. Claude Code itself can make intelligent research decisions given good context — we don't need complex Go-side logic.

3. **Task 4.3 (Vault Writer Upgrade)**: Reduced scope. The existing `vault/writer.go:28-83` and `vault/templates.go` handle async writes and task completion templates. The upgrade adds research and proposal templates only — not a full rewrite. Decision records and learning records deferred to post-Phase 4 (premature without usage data).

4. **Task 4.4 (Project Proposals)**: The project approval flow already exists in `server/api_projects.go`. Revised to clarify this task adds the Researcher-to-PROPOSED-project pipeline, not the approval UI (already built).

5. **Task 4.5 (Deployment Automation)**: The auto-updater (`internal/updater/`) already handles build and restart via `launchctl kickstart`. Revised to narrow scope: add `make deploy` target and enhance the existing updater, not build a parallel deploy system. The `deploy/` directory already has a launchd plist.

6. **Task 4.6 (Self-Improvement)**: Safety protocol enhanced with additional checks. File paths for excluded files updated to match actual codebase structure (Go packages are under `go/src/internal/`, not `internal/`). Added `gosec` installation as a prerequisite step.

7. **Task 4.7 (Research UI)**: File paths corrected from `web/src/` to `frontend/src/`. API design simplified — research data is already in the tasks table (type=RESEARCH), so a separate research API is unnecessary. Reuse task filtering.

8. **New Task 4.0 added**: ScaleManager Executor:Researcher Ratio — Phase 3 deferred Researcher ratio to Phase 4. This task upgrades the ScaleManager to manage both Executor and Researcher pods.

9. **Task ordering revised**: The original plan had loose dependencies. Revised with explicit dependency chain and a clear critical path.

10. **New Task 4.8 added**: Obsidian CLI Integration for Pods — From Obsidian integration plan Phase 3d. Executor and Researcher pods use Obsidian CLI for reading project context and recording learnings. This task adds prompt templates and integration patterns for pods to use the Obsidian CLI effectively.

11. **Insight features**: The comprehensive analytics and observability features from `docs/insight-plan.md` are implemented as Phase 3 features (Task 3.17-3.21) and do not require Phase 4 changes. They provide operational intelligence through the existing task/usage data and are independent of the research and autonomy features in Phase 4.

---

## Task Breakdown

### Task 4.0: ScaleManager Executor:Researcher Ratio

**Description**: Upgrade the Phase 3 ScaleManager (Executor-only) to manage both Executor and Researcher Pods with dynamic ratio based on workload.

**Files to modify**:
```
internal/orchestrator/scale_manager.go
```

**Implementation details**:
- Phase 3 ScaleManager only scales Executor count (0 to maxPods)
- Add Researcher Pod management:
  - Track researchers []*researcher.Researcher alongside executors
  - Ratio determination (from spec Section 15.2):
    - Urgent tasks (P1-5) → 9:1 Executor:Researcher
    - Operator tasks → 8:2
    - System tasks (moderate queue) → 7:3
    - Queue nearly empty → 3:7
    - Queue empty → 0:10 (pure research)
    - Service incident → 10:0
  - R&D protection: minimum `config.Orchestrator.MinResearchRatio` (0.2) except during incidents
- Pod lifecycle for Researchers mirrors Executors: goroutine + Stop() + PodRegistry
- Cooldown (15 min) applies to ratio changes, not just count changes

**Acceptance criteria**:
- [ ] Executor:Researcher ratio follows spec
- [ ] R&D protection (min 20%) enforced
- [ ] Researchers start/stop cleanly (no leaked goroutines)
- [ ] PodRegistry shows Researcher pods
- [ ] Unit tests for ratio logic

**Complexity**: Medium

**Depends on**: Phase 3 Task 3.2, Task 4.1

---

### Task 4.1: Researcher Pod + Workspace

**Description**: Implement the Researcher Pod with independent per-Pod workspaces for parallel research without conflicts.

**Files to create**:
```
internal/researcher/researcher.go
internal/researcher/types.go
internal/researcher/workspace.go
```

**Implementation details**:

**researcher.go**:
- `Researcher` struct mirrors Executor patterns (`executor/executor.go:38-71`):
  - id (e.g., "researcher-01"), config, claude (ClaudeCodeRunner), workspace, manager, vault, notifier, stopCh
- `Run(ctx)`: main loop
  1. Register with PodRegistry (same pattern as `executor/executor.go:76-81`)
  2. Request research task from Manager (`pod_type="researcher"`)
  3. If no task: autonomous research decision (Task 4.2)
  4. Execute Claude Code in per-Pod workspace
  5. Write findings to Vault
  6. Report completion to Manager
  7. Sleep 5s, loop
- `Stop()`: close stopCh, deregister from PodRegistry
- Reuses existing `claudecli` package for Claude Code execution
- Reuses existing `apiclient` package for Manager API calls

**types.go**:
- Research type constants: `github-scan`, `dependency-check`, `industry-research`, `project-ideas`, `opensource-scan`, `library-audit`, `service-review`, `goal-research`
- Each type has: description, prompt template, Vault destination path
- Map research types to Vault paths:
  - `github-scan`, `opensource-scan` → `Research/Tools/`
  - `industry-research` → `Research/Industry/`
  - `project-ideas` → `Research/Ideas/`
  - `dependency-check`, `library-audit` → `Research/Tools/`
  - `goal-research` → `Research/Goals/`

**workspace.go**:
- Per-Pod workspace at `{config.Orchestrator.WorkspaceBase}/research/{pod-id}/`
  ```
  workspaces/research/researcher-01/
  ├── workspace/     # CLAUDE.md, .claude/settings.json
  └── output/        # Temporary → moved to Obsidian
  ```
- `SetupWorkspace(podID string) (string, error)`: create directories, write CLAUDE.md with research context, write `.claude/settings.json` with allowed tools
- Workspace is created once at Pod startup, reused across research sessions
- Cleanup: remove workspace when Pod stops permanently (not between tasks)
- CLAUDE.md template includes Obsidian CLI usage patterns (see Task 4.8 for details)

**Acceptance criteria**:
- [ ] Researcher Pod runs independently with own workspace
- [ ] Per-Pod workspace isolation (no conflicts between Researchers)
- [ ] Research tasks fetched from Manager (type=RESEARCH filter)
- [ ] Claude Code execution in workspace
- [ ] Findings written to Vault via existing VaultWriter
- [ ] Stop signal honored cleanly
- [ ] PodRegistry integration working
- [ ] Unit tests for workspace setup and research type routing

**Complexity**: High

---

### Task 4.2: Autonomous Research Scheduling

**Description**: When no research tasks are in the queue, Researchers autonomously decide what to research based on current system state.

**Files to modify**:
```
internal/researcher/researcher.go
```

**Implementation details**:
- When `manager.NextTask(podID, "researcher")` returns nil:
  1. Query active Goal from Manager
  2. Select research type based on simple priority:
     - If active Goal exists → `goal-research` (research aligned with Goal)
     - If active projects have outdated dependencies → `dependency-check`
     - Otherwise → rotate through: `industry-research`, `opensource-scan`, `project-ideas`
  3. Create self-assigned task: type=RESEARCH, source=SELF, priority=70 (low, won't compete with real work)
  4. Build research prompt including Goal context and research history hint
- Research history: append one-line summary to `Research/_history.md` in Vault after each research session
- Avoid duplicates: use Obsidian CLI to search existing research (`obsidian search query="{topic}"`) before starting new research. Fallback to checking `_history.md` if CLI unavailable (simple string match on topic)
- **Design principle**: Keep Go-side logic simple. Let Claude Code make the intelligent decisions about *what* to research within the selected type. The Go code just picks the category and provides context.

**Acceptance criteria**:
- [ ] Autonomous research triggered when no tasks available
- [ ] Research type selected based on system state
- [ ] Goal alignment considered when Goal is active
- [ ] Research history logged in Vault
- [ ] Self-created tasks have correct priority (70)
- [ ] No rapid-fire research (minimum 5-min gap between autonomous sessions)
- [ ] Unit tests for type selection logic

**Complexity**: Medium

**Depends on**: Task 4.1

---

### Task 4.3: Vault Writer Upgrade

**Description**: Add research-specific templates and path routing to the Vault Writer.

**Files to modify**:
```
internal/vault/writer.go
internal/vault/templates.go
```

**Implementation details**:

**New templates in templates.go**:
- Research finding template:
  ```markdown
  # {title}
  Date: {date}
  Type: {research_type}
  Researcher: {pod_id}

  ## Key Findings
  {findings}

  ## Recommendations
  {recommendations}

  ## Sources
  {sources}
  ```
- Project proposal template:
  ```markdown
  # Project Proposal: {name}
  Date: {date}
  Proposed by: {pod_id}
  Goal alignment: {goal_title}

  ## Description
  {description}

  ## Tech Stack
  {tech_stack}

  ## Estimated Effort
  {effort}

  ## Inspiration
  {inspiration}
  ```

**Path routing in writer.go**:
- Add `WriteResearch(podID, researchType string, content string)` convenience method
- Add `WriteProposal(podID string, content string)` convenience method
- Route to Vault paths based on research type (from types.go mapping)
- Existing `Write()` method unchanged — new methods are additive
- Note: If Phase 3 Obsidian integration is complete, these methods should use VaultFacade (CLI-first, Writer fallback) instead of Writer directly. If Phase 3 is not complete, use Writer directly as a temporary measure.

**Acceptance criteria**:
- [ ] Research finding template produces well-formatted Obsidian markdown
- [ ] Project proposal template renders correctly
- [ ] Path routing places files in correct Vault directories
- [ ] Existing task completion templates unchanged
- [ ] Sequential write ordering preserved (existing channel-based writer)
- [ ] Unit tests for new templates

**Complexity**: Low-Medium

**Depends on**: Phase 2B Vault Writer (complete)

---

### Task 4.4: New Project Proposals

**Description**: Researchers can propose new projects based on discoveries. Projects require Operator approval before creation.

**Files to modify**:
```
internal/researcher/researcher.go
internal/server/api_projects.go (minor)
```

**Implementation details**:
- During research execution, if Claude Code output contains a project proposal (JSON marker):
  ```json
  {"proposal": true, "name": "...", "type": "REPO", "description": "...", "tech_stack": [...], "inspiration": "..."}
  ```
- Researcher parses proposal and:
  1. Creates project via Manager API with status=PROPOSED
  2. Links to current Goal if relevant (goal_id)
  3. Writes proposal to Vault (`Research/Ideas/{name}.md`)
  4. Discord notification: "New project proposed: {name}. Review in Web UI."
- Existing approval flow in `api_projects.go` handles:
  - `POST /api/projects/{id}/approve` → ACTIVE, creates GitHub repo via `github/client.go`
  - `POST /api/projects/{id}/reject` → REJECTED
- Minor change to `api_projects.go`: ensure PROPOSED projects appear in project list (verify existing filter doesn't exclude them)

**Acceptance criteria**:
- [ ] Researchers detect and parse project proposals from Claude Code output
- [ ] PROPOSED project created in DB with correct fields
- [ ] Discord notification sent
- [ ] Existing approval/rejection flow works for PROPOSED projects
- [ ] Proposal written to Vault
- [ ] Unit tests for proposal parsing

**Complexity**: Medium

**Depends on**: Tasks 4.1, 4.2, 4.3

---

### Task 4.5: Deployment Automation Enhancement

**Description**: Enhance the existing auto-updater and add `make deploy` target for manual deployments.

**Files to modify**:
```
Makefile
```

**Files to create**:
```
deploy/deploy.sh (if not already handled by updater)
```

**Implementation details**:
- The auto-updater (`internal/updater/`) already:
  - Polls git remote for changes
  - Rebuilds via `make build`
  - Restarts via `launchctl kickstart -k`
  - Reports status via WebSocket
- Add Makefile targets:
  - `make deploy`: `make build && deploy/deploy.sh`
  - `deploy/deploy.sh`:
    1. Copy binary to deployment location
    2. Restart via `launchctl kickstart -k gui/$(id -u)/com.circle-oo.flux`
    3. Verify restart successful (health check)
  - Safety: refuse to deploy if tasks are RUNNING (query `/api/pods`)
- This is primarily a convenience wrapper — the auto-updater handles the common case

**Acceptance criteria**:
- [ ] `make deploy` builds and deploys
- [ ] launchd restarts Flux
- [ ] Deploy blocked if tasks running
- [ ] Health check after restart

**Complexity**: Low

---

### Task 4.6: Self-Improvement

**Description**: Flux can modify its own code, test changes, and create PRs for self-improvement. Safety mechanisms prevent destructive changes.

**Files to create**:
```
internal/executor/self_improve.go
```

**Implementation details**:

**Safety protocol (executed in order)**:
1. Create worktree from flux repo using existing `executor/worktree.go`
2. Create safety tag: `flux-safe-{unix_timestamp}`
3. Execute Claude Code for modification
4. Run full test suite: `go test ./... -v` (from `go/src/` directory)
5. Run `gosec ./...` static security analysis (must be pre-installed: `go install github.com/securego/gosec/v2/cmd/gosec@latest`)
6. Run `go mod verify` dependency integrity check
7. Verify `go.mod` and `go.sum` unchanged (no dependency modifications without explicit approval)
8. **Pass**: create PR → **NEVER auto-merge** (always require Operator review)
9. **Fail**: `git checkout {safe_tag}` → revert → report FAILED
10. Add audit log entry to Vault (`Tasks/self-improvement/{timestamp}.md`)

**File restrictions** (immutable — cannot be modified by self-improvement):
- `go/src/internal/server/auth.go`
- `go/src/internal/config/config.go`
- `go/src/internal/shutdown/shutdown.go`
- `go/src/internal/executor/self_improve.go` (the safety mechanism itself)
- `go/src/internal/orchestrator/rate_limit_handler.go`
- `go/src/internal/db/schema.go` (DB schema changes require Operator task)
- `go/src/cmd/flux/main.go`

**Implementation in self_improve.go**:
- `ExecuteSelfImprovement(task *models.Task) error`:
  - Check task has source=SELF and appropriate tags
  - Validate target files against allowlist (reject if touching immutable files)
  - Execute safety protocol steps 1-10
  - Post-execution: check git diff for immutable file modifications → revert if found
- Called from Executor when task type is self-improvement (detected via tags or source)
- Self-improvement tasks: source=SELF, always require PR review (never auto-merge)

**Restart logic**:
- After self-improvement PR is merged (detected by auto-updater or manual trigger):
  - If all Pods are idle → trigger restart
  - If Pods are busy → queue restart for next idle window
  - Reuse existing auto-updater restart logic

**Acceptance criteria**:
- [ ] Safety tag created before any changes
- [ ] Full test suite runs after modification
- [ ] gosec security analysis passes
- [ ] Dependency integrity verified (go mod verify)
- [ ] go.mod and go.sum changes detected and rejected
- [ ] Immutable files protected (modification detected and reverted)
- [ ] Revert on any test/security failure
- [ ] PR created for passing changes
- [ ] Self-improvement PRs NEVER auto-merged (ShouldAutoMerge returns false for source=SELF)
- [ ] Audit log written to Vault
- [ ] Restart only at idle
- [ ] Unit tests for safety protocol (file allowlist, immutable check, revert logic)

**Complexity**: High

**Depends on**: Task 4.5

---

### Task 4.7: Research UI

**Description**: Add Research page to Web UI for viewing research history and findings.

**Files to create**:
```
frontend/src/pages/Research.tsx
frontend/src/stores/researchStore.ts
```

**Implementation details**:
- Research data lives in the existing `tasks` table (type=RESEARCH)
- No new API endpoints needed — reuse `GET /api/tasks?type=RESEARCH` with existing filtering
- If additional research-specific data is needed, add:
  - `GET /api/research/stats` — research type distribution (simple GROUP BY query)
- Create Zustand store (`researchStore.ts`) with filtered task fetching
- Research.tsx page:
  - Research history timeline (tasks with type=RESEARCH, sorted by date)
  - Filter by research type tag
  - Research findings preview (result field from task)
  - Active Researcher Pods status (from `/api/pods` filtered by id prefix "researcher-")
  - Simple type distribution summary
- Add to App.tsx navigation

**Acceptance criteria**:
- [ ] Research page shows research task history
- [ ] Type filtering works
- [ ] Active Researcher pod status displayed
- [ ] Findings previewed from task result field
- [ ] Responsive layout (Tailwind)
- [ ] Unit tests for any new API endpoints

**Complexity**: Medium

---

### Task 4.8: Obsidian CLI Integration for Pods

**Description**: Add Obsidian CLI usage patterns to Executor and Researcher pod prompts for reading project context and recording learnings. Pods use the CLI to access the knowledge base during execution.

**Files to modify**:
```
internal/executor/executor.go (prompt template updates)
internal/researcher/researcher.go (prompt template updates - if not already in 4.1)
```

**Implementation details**:
- Executor pods should read project context before starting work:
  - `obsidian read path="Projects/{project}/_index.md"` — get project overview
  - `obsidian search query="{keyword}" folder="Projects/{project}" format=json matches` — find relevant prior work
  - `obsidian backlinks path="Projects/{project}/_index.md"` — discover related notes
- Executor pods should save learnings after task completion:
  - `obsidian create name="Projects/{project}/learnings/{slug}" content="..." silent` — record new learnings
  - `obsidian daily:append content="- ✅ {task}: {summary}"` — log to daily note
  - `obsidian property:set name=status value=completed path="Tasks/{task}.md"` — update task properties
- Researcher pods should check existing research before starting:
  - `obsidian search query="{topic}" format=json matches` — avoid duplicate research
  - `obsidian tags all counts` — understand knowledge base structure
- Researcher pods should save research findings:
  - `obsidian create name="Research/{category}/{slug}" content="..." silent` — store findings
  - `obsidian daily:append content="- 🔬 Research: {topic}"` — log research activity
- Add these patterns to the CLAUDE.md templates injected into pod workspaces
- Include error handling guidance: if CLI fails, fall back to file-based operations
- Emphasize `silent` flag usage to prevent Obsidian UI pop-ups during background operations
- Note: This assumes Phase 3 Obsidian CLI integration (vault/client.go, vault/facade.go) is complete

**Acceptance criteria**:
- [ ] Executor CLAUDE.md template includes Obsidian CLI read/write patterns
- [ ] Researcher CLAUDE.md template includes research-specific CLI patterns
- [ ] Prompt includes examples of reading project context before work
- [ ] Prompt includes examples of saving learnings after completion
- [ ] Prompt includes fallback guidance when CLI unavailable
- [ ] `silent` flag usage is emphasized for all create operations
- [ ] Daily note logging patterns included
- [ ] Property management patterns included for task status updates

**Complexity**: Low-Medium (primarily prompt/template updates)

**Depends on**: Phase 3 Obsidian CLI integration (vault/client.go and vault/facade.go must exist), Task 4.1 (Researcher Pod)

---

## Recommended Implementation Order

```
Task 4.1  (Researcher Pod + Workspace)         — Foundation (must be first)
  ├── Task 4.2  (Autonomous Research)           — Core research logic
  │   └── Task 4.4  (Project Proposals)         — Research output
  └── Task 4.0  (ScaleManager Ratio Upgrade)    — Integrates Researchers into scaling

Task 4.3  (Vault Writer Upgrade)               — Independent (can parallel with 4.1)

Task 4.5  (Deployment Automation)              — Independent
  └── Task 4.6  (Self-Improvement)             — Requires deployment infra

Task 4.7  (Research UI)                        — After 4.1 + 4.2 (needs data to display)

Task 4.8  (Obsidian CLI Integration)           — After Phase 3 Obsidian CLI + 4.1 (Researcher Pod)
```

**Critical path**: 4.1 → 4.2 → 4.0 (gets Researchers running and auto-scaled)
**Parallel track 1**: 4.3 (Vault templates) → 4.8 (Obsidian CLI integration for pods)
**Parallel track 2**: 4.5 → 4.6 (self-improvement)
**Parallel track 3**: 4.7 (UI)

---

## Phase 4 Completion Checklist

- [ ] Researcher Pods run with independent workspaces
- [ ] Autonomous research when no tasks queued
- [ ] Research types correctly categorized and routed to Vault
- [ ] Vault Writer handles research findings and project proposals
- [ ] ScaleManager manages Executor:Researcher ratio per spec
- [ ] R&D protection (min 20% research) enforced
- [ ] Project proposals → Operator approval → GitHub repo
- [ ] Self-improvement: safety tag → modify → test → gosec → PR/revert
- [ ] Immutable files protected from self-improvement
- [ ] DB schema changes excluded from self-improvement
- [ ] Deployment enhanced via Makefile
- [ ] Restart at idle only
- [ ] Research UI functional
- [ ] Audit trail for all self-modifications
- [ ] Executor pods use Obsidian CLI for context and recording
- [ ] Researcher pods use Obsidian CLI for research history and findings storage
- [ ] Pod CLAUDE.md templates include Obsidian CLI usage patterns

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~5 files | ~4 files |
| React frontend | ~2 files | ~1 file |
| Deploy/Build | ~1 file | ~1 file |
| Prompt templates | 0 files | ~2 files (executor, researcher) |
| **Total** | **~8 files** | **~8 files** |

---

## Post-Phase 4: Future Work

After Phase 4, Flux is fully autonomous. Future improvements are documented in the spec under "Future Work".

### Related Plans

Phase 4 integrates with or depends on features from other plan documents:

- **Obsidian Integration** (`docs/obsidian-integration-plan.md`): Phase 3a-3c (CLI Client, Knowledge API, Knowledge Frontend) are prerequisites. Phase 3d is integrated as Task 4.8. The Knowledge UI provides the interface for viewing research findings created by Researcher pods.

- **Insight/Analytics** (`docs/insight-plan.md`): Comprehensive analytics and observability features (Tasks 1-5 from insight plan) are implemented as Phase 3 features and provide operational intelligence through existing task/usage data. They are independent of Phase 4 research features but complement them by showing research productivity metrics.

### Original Design Deferrals
- Service monitoring (HTTP healthchecks, incident reports) — upgrade `/api/services` stub
- Multiple Goals + ranking
- Issue system, limit learning
- Log analysis monitoring, process/resource monitoring
- Watchdog process
- DB schema migration system
- PR table separation (tasks table getting large)
- Rate Limit 3rd-tier detection

### Competitive Analysis Gaps
- Execution environment isolation (deeper sandboxing)
- Code review feedback learning (learn from PR review patterns)
- External event triggers (GitHub webhooks, Discord bot commands)
- CI/CD integration
- Multi-repo tasks (cross-repository changes)
- Session persistence / context sharing between Pods
- Mobile accessibility

### OMC-Inspired Ideas
- Agent definition patterns (`.claude/agents/` per task type)
- CLAUDE.md auto-refresh (3-tier notepad structure)
- Native Agent Teams utilization (sub-agent parallel execution within single Executor)

**Note**: OMC is an interactive-session orchestrator and should NOT be used as a Flux plugin (dual orchestration conflict). Only ideas are borrowed.
