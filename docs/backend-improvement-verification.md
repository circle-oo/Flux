# Flux Backend Improvement Verification

This checklist validates the backend hardening changes across crash safety, data integrity, memory cleanup, security, and operations.

## 1. Automated Verification

Run from project root:

```bash
cd go/src
go test ./...
go build ./...
```

Expected:
- All tests pass.
- Build succeeds without compile errors.

## 2. Crash/Data Safety Scenarios

### 2.1 Executor post-processing failure transitions task to FAILED
- Trigger a `processResults` failure path (for example: force build/test/commit failure in executor flow).
- Verify task status transitions from `RUNNING` to `FAILED` and does not remain orphaned in `RUNNING`.
- Verify error log contains `post-processing failed`.

### 2.2 DELETE task is soft-delete
- Create a task via `POST /api/tasks`.
- Call `DELETE /api/tasks/{id}`.
- Verify task record still exists and status is `ARCHIVED`.

### 2.3 Subtask dependency failure cleanup
- Create a parent task.
- Call `POST /internal/subtasks` with circular dependencies.
- Verify response is `400`.
- Verify no subtasks remain for the parent (`GET /api/tasks/{id}/subtasks` returns empty list).

## 3. Memory/Resource Hygiene

### 3.1 Auth cleanup loop
- Generate failed login attempts from several IPs.
- Wait for cleanup interval or invoke cleanup path in tests.
- Verify stale IP entries are pruned.

### 3.2 Session expiry
- Create session with finite expiry.
- Validate before expiry: accepted.
- Validate after expiry: rejected and removed.

### 3.3 Pod registry stale cleanup
- Register pods, then stop heartbeat for one pod.
- Wait beyond stale threshold.
- Verify stale pod is removed from `/api/pods`.

## 4. Security Validation

Verify `ValidatePrompt` rejects prompts containing:
- `` `command` ``
- `; sh`, `; bash`
- `| zsh`
- `> /dev/`

Also verify null bytes are sanitized before validation.

## 5. Operational Behavior

### 5.1 WebSocket capacity
- Open more than 10 clients (target up to 50).
- Verify clients up to 50 connect and receive events.

### 5.2 Triage failure sentinel
- Force triager execution failure.
- Verify `ReportTriaged` analysis is:
  - `[triage failed - manual review recommended]`

### 5.3 Deploy/restart failure feedback
- Trigger legacy restart failure paths (build fail, git fail, signal fail).
- Verify `DEPLOY_STATUS` events are broadcast over WebSocket with failure/warning details.

### 5.4 Invalid integer query visibility
- Call endpoints with invalid integer query values (for example `?page=abc`).
- Verify behavior remains backward-compatible (`0` fallback) and debug logs are emitted.
