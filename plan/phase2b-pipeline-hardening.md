# Phase 2B: Pipeline Hardening — "Runs reliably"

## Goal

Add safety nets, usage tracking, knowledge recording, and crash recovery so Flux can run unattended. The Operator still registers tasks and reviews PRs, but rate limits, crashes, and reboots are handled automatically.

## Deliverable

Rate limit basic response (fixed 5h wait), model selection (Sonnet/Opus), Goal prompt injection, Vault recording, launchd registration, crash recovery.

## Prerequisites

- Phase 2A complete (end-to-end task → PR pipeline working)
- Phase 2A experiments completed (rate limit patterns, sandbox decision)
- ccusage installed (optional but recommended)

---

## Task Breakdown

### Task 2B.1: Rate Limit Detection Implementation

**Description**: Implement production rate limit detection using patterns discovered in the Phase 2A experiment.

**Files to create**:
```
internal/orchestrator/rate_limit_handler.go
```

**Implementation details**:
- `RateLimitDetector` struct
- `Detect(exitCode int, stderr string) bool`
- Two-stage detection:
  1. Exit code check (429 or value from experiment)
  2. Stderr pattern matching (confirmed patterns from Phase 2A experiment)
- Default patterns: "rate limit", "too many requests", "429", "capacity", "try again"
- Patterns should be configurable (update based on experiment results)

**Acceptance criteria**:
- [ ] Detection matches Phase 2A experiment findings
- [ ] Both exit code and stderr patterns checked
- [ ] Case-insensitive pattern matching
- [ ] No false positives on normal error messages

**Complexity**: Low

**Depends on**: Phase 2A Task 2A.13 results

---

### Task 2B.2: Rate Limit Basic Response

**Description**: When rate limit is detected: stop all Pods, wait fixed 5 hours, resume.

**Files to modify**:
```
internal/orchestrator/rate_limit_handler.go
```

**Implementation details**:
- `HandleRateLimit()`:
  1. Stop all running Pods (signal stop channel)
  2. Send Discord WARNING: "Rate limit detected. Stopping all pods. Waiting 5 hours."
  3. Record event in `rate_limit_events` table
  4. Sleep 5 hours (blocking, but cancellable via context)
  5. Resume Pods
- `RecentlyLimited() bool`: check if rate limit occurred in last 6 hours (for model selection)
- `IsLimited() bool`: check if currently in rate limit wait state

**Acceptance criteria**:
- [ ] All Pods stop on rate limit detection
- [ ] Discord notification sent
- [ ] Event recorded in DB
- [ ] 5-hour wait observed (cancellable)
- [ ] Pods resume after wait
- [ ] `RecentlyLimited()` returns correct state

**Complexity**: Medium

**Depends on**: Task 2B.1

---

### Task 2B.3: Model Selection

**Description**: Implement Sonnet/Opus model selection logic based on task complexity and rate limit state.

**Files to create/modify**:
```
internal/orchestrator/orchestrator.go (SelectModel method)
internal/server/internal_api.go (update model endpoint)
```

**Implementation details**:
- `SelectModel(task *Task) string`:
  - If `RecentlyLimited()` → always Sonnet
  - If `task.NeedsOpus()` → Opus
  - Otherwise → Sonnet
- `NeedsOpus()` conditions:
  - Priority ≤ 5 (service incidents)
  - Operator source + complex keywords
  - Tag "initial-design"
  - Tag "goal-strategy"
- Update `GET /internal/model/:task_id` to use this logic

**Acceptance criteria**:
- [ ] Default model is Sonnet
- [ ] Opus selected only for qualifying tasks
- [ ] Rate limit suppresses Opus selection
- [ ] Internal API returns correct model per task

**Complexity**: Low

**Depends on**: Task 2B.2

---

### Task 2B.4: Goal Prompt Injection

**Description**: Inject the current active Goal into every Claude Code execution via `--append-system-prompt`.

**Files to modify**:
```
internal/executor/executor.go (buildSystemPrompt method)
```

**Implementation details**:
- `buildSystemPrompt(task *Task) string`:
  - Fetch current ACTIVE Goal from Manager
  - Format: "Current Goal: {title}\nDescription: {desc}\nPriorities: {p1, p2}\nMetrics: {m1, m2}\nAll your work should align with this Goal."
  - Return empty string if no active Goal
- Pass to Claude Code via `--append-system-prompt`

**Acceptance criteria**:
- [ ] Goal injected into every Claude Code execution
- [ ] No crash when no active Goal
- [ ] Goal content matches current ACTIVE goal
- [ ] Prompt format matches spec

**Complexity**: Low

**Depends on**: Phase 2A Task 2A.8

---

### Task 2B.5: Subtask Decomposition

**Description**: Enable Claude Code to autonomously decide when a task is too large and output a decomposition plan instead of code.

**Files to create/modify**:
```
internal/executor/subtask.go
internal/executor/executor.go (add decomposition check)
```

**Implementation details**:
- Every Executor prompt includes the decomposition instruction:
  ```
  If this task is too large to complete in a single session,
  DO NOT write code. Instead, output only a decomposition plan as JSON:
  {"decompose": true, "subtasks": [{"title": "...", "description": "..."}, ...]}
  Maximum 5 subtasks. Each should be independently completable.
  ```
- `parseDecomposition(stdout string) *Decomposition`: parse JSON from Claude Code output
- If decomposition detected: create subtasks via internal API, report task as COMPLETED with "decomposed into subtasks"
- Manager validates: depth ≤ 1, max 5 subtasks per parent
- Subtasks inherit parent's priority and goal_id

**Acceptance criteria**:
- [ ] Decomposition prompt included in every Executor execution
- [ ] JSON decomposition parsed correctly
- [ ] Subtasks created via Manager API
- [ ] Depth and count limits enforced
- [ ] Priority and goal_id inherited
- [ ] Parent task marked COMPLETED after decomposition

**Complexity**: Medium

**Depends on**: Phase 2A Tasks 2A.7, 2A.8

---

### Task 2B.6: Manager Enhancement

**Description**: Add Goal boost and dependency checking to the Manager.

**Files to modify**:
```
internal/manager/manager.go
internal/manager/priority.go
```

**Implementation details**:
- **Goal boost**: When popping tasks, Goal-related tasks (matching current goal_id) get priority boost within their tier
  - E.g., a P:50 task related to the active Goal gets popped before a P:48 task unrelated to the Goal (within same tier)
  - Boost only within tier, never cross-tier
- **Dependency check**: Before assigning a task, verify all `depends_on` tasks are COMPLETED
  - If dependencies not met: skip task, try next in queue
  - Task stays in READY state until dependencies resolve

**Acceptance criteria**:
- [ ] Goal-related tasks prioritized within tier
- [ ] Tasks with unmet dependencies not assigned
- [ ] Dependencies resolved when blocking tasks complete
- [ ] No deadlocks with circular dependencies (reject at creation time)

**Complexity**: Medium

**Depends on**: Phase 2A Task 2A.7

---

### Task 2B.7: Vault Writer

**Description**: Implement the single-goroutine channel-based writer for Obsidian Vault.

**Files to create**:
```
internal/vault/writer.go
internal/vault/templates.go
```

**Implementation details**:
- `Writer` struct: basePath, requests channel (buffered 100)
- `NewWriter(basePath)`: start goroutine processing requests
- `run()`: range over channel, process each WriteRequest sequentially
- `Write(path, content, mode) error`: send request, wait for completion via Done channel
- `Close()`: close channel, drain remaining requests
- Write modes: Create (error if exists), Append, Replace
- Atomic operations: ensure directory exists, write file
- `templates.go`: markdown templates for task summaries, project docs, decision records

**Acceptance criteria**:
- [ ] Sequential writes (no file conflicts)
- [ ] All write modes work (create, append, replace)
- [ ] Directory auto-creation
- [ ] Done channel signals completion
- [ ] Close drains remaining requests
- [ ] Templates produce valid Obsidian markdown

**Complexity**: Medium

---

### Task 2B.8: Minimal Vault Recording

**Description**: Record task completion summaries to Obsidian Vault.

**Files to modify**:
```
internal/executor/executor.go (add Vault write after completion)
```

**Implementation details**:
- After task completion, write summary to `Tasks/completed/{taskID[:8]}.md`
- Template includes: task title, type, status, duration, model used, PR URL, diff stats, result summary
- Use Vault Writer (Task 2B.7)
- No complex knowledge extraction yet (Phase 4)

**Acceptance criteria**:
- [ ] Task summary written to Vault on completion
- [ ] Summary includes key metadata
- [ ] File created in correct Vault path
- [ ] Multiple tasks don't conflict (sequential via Writer)

**Complexity**: Low

**Depends on**: Task 2B.7

---

### Task 2B.9: ccusage Project Name Mapping Verification

**Description**: Verify that Flux's worktree path encoding matches ccusage's `--project` identifier.

**Files to create/modify**:
```
internal/executor/executor.go (collectTaskUsage method)
```

**Implementation details**:
- `encodeCCProjectName(absolutePath) string`: replace `/` and `.` with `-`
- Run ccusage with `--project {encoded}` and verify results match
- After task completion: `npx ccusage@latest daily --project {encoded} --json`
- Parse JSON for total_tokens and total_cost
- Graceful degradation: log error but don't block task completion if ccusage fails

**Acceptance criteria**:
- [ ] Path encoding matches ccusage project identification
- [ ] Token and cost data extracted correctly
- [ ] Task updated with usage data after completion
- [ ] ccusage failure doesn't block task completion

**Complexity**: Low-Medium

---

### Task 2B.10: Minimal Graceful Shutdown

**Description**: Handle SIGTERM gracefully: stop new assignments, wait for running Pods, force kill if needed.

**Files to create**:
```
internal/shutdown/shutdown.go
```

**Files to modify**:
```
cmd/flux/main.go (add signal handling)
```

**Implementation details**:
- `GracefulShutdown(ctx, cfg, pods, db, vaultWriter)`:
  1. Stop Manager from assigning new tasks
  2. Signal all running Pods to finish current task (`stopCh`)
  3. Wait up to `pod_grace_period` (10 min) for Pods to finish
  4. If timeout: SIGKILL remaining Pods, move their tasks to RETRY with `crash_recovery=true`
  5. Close DB connection
  6. Drain Vault Writer
- Wire into `main.go`: listen for SIGTERM/SIGINT → call GracefulShutdown

**Acceptance criteria**:
- [ ] SIGTERM triggers graceful shutdown
- [ ] Running Pods finish current task within grace period
- [ ] Pods killed after grace period with tasks set to RETRY
- [ ] crash_recovery flag set correctly
- [ ] DB and Vault Writer properly closed
- [ ] No orphaned processes after shutdown

**Complexity**: Medium-High

---

### Task 2B.11: launchd Plist Registration

**Description**: Create and register launchd plist for automatic startup and crash recovery on macOS.

**Files to create**:
```
deploy/com.circle-oo.flux.plist
deploy/install-launchd.sh
```

**Implementation details**:
- plist: Label, ProgramArguments (flux binary + --config), WorkingDirectory, KeepAlive=true, RunAtLoad=true
- StandardOutPath/ErrorPath → logs/
- EnvironmentVariables: PATH, HOME
- Install script: copy plist to ~/Library/LaunchAgents/, launchctl load

**Acceptance criteria**:
- [ ] Flux starts automatically on login
- [ ] Flux restarts after crash (KeepAlive)
- [ ] Logs written to configured paths
- [ ] Environment variables available

**Complexity**: Low

---

### Task 2B.12: Basic Error Recovery

**Description**: Recover from crashes by transitioning interrupted tasks back to RETRY state.

**Files to create/modify**:
```
internal/shutdown/recovery.go
cmd/flux/main.go (add recovery step to bootstrap)
```

**Implementation details**:
- `RecoverFromCrash(db, notifier)`:
  - Find all tasks with status RUNNING
  - Transition to RETRY with `crash_recovery=true` (doesn't consume retry_count)
  - Discord WARNING: "Recovered from crash. N tasks moved to RETRY."
- Add to bootstrap sequence: after DB open, before starting Pods

**Acceptance criteria**:
- [ ] RUNNING tasks recovered to RETRY on startup
- [ ] crash_recovery=true set (retry_count not consumed)
- [ ] Discord notification sent
- [ ] Recovery runs before Pods start (tasks re-queued first)

**Complexity**: Low

**Depends on**: Task 2B.10

---

## Phase 2B Completion Checklist

- [ ] Rate limit detected → all Pods stop → 5h wait → resume
- [ ] Model selection: Sonnet default, Opus for complex tasks
- [ ] Goal injected into every Claude Code execution
- [ ] Subtask decomposition works (Claude Code → JSON → Manager)
- [ ] Manager: Goal boost + dependency checking
- [ ] Vault Writer records task completions
- [ ] ccusage tracks per-task usage
- [ ] Graceful shutdown: SIGTERM → wait → cleanup
- [ ] launchd: auto-start on boot, restart on crash
- [ ] Crash recovery: RUNNING tasks → RETRY on startup

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~7 files | ~5 files |
| Deploy | ~2 files | — |
| **Total** | **~9 files** | **~5 files** |
