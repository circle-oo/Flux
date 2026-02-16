# Obsidian CLI Evaluation

**Date**: 2026-02-16
**Status**: Implemented (minimal wrapper)
**Task**: 2C.6

## Overview

The native Obsidian CLI (`obsidian` command) is not installed on the Mac Mini. A minimal bash wrapper script (`tools/obsidian-cli`) was created to provide vault access for Flux agents via the Bash tool.

## Implementation

### CLI Wrapper: `tools/obsidian-cli`

A self-contained bash script providing five commands:

| Command | Description | Example |
|---------|-------------|---------|
| `search <query>` | Case-insensitive full-text search across .md files | `obsidian-cli search "decomposition" --vault ~/ObsidianVault/Flux` |
| `read <path>` | Read a note's content | `obsidian-cli read "Tasks/completed/1f0ae678.md" --vault ~/ObsidianVault/Flux` |
| `write <path>` | Create/overwrite a note | `obsidian-cli write "Research/findings.md" --content "..." --vault ~/ObsidianVault/Flux` |
| `append <path>` | Append to existing note | `obsidian-cli append "Research/findings.md" --content "..." --vault ~/ObsidianVault/Flux` |
| `list [folder]` | List .md files | `obsidian-cli list Research --vault ~/ObsidianVault/Flux` |

All commands require `--vault <path>`. Write/append require `--content <text>`.

### Agent Access by Type

| Agent | Can Search | Can Read | Can Write | Notes |
|-------|-----------|----------|-----------|-------|
| dev | Yes | Yes | No (not in prompt) | Reads project notes for context |
| qa | Yes | Yes | No (not in prompt) | Reads task logs to understand past failures |
| devops | Yes | Yes | No (not in prompt) | Reads deployment/infrastructure notes |
| rnd | Yes | Yes | Yes | Writes research findings to `Research/` |

Write instructions are only included in the `rnd` agent's system prompt to prevent non-research agents from cluttering the vault.

### Config Integration

`agent_manager/config.py` provides:
- `VAULT_PATH` — defaults to `~/ObsidianVault/Flux`, overridable via `FLUX_VAULT_PATH` env var
- `OBSIDIAN_CLI_PATH` — defaults to `tools/obsidian-cli`, overridable via `OBSIDIAN_CLI_PATH` env var
- System prompts include full command examples with resolved paths

## Vault Writer Conflict Analysis

**No conflict.** The Go Vault Writer and obsidian-cli operate on separate concerns:

| Aspect | Vault Writer (Go) | obsidian-cli (Agent) |
|--------|-------------------|---------------------|
| Trigger | System events (task completion, PR creation) | Agent decision during execution |
| Directories | `Tasks/`, `Goals/`, `Projects/`, `Services/` | `Research/` (rnd agents) |
| Write pattern | Async channel, sequential processing | Synchronous per-command |
| Concurrency | Single writer goroutine, mutex-protected | One invocation per agent turn |

The Go Writer owns system-managed vault content. Agents own research content. No overlapping paths.

## Test Results

All operations verified against the live vault at `~/ObsidianVault/Flux/`:

1. **list** — Returns sorted .md file paths relative to vault root
2. **list (folder)** — Correctly scopes to subfolder
3. **read** — Returns full note content, auto-appends `.md` extension
4. **search** — Finds matching files and shows context lines (grep-based)
5. **write** — Creates note with content, creates parent dirs if needed
6. **append** — Appends to existing note with newline separator
7. **Error: missing note** — Returns `error: note not found` with exit code 1
8. **Error: missing vault** — Returns `error: --vault flag is required` with exit code 1

## Limitations

1. **No frontmatter parsing** — The wrapper does plain text search, not YAML frontmatter-aware queries
2. **No backlinks/outlinks** — Requires Obsidian's graph database (Phase 3 VaultFacade will add this)
3. **No tag indexing** — Search finds `#tag` in text but doesn't provide tag aggregation
4. **No concurrent write protection** — If two rnd agents write to the same file simultaneously, last write wins. Unlikely in practice since tasks are sequential per agent.

These limitations are acceptable for Phase 2C. Phase 3 (Task 3.13: VaultFacade) will implement the full Obsidian CLI integration with backlinks, tags, and degraded-mode fallbacks.

## Migration Path

When the native Obsidian CLI is installed (Phase 3 prerequisite):
1. Replace the wrapper with `obsidian` CLI commands (different syntax: `obsidian read path=X`)
2. Update `OBSIDIAN_CLI_PATH` env var to point to native binary
3. The wrapper can remain as fallback if Obsidian app isn't running
