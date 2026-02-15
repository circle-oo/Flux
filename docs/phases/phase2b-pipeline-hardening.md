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

### Task 2B.2: Rate Limit Non-Blocking Response

**Description**: When rate limit is detected, set state to prevent new task requests. Instead of blocking with time.Sleep, use a state field: `rateLimitUntil time.Time`. The Executor checks this before requesting tasks. The Orchestrator (when it exists in Phase 3) checks this in each tick. This allows the system to remain responsive during rate limit events.

**Files to modify**:
```
internal/orchestrator/rate_limit_handler.go
```

**Implementation details**:
- `HandleRateLimit()`:
  1. Set `rateLimitUntil = time.Now().Add(5*time.Hour)`
  2. Set `isLimited = true`
  3. Send Discord WARNING with expected resume time
  4. Record event in DB
  5. (Pods check `IsLimited()` before requesting tasks - no explicit stop needed)
- `CheckAndRecover()`: called periodically, checks if `time.Now().After(rateLimitUntil)`, clears limited state
- `IsLimited() bool`: returns true if `isLimited == true && time.Now().Before(rateLimitUntil)`
- `RecentlyLimited() bool`: check if rate limit occurred in last 6 hours (for model selection)
- No `stopAllPods()` or `resumePods()` - Pods self-regulate by checking `IsLimited()`

**Acceptance criteria**:
- [ ] Rate limit state prevents Pods from requesting tasks
- [ ] Discord notification includes expected resume time
- [ ] Event recorded in DB
- [ ] 5-hour wait observed without blocking
- [ ] Pods resume automatically after wait expires
- [ ] `RecentlyLimited()` returns correct state
- [ ] System remains responsive during rate limit period
- [ ] Unit tests cover state transitions

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
  - Operator source + complex keywords (see `hasComplexKeywords()`)
  - Tag "initial-design"
  - Tag "goal-strategy"
- `hasComplexKeywords()` checks if task Title or Description contains: 'architect', 'refactor', 'redesign', 'migration', 'security', 'overhaul' (case-insensitive). Already defined in Phase 1 Task 1.3.
- Update `GET /internal/model/:task_id` to use this logic

**Acceptance criteria**:
- [ ] Default model is Sonnet
- [ ] Opus selected only for qualifying tasks
- [ ] Rate limit suppresses Opus selection
- [ ] Internal API returns correct model per task
- [ ] Unit tests cover all NeedsOpus conditions

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

**Description**: Enable Claude Code to autonomously decide when a task is too large and output a decomposition plan instead of code. This implements the subtask decomposition that was stubbed in Phase 2A Task 2A.8 step 9.

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
- [ ] Unit tests for decomposition parsing and validation

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
- **Goal boost**: Goal boost is a TIEBREAKER, not a reordering. Within the same priority value, Goal-related tasks are popped first. A P:50 Goal-related task does NOT jump ahead of a P:48 unrelated task. Only among multiple P:50 tasks does the Goal relation break the tie.
  - When popping tasks at priority P, Goal-related tasks (matching current goal_id) are selected before unrelated tasks
  - Boost only within same priority value, never cross-priority
- **Dependency check**: Before assigning a task, verify all `depends_on` tasks are COMPLETED
  - If dependencies not met: skip task, try next in queue
  - Task stays in READY state until dependencies resolve

**Acceptance criteria**:
- [ ] Goal-related tasks prioritized as tiebreaker within same priority
- [ ] Goal relation never overrides priority ordering
- [ ] Tasks with unmet dependencies not assigned
- [ ] Dependencies resolved when blocking tasks complete
- [ ] No deadlocks with circular dependencies (reject at creation time)
- [ ] Unit tests cover tiebreaker logic and dependency resolution

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
- `Writer` struct: basePath, requests channel (buffered 100), wg sync.WaitGroup
- `NewWriter(basePath)`: start goroutine processing requests
- `run()`: range over channel, process each WriteRequest sequentially
- `Write(path, content, mode) error`: send request with timeout to prevent blocking forever:
  ```go
  select {
  case w.requests <- req:  // sent
  case <-time.After(5*time.Second): return fmt.Errorf("vault writer full")
  }
  ```
- `Close()`: use `sync.WaitGroup` to drain safely:
  1. close(w.requests)
  2. wg.Wait() to ensure all requests processed
- Write modes: Create (error if exists), Append, Replace
- Atomic operations: ensure directory exists, write file
- `templates.go`: markdown templates for task summaries, project docs, decision records

**Acceptance criteria**:
- [ ] Sequential writes (no file conflicts)
- [ ] All write modes work (create, append, replace)
- [ ] Directory auto-creation
- [ ] Write timeout prevents blocking on full channel
- [ ] Close drains remaining requests safely using WaitGroup
- [ ] Templates produce valid Obsidian markdown
- [ ] Unit tests cover timeout and drain logic

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

### Task 2B.9: ccusage Project Name Mapping Verification (Experiment)

**Description**: This is a VERIFICATION task to confirm path encoding works. Production integration into the Executor pipeline is Task 3.6. Verify that Flux's worktree path encoding matches ccusage's `--project` identifier.

**Files to create/modify**:
```
internal/executor/executor.go (collectTaskUsage method - experimental)
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
- [ ] Encoding verified with manual test cases
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

### Task 2B.11: Basic Error Recovery

**Description**: Recover from crashes by transitioning interrupted tasks back to RETRY state. CRITICAL: This MUST be implemented before launchd registration. If launchd restarts Flux after a crash and recovery isn't implemented yet, RUNNING tasks get permanently stuck.

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
- [ ] Unit tests cover recovery logic

**Complexity**: Low

**Depends on**: Task 2B.10

---

### Task 2B.12: launchd Plist Registration

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

**Depends on**: Task 2B.11

---

### Task 2B.13: Security Hardening

**Description**: Add basic security controls: internal API authentication, session persistence, audit logging, and prompt validation.

**Files to create/modify**:
```
internal/server/auth.go (update with shared secret validation)
internal/db/schema.go (add audit_log table)
internal/server/session.go (add SQLite persistence)
internal/server/validation.go (new - prompt validation)
```

**Implementation details**:
- **Internal API shared secret**:
  - Generate random 32-byte key at boot, store in memory
  - Pods include in X-Flux-Internal-Secret header
  - API middleware validates header before processing requests
- **Session persistence**:
  - Store sessions in SQLite `sessions` table (session_id, user_id, created_at, expires_at)
  - Sessions survive restart
  - Clean up expired sessions on boot
- **Audit log**:
  - New table: `audit_log` (id, event_type, actor, target, details, ip_address, timestamp)
  - Log: auth attempts, task creation, status changes, API calls
- **Prompt validation**:
  - Length limit: 10KB max for task title/description
  - Forbidden patterns: SQL injection attempts, shell metacharacters in unexpected places
  - Validate before task creation

**Acceptance criteria**:
- [ ] Internal API rejects requests without valid secret
- [ ] Sessions survive Flux restart
- [ ] Audit log records auth events
- [ ] Prompt validation blocks oversized or malicious inputs
- [ ] Unit tests cover secret validation and prompt validation
- [ ] Integration test: restart Flux, verify session persists

**Complexity**: Medium

**Depends on**: Task 2B.10

---

## Phase 2B Completion Checklist

- [ ] Rate limit detected → state-based response (non-blocking)
- [ ] Model selection: Sonnet default, Opus for complex tasks
- [ ] Goal injected into every Claude Code execution
- [ ] Subtask decomposition works (Claude Code → JSON → Manager)
- [ ] Manager: Goal boost (tiebreaker) + dependency checking
- [ ] Vault Writer records task completions (safe drain with WaitGroup)
- [ ] ccusage verification experiment complete
- [ ] Graceful shutdown: SIGTERM → wait → cleanup
- [ ] Crash recovery: RUNNING tasks → RETRY on startup (BEFORE launchd)
- [ ] launchd: auto-start on boot, restart on crash
- [ ] Security: internal API auth, session persistence, audit log, prompt validation

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~9 files | ~6 files |
| Deploy | ~2 files | — |
| **Total** | **~11 files** | **~6 files** |
