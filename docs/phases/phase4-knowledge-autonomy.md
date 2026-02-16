# Phase 4: Knowledge & Autonomy — "Autonomous growth"

## Goal

Flux becomes a fully autonomous system: researcher agents (via Python Agent SDK) explore technologies, accumulate knowledge in Obsidian (via obsidian-cli), propose new projects, and improve Flux's own code. The Operator only sets strategic direction (Goals), approves new projects, and reviews complex PRs.

## Deliverable

Researcher agents running autonomous research via Agent SDK, systematic Obsidian knowledge base accessible to all agents, project proposals with Operator approval, Flux self-improvement with safety mechanisms, Research UI.

## Prerequisites

- Phase 3 complete (autonomous scaling, insights analytics, usage tracking, Knowledge API + UI)
- Obsidian CLI working and accessible to agents (Phase 2C)
- Python Agent SDK running with dev/qa/devops/rnd agent types (Phase 2C)
- Obsidian Go CLI Client + VaultFacade (Phase 3 Knowledge backend)

---

## Architecture Context (Post-Phase 2C)

```
Operator → Frontend (Connect-RPC) → Go Backend → gRPC → Python Agent Manager
                                                           ├── dev agent   (coding)
                                                           ├── qa agent    (testing)
                                                           ├── devops agent(deploy)
                                                           └── rnd agent   (research) ← Phase 4 focus
                                                                 │
                                                                 ├── Bash("obsidian-cli search ...")
                                                                 ├── Bash("obsidian-cli read ...")
                                                                 └── Bash("obsidian-cli write ...")
```

---

## Task Breakdown

### Task 4.1: Researcher Agent Enhancement — Specialized Research Types

**Description**: Enhance the `rnd` agent type in the Python Agent Manager with specialized research type configurations, structured output formats, and deep Obsidian knowledge integration.

**Files to create/modify**:
```
agent_manager/research_types.py         (new)
agent_manager/config.py                 (modify)
```

**Implementation details**:

**research_types.py** — Research type definitions:
- Research types: `github-scan`, `dependency-check`, `industry-research`, `project-ideas`, `opensource-scan`, `self-improve`, `library-audit`, `service-review`, `goal-research`
- Each type has:
  - Description
  - Prompt template (injected into agent system prompt)
  - Output format instructions (structured markdown)
  - Vault destination path (e.g., `Research/Industry/`, `Research/Tools/`)
- Structured output instruction appended to rnd system prompt:
  ```
  After completing research, write findings to Obsidian using:
  $ obsidian-cli write "Research/{category}/{topic}.md" --content "..." --vault {vault_path}
  $ obsidian-cli daily:append content="- 🔬 Research: {topic}" --vault {vault_path}

  Use this format for findings:
  # {Topic}
  **Date**: {date}
  **Type**: {research_type}
  ## Summary
  {key findings}
  ## Details
  {detailed analysis}
  ## Recommendations
  {actionable items}
  ## Sources
  {references}
  ```

**Enhanced rnd config in config.py**:
- System prompt includes: research methodology, obsidian-cli usage patterns, structured output format
- max_turns=200 (research requires more exploration)
- allowed_tools includes WebSearch if MCP available
- Research type is passed via `metadata["research_type"]` in ExecuteTaskRequest

**Acceptance criteria**:
- [ ] All 9 research types defined with templates
- [ ] rnd agents produce structured Obsidian-compatible output
- [ ] Research findings written to correct Vault paths via obsidian-cli
- [ ] Research type selection works based on task metadata
- [ ] System prompt properly assembled from type template + base config

**Complexity**: Medium

---

### Task 4.2: Autonomous Research Scheduling

**Description**: When no research tasks are in the queue, the Go Orchestrator creates self-assigned research tasks and dispatches them to the Python Agent Manager.

**Files to create**:
```
go/src/internal/orchestrator/research_scheduler.go
```

**Implementation details**:
- `ResearchScheduler` struct: config, manager, db, notifier
- `ScheduleIfIdle()`: Called by Orchestrator when rnd agents have no tasks
- Decision logic when rnd queue empty:
  1. Check current Goal → research topics aligned with Goal (`goal-research`)
  2. Check active projects → `dependency-check`, `library-audit`
  3. Check recent research tasks in DB (avoid duplicates in last 7 days)
  4. Select research type based on simple priority order:
     - Goal-aligned research (highest, when Goal active)
     - Dependency checks for active projects
     - Industry trends
     - Tool evaluations
     - Project ideas (lowest)
- Create self-assigned task:
  - source=SYSTEM, type=RESEARCH, priority=70
  - metadata: `agent_type=rnd`, `research_type={selected_type}`
  - Prompt includes Goal context and type-specific instructions
- Minimum 30-min gap between autonomous research sessions
- Log research scheduling decisions in DB
- **Design principle**: Keep Go-side logic simple. Let the Python Agent (via Claude) make intelligent decisions about *what* to research within the selected category.

**Acceptance criteria**:
- [ ] Autonomous research triggered when rnd queue empty
- [ ] Research type selected based on system state
- [ ] Goal alignment considered when Goal is active
- [ ] Recent research checked to avoid duplicates
- [ ] Self-created tasks have correct priority (P:70)
- [ ] Minimum 30-min gap enforced between autonomous sessions
- [ ] Unit tests for scheduling logic

**Complexity**: Medium-High

**Depends on**: Task 4.1

---

### Task 4.3: Knowledge Context Injection for All Agents

**Description**: Inject relevant Obsidian knowledge context into all agent system prompts, not just rnd agents. Dev agents should read project docs before coding, qa agents should check known issues, etc.

**Files to modify**:
```
agent_manager/config.py                 (modify — knowledge patterns per type)
agent_manager/knowledge.py              (new)
go/src/internal/handler/flux_service.go (modify — enrich system prompt)
```

**Implementation details**:

**knowledge.py** — Knowledge context builder:
- `build_knowledge_prompt(agent_type, task_metadata, vault_path) → str`:
  - Returns obsidian-cli usage patterns specific to the agent type
  - Dev: "Read project docs before starting, save learnings after"
  - QA: "Check known issues, read test patterns"
  - R&D: "Check existing research, write structured findings"
  - DevOps: "Read deployment docs, check service configs"

**Agent-specific obsidian-cli patterns**:
```
# Dev agents:
Before starting work:
  $ obsidian-cli search "{project_name} architecture" --vault {vault_path}
  $ obsidian-cli read "Projects/{project}/_index.md" --vault {vault_path}
After completing work:
  $ obsidian-cli write "Projects/{project}/learnings/{slug}.md" --content "..." --vault {vault_path}

# QA agents:
  $ obsidian-cli search "known issues {project}" --vault {vault_path}

# R&D agents:
  $ obsidian-cli search "{topic}" --vault {vault_path}   # check existing research
  $ obsidian-cli write "Research/{category}/{topic}.md" --content "..." --vault {vault_path}
```

**Go-side enhancement**:
- When building `ExecuteTaskRequest` in `flux_service.go`, append knowledge context to system_prompt
- Use VaultFacade (Phase 3) to pre-fetch relevant vault snippets:
  - Search vault for notes related to task title (top 3 results)
  - Include snippets in system_prompt (max 2000 chars of context)
  - This gives the agent a head start before it needs to use obsidian-cli

**Acceptance criteria**:
- [ ] All agent types receive obsidian-cli usage instructions
- [ ] Dev agents get project-specific context
- [ ] Pre-fetched vault context included in system prompt
- [ ] Context size limited to prevent prompt overflow
- [ ] No regression in existing agent behavior

**Complexity**: Medium

**Depends on**: Phase 3 Knowledge backend (VaultFacade)

---

### Task 4.4: New Project Proposals

**Description**: Researcher agents can propose new projects based on discoveries. Proposals are detected by the Orchestrator and require Operator approval.

**Files to modify**:
```
go/src/internal/orchestrator/research_scheduler.go  (add proposal detection)
agent_manager/config.py                             (rnd prompt includes proposal format)
```

**Implementation details**:
- rnd agent system prompt includes project proposal format:
  ```
  If you discover a valuable project opportunity, create a proposal:
  $ obsidian-cli write "Goals/proposals/{project-name}.md" --content "..." --vault {vault_path}

  Proposal format:
  # Project Proposal: {name}
  **Type**: REPO|SERVICE|LIBRARY|TOOL
  **Tech Stack**: [list]
  **Inspiration**: {what research led to this}
  **Description**: {what and why}
  ```
- Go-side: ResearchScheduler periodically scans Vault `Goals/proposals/` for new files (via VaultFacade)
- When proposal found:
  1. Parse markdown for project metadata
  2. Create Project in DB with status=PROPOSED
  3. Discord notification: "New project proposed: {name}. Review in Web UI."
- Existing approval flow handles the rest:
  - Approve → ACTIVE, create GitHub repo
  - Reject → REJECTED

**Acceptance criteria**:
- [ ] rnd agents create proposals in Vault via obsidian-cli
- [ ] Orchestrator detects new proposals
- [ ] PROPOSED projects created in DB
- [ ] Discord notification sent
- [ ] Existing approval/rejection flow works
- [ ] Unit tests for proposal parsing

**Complexity**: Medium

**Depends on**: Tasks 4.1, 4.2

---

### Task 4.5: Deployment Automation

**Description**: Automate Flux deployment for both Go backend and Python Agent Manager after self-improvement builds pass.

**Files to create/modify**:
```
deploy/deploy.sh                        (new or update)
Makefile                                (add deploy target)
```

**Implementation details**:
- `deploy.sh`:
  1. Regenerate proto: `buf generate proto`
  2. Build Go: `go build -o flux ./cmd/flux`
  3. Build Frontend: `cd frontend && npm run build`
  4. Run Go tests: `go test ./...`
  5. Run Python tests: `pytest agent_manager/`
  6. Copy Go binary to deployment location
  7. Install Python dependencies if changed: `pip install -r agent_manager/requirements.txt`
  8. Restart Go via launchd: `launchctl kickstart -k gui/$(id -u)/com.circle-oo.flux`
  9. Restart Python via launchd: `launchctl kickstart -k gui/$(id -u)/com.circle-oo.flux-agent`
- Makefile targets:
  - `make proto`: `buf generate proto`
  - `make build`: proto + Go build + Frontend build
  - `make test`: Go tests + Python tests
  - `make deploy`: build + test + deploy.sh
- Deploy only at idle (no running agent tasks)
- Health check after restart (verify gRPC connectivity)

**Acceptance criteria**:
- [ ] `make deploy` builds and deploys in one command
- [ ] Proto regenerated before build
- [ ] Both Go and Python services restarted
- [ ] Deploy blocked if agent tasks running
- [ ] Health check verifies both services after restart

**Complexity**: Low-Medium

---

### Task 4.6: Self-Improvement

**Description**: Flux can modify its own code (both Go and Python), test changes, and create PRs for self-improvement. Safety mechanisms prevent destructive changes.

**Files to create**:
```
go/src/internal/executor/self_improve.go
```

**Implementation details**:

**Safety protocol (executed in order)**:
1. Create worktree from flux repo
2. Create safety tag: `flux-safe-{unix_timestamp}`
3. Execute dev agent (via Python Agent Manager) for modification
4. Run full Go test suite: `go test ./... -v`
5. Run `gosec ./...` static security analysis
6. Run `go mod verify` dependency integrity check
7. Verify `go.mod` and `go.sum` unchanged
8. Run Python tests: `pytest agent_manager/`
9. Verify `requirements.txt` unchanged
10. Verify `proto/flux/v1/flux.proto` unchanged (API contract immutable)
11. **Pass**: create PR → **NEVER auto-merge** (always Operator review)
12. **Fail**: `git checkout {safe_tag}` → revert → report FAILED
13. Add audit log entry to Vault (`Tasks/self-improvement/{timestamp}.md`)

**Immutable files** (cannot be modified by self-improvement):
- `go/src/internal/server/auth.go`
- `go/src/internal/config/config.go`
- `go/src/internal/shutdown/shutdown.go`
- `go/src/internal/executor/self_improve.go`
- `go/src/internal/orchestrator/rate_limit_handler.go`
- `go/src/cmd/flux/main.go`
- `agent_manager/server.py` (core gRPC server)
- `proto/flux/v1/flux.proto` (API contract)
- DB schema files

**Restart logic**:
- After self-improvement PR merged: check if agents are idle
- If idle: trigger `make deploy`
- If busy: queue restart for next idle period

**Acceptance criteria**:
- [ ] Safety tag created before changes
- [ ] Full Go + Python test suites run
- [ ] Security analysis passes
- [ ] Dependency integrity verified
- [ ] API contract (proto) unchanged
- [ ] Revert on any failure
- [ ] PR created (never auto-merged)
- [ ] Immutable files protected
- [ ] Audit trail in Vault
- [ ] Restart only at idle

**Complexity**: High

**Depends on**: Task 4.5

---

### Task 4.7: Research UI

**Description**: Add or enhance Research page in the frontend for viewing research history, findings, and researcher agent activity.

**Files to create/modify**:
```
frontend/src/pages/Research.tsx             (new or enhance from Phase 3 Knowledge)
proto/flux/v1/flux.proto                    (add Research RPCs if needed)
go/src/internal/handler/flux_service.go     (implement Research RPCs)
```

**Implementation details**:

If Phase 3 already has a Knowledge page with Research tab, this task enhances it. Otherwise, create standalone:

**New RPCs** (if not covered by Phase 3 Knowledge API):
```protobuf
rpc ListResearch(ListResearchRequest) returns (ListResearchResponse);
rpc GetResearchStats(GetResearchStatsRequest) returns (GetResearchStatsResponse);
```

**Research UI features**:
- Research history timeline (completed rnd agent tasks, sorted by date)
- Filter by research type (github-scan, industry-research, etc.)
- Research findings preview (from Vault via Knowledge API)
- Active researcher agent status (from `GetAgentStatus`)
- Research type distribution chart (pie/donut)
- Recent project proposals from researchers
- Autonomous research scheduling status (next scheduled, current type weights)

**Acceptance criteria**:
- [ ] Research page shows research task history
- [ ] Type filtering works
- [ ] Findings preview from Vault
- [ ] Agent status displayed
- [ ] Statistics chart renders
- [ ] Proposals section shows recent proposals

**Complexity**: Medium

---

## Recommended Implementation Order

```
Task 4.1  (Researcher Agent Types)          — Foundation
  ├── Task 4.2  (Autonomous Research)       — Core scheduling
  │   └── Task 4.4  (Project Proposals)     — Research output
  └── Task 4.3  (Knowledge Context)         — All agents benefit

Task 4.5  (Deployment Automation)           — Independent
  └── Task 4.6  (Self-Improvement)          — Requires deploy infra

Task 4.7  (Research UI)                     — After 4.1 + 4.2 (needs data)
```

**Critical path**: 4.1 → 4.2 → 4.4 (gets autonomous research and proposals working)
**Parallel track 1**: 4.3 (knowledge context for all agents)
**Parallel track 2**: 4.5 → 4.6 (self-improvement)
**Final**: 4.7 (Research UI, once data exists)

---

## Phase 4 Completion Checklist

- [ ] Researcher agents run with specialized research types via Agent SDK
- [ ] Autonomous research scheduling when no rnd tasks queued
- [ ] All agents have Obsidian knowledge access via obsidian-cli
- [ ] Knowledge context pre-fetched and injected into system prompts
- [ ] Research findings written to correct Vault paths
- [ ] Project proposals → Operator approval → GitHub repo
- [ ] Self-improvement: safety tag → modify → test (Go + Python) → PR/revert
- [ ] Proto file and immutable files protected
- [ ] Deployment automated via Makefile (both Go + Python)
- [ ] Restart at idle only
- [ ] Research UI functional with charts and filtering
- [ ] Audit trail for all self-modifications

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~2 files | ~2 files |
| Python agent | ~2 files | ~1 file |
| Frontend | ~1 file | ~1 file |
| Proto | — | ~1 file (if needed) |
| Deploy | ~1 file | ~1 file |
| **Total** | **~6 files** | **~6 files** |

---

## Post-Phase 4: Future Work

After Phase 4, Flux is fully autonomous with the protobuf-unified architecture. Future improvements:

### Architecture Extensions
- Multi-agent collaboration (agents communicating via shared context)
- MCP server integration for web search, external APIs
- Session persistence / context sharing between agent runs
- Agent-to-agent delegation (dev agent spawning qa agent for review)

### Operational Extensions
- Service monitoring (HTTP healthchecks via devops agents)
- External event triggers (GitHub webhooks, Discord bot commands)
- CI/CD integration (agents triggered by PR events)
- Multi-repo tasks (cross-project work)
- Mobile-friendly dashboard

### Knowledge Extensions
- Knowledge quality verification (agents review each other's research)
- Automatic knowledge graph construction via Obsidian links
- Research quality scoring and feedback loops
- Obsidian dashboard auto-refresh

### Scale Extensions
- Multiple Mac Minis (distributed agent pool)
- PostgreSQL migration for larger datasets
- Agent Teams utilization (sub-agent parallel execution)
- Rate Limit 3rd-tier detection and response
