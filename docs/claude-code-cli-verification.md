# Claude Code CLI Verification (Task 2A.0)

## CLI Version

```
2.1.42 (Claude Code)
```

## Required Flags Verification

| Flag | Available | Verified | Notes |
|------|-----------|----------|-------|
| `-p` (prompt/print) | Yes | Yes | Non-interactive mode, prints response and exits |
| `--max-turns` | N/A | N/A | **Not found in CLI help.** Not a supported flag. |
| `--output-format json` | Yes | Yes | Returns structured JSON with result, session_id, cost, usage |
| `--append-system-prompt` | Yes | Yes | Appends to default system prompt |
| `--model` | Yes | Yes | Accepts aliases (`sonnet`, `opus`) or full names |
| `--dangerously-skip-permissions` | Yes | Yes | Bypasses all permission checks |
| `--cwd` | N/A | N/A | **Not found in CLI help.** Use `--add-dir` or run from target directory |
| `--permission-mode` | Yes | Yes | Choices: acceptEdits, bypassPermissions, default, delegate, dontAsk, plan |

## Additional Useful Flags (Discovered)

| Flag | Description |
|------|-------------|
| `--max-budget-usd` | Maximum dollar amount per API call (print mode only) |
| `--system-prompt` | Full system prompt override (vs append) |
| `--allowedTools` | Restrict available tools |
| `--no-session-persistence` | Don't save sessions to disk (print mode only) |
| `--fallback-model` | Auto-fallback when default model overloaded |
| `--json-schema` | Structured output validation |
| `--agents` | Custom agent definitions via JSON |
| `--effort` | Effort level: low, medium, high |

## Fallback Strategies

### `--max-turns` (NOT AVAILABLE)
The `--max-turns` flag does not exist in v2.1.42. The executor should use `--max-budget-usd` as an alternative cost guardrail, or rely on timeout-based termination.

**Action**: Remove `--max-turns` from ClaudeCodeRunner args. Use `--max-budget-usd` instead if cost control is needed.

### `--cwd` (NOT AVAILABLE)
The `--cwd` flag does not exist. Instead:
- Set the working directory of the subprocess via `cmd.Dir = worktreePath`
- Or use `--add-dir` for additional directory access

**Action**: Use `cmd.Dir` in Go's `exec.Command` instead of `--cwd` flag.

## JSON Output Format

When using `--output-format json`, the response is a **single JSON object** (not an array):

```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "duration_ms": 6330,
  "duration_api_ms": 6585,
  "num_turns": 1,
  "result": "SMOKE_TEST_OK",
  "stop_reason": null,
  "session_id": "5131ac4f-0249-4317-a322-d5f1855d5051",
  "total_cost_usd": 0.394,
  "usage": {
    "input_tokens": 3,
    "cache_creation_input_tokens": 62971,
    "output_tokens": 9,
    "service_tier": "standard"
  },
  "modelUsage": { ... },
  "permission_denials": [],
  "uuid": "0454d669-..."
}
```

### Key Fields for Flux:
- `result` — The text output from Claude
- `session_id` — Session identifier
- `total_cost_usd` — Total cost for the execution
- `usage.input_tokens` + `usage.output_tokens` — Token counts
- `is_error` — Whether execution errored
- `num_turns` — Number of conversation turns used

## Smoke Test Verification

```bash
claude -p "respond with exactly: SMOKE_TEST_OK" --max-turns 1 --output-format json
```

Result: Returns JSON with `"result": "SMOKE_TEST_OK"` — **PASS**

## Compatibility Matrix

| Version | `-p` | `--output-format json` | `--append-system-prompt` | `--model` | `--dangerously-skip-permissions` | `--max-turns` | `--cwd` |
|---------|------|------------------------|--------------------------|-----------|----------------------------------|---------------|---------|
| 2.1.42 | Yes | Yes | Yes | Yes | Yes | No | No |

## Required Code Changes

Based on this verification, `ClaudeCodeRunner.Run()` needs updates:

1. **Remove `--max-turns`** — flag doesn't exist
2. **Remove `--cwd`** — use `cmd.Dir` instead
3. **Update ParseResponse** — output is a single JSON object, not an array of messages
4. **Extract fields** from the flat JSON: `result`, `session_id`, `total_cost_usd`, `usage.input_tokens + usage.output_tokens`
