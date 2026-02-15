# Phase 1: Foundation — "Skeleton"

## Goal

Flux boots as a single binary. Web UI allows CRUD management of Goals, Tasks, and Projects. No autonomous execution yet — the Operator does everything manually.

## Deliverable

`go build -o flux ./cmd/flux` → single binary with embedded React Web UI → full CRUD via browser.

## Prerequisites

- Go 1.22+
- Node.js 20+ (for Web UI build)
- Git

---

## Task Breakdown

### Task 1.1: Go Project Init + Config Loader

**Description**: Initialize the Go module, set up project structure, implement YAML config loading with environment variable resolution and `~` path expansion.

**Files to create**:
```
cmd/flux/main.go
internal/config/config.go
config.yaml
go.mod
go.sum
Makefile
.gitignore
```

**Implementation details**:
- `go mod init github.com/circle-oo/flux`
- Dependencies: `modernc.org/sqlite`, `gopkg.in/yaml.v3`, `github.com/google/uuid`, `nhooyr.io/websocket`, `golang.org/x/crypto/bcrypt`
- Config struct must match the full `config.yaml` template from spec (all sections)
- `Load(path)` function: read YAML → unmarshal → resolve `_env` fields from environment → expand `~` in vault path
- `main.go`: parse `--config` flag, load config, log startup
- Add `log/slog` structured logging setup: configure file handler from `LoggingConfig`, create `logs/` directory
- Add `Validate()` method on Config struct: port range, positive durations, required fields non-empty

**Acceptance criteria**:
- [ ] `go build ./cmd/flux` compiles without errors
- [ ] Config loads from `config.yaml` with all fields populated
- [ ] Environment variables resolved for `password_env`, `token_env`, `webhook_url_env`
- [ ] `~` expanded in `vault.path`
- [ ] Missing config file returns clear error
- [ ] Log output written to configured file with configured level
- [ ] Invalid config returns descriptive error

**Complexity**: Low

---

### Task 1.2: SQLite Schema + Bootstrap

**Description**: Create SQLite database connection with WAL mode, define complete schema (all 7 tables), implement bootstrap sequence that creates DB + Vault directories + seed projects.

**Files to create**:
```
internal/db/db.go
internal/db/schema.go
internal/db/queries.go
```

**Implementation details**:
- `db.Open(path)`: open SQLite, set WAL mode, busy_timeout=5000, synchronous=NORMAL, foreign_keys=ON
- `schema.go`: CREATE TABLE IF NOT EXISTS for: `goals`, `projects`, `tasks`, `alerts`, `usage_snapshots`, `rate_limit_events`, `service_metrics`
- All indexes: `idx_tasks_status_priority`, `idx_tasks_project`, `idx_tasks_pr_status`, `idx_tasks_parent`, `idx_usage_snapshots_type_time`, `idx_metrics_service_time`
- Convention: all optional TEXT fields default to `''`, JSON arrays default to `'[]'`
- `queries.go`: shared helpers — `InsertRow`, `UpdateRow`, `GetByID`, `ListByStatus`
- Bootstrap function: create DB if not exists → create schema → create Vault directory structure → register seed projects from config
- Create `internal/testutil/` package with: test DB helper (in-memory SQLite), fixture helpers

**Vault directory structure to create**:
```
~/ObsidianVault/Flux/
├── Goals/ (_current.md, completed/, proposals/)
├── Projects/
├── Research/ (Industry/, Tools/, Ideas/, _history.md)
├── Tasks/completed/
├── Services/alerts/
└── Templates/ (project.md, research.md, task-summary.md, decision.md)
```

**Acceptance criteria**:
- [ ] DB created in WAL mode at configured path
- [ ] All 7 tables created with correct schema
- [ ] All indexes created
- [ ] `data/` directory auto-created
- [ ] Vault directories created at configured path
- [ ] Seed projects inserted (flux project)
- [ ] Re-running bootstrap is idempotent (IF NOT EXISTS)
- [ ] Test helper creates in-memory DB with schema
- [ ] Unit tests written for exported functions

**Complexity**: Medium

**Depends on**: Task 1.1

---

### Task 1.3: Core Models + CRUD

**Description**: Define Go structs for Goal, Task, Project, Alert, UsageSnapshot. Implement full CRUD operations for each.

**Files to create**:
```
internal/models/goal.go
internal/models/project.go
internal/models/task.go
internal/models/alert.go
internal/models/usage.go
```

**Implementation details**:
- **Goal**: ID, Title, Description, Priorities (JSON), Metrics (JSON), Status, Source, CreatedAt, ActiveSince
  - Status constants: PROPOSED, ACTIVE, COMPLETED, SUPERSEDED
  - Source constants: OPERATOR, ORCHESTRATOR
  - Constraint: only one ACTIVE goal at a time
- **Task**: Full struct per spec (30+ fields including crash_recovery, depends_on, tags as JSON)
  - Type constants: CODING, RESEARCH, DOCUMENT, MAINTENANCE, DEPLOY, BUGFIX, PLANNING
  - Status constants: PENDING, READY, RUNNING, COMPLETED, FAILED, RETRY, ARCHIVED
  - Source constants: OPERATOR, RESEARCHER, SELF, SYSTEM
  - Priority ranges: P1-5 incidents, P6-20 operator, P21-40 maintenance, P41-60 improvements, P61-80 research, P81-100 new projects
  - Methods: `NeedsOpus()`, `RequiresTest()`, `hasComplexKeywords()`
  - `hasComplexKeywords()` definition:
```go
func (t *Task) hasComplexKeywords() bool {
    keywords := []string{"architect", "refactor", "redesign", "migration", "security", "overhaul"}
    lower := strings.ToLower(t.Title + " " + t.Description)
    for _, kw := range keywords {
        if strings.Contains(lower, kw) { return true }
    }
    return false
}
```
- **Project**: ID, Name, Type, RepoURL, Description, VaultPath, Status, TechStack (JSON), Inspiration, GoalID
  - Status constants: PROPOSED, ACTIVE, ARCHIVED, REJECTED
- JSON fields (Priorities, Metrics, DependsOn, Tags, TechStack): marshal/unmarshal between `[]string` and TEXT column
- All CRUD: Create, GetByID, List (with filters), Update, Delete
- UUID v4 generation for all IDs

**Acceptance criteria**:
- [ ] All model structs match spec exactly
- [ ] JSON array fields serialize/deserialize correctly
- [ ] CRUD operations work with SQLite
- [ ] Goal activation enforces single-ACTIVE constraint
- [ ] Task `NeedsOpus()` and `RequiresTest()` match spec logic
- [ ] Empty string defaults (not NULL) for optional fields
- [ ] Unit tests for NeedsOpus() and hasComplexKeywords()
- [ ] Unit tests written for exported functions

**Complexity**: Medium

**Depends on**: Task 1.2

---

### Task 1.4: External API (Goals, Tasks, Projects)

**Description**: Implement REST API handlers for Goals, Tasks, and Projects CRUD operations.

**Files to create**:
```
internal/server/server.go
internal/server/auth.go
internal/server/api_goals.go
internal/server/api_tasks.go
internal/server/api_projects.go
```

**Implementation details**:

**server.go**:
- HTTP server setup with Go 1.22+ enhanced `http.ServeMux` supporting `{id}` path patterns
- **Note**: Go 1.22+ ServeMux supports `GET /api/goals/{id}` pattern syntax. Do NOT use `:id` (chi syntax) or pull in external router.
- Route registration: `/api/...` (requires auth), `/internal/...` (localhost only), `/ws/...`, `/` (static)
- Auth middleware: cookie-based session validation
- Localhost-only middleware for `/internal/` routes
- Content-Type header set to `application/json` on all API responses

**auth.go**:
- `POST /api/auth/login`: password → bcrypt compare → UUID session token → cookie (no expiry, HttpOnly, SameSite=Strict)
- `POST /api/auth/logout`: invalidate session
- Thread-safe session store: `map[string]bool` protected by `sync.RWMutex`
- Login rate limiting: 5 failed attempts per IP per hour
- Config validation: reject startup if password env var is empty when auth enabled

**api_goals.go**:
```
POST   /api/goals              — Create/propose goal
GET    /api/goals              — List goals
GET    /api/goals/current      — Current active goal
PATCH  /api/goals/:id          — Update goal
POST   /api/goals/:id/activate — Activate (sets ACTIVE, supersedes previous)
```

**api_tasks.go**:
```
POST   /api/tasks              — Create task (OPERATOR source → READY)
GET    /api/tasks              — List (?status=&project_id=&page=&limit=)
GET    /api/tasks/:id          — Detail
PATCH  /api/tasks/:id          — Update
DELETE /api/tasks/:id          — Delete
POST   /api/tasks/:id/cancel   — Cancel → FAILED + "cancelled by operator"
```

**api_projects.go**:
```
POST   /api/projects           — Register project
GET    /api/projects           — List
GET    /api/projects/:id       — Detail
POST   /api/projects/:id/approve — Approve (PROPOSED → ACTIVE)
POST   /api/projects/:id/reject  — Reject (PROPOSED → REJECTED)
```

**Acceptance criteria**:
- [ ] All endpoints respond with correct JSON
- [ ] Auth middleware blocks unauthenticated requests
- [ ] Login/logout flow works with cookie
- [ ] Goal activation supersedes previous ACTIVE goal
- [ ] Task cancellation sets FAILED + error_log
- [ ] Operator-created tasks go directly to READY status
- [ ] List endpoints support pagination and filtering
- [ ] Session store is goroutine-safe (RWMutex)
- [ ] Login rate limiting blocks after 5 failures
- [ ] Unit tests written for exported functions

**Complexity**: Medium-High

**Depends on**: Task 1.3

---

### Task 1.5: Internal API Framework

**Description**: Implement internal API endpoints for Pod ↔ Manager communication. Stub implementations for Phase 1 (no actual Pods yet).

**Files to create**:
```
internal/server/internal_api.go
```

**Implementation details**:
```
POST /internal/tasks/next       — Returns next highest-priority READY task (stub: always returns null)
POST /internal/tasks/:id/done   — Reports task completion (stub: updates status)
POST /internal/subtasks         — Creates subtasks (stub: validates depth/count, creates tasks)
GET  /internal/model/:task_id   — Returns model decision (stub: always returns "sonnet")
GET  /health                    — Returns {"status":"ok","version":"..."} (no auth required)
```

- All internal endpoints: localhost-only middleware enforced
- Request/response JSON structures per spec
- Validation: subtask depth ≤ 1, max 5 per parent

**Acceptance criteria**:
- [ ] All 4 internal endpoints respond correctly
- [ ] Localhost-only middleware rejects non-localhost requests
- [ ] Subtask validation enforces depth and count limits
- [ ] Endpoints are functional stubs (real logic in Phase 2A)
- [ ] Health endpoint returns 200 OK

**Complexity**: Low

**Depends on**: Task 1.4

---

### Task 1.6: Discord Notifier

**Description**: Implement Discord webhook-based notification system.

**Files to create**:
```
internal/notifier/discord.go
```

**Implementation details**:
- `Discord` struct with `webhookURL` and `http.Client`
- `Send(level NotificationLevel, message string) error`
- Levels: INFO, WARNING, CRITICAL
- Format: `[LEVEL] message`
- Graceful degradation: log error but don't crash if webhook fails
- Used throughout all phases for: service failures, proposals, task failures, auth expiry, daily summary

**Acceptance criteria**:
- [ ] Messages sent to Discord webhook
- [ ] Supports INFO, WARNING, CRITICAL levels
- [ ] Graceful failure (logs error, doesn't crash)
- [ ] Empty webhook URL logs warning and returns nil (no crash)

**Complexity**: Low

**Depends on**: Task 1.1

---

### Task 1.7: GitHub Client (Repo Creation Only)

**Description**: Implement GitHub API client with repository creation. PR methods stubbed for Phase 2A.

**Files to create**:
```
internal/github/client.go
internal/github/repo.go
internal/github/pr.go
```

**Implementation details**:
- `Client` struct: token, username, http.Client (30s timeout)
- `NewClient(token, username) *Client`
- `CreateRepo(name string, private bool) (string, error)` — GitHub REST API v3
- `pr.go`: stub methods for `CreatePR()`, `MergePR()`, `FetchPRComments()` — return "not implemented" errors

**Acceptance criteria**:
- [ ] `CreateRepo` creates a real GitHub repository
- [ ] Returns repo URL on success
- [ ] Handles API errors (auth failure, name conflict)
- [ ] PR methods exist as stubs (Phase 2A implementation)

**Complexity**: Low

**Depends on**: Task 1.1

---

### Task 1.8: WebSocket Stub

**Description**: Implement WebSocket endpoint for real-time event streaming (stub for Phase 1).

**Files to create**:
```
internal/server/websocket.go
```

**Implementation details**:
- `GET /ws/events` — WebSocket upgrade
- Maintain set of connected clients
- `Broadcast(event Event)` method for sending events to all clients
- Event types: TASK_UPDATED, GOAL_CHANGED, PR_STATUS, POD_STATUS (enum)
- Phase 1: connection management only, events sent by API handlers on state changes
- Ping/pong keepalive every 30 seconds
- Connection limit (max 10 concurrent clients)

**Acceptance criteria**:
- [ ] WebSocket connection established
- [ ] Multiple clients can connect
- [ ] Broadcast sends to all connected clients
- [ ] Graceful disconnect handling

**Complexity**: Low

**Depends on**: Task 1.4

---

### Task 1.9a: React Scaffolding

**Description**: Set up Vite + React + TypeScript + Tailwind CSS with Go embed integration.

**Files to create**:
```
web/package.json
web/vite.config.ts
web/tailwind.config.js
web/tsconfig.json
web/index.html
web/src/main.tsx
web/src/App.tsx
```

**Implementation details**:
- React 18 + TypeScript + Tailwind CSS + Vite
- Go embedding setup:
```go
//go:embed web/dist/*
var webFS embed.FS
```
- Vite build outputs to `web/dist/`

**Acceptance criteria**:
- [ ] `npm run build` in `web/` produces `dist/`
- [ ] `go build` embeds `web/dist/` into binary
- [ ] Basic React app renders in browser

**Complexity**: Low

**Depends on**: Task 1.4

---

### Task 1.9b: API Client + Auth Store + Login Page

**Description**: Implement API client library, Zustand auth store, and login page.

**Files to create**:
```
web/src/lib/api.ts
web/src/stores/authStore.ts
web/src/pages/Login.tsx
```

**Implementation details**:
- API client: fetch wrapper with cookie credentials
- Auth store: login/logout state management
- Login page: Password input → POST /api/auth/login → redirect to Dashboard

**Acceptance criteria**:
- [ ] Login flow works (password → cookie → redirect)
- [ ] Auth state persists across page reloads
- [ ] Unit tests written for exported functions

**Complexity**: Medium

**Depends on**: Task 1.9a

---

### Task 1.9c: Layout/Sidebar + Dashboard Page

**Description**: Build shared layout component with sidebar navigation and Dashboard page.

**Files to create**:
```
web/src/components/Layout.tsx
web/src/components/Sidebar.tsx
web/src/pages/Dashboard.tsx
```

**Implementation details**:
- Layout: responsive with Tailwind, sidebar navigation
- Dashboard: Active Goal summary, recent tasks (by status), active projects count, system status placeholder

**Acceptance criteria**:
- [ ] Responsive layout with Tailwind
- [ ] Dashboard shows active goal and recent tasks
- [ ] Unit tests written for exported functions

**Complexity**: Medium

**Depends on**: Task 1.9b

---

### Task 1.9d: Goals + Tasks + Projects Pages + Stores

**Description**: Implement CRUD pages for Goals, Tasks, and Projects with Zustand stores.

**Files to create**:
```
web/src/pages/Goals.tsx
web/src/pages/Tasks.tsx
web/src/pages/Projects.tsx
web/src/stores/goalStore.ts
web/src/stores/taskStore.ts
web/src/stores/projectStore.ts
```

**Implementation details**:
- **Goals**: List all goals, create new, activate, view current goal details
- **Tasks**: List with filters (status, project), create new task, view detail, cancel task
- **Projects**: List, register new, approve/reject proposed projects
- Zustand stores for state management

**Acceptance criteria**:
- [ ] Goals CRUD works through UI
- [ ] Tasks CRUD works with filtering
- [ ] Projects CRUD works with approve/reject
- [ ] Unit tests written for exported functions

**Complexity**: High

**Depends on**: Task 1.9c

---

### Task 1.9e: WebSocket Store Integration

**Description**: Implement WebSocket connection with Zustand store for real-time updates.

**Files to create**:
```
web/src/stores/wsStore.ts
```

**Implementation details**:
- Connect on login, reconnect on disconnect
- Update Zustand stores on events (TASK_UPDATED, GOAL_CHANGED, etc.)

**Acceptance criteria**:
- [ ] WebSocket connection established
- [ ] Stores update on events
- [ ] Reconnection works after disconnect

**Complexity**: Low

**Depends on**: Task 1.9d, Task 1.8

---

### Task 1.10: Bootstrap Integration + Startup

**Description**: Wire everything together in `main.go`. Implement the full bootstrap sequence.

**Files to modify**:
```
cmd/flux/main.go
```

**Implementation details**:

Bootstrap sequence:
1. Load config (`--config` flag)
2. Open/create SQLite DB (WAL mode)
3. Create schema (all tables)
4. Create Vault directory structure
5. Initialize Notifier
6. Register seed projects from config
7. Start HTTP server (API + Web UI)
8. Send Discord: "Flux initialized. Please set a Goal."
9. Block on SIGTERM/SIGINT

**Acceptance criteria**:
- [ ] `./flux --config config.yaml` starts successfully
- [ ] DB created if not exists
- [ ] Vault directories created
- [ ] Web UI accessible at configured port
- [ ] Discord notification sent on startup
- [ ] SIGTERM triggers graceful HTTP server shutdown
- [ ] Re-running on existing DB is idempotent
- [ ] Unit tests written for exported functions

**Complexity**: Medium

**Depends on**: All previous tasks

---

### Task 1.11: Services & Alerts API Stubs

**Description**: Implement stub API endpoints for Services and Alerts (used by Web UI, real functionality in Phase 3).

**Files to create**:
```
internal/server/api_services.go
```

**Implementation details**:
- `GET /api/services` — Returns empty list `{"services":[]}`
- `GET /api/alerts` — Returns empty list `{"alerts":[]}`
- Phase 1: stubs only, real monitoring starts in Phase 3

**Acceptance criteria**:
- [ ] Both endpoints respond with correct JSON structure
- [ ] Endpoints require authentication
- [ ] Return empty arrays

**Complexity**: Low

**Depends on**: Task 1.4

---

## Phase 1 Completion Checklist

- [ ] `go build -o flux ./cmd/flux` produces single binary
- [ ] Binary starts with `--config config.yaml`
- [ ] Web UI loads in browser at `http://localhost:8080`
- [ ] Login with password works
- [ ] Goals: create, list, activate, view current
- [ ] Tasks: create, list, filter, view, cancel
- [ ] Projects: register, list, approve, reject
- [ ] SQLite DB persists across restarts
- [ ] Discord notifications fire on startup
- [ ] Internal API endpoints respond (stubs)
- [ ] WebSocket connection works

## File Count Summary

| Category | Files |
|----------|-------|
| Go backend | ~18 files |
| React frontend | ~18 files |
| Config/Build | ~5 files |
| **Total** | **~41 files** |
