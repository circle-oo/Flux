# Sandbox Evaluation (Task 2A.3)

## Objective

Test Claude Code's native sandbox/permission modes for compatibility with Flux's worktree-based workflow. Determine whether to keep `--dangerously-skip-permissions` or switch to a safer alternative.

## Available Permission Modes (v2.1.42)

From `--permission-mode` flag:
- `acceptEdits` — Auto-accept file edits
- `bypassPermissions` — Skip all permission checks
- `default` — Normal interactive permissions
- `delegate` — Delegate permissions
- `dontAsk` — Don't ask for permissions
- `plan` — Plan mode (read-only)

From `--dangerously-skip-permissions`:
- Bypasses ALL permission checks
- Recommended "only for sandboxes with no internet access"

Alternative: `--allow-dangerously-skip-permissions`:
- Enables bypassing as an option without making it the default

## Test Results

### Test 1: File Read/Write in Worktree
- **Method**: `--dangerously-skip-permissions` with `cmd.Dir = worktreePath`
- **Result**: Full filesystem access within and outside worktree
- **Concern**: No containment — can modify files anywhere on the system

### Test 2: Permission Mode `bypassPermissions`
- **Method**: `--permission-mode bypassPermissions`
- **Result**: Equivalent to `--dangerously-skip-permissions`
- **Concern**: Same lack of containment

### Test 3: `.claude/settings.json` with Tool Allowlist
- **Method**: Per-worktree settings with explicit tool permissions
- **Result**: Tools restricted to allowlist, but no filesystem path restriction
- **Configuration**:
```json
{
  "permissions": {
    "allow": [
      "Bash(*)", "Read(*)", "Write(*)", "Edit(*)",
      "Grep(*)", "Glob(*)"
    ]
  }
}
```

## Findings

1. **No native filesystem sandbox** in Claude Code v2.1.42 — there is no built-in way to restrict file access to a specific directory
2. `--dangerously-skip-permissions` is the only way to run non-interactively without permission prompts
3. `.claude/settings.json` restricts which tools are available but NOT which paths they can access
4. The `--permission-mode bypassPermissions` is functionally identical to `--dangerously-skip-permissions`

## Decision

**Keep `--dangerously-skip-permissions` for Phase 2A.**

Rationale:
- No native sandbox alternative exists in the current CLI version
- The `.claude/settings.json` tool allowlist provides some control but no path restriction
- Post-execution verification (Task 2A.9) provides a detection layer for unauthorized modifications
- Flux runs on a dedicated machine (macOS with Tailscale), reducing risk surface

## Mitigation Strategy

1. **Post-execution verification** (implemented in executor.go): Check ~/.ssh, ~/.aws, ~/.gitconfig, ~/.zshrc, ~/.bashrc for unexpected modifications
2. **Worktree isolation**: Each task runs in its own git worktree with a dedicated branch
3. **CLAUDE.md instructions**: Task-specific instructions guide Claude to stay within scope
4. **.claude/settings.json**: Tool allowlist in each worktree

## Future Considerations

- Monitor Claude Code releases for native sandbox/containment features
- Consider Docker/container-based execution if stronger isolation is needed
- The `--allow-dangerously-skip-permissions` flag could be useful if combined with a future sandbox feature
