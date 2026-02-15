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

## Task Breakdown

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
- `Researcher` struct: id (e.g., "researcher-01"), config, claude, workspace, manager, vault, notifier, stopCh
- `Run(ctx)`: main loop similar to Executor
  - Request research task from Manager (pod_type="researcher")
  - If no task: autonomous research decision (see Task 4.2)
  - Execute in independent workspace
  - Write findings to Vault
  - Report completion

**types.go**:
- Research types: `github-scan`, `dependency-check`, `industry-research`, `project-ideas`, `opensource-scan`, `self-improve`, `library-audit`, `service-review`, `goal-research`
- Each type has: description, typical prompt template, output format, Vault destination

**workspace.go**:
- Per-Pod workspace structure:
  ```
  workspaces/research/researcher-01/
  ├── workspace/     # CLAUDE.md, .claude/settings.json
  └── output/        # Temporary → moved to Obsidian
  ```
- `SetupWorkspace(podID)`: create directories, CLAUDE.md with research context, .claude/settings.json
- Independent workspaces prevent parallel conflicts between Researcher Pods

**Acceptance criteria**:
- [ ] Researcher Pod runs independently
- [ ] Per-Pod workspace isolation
- [ ] Research tasks fetched from Manager
- [ ] Autonomous research when no tasks
- [ ] Findings written to Vault
- [ ] Stop signal honored

**Complexity**: High

---

### Task 4.2: Autonomous Research Scheduling

**Description**: When no research tasks are in the queue, Researchers autonomously decide what to research.

**Files to modify**:
```
internal/researcher/researcher.go
```

**Implementation details**:
- Decision logic when queue empty:
  1. Check current Goal → research topics aligned with Goal
  2. Check active projects → dependency updates, security audits
  3. Check research history (Vault `_history.md`) → avoid duplicates
  4. Select research type based on weighted priorities:
     - Goal-aligned research (highest weight when Goal active)
     - Dependency checks for active projects
     - Industry trends
     - Tool evaluations
     - Project ideas (lowest weight, but always possible)
- Create self-assigned research task with appropriate type and priority (P:61-80)
- Log research decision in Vault `Research/_history.md`

**Acceptance criteria**:
- [ ] Autonomous research triggered when queue empty
- [ ] Research type selected intelligently
- [ ] Goal alignment considered
- [ ] Research history consulted to avoid duplicates
- [ ] Self-created tasks have correct priority range

**Complexity**: Medium-High

**Depends on**: Task 4.1

---

### Task 4.3: Vault Writer Upgrade

**Description**: Upgrade Vault Writer to handle all Pod types writing to Obsidian with proper templates and organization.

**Files to modify**:
```
internal/vault/writer.go
internal/vault/templates.go
```

**Implementation details**:

**Enhanced templates**:
- Task completion summary (already in Phase 2B, enhance)
- Research finding: type, sources, key findings, recommendations, links
- Project proposal: name, description, inspiration, tech stack, estimated effort
- Decision record: context, options considered, decision, rationale
- Learning record: topic, what was learned, how it applies

**Vault path routing**:
- Task summaries → `Tasks/completed/`
- Research findings → `Research/{category}/` (Industry, Tools, Ideas)
- Project knowledge → `Projects/{name}/learnings/`
- Decision records → `Projects/{name}/decisions/`
- Goal progress → `Goals/_current.md` (append)
- Research history → `Research/_history.md` (append)

**Acceptance criteria**:
- [ ] All Pod types can write to Vault
- [ ] Templates produce well-formatted Obsidian markdown
- [ ] Correct path routing per content type
- [ ] Sequential writes maintained (no file conflicts)
- [ ] Research history appended correctly

**Complexity**: Medium

**Depends on**: Phase 2B Task 2B.7

---

### Task 4.4: New Project Proposals

**Description**: Researchers can propose new projects based on discoveries. Projects require Operator approval before creation.

**Files to modify**:
```
internal/researcher/researcher.go
internal/server/api_projects.go
```

**Implementation details**:
- Researcher identifies project opportunity during research
- Creates project with status PROPOSED:
  - Name, type (REPO, SERVICE, LIBRARY, TOOL)
  - Description, tech_stack, inspiration (research context)
  - goal_id (link to current Goal if relevant)
- Discord notification: "New project proposed: {name}. Review in Web UI."
- Operator approval flow (already in Phase 1):
  - Approve → ACTIVE, create GitHub repo, create Vault `Projects/{name}/` structure
  - Reject → REJECTED

**Acceptance criteria**:
- [ ] Researchers create PROPOSED projects
- [ ] Discord notification for proposals
- [ ] Approval creates GitHub repo
- [ ] Approval creates Vault structure
- [ ] Rejection stops further work
- [ ] Project linked to Goal and research

**Complexity**: Medium

**Depends on**: Tasks 4.1, 4.2

---

### Task 4.5: Deployment Automation

**Description**: Automate Flux binary deployment after self-improvement builds pass.

**Files to create**:
```
deploy/deploy.sh
Makefile (add deploy target)
```

**Implementation details**:
- `deploy.sh`:
  1. Build: `go build -o flux ./cmd/flux`
  2. Build Web UI: `cd web && npm run build`
  3. Copy binary to deployment location (`/usr/local/bin/flux`)
  4. Restart via launchd: `launchctl kickstart -k gui/$(id -u)/com.circle-oo.flux`
- Makefile targets:
  - `make build`: Go build + Web UI build
  - `make test`: `go test ./...`
  - `make deploy`: build + deploy.sh
- Deploy only at idle (no running tasks)

**Acceptance criteria**:
- [ ] `make deploy` builds and deploys in one command
- [ ] Binary placed in correct location
- [ ] launchd restarts Flux
- [ ] Web UI built and embedded
- [ ] Deploy blocked if tasks running

**Complexity**: Low-Medium

---

### Task 4.6: Self-Improvement

**Description**: Flux can modify its own code, test the changes, and create PRs for self-improvement. Safety mechanisms prevent destructive changes.

**Files to create/modify**:
```
internal/executor/self_improve.go
```

**Implementation details**:

**Safety protocol**:
1. Create worktree from flux repo
2. Create safety tag: `flux-safe-{unix_timestamp}`
3. Execute Claude Code for modification
4. Run **full test suite**: `go test ./... -v`
5. **Pass**: create PR → auto-merge (restart at idle)
6. **Fail**: `git checkout {safe_tag}` → revert → report FAILED

**Restrictions**:
- DB schema changes EXCLUDED from self-improvement scope
- Schema changes → create PROPOSED task for Operator approval
- Self-improvement tasks: source=SELF, auto-merge eligible

**Restart logic**:
- After successful self-improvement merge: check if Pods are idle
- If idle: trigger restart via `launchctl kickstart`
- If busy: queue restart for next idle period

**Acceptance criteria**:
- [ ] Self-improvement creates safety tag before changes
- [ ] Full test suite runs after modification
- [ ] Revert on test failure
- [ ] PR created for passing changes
- [ ] DB schema changes detected and excluded
- [ ] Restart only at idle
- [ ] Safety tag preserved for rollback

**Complexity**: High

**Depends on**: Task 4.5

---

### Task 4.7: Research UI

**Description**: Add Research page to Web UI for viewing research history and findings.

**Files to create**:
```
web/src/pages/Research.tsx
web/src/stores/researchStore.ts
```

**Files to modify**:
```
internal/server/api_research.go (new file)
```

**API endpoints**:
```
GET /api/research              — List research tasks (type filter)
GET /api/research/history      — Research activity timeline
GET /api/research/stats        — Research type distribution
```

**UI features**:
- Research history timeline
- Filter by research type (github-scan, industry-research, etc.)
- Research findings preview (from Vault)
- Active Researcher Pods status
- Research type distribution chart

**Acceptance criteria**:
- [ ] Research page shows history
- [ ] Type filtering works
- [ ] Findings linked to Vault content
- [ ] Pod status displayed
- [ ] Statistics chart renders

**Complexity**: Medium

---

## Phase 4 Completion Checklist

- [ ] Researcher Pods run with independent workspaces
- [ ] Autonomous research when no tasks queued
- [ ] Research types correctly categorized and routed
- [ ] Vault Writer handles all content types
- [ ] Obsidian knowledge organized by structure
- [ ] Project proposals → Operator approval → GitHub repo
- [ ] Self-improvement: safety tag → modify → test → PR/revert
- [ ] DB schema changes excluded from self-improvement
- [ ] Deployment automated via Makefile
- [ ] Restart at idle only
- [ ] Research UI functional

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~6 files | ~4 files |
| React frontend | ~2 files | ~2 files |
| Deploy | ~2 files | — |
| **Total** | **~10 files** | **~6 files** |

---

## Post-Phase 4: Future Work

After Phase 4, Flux is fully autonomous. Future improvements are documented in the spec under "Future Work":

### Original Design Deferrals
- Service monitoring (HTTP healthchecks, incident reports)
- Multiple Goals + ranking
- Issue system, limit learning
- Log analysis monitoring, process/resource monitoring
- Watchdog process
- DB schema migration system
- PR table separation
- Rate Limit 3rd-tier detection

### Competitive Analysis Gaps
- Execution environment isolation (deeper sandboxing)
- Code review feedback learning
- External event triggers (GitHub webhooks, Discord bot commands)
- CI/CD integration
- Multi-repo tasks
- Session persistence / context sharing
- Mobile accessibility

### OMC-Inspired Ideas
- Agent definition patterns (`.claude/agents/` per task type)
- CLAUDE.md auto-refresh (3-tier notepad structure)
- Native Agent Teams utilization (sub-agent parallel execution within single Executor)

**Note**: OMC is an interactive-session orchestrator and should NOT be used as a Flux plugin (dual orchestration conflict). Only ideas are borrowed.
