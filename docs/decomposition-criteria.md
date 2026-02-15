# Task Decomposition Criteria

## Overview

Task decomposition is the process of breaking down complex, multi-step coding tasks into smaller, focused subtasks. This document defines when and how tasks should be decomposed in the Flux autonomous engineering system.

## When to Decompose Tasks

### Primary Signals (Must Match at Least One)

A task should be decomposed if it exhibits any of these characteristics:

1. **Multiple Independent Components**
   - Task requires changes across 3+ unrelated subsystems
   - Example: "Add authentication" → separate subtasks for frontend, backend API, database schema

2. **Sequential Multi-Phase Work**
   - Task requires distinct phases that build on each other
   - Example: "Database migration" → plan schema, write migration, test migration, rollback plan

3. **High Complexity Score** (See Complexity Scoring below)
   - Combined score from multiple complexity factors exceeds threshold

4. **Large Scope**
   - Estimated to touch 10+ files across multiple directories
   - Estimated to produce 500+ lines of diff

5. **Cross-Cutting Concerns**
   - Task requires coordination between multiple tech stack layers
   - Example: "Add feature flag" → backend flag storage, frontend flag check, config, tests

### Secondary Signals (Supporting Indicators)

These signals strengthen the case for decomposition:

- Task description contains words like "and", "also", "additionally", "then"
- Task title contains multiple verbs (e.g., "Refactor and optimize")
- Task has multiple acceptance criteria (3+)
- Task mentions multiple file paths or modules
- Task type is CODING with priority ≤ 20 (high-value features)
- Original user description is ambiguous or open-ended

## When NOT to Decompose

Do NOT decompose tasks that:

1. **Single Atomic Change**
   - Fixes one bug in one location
   - Adds one function/method
   - Updates one configuration file

2. **Already Small**
   - Estimated < 100 lines of diff
   - Changes confined to 1-2 files in same directory

3. **Quick Wins**
   - Documentation updates
   - Simple config changes
   - Typo fixes

4. **Well-Defined Single Feature**
   - Clear, focused requirement
   - Single acceptance criterion
   - Single tech stack layer

## Complexity Scoring

Calculate a complexity score to determine if decomposition is warranted.

### Scoring Factors

| Factor | Points | Indicators |
|--------|--------|------------|
| **Multi-layer architecture** | +3 | Frontend + Backend + Database |
| **External integrations** | +2 | Third-party API, webhooks, external service |
| **Data migration** | +3 | Schema changes, data transformation |
| **Security/auth changes** | +2 | Authentication, authorization, encryption |
| **Performance optimization** | +2 | Profiling, benchmarking, algorithm redesign |
| **Testing requirements** | +1 | Integration tests, E2E tests required |
| **Breaking changes** | +2 | API changes, backward compatibility concerns |
| **Multi-repo changes** | +3 | Changes span multiple repositories |
| **Ambiguous requirements** | +2 | Unclear acceptance criteria, exploratory work |

### Decomposition Thresholds

- **Score 0-4**: No decomposition needed (simple task)
- **Score 5-7**: Consider decomposition if other signals present
- **Score 8+**: Strong candidate for decomposition

## Decomposition Rules

### Maximum Subtask Count

- **Minimum**: 2 subtasks (no point decomposing into 1)
- **Maximum**: 5 subtasks (enforced by `subtask.go:104`)
- **Optimal**: 2-3 subtasks for most cases

### Maximum Depth

- **Maximum tree depth**: 2 levels (parent → child → grandchild)
- Tasks at depth 2 cannot be further decomposed
- Prevents excessive fragmentation and overhead

### Subtask Dependency Guidelines

- Subtasks should be **as independent as possible**
- If strict ordering is required, use `depends_on` field
- Each subtask should be executable in isolation (given dependencies)

### Subtask Size

Each subtask should be:
- **Completable in one execution**: Claude Code can finish in one session
- **Testable independently**: Has clear acceptance criteria
- **Focused**: Single responsibility per subtask
- **Estimated < 300 lines of diff** per subtask

## Decomposition Patterns

### Pattern 1: By Architecture Layer

**Good for**: Full-stack features

```
Parent: "Add user profile editing"
├── Subtask 1: "Implement backend API endpoint for profile updates"
├── Subtask 2: "Create frontend profile edit form component"
└── Subtask 3: "Add profile update integration tests"
```

### Pattern 2: By Phase

**Good for**: Multi-step processes

```
Parent: "Migrate database to PostgreSQL"
├── Subtask 1: "Create PostgreSQL schema and migration scripts"
├── Subtask 2: "Implement data migration script with validation"
└── Subtask 3: "Update application DB connection and test"
```

### Pattern 3: By Component

**Good for**: Modular systems

```
Parent: "Add payment processing"
├── Subtask 1: "Integrate Stripe payment gateway"
├── Subtask 2: "Implement payment webhook handlers"
├── Subtask 3: "Add payment status UI and error handling"
└── Subtask 4: "Create payment reconciliation background job"
```

### Pattern 4: By Risk/Research

**Good for**: Uncertain or exploratory work

```
Parent: "Optimize slow query performance"
├── Subtask 1: "Profile queries and identify bottlenecks"
├── Subtask 2: "Add database indexes based on profiling results"
└── Subtask 3: "Implement query result caching if needed"
```

## Validation Criteria for Subtasks

Before creating subtasks, validate that each subtask:

1. **Has a clear, actionable title** (not vague like "Other work")
2. **Has a specific description** with concrete acceptance criteria
3. **Is independently testable** (can verify completion)
4. **Makes sense in context** (not overlapping with other subtasks)
5. **Contributes to parent goal** (all subtasks together complete parent)
6. **Avoids circular dependencies** (A depends on B depends on A)

## Examples

### ✅ Well-Decomposed Task

**Parent**: "Implement real-time notifications system"
**Complexity Score**: 10 (multi-layer: +3, external integration: +2, testing: +1, architecture: +2, security: +2)
**Reasoning**: Spans backend, frontend, WebSocket infrastructure, and has security considerations

**Subtasks**:
1. "Set up WebSocket server infrastructure and connection handling"
2. "Implement backend notification queue and delivery system"
3. "Create frontend notification UI component and state management"
4. "Add notification preferences and subscription management"

**Why this is good**:
- Each subtask is focused and independently testable
- Clear separation of concerns (infra, backend, frontend, config)
- Reasonable scope per subtask (estimated 150-250 lines each)
- Subtasks can be parallelized or executed sequentially

### ❌ Poorly-Decomposed Task

**Parent**: "Add user authentication"
**Complexity Score**: 8
**Reasoning**: Task should be decomposed, but decomposition is flawed

**Bad Subtasks**:
1. "Set up authentication" ← Too vague
2. "Fix bugs in authentication" ← No bugs exist yet
3. "Write tests" ← Too generic, should be per subtask
4. "Refactor old code" ← Out of scope
5. "Documentation" ← Out of scope
6. "Deploy to production" ← System handles deployment

**Why this is bad**:
- Vague, generic subtask titles
- Includes out-of-scope work (refactoring, deployment)
- Tests should be part of each implementation subtask
- Missing concrete implementation steps

### ✅ Better Decomposition

**Parent**: "Add user authentication"

**Good Subtasks**:
1. "Implement JWT token generation and validation in backend API"
2. "Create login/signup forms and auth state management in frontend"
3. "Add protected route middleware and session management"

**Why this is good**:
- Specific, actionable titles
- Focused on implementation, not meta-work
- Tests are implicitly part of each subtask
- Complete coverage of authentication system

### ❌ Task That Should NOT Be Decomposed

**Parent**: "Fix typo in README file"
**Complexity Score**: 0
**Reasoning**: Single file, single change, trivial scope

**If decomposed (incorrectly)**:
1. "Find the typo"
2. "Fix the typo"
3. "Commit the fix"

**Why this is bad**:
- Overhead of 3 tasks for a 1-line change
- No benefit from decomposition
- Tasks are artificially split

**Correct approach**: Execute as a single atomic task.

## Implementation Notes

### For Task Triager

The triager (`internal/triager/triager.go`) should:
- Analyze task description and triage analysis
- Calculate complexity score using defined factors
- Set a `should_decompose` hint in triage analysis
- Do NOT create subtasks directly (that's the executor's job)

### For Executor

The executor (`internal/executor/executor.go`) should:
- Read decomposition signals from triage analysis
- Include decomposition guidance in prompt when signals are present
- Parse decomposition response from Claude Code
- Validate subtasks before creating them
- Enforce max subtasks (5) and max depth (2)
- Set parent task status to `DECOMPOSED` and create subtasks with `READY` status

### For Prompt Templates

The autopilot prompt (`prompts/autopilot.txt`) should:
- Include decomposition guidance when complexity threshold is met
- Provide decomposition JSON schema
- Give examples of good decomposition patterns
- Remind Claude to only decompose when appropriate

## Revision History

- 2026-02-16: Initial criteria and guidelines established
