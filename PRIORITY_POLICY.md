# Flux Priority Policy

**Last Updated:** 2026-02-16
**Version:** 2.0

## Overview

This document defines the priority policy for task management in the Flux autonomous coding system. Priorities determine task execution order, UI display, and model selection.

## Priority Scale (1-100)

### 1-15: CRITICAL
**Active production emergencies only**

- Production outage currently affecting users
- Data loss in progress
- Security breach being actively exploited
- System completely non-functional

**Examples:**
- Database corruption causing data loss
- Security breach with active exploitation
- Complete service outage

**Default Model:** Opus (priority ≤ 5 only)

---

### 16-30: HIGH
**Urgent work blocking users or development**

- Blocking bugs preventing user actions
- Important security fixes (not actively exploited)
- Critical features with hard deadlines
- Issues blocking multiple developers

**Examples:**
- Users cannot complete checkout flow
- API endpoint returning 500 errors
- Security vulnerability with patch available
- Blocker preventing release

**Implications:**
- Auto-merge enabled for bugfixes (priority ≤ 15)
- Decomposition encouraged for complex features (priority ≤ 25)

---

### 31-55: NORMAL (DEFAULT)
**Standard development work**

- New features and enhancements
- Refactoring and code improvements
- Most bug fixes
- Test additions
- Performance optimizations

**Default Priority:** 40 (middle of range)

**Examples:**
- Add new dashboard widget
- Refactor authentication module
- Fix minor UI glitch
- Add unit tests for service layer
- Optimize database query

**This is where 90% of tasks should fall.**

---

### 56-75: LOW
**Nice-to-have improvements**

- Minor enhancements
- Non-blocking issues
- Quality-of-life improvements
- Technical debt that's not urgent

**Examples:**
- Add keyboard shortcuts
- Improve error messages
- Extract common utility function
- Minor performance tweaks

---

### 76-100: BACKLOG
**Future work, no immediate action**

- Cosmetic changes
- Documentation updates
- Code cleanup
- Ideas for future consideration

**Examples:**
- Update README with better examples
- Standardize variable naming
- Add code comments
- Brainstorm feature ideas

---

## Default Behavior

### When Priority is Not Specified
- **Default to 40** (middle of NORMAL range)
- Applied when `priority` field is 0 or missing

### Triage System
- **Model suggestion:** When in doubt, assign 40
- **Sanity guard:** Short/garbage input with priority < 20 → override to 40
- **Range enforcement:** Valid priorities must be 1-100

---

## System Integration

### Task Selection (Manager)
Tasks are selected in this order:
1. **Priority (ascending)** — lower numbers execute first
2. **Goal boost** — tasks matching current goal preferred
3. **Created timestamp (ascending)** — older tasks within same priority

### Model Selection (Orchestrator)
- **Priority ≤ 5:** Uses Opus model (highest capability)
- **Priority > 5:** Uses Sonnet model (cost-effective)

### Auto-Merge Rules (Executor)
- **Maintenance tasks:** Always auto-merge
- **High-priority bugfixes (≤ 15):** Auto-merge
- **Small changes (≤ 3 files, < 100 lines):** Auto-merge
- **Everything else:** Requires operator review

### Decomposition Signals (Executor)
- **High-value features (≤ 25):** Encouraged to decompose for phased delivery

---

## Frontend UI

### Priority Presets
```
Critical:  10  (red)
High:      25  (amber)
Normal:    40  (blue)    ← default
Low:       65  (gray)
Backlog:   85  (light gray)
```

### Display Format
- Tasks show as `P{number}` (e.g., P40, P10)
- Sorting by priority: ascending (lower = more urgent)

---

## Migration Notes

### Changed from v1.0
- **CRITICAL range:** 1-10 → 1-15 (wider emergency band)
- **HIGH range:** 11-30 → 16-30 (clearer boundary)
- **NORMAL range:** 31-50 → 31-55 (more room for standard work)
- **Default priority:** 50 → 40 (better positioned in range)
- **Sanity guard threshold:** < 10 → < 20 (catches more problematic assignments)

### Backward Compatibility
- Existing tasks with priority 50 remain valid
- Schema default changed from 50 → 40 for new tasks
- No migration of existing task priorities required

---

## Examples by Category

### Production Emergency (1-15)
- ✅ "Database replication failing, losing user data"
- ✅ "Authentication broken, nobody can log in"
- ❌ "Critical bug that needs fixing soon" (use 16-30)
- ❌ "Very important feature for big customer" (use 16-30)

### High Priority (16-30)
- ✅ "Payment flow broken on checkout page"
- ✅ "XSS vulnerability discovered in user input"
- ✅ "Hotfix required for v2.0 release tomorrow"
- ❌ "Would be nice to fix before release" (use 31-55)

### Normal Priority (31-55)
- ✅ "Add export to CSV feature"
- ✅ "Refactor user service to use repository pattern"
- ✅ "Fix typo in error message"
- ✅ "Improve test coverage for auth module"

### Low Priority (56-75)
- ✅ "Add dark mode toggle to settings"
- ✅ "Optimize image loading on dashboard"
- ✅ "Extract repeated form validation logic"

### Backlog (76-100)
- ✅ "Update architecture docs"
- ✅ "Brainstorm: AI-powered search feature"
- ✅ "Consider migrating to React 19 next year"

---

## Validation Rules

### Triage Prompt Rules
1. When in doubt → assign 40
2. Unclear/test input → assign 40
3. Priority 1-15 requires **active** production impact evidence
4. Priority 16-30 requires **clear** blocking issue evidence
5. Most development work belongs in 31-55

### Parser Sanity Checks
- Input length < 10 chars AND priority < 20 → override to 40
- Priority outside 1-100 range → keep task default
- Priority = 0 → use default (40)

---

## Best Practices

### For Operators Creating Tasks
1. **Default to Normal (40)** unless you have a specific reason
2. **Be honest about urgency** — avoid priority inflation
3. **CRITICAL is rare** — reserve for true emergencies
4. **When unsure** → pick 40 and let triage refine it

### For Triage Analysis
1. **Start conservative** — easier to escalate than de-escalate
2. **Require evidence** for HIGH/CRITICAL assignments
3. **Most bugs are NORMAL** — blocking users != production down
4. **Features are rarely urgent** — even "important" ones

### For System Design
1. **Avoid hardcoded thresholds** — reference this document
2. **Test with realistic priorities** — use 25, 40, 65 in tests
3. **Document assumptions** — explain why threshold was chosen

---

## Related Files

| File | Purpose |
|------|---------|
| `go/src/internal/triager/triage.txt` | Triage prompt template with priority guidelines |
| `go/src/internal/models/task.go` | Task model, NeedsOpus logic (≤ 5) |
| `go/src/internal/manager/manager.go` | Task selection ordering |
| `go/src/internal/executor/executor.go` | Auto-merge rules (≤ 15) |
| `go/src/internal/executor/decomposition_criteria.go` | Decomposition signals (≤ 25) |
| `go/src/internal/db/schema.go` | Database default (40) |
| `frontend/src/pages/Tasks.tsx` | Priority presets and UI defaults |

---

## Version History

### v2.0 (2026-02-16)
- Refined priority ranges (1-15, 16-30, 31-55, 56-75, 76-100)
- Changed default from 50 → 40
- Updated sanity check threshold from < 10 → < 20
- Aligned UI presets with scale (10, 25, 40, 65, 85)
- Clearer semantic boundaries and terminology

### v1.0 (original)
- Initial policy (1-10, 11-30, 31-50, 51-70, 71-100)
- Default priority 50
- Basic triage guidance
