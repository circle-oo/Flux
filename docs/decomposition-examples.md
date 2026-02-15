# Task Decomposition Examples

This document provides concrete examples of well-decomposed tasks versus poorly-decomposed tasks to illustrate the decomposition criteria in practice.

## Example 1: Real-Time Notifications System

### ✅ Good Decomposition

**Parent Task:**
```
Title: Implement real-time notifications system
Type: CODING
Priority: 15
Description:
Build a real-time notification system for the application. Users should receive
instant notifications for important events (new messages, mentions, system alerts).
The system needs WebSocket infrastructure, backend delivery, and frontend display.
```

**Complexity Analysis:**
- Multi-layer architecture: +3 (frontend + backend + WebSocket infra)
- External integration: 0 (internal system)
- Testing requirements: +1 (integration tests needed)
- Security: +2 (WebSocket auth, subscription management)
- **Total Score: 6** → Moderate complexity with strong signals

**Signals Detected:**
- Multiple Independent Components (strength: 3)
- Cross-Cutting Concerns (strength: 2)
- Multiple acceptance criteria (strength: 1)

**Decision:** DECOMPOSE

**Subtasks:**

1. **Title:** "Set up WebSocket server infrastructure and connection handling"
   **Description:** Implement WebSocket server using gorilla/websocket. Add connection lifecycle management (connect, disconnect, heartbeat). Include authentication middleware for WebSocket connections. Set up connection pool and message routing.

2. **Title:** "Implement backend notification queue and delivery system"
   **Description:** Create notification service that accepts events from application code. Implement queue/buffer for notification delivery. Add logic to route notifications to connected users. Include persistence for undelivered notifications.

3. **Title:** "Create frontend notification UI component and state management"
   **Description:** Build React notification component with toast/banner display. Implement WebSocket client connection with reconnection logic. Add notification state management (Redux/Context). Include notification history panel.

4. **Title:** "Add notification preferences and subscription management"
   **Description:** Implement user notification preferences (email, in-app, push). Add subscription management API for users to control which notifications they receive. Include unsubscribe/mute functionality.

**Why this is good:**
- Each subtask is focused on one area (infra, backend, frontend, config)
- Clear separation of concerns
- Each subtask is independently testable
- Reasonable scope per subtask (~150-250 lines)
- Subtasks can be executed sequentially or in parallel (with some dependencies)
- No meta-tasks (tests are implicit in each implementation)

---

## Example 2: User Authentication (Poorly Decomposed)

### ❌ Bad Decomposition

**Parent Task:**
```
Title: Add user authentication
Type: CODING
Priority: 12
Description:
The application needs user authentication. Users should be able to log in and sign up.
```

**Bad Subtasks:**

1. **Title:** "Set up authentication" ← TOO VAGUE
   **Description:** Set up the authentication system

2. **Title:** "Fix bugs in authentication" ← NO BUGS EXIST YET
   **Description:** Debug and fix authentication issues

3. **Title:** "Write tests" ← META-TASK
   **Description:** Write unit and integration tests for authentication

4. **Title:** "Other work" ← COMPLETELY VAGUE
   **Description:** Handle additional authentication requirements

5. **Title:** "Documentation" ← OUT OF SCOPE
   **Description:** Document the authentication flow

6. **Title:** "Deploy to production" ← SYSTEM HANDLES DEPLOYMENT
   **Description:** Deploy authentication changes

**Why this is bad:**
- Subtask 1: Too vague, doesn't specify what to implement
- Subtask 2: Assumes bugs exist before implementation
- Subtask 3: Tests should be part of each implementation subtask
- Subtask 4: Completely non-actionable
- Subtask 5: Documentation is out of scope for implementation tasks
- Subtask 6: Deployment is handled by the system, not a subtask
- No clear implementation path
- Overlapping and unclear responsibilities

### ✅ Good Decomposition (Same Task)

**Improved Subtasks:**

1. **Title:** "Implement JWT token generation and validation in backend API"
   **Description:** Create authentication service that generates JWT tokens on login. Implement token validation middleware for protected routes. Add refresh token mechanism. Include bcrypt password hashing.

2. **Title:** "Create login/signup forms and auth state management in frontend"
   **Description:** Build React login and signup forms with validation. Implement auth state management (Redux/Context). Add JWT storage in httpOnly cookies. Include protected route wrapper component.

3. **Title:** "Add protected route middleware and session management"
   **Description:** Implement middleware to check JWT tokens on protected endpoints. Add session management with expiration. Include logout functionality and token revocation.

**Why this is better:**
- Specific, actionable titles
- Focused on implementation, not meta-work
- Clear scope and acceptance criteria
- Complete coverage without duplication

---

## Example 3: Simple Bug Fix (Should NOT Decompose)

### ❌ Incorrectly Decomposed

**Parent Task:**
```
Title: Fix typo in README file
Type: BUGFIX
Priority: 80
Description: There's a spelling mistake in README.md line 42
```

**Bad Subtasks (should not exist):**

1. **Title:** "Find the typo"
   **Description:** Locate the typo in README.md

2. **Title:** "Fix the typo"
   **Description:** Correct the spelling mistake

3. **Title:** "Commit the fix"
   **Description:** Commit and push the change

**Why this is bad:**
- Creates 3 tasks for a 1-line change
- Massive overhead for trivial work
- No benefit from decomposition
- Each "subtask" takes longer to describe than to do

### ✅ Correct Approach

**Do NOT decompose.** Execute as a single atomic task.

**Complexity Analysis:**
- Score: 0 (no complexity factors)
- Signals: Quick win detected
- Decision: SKIP DECOMPOSITION

---

## Example 4: Database Migration

### ✅ Good Decomposition

**Parent Task:**
```
Title: Migrate database from MySQL to PostgreSQL
Type: CODING
Priority: 10
Description:
Our application currently uses MySQL, but we need to migrate to PostgreSQL for
better performance and features. This includes schema conversion, data migration,
application code updates, and comprehensive testing.
```

**Complexity Analysis:**
- Data migration: +3
- Multi-phase work: detected
- Breaking changes: +2
- Testing requirements: +1
- **Total Score: 6**

**Signals:**
- Sequential Multi-Phase Work (strength: 3)
- High complexity (strength: 2)

**Decision:** DECOMPOSE

**Subtasks:**

1. **Title:** "Create PostgreSQL schema and migration scripts"
   **Description:** Convert MySQL schema to PostgreSQL syntax. Create migration scripts for DDL changes. Set up PostgreSQL development and staging environments. Document schema differences and compatibility notes.

2. **Title:** "Implement data migration script with validation"
   **Description:** Write data migration script that transforms MySQL data to PostgreSQL format. Include data validation checks pre and post-migration. Add rollback capabilities. Test migration on staging data.

3. **Title:** "Update application database driver and queries"
   **Description:** Replace MySQL driver with PostgreSQL driver. Update queries to use PostgreSQL-specific syntax where needed. Modify connection pooling configuration. Run full test suite against PostgreSQL.

**Why this is good:**
- Clear sequential phases (schema → data → code)
- Each phase is independently testable with validation checkpoints
- Reasonable scope per subtask
- Captures the migration workflow naturally

---

## Example 5: Feature Flag System

### ✅ Good Decomposition

**Parent Task:**
```
Title: Add feature flag system
Type: CODING
Priority: 20
Description:
Implement a feature flag system to enable gradual rollouts and A/B testing.
Flags should be configurable per environment and user segment. Need backend
storage, frontend flag checks, and admin UI for toggling.
```

**Complexity Analysis:**
- Multi-layer: +3 (backend + frontend + admin UI)
- External integration: 0
- Testing: +1
- **Total Score: 4** → Below threshold BUT has strong multi-layer signal

**Signals:**
- Multiple Independent Components (strength: 3) ← STRONG SIGNAL
- Cross-Cutting Concerns (strength: 2)

**Decision:** DECOMPOSE (strong signal overrides moderate score)

**Subtasks:**

1. **Title:** "Implement backend feature flag service and storage"
   **Description:** Create feature flag data model and database table. Implement CRUD API for managing flags. Add environment and user segment targeting logic. Include flag evaluation service.

2. **Title:** "Add frontend feature flag client and React hooks"
   **Description:** Create JavaScript client that fetches and caches flags. Implement React hooks (useFeatureFlag) for component-level checks. Add flag evaluation logic on frontend. Include fallback behavior.

3. **Title:** "Build admin UI for feature flag management"
   **Description:** Create admin panel to list, create, edit, and delete feature flags. Add toggle interface for enabling/disabling flags. Include targeting configuration UI (environments, user segments). Add audit log display.

**Why this is good:**
- Natural split by architectural layer
- Each subtask delivers value independently
- Clear interfaces between components
- Testable at each layer

---

## Example 6: Performance Optimization (Multi-Phase)

### ✅ Good Decomposition

**Parent Task:**
```
Title: Optimize slow query performance in dashboard
Type: CODING
Priority: 15
Description:
Dashboard queries are taking 5-10 seconds to load. Need to identify bottlenecks,
optimize queries, and potentially add caching. Performance target: < 1 second load time.
```

**Complexity Analysis:**
- Performance optimization: +2
- Multi-phase (profile → fix → validate): detected
- **Total Score: 2** → Low BUT has sequential phase signal

**Signals:**
- Sequential Multi-Phase Work (strength: 3) ← STRONG SIGNAL
- Performance optimization keywords (strength: 1)

**Decision:** DECOMPOSE (strong phase signal)

**Subtasks:**

1. **Title:** "Profile dashboard queries and identify bottlenecks"
   **Description:** Add query timing logs. Use database EXPLAIN to analyze query plans. Identify N+1 queries and missing indexes. Document findings with specific slow queries and execution times.

2. **Title:** "Add database indexes and optimize query structure"
   **Description:** Create indexes based on profiling results. Refactor N+1 queries to use eager loading. Optimize JOIN operations. Verify performance improvement with benchmarks.

3. **Title:** "Implement query result caching with Redis"
   **Description:** Set up Redis connection. Add caching layer for expensive queries. Implement cache invalidation on data updates. Configure TTL policies. Measure final performance against target.

**Why this is good:**
- Natural phases: measure → fix → optimize further
- Each phase informs the next (can't optimize without profiling)
- Each phase is a checkpoint (can stop after phase 2 if target is met)
- Clear validation criteria at each step

---

## Anti-Patterns to Avoid

### ❌ Anti-Pattern 1: Meta-Tasks as Subtasks

**Bad:**
```
1. Implement feature
2. Write tests
3. Write documentation
4. Code review
```

**Why bad:** Tests and documentation should be part of implementation. Code review is a process step, not a task.

**Good:**
```
1. Implement backend API with tests
2. Implement frontend UI with tests
```

---

### ❌ Anti-Pattern 2: Artificial Splitting

**Bad (for a simple config change):**
```
1. Read current config
2. Update config values
3. Validate config
4. Deploy config
```

**Why bad:** Entire change is 5 minutes of work. Don't decompose.

---

### ❌ Anti-Pattern 3: Overlapping Responsibilities

**Bad:**
```
1. Implement authentication backend
2. Add security features
3. Handle user sessions
```

**Why bad:** "Security features" and "user sessions" overlap with authentication. Unclear boundaries.

**Good:**
```
1. Implement authentication service (login, signup, token generation)
2. Add session management (session storage, expiration, refresh)
3. Implement authorization middleware (role checks, permissions)
```

---

## Decision Tree

Use this decision tree when considering decomposition:

```
Is task at depth 2?
├─ YES → DO NOT DECOMPOSE (max depth reached)
└─ NO → Continue

Is it a quick win? (typo, docs, simple config)
├─ YES → DO NOT DECOMPOSE
└─ NO → Continue

Calculate complexity score:
├─ Score ≥ 8 → DECOMPOSE
├─ Score 5-7 AND has 2+ signals → DECOMPOSE
├─ Has strength-3 primary signal → DECOMPOSE
└─ Otherwise → DO NOT DECOMPOSE
```

---

## Summary

**Good decomposition:**
- Natural split by architecture layer, phase, or component
- Each subtask is independently testable and completable
- Clear, specific, actionable titles and descriptions
- No meta-tasks (tests, docs, deployment)
- 2-5 subtasks total
- Reasonable scope per subtask (< 300 lines)

**Bad decomposition:**
- Vague titles ("Set up X", "Other work")
- Meta-tasks as subtasks ("Write tests", "Documentation")
- Overlapping or unclear responsibilities
- Too granular (1-line changes split into multiple tasks)
- Includes system processes (deployment, code review)
- More than 5 subtasks (too fragmented)
