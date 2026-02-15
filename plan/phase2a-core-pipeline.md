# Phase 2A: Core Pipeline — "A task becomes a PR"

## Goal

An Operator-registered task flows through the complete pipeline: Executor picks it up → Claude Code generates code → tests run → PR is created → auto-merge or Operator review. This is the critical phase that proves the end-to-end autonomous coding loop.

## Deliverable

Complete execution pipeline: Task → Claude Code → Test → PR → Merge.

**Phase 2A completion = "Flux builds Flux" transition point.**

## Prerequisites

- Phase 1 complete (Go binary, Web UI CRUD, DB, Discord, GitHub client)
- Claude Code CLI installed and authenticated (`claude login`)
- GitHub token with repo/PR permissions

---

## Task Breakdown

### Task 2A.0: Claude Code CLI Verification

**Description**: Verify all required Claude Code CLI flags are available and document the CLI version for compatibility tracking.

**Files to create**:
```
docs/claude-code-cli-verification.md
```

**Implementation details**:
- Run `claude --help` and document all available flags
- Verify required flags exist:
  - `-p` (prompt)
  - `--max-turns`
  - `--output-format json`
  - `--append-system-prompt`
  - `--model`
  - `--dangerously-skip-permissions`
- Test each flag with a simple command to verify syntax
- Document fallback strategies if any flag is missing:
  - If `--append-system-prompt` unavailable: embed system prompt in user prompt
  - If `--output-format json` unavailable: parse text output
- Record Claude Code version: `claude --version`
- Create compatibility matrix: version → available flags

**Acceptance criteria**:
- [ ] All required flags verified or fallback documented
- [ ] Claude Code version recorded
- [ ] Compatibility matrix created
- [ ] Flag syntax verified with test commands

**Complexity**: Low

---

### Task 2A.1: Claude Code CLI Integration

**Description**: Implement the Claude Code CLI wrapper that executes prompts in non-interactive mode with full stdout/stderr capture.

**Files to create**:
```
internal/executor/claude_code.go
```

**Implementation details**:
- `ClaudeCodeRunner` struct with `ExecutorConfig`
- `ClaudeCodeResult`: ExitCode, Stdout, Stderr, Duration, TokensUsed, CostUSD, SessionID
- `Run(ctx, ClaudeCodeOpts) (*ClaudeCodeResult, error)`
- CLI flags: `-p` (prompt), `--cwd`, `--model`, `--max-turns 30`, `--output-format json`, `--dangerously-skip-permissions` (default, may change after Task 2A.3), `--append-system-prompt`
- Separate stdout/stderr capture via `cmd.Stdout` and `cmd.Stderr` buffers
- Timeout via `context.WithTimeout` (from `max_execution_time` config)
- Output size guardrail: reject if stdout > `max_output_size`
- Exit code extraction from `exec.ExitError`

**WARNING - CRITICAL TIMEOUT BUG**:
- Create `timeoutCtx` BEFORE `exec.CommandContext`, and pass `timeoutCtx` (not `ctx`) to it
- The spec code sample has a bug where `timeoutCtx` is created after the command
- Use process group management: `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` so child processes are killed when timeout fires
- Use `io.LimitReader` or `limitedBuffer` for stdout/stderr to prevent memory exhaustion before the output size check

**Acceptance criteria**:
- [ ] Successfully executes `claude -p "hello" --output-format json`
- [ ] Captures stdout and stderr separately
- [ ] Timeout actually kills the process (verified with a test that sleeps)
- [ ] Output size limit enforced
- [ ] Exit code correctly extracted
- [ ] All CLI flags correctly passed
- [ ] Unit tests pass

**Complexity**: Medium

---

### Task 2A.2: JSON Response Parsing Strategy

**Description**: Test actual Claude Code JSON output format and implement minimum viable parsing.

**Files to modify**:
```
internal/executor/claude_code.go
```

**Implementation details**:
- Run Claude Code with `--output-format json` and capture actual response
- Determine minimum fields: result text, session_id, cost data (if available)
- Implement parser that extracts known fields, ignores unknown
- Log full response for debugging during Phase 2A
- Document actual response format in code comments

**Acceptance criteria**:
- [ ] Actual Claude Code JSON response captured and documented
- [ ] Parser extracts result text reliably
- [ ] Unknown fields do not cause errors
- [ ] Token/cost data extracted if available in response
- [ ] Unit tests pass

**Complexity**: Low-Medium (depends on actual API response)

**Depends on**: Task 2A.1

---

### Task 2A.3: Sandbox Evaluation (Experiment)

**Description**: Test Claude Code's native sandbox mode for compatibility with Flux's worktree-based workflow. This experiment determines whether to keep `--dangerously-skip-permissions` or switch to sandbox.

**Files to create**:
```
docs/experiments/sandbox-evaluation.md (results document)
```

**Implementation details**:
- Test native sandbox in a worktree directory
- Check: Can Claude Code read/write files within worktree?
- Check: Can Claude Code run `git` commands?
- Check: Can Claude Code run `go test`?
- Check: Network access for `go get` dependencies?
- Check: Any filesystem restrictions that break the workflow?
- Decision output: continue with `--dangerously-skip-permissions` OR switch to sandbox
- If sandbox works: update `ClaudeCodeRunner` to remove `--dangerously-skip-permissions`

**Acceptance criteria**:
- [ ] Sandbox tested with representative Flux workflow
- [ ] Results documented with clear recommendation
- [ ] `ClaudeCodeRunner` updated based on decision
- [ ] Post-execution verification scope adjusted (Task 2A.9) based on sandbox capabilities
- [ ] Unit tests pass

**Complexity**: Medium (experimental)

**Depends on**: Task 2A.1

---

### Task 2A.4: Smoke Test

**Description**: Add Claude Code smoke test to the bootstrap sequence to verify CLI availability and authentication.

**Files to modify**:
```
internal/db/schema.go (or new bootstrap.go)
cmd/flux/main.go
```

**Implementation details**:
- `claudeCodeSmokeTest()`: run `claude -p "respond with exactly: SMOKE_TEST_OK" --max-turns 1 --output-format json`
- Verify output contains "SMOKE_TEST_OK"
- On failure: Discord CRITICAL alert + return error (don't start pods)
- Add to bootstrap sequence after seed projects, before "ready" notification

**Acceptance criteria**:
- [ ] Smoke test runs during bootstrap
- [ ] Passes when Claude Code is authenticated
- [ ] Fails with clear error when CLI not available
- [ ] Discord alert sent on failure
- [ ] Flux does not start pods if smoke test fails

**Complexity**: Low

**Depends on**: Task 2A.1

---

### Task 2A.5: Git Worktree Management

**Description**: Implement bare repo cloning and per-task worktree creation/cleanup for parallel task execution.

**Files to create**:
```
internal/executor/worktree.go
```

**Implementation details**:
- `WorktreeManager` struct: reposDir (`workspaces/repos/`), treesDir (`workspaces/trees/`)
- `EnsureBareRepo(project)`: clone bare if not exists, `git fetch --all` if exists
- `CreateWorktree(project, task)`: create worktree with new branch `task/{taskID[:8]}` from main
  - Path: `{treesDir}/{projectName}--task-{taskID[:8]}`
  - Set up `.claude/settings.json` with tool permissions
  - Create `CLAUDE.md` in worktree with project context, coding conventions, and task-specific instructions (significantly improves Claude Code output quality)
- `FindByBranch(project, branchName)`: find existing worktree for CHANGES_REQUESTED fixes
- `CleanupWorktree(project, worktreePath)`: remove worktree via `git worktree remove --force`
- `RunCleanup(tasks)`: implement cleanup policy:
  - COMPLETED + PR MERGED → delete immediately
  - COMPLETED + PR pending review → preserve
  - FAILED → preserve 24 hours
  - CHANGES_REQUESTED → preserve for fix task
- `.claude/settings.json` setup: permission bypass for Bash, Read, Write, Edit, Grep, Glob

**Acceptance criteria**:
- [ ] Bare repo cloned from GitHub URL
- [ ] Worktree created with correct branch naming
- [ ] `.claude/settings.json` created in worktree
- [ ] `CLAUDE.md` created in worktree with project context
- [ ] Multiple worktrees can coexist (parallel tasks)
- [ ] Cleanup policy correctly applied per task status
- [ ] FindByBranch locates existing worktree
- [ ] `git fetch --all` updates bare repo before creating new worktree
- [ ] Unit tests pass

**Complexity**: Medium-High

---

### Task 2A.6: GitHub PR Client

**Description**: Implement full GitHub PR operations: create, merge, fetch comments.

**Files to modify**:
```
internal/github/pr.go
```

**Implementation details**:
- `CreatePR(owner, repo, head, base, title, body) (string, error)` — returns PR URL
  - Add retry with exponential backoff (3 attempts, starting at 1 second) for transient failures (502, 503)
  - Check for existing PR on same branch before retry to avoid duplicates
- `MergePR(owner, repo, prNumber) error` — merge via API
  - Retry transient GitHub API failures with exponential backoff
- `FetchPRComments(owner, repo, prNumber) ([]Comment, error)` — single fetch, no polling
- `Comment` struct: Author, Body, CreatedAt
- Use GitHub REST API v3 with token auth
- Handle common errors: branch not found, merge conflicts, auth failure
- Handle secondary rate limit responses (retry with backoff)

**Acceptance criteria**:
- [ ] PR created successfully on GitHub
- [ ] Transient GitHub failures retried automatically
- [ ] PR merged via API
- [ ] Comments fetched in single request
- [ ] Error handling for common failure cases
- [ ] PR URL correctly returned
- [ ] Unit tests pass

**Complexity**: Medium

**Depends on**: Phase 1 Task 1.7

---

### Task 2A.7: Manager Basic Implementation

**Description**: Implement the task Manager with Priority Queue, task assignment via internal API, and state transition enforcement.

**Files to create**:
```
internal/manager/manager.go
internal/manager/priority.go
internal/executor/manager_client.go
```

**Implementation details**:
- `Manager` struct: db, config
- `PopNextTask(podType) *Task`: pop highest-priority READY task
  - **CRITICAL**: Use database transaction (BEGIN...UPDATE...COMMIT) to prevent two Pods from getting the same task
  - Pattern: `SELECT task WHERE status=READY ORDER BY priority LIMIT 1 FOR UPDATE`, then `UPDATE status=RUNNING` in the same transaction
  - Executor pods: any type except RESEARCH
  - Researcher pods: RESEARCH type only
- `TransitionTask(taskID, newStatus)`: enforce valid transitions per state machine
  - Valid: PENDING→READY, READY→RUNNING, RUNNING→COMPLETED/FAILED, FAILED→RETRY/ARCHIVED, RETRY→RUNNING, COMPLETED→ARCHIVED
  - RETRY validation: retry_count < 3, not cancelled
  - Crash recovery: don't increment retry_count if crash_recovery=true
- `CreateTask(task)`: insert with validation
- `GetCurrentGoal() *Goal`: return single ACTIVE goal
- Priority Queue: ORDER BY priority ASC, created_at ASC (lowest number = highest priority)

**Update internal API**:
- `POST /internal/tasks/next`: use Manager.PopNextTask, transition to RUNNING
- `POST /internal/tasks/:id/done`: use Manager.TransitionTask
- `POST /internal/subtasks`: validate depth/count, create tasks inheriting priority+goal_id
- `GET /internal/model/:task_id`: delegate to Orchestrator (stub: return sonnet)

**ManagerClient HTTP client** (`internal/executor/manager_client.go`):
- Methods:
  - `NextTask(podID, podType) (*Task, error)` — POST /internal/tasks/next
  - `ReportTaskDone(taskID, status, result, errorLog) error` — POST /internal/tasks/:id/done
  - `CreateSubtasks(parentID, subtasks) error` — POST /internal/subtasks
  - `GetModel(taskID) (string, error)` — GET /internal/model/:task_id
  - `GetProject(projectID) (*Project, error)` — GET /internal/projects/:id
- All methods call internal HTTP API
- Used by Executor pods to communicate with Manager

**Acceptance criteria**:
- [ ] Priority Queue returns highest-priority READY task
- [ ] Concurrent PopNextTask calls never return the same task (verified with test)
- [ ] State transitions enforced (invalid transitions rejected)
- [ ] RETRY respects retry_count limit and cancellation check
- [ ] Crash recovery RETRY doesn't consume retry_count
- [ ] Subtask creation validates depth ≤ 1 and max 5 per parent
- [ ] Internal API endpoints fully functional
- [ ] ManagerClient HTTP client implemented with all methods
- [ ] Unit tests pass

**Complexity**: High

**Depends on**: Phase 1 Tasks 1.3, 1.5

---

### Task 2A.8: Executor Pod + Guardrails

**Description**: Implement the Executor Pod main loop: fetch task → execute Claude Code → handle results.

**Files to create**:
```
internal/executor/executor.go
internal/executor/guardrails.go
```

**Implementation details**:

**executor.go**:
- `Executor` struct: id, config, claude, worktree, manager (HTTP client), vault, notifier, stopCh
- `Run(ctx)`: main loop — executeOnce() → sleep 5s → repeat
- `executeOnce(ctx)`: full pipeline per spec:
  1. Request next task via internal API
  2. Get model decision
  3. Build system prompt (Goal injection)
  4. Create/reuse worktree
  5. Build prompt
  6. Execute Claude Code with guardrails
  7. Check rate limit (exit code + stderr patterns)
     - **Note**: Phase 2A detection only - log and fail task. Full response (stop all pods) deferred to Phase 2B.
  8. Post-execution verification
  9. Check subtask decomposition response
     - **Note**: Phase 2A STUB - always returns nil. Full implementation in Phase 2B Task 2B.5.
  10. QA (run tests if required)
  11. Commit + diff check
  12. Create PR
      - Add retry with exponential backoff (3 attempts) for GitHub API transient failures (502, 503)
      - Check for existing PR on same branch before retry to avoid duplicates
  13. Auto-merge decision
      - Handle merge conflict: if merge fails due to conflict, create a rebase task instead of marking FAILED
  14. Report completion

**guardrails.go**:
- Timeout: 30 min (from config)
- Output size: 10 MB
- Max turns: 30
- Diff lines: 2000
- Files changed: 20
- `CheckDiffLimits(worktreePath) (diffLines, filesChanged int, error)`: git diff --stat
- `ExceedsGuardrails(diffLines, filesChanged) bool`

**Acceptance criteria**:
- [ ] Executor fetches tasks via internal API
- [ ] ManagerClient used for all internal API calls
- [ ] Claude Code executes with correct prompt and flags
- [ ] Guardrails enforced (timeout, output, diff limits)
- [ ] Rate limit detected via exit code and stderr patterns
- [ ] Subtask decomposition JSON parsed correctly
- [ ] Tests run for coding tasks
- [ ] PR created on completion
- [ ] GitHub PR creation retries on transient failure
- [ ] Auto-merge decision applied
- [ ] Stop signal honored for graceful shutdown
- [ ] Unit tests pass

**Complexity**: Very High

**Depends on**: Tasks 2A.1-2A.7

---

### Task 2A.9: Post-Execution Verification

**Description**: Verify that Claude Code did not modify files outside the worktree directory.

**Files to modify**:
```
internal/executor/executor.go (add verification step)
```

**Implementation details**:
- Snapshot key directories before execution: `~/.ssh`, `~/.aws`, `~/.gitconfig`, `~/.zshrc`, `~/.bashrc`
- After execution: compare modification times against execution start
- If external modification detected: FAILED + Discord CRITICAL alert
- Scope adjustment based on Task 2A.3 sandbox evaluation results

**Acceptance criteria**:
- [ ] Key directories monitored for changes
- [ ] External modification triggers task failure
- [ ] Discord alert sent on integrity violation
- [ ] Scope adjustable based on sandbox evaluation
- [ ] Unit tests pass

**Complexity**: Low-Medium

**Depends on**: Task 2A.3, Task 2A.8

---

### Task 2A.10: QA (Test Running)

**Description**: Implement test detection, execution, and the "write tests if none exist" logic.

**Files to modify**:
```
internal/executor/executor.go (add QA methods)
```

**Implementation details**:
- `runTests(worktreePath) bool`: detect test framework → run tests → return pass/fail
- Go projects: `go test ./...`
- Node projects: `npm test`
- Python: `pytest`
- If no tests found and task type requires tests: instruct Claude Code to write tests first, then re-run
- Max 3 retry attempts on test failure (task-level retries)
- RESEARCH and DOCUMENT task types exempt from testing

**Acceptance criteria**:
- [ ] Tests detected and run for supported languages
- [ ] Test results correctly determine pass/fail
- [ ] "Write tests" logic triggers when none exist
- [ ] RESEARCH/DOCUMENT tasks skip testing
- [ ] Test output captured for error reporting
- [ ] Unit tests pass

**Complexity**: Medium

**Depends on**: Task 2A.8

---

### Task 2A.11: PR + Auto-Merge + Operator Review

**Description**: Implement PR creation, auto-merge decision logic, and the Operator review flow with "Request Changes" handling.

**Files to modify**:
```
internal/executor/executor.go (PR creation step)
internal/server/api_prs.go (new file)
```

**Implementation details**:

**Auto-merge decision** (`ShouldAutoMerge`):
- Guardrail override: >2000 diff lines OR >20 files → always Operator review
- Auto-merge when: system/self source, maintenance type, low-priority bugfix (P≤10), small PR (≤3 files, <100 additions)
- Otherwise: Operator review required

**PR API endpoints**:
```
GET  /api/prs/pending              — List tasks with pr_status=OPEN
POST /api/prs/:task_id/approve     — Merge PR → cleanup worktree
POST /api/prs/:task_id/request-changes — Fetch comments → create fix task P:6
```

**Request Changes flow**:
1. Operator clicks "Request Changes" in Web UI
2. API fetches GitHub PR comments (single fetch)
3. Creates fix task with P:6 priority, same branch/PR
4. Updates original task pr_status to CHANGES_REQUESTED
5. Fix task Executor reuses existing worktree
6. Pushes to same PR branch

**Acceptance criteria**:
- [ ] PR created on GitHub after successful task
- [ ] Auto-merge applies for qualifying tasks
- [ ] Guardrail-exceeding PRs always go to Operator
- [ ] Operator can approve PR in Web UI → merge + cleanup
- [ ] Request Changes creates fix task with correct priority
- [ ] Fix task reuses existing worktree and PR
- [ ] Discord notification sent for PRs needing review
- [ ] Unit tests pass

**Complexity**: High

**Depends on**: Tasks 2A.6, 2A.8

---

### Task 2A.12: Web UI — PRs Page

**Description**: Add PRs page to Web UI for reviewing and managing pull requests.

**Files to create**:
```
web/src/pages/PRs.tsx
web/src/stores/prStore.ts
```

**Implementation details**:
- List pending PRs with: title, diff stats, auto-merge eligibility, GitHub link
- Approve button → POST /api/prs/:task_id/approve
- Request Changes button → POST /api/prs/:task_id/request-changes
- Status indicators: OPEN, APPROVED, CHANGES_REQUESTED, MERGED
- Link to GitHub PR for full diff view

**Acceptance criteria**:
- [ ] PRs page shows pending reviews
- [ ] Approve and Request Changes buttons work
- [ ] PR status updates in real-time via WebSocket
- [ ] GitHub PR link navigates correctly

**Complexity**: Medium

**Depends on**: Task 2A.11

---

### Task 2A.13: Rate Limit Detection Experiment

**Description**: Intentionally trigger Claude Code rate limits to observe actual exit codes and stderr patterns. Document findings for Phase 2B implementation.

**Files to create**:
```
docs/experiments/rate-limit-detection.md (results document)
```

**Implementation details**:
- Run multiple Claude Code sessions in rapid succession
- Observe: exit code (expect 429?), stderr output patterns
- Test patterns from spec: "rate limit", "too many requests", "429", "capacity", "try again"
- Document which patterns actually fire
- Note: this is a deliberate experiment, not production behavior
- Record timing: how long before rate limit triggers, how long until recovery

**Acceptance criteria**:
- [ ] At least one rate limit triggered
- [ ] Exit code documented
- [ ] Stderr patterns documented
- [ ] Findings written to experiment doc
- [ ] Recommendations for Phase 2B detection implementation

**Complexity**: Medium (experimental, timing-dependent)

**Depends on**: Task 2A.1

---

### Task 2A.14: Agent Teams Compatibility Experiment

**Description**: Test Claude Code's Agent Teams feature for potential use within Executor pods.

**Files to create**:
```
docs/experiments/agent-teams-evaluation.md (results document)
```

**Implementation details**:
- Test `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` environment variable in `-p` mode
- Test `.claude/agents/` definition files in worktree
- Check: Does `-p` mode support agent definitions?
- Check: Can sub-agents run in parallel within single execution?
- Check: OMC agent definition patterns compatibility
- Document: benefits, limitations, recommended usage (if any)

**Note**: OMC itself should NOT be used as a Flux plugin (dual orchestration conflict). Only evaluate ideas from OMC's agent definition patterns.

**Acceptance criteria**:
- [ ] Agent Teams tested in non-interactive mode
- [ ] `.claude/agents/` definitions tested
- [ ] Results documented with clear recommendation
- [ ] No OMC installation in Flux (ideas only)

**Complexity**: Medium (experimental)

**Depends on**: Task 2A.1

---

## Phase 2A Completion Checklist

- [ ] Task registered by Operator → Executor picks up → Claude Code runs
- [ ] Tests pass (or written if missing)
- [ ] PR created on GitHub
- [ ] Auto-merge works for qualifying PRs
- [ ] Operator review flow works (approve + request changes)
- [ ] Worktree management: create, reuse, cleanup
- [ ] Guardrails enforced (timeout, output, diff limits)
- [ ] Post-execution verification catches external modifications
- [ ] Rate limit detection experiment completed
- [ ] Sandbox evaluation completed
- [ ] Agent Teams experiment completed
- [ ] PRs page in Web UI functional

## File Count Summary

| Category | New Files | Modified Files |
|----------|-----------|----------------|
| Go backend | ~8 files | ~4 files |
| React frontend | ~2 files | ~1 file |
| Docs/Experiments | ~4 files | — |
| **Total** | **~14 files** | **~5 files** |

**Notes**:
- New Task 2A.0 adds 1 doc file
- Task 2A.7 adds 1 new file (manager_client.go)
- Task 2A.5 now creates CLAUDE.md in worktrees (implementation detail, not a new file in repo)
