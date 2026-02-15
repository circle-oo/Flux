# Agent Teams Compatibility Experiment (Task 2A.14)

## Objective

Test Claude Code's Agent Teams and agent definition features for potential use within Flux Executor pods.

## Available Features (v2.1.42)

### 1. `--agents` Flag
```
--agents <json>  JSON object defining custom agents
```

Example:
```bash
claude -p "review this code" --agents '{"reviewer": {"description": "Reviews code", "prompt": "You are a code reviewer"}}'
```

### 2. `--agent` Flag
```
--agent <agent>  Agent for the current session
```

Selects a specific agent definition for the session.

### 3. `.claude/agents/` Directory
Agent definition files can be placed in `.claude/agents/` within the project:
```
.claude/agents/
  reviewer.md
  planner.md
  tester.md
```

Each file contains a system prompt that defines the agent's behavior.

## Test Results

### Test 1: `--agents` in `-p` Mode
- **Method**: Pass JSON agent definitions via `--agents` flag with `-p`
- **Finding**: The flag is accepted by the CLI. Agent definitions can be passed inline.
- **Use case for Flux**: Could define task-type-specific agents (coding agent, research agent, testing agent) per executor run

### Test 2: `.claude/agents/` in Worktree
- **Method**: Create agent definition files in worktree's `.claude/agents/` directory
- **Finding**: Agent files are read from the project directory. Can be set up per-worktree by `WorktreeManager.CreateWorktree()`.
- **Use case for Flux**: Pre-configure agent definitions per project type

### Test 3: Sub-Agent Parallel Execution
- **Finding**: The `--agents` flag defines available agents but does not spawn parallel sub-agents within a single `-p` execution. Agents are session-level configurations, not parallel workers.
- **Limitation**: No built-in way to run multiple agents in parallel within a single Claude Code invocation

## Evaluation Summary

| Feature | Works in `-p` mode? | Useful for Flux? | Recommendation |
|---------|---------------------|-------------------|----------------|
| `--agents` (inline) | Yes | Medium | Could specialize executor behavior per task type |
| `--agent` (select) | Yes | Medium | Select pre-defined agent for task |
| `.claude/agents/` files | Yes | High | Per-project agent definitions in worktrees |
| Parallel sub-agents | No | N/A | Not available in non-interactive mode |

## Recommendations

### Use: `.claude/agents/` in Worktrees (Phase 2B+)
Create task-type-specific agent definitions in each worktree:

```
.claude/agents/
  coder.md        — For CODING tasks: "Write clean, tested code..."
  bugfixer.md     — For BUGFIX tasks: "Diagnose and fix the bug..."
  maintainer.md   — For MAINTENANCE tasks: "Refactor with minimal changes..."
  documenter.md   — For DOCUMENT tasks: "Write clear documentation..."
```

Then pass `--agent coder` (or appropriate agent) based on task type.

### Don't Use: Inline `--agents`
The JSON format is cumbersome for complex prompts. File-based agents in `.claude/agents/` are more maintainable.

### Don't Use: OMC Patterns
Per the spec: "OMC itself should NOT be used as a Flux plugin (dual orchestration conflict). Only evaluate ideas from OMC's agent definition patterns."

The agent definition pattern (`.claude/agents/` files with markdown prompts) is borrowed from OMC's approach but implemented natively using Claude Code's built-in feature.

## Implementation Plan (Deferred to Phase 2B+)

1. **WorktreeManager.setupAgentDefinitions()**: Create `.claude/agents/` with task-type agents
2. **Executor.buildClaudeArgs()**: Add `--agent {taskType}` based on task.Type
3. **Per-project customization**: Allow project-specific agent overrides via project config

## Note on CLAUDE.md

The current `CLAUDE.md` created by `WorktreeManager.setupClaudeMD()` serves a similar purpose to agent definitions — it provides project context and instructions. Agent definitions would complement (not replace) CLAUDE.md by adding task-type-specific behavioral instructions.
