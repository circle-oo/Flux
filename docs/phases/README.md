# Flux Implementation Plans

## Overview

This directory contains detailed implementation plans for the Flux Autonomous Engineering System. Each phase builds on the previous one, progressing from a basic skeleton to a fully autonomous system.

**Major Architecture Change (Phase 2C)**: The system migrates from `claude -p` subprocess + REST API to Python Agent SDK (via gRPC) + Connect-RPC (protobuf). This unifies all communication through a single `flux.proto` definition.

## Phase Summary

| Phase | Name | Goal | Key Deliverable |
|-------|------|------|-----------------|
| [Phase 1](./phase1-foundation.md) | Foundation | Flux boots, Web UI CRUD | `go build` → single binary → Web UI |
| [Phase 2A](./phase2a-core-pipeline.md) | Core Pipeline | Task → PR pipeline | End-to-end execution pipeline |
| [Phase 2B](./phase2b-pipeline-hardening.md) | Pipeline Hardening | Reliable unattended ops | Rate limit, Vault Writer, launchd |
| [Phase 2C](./phase2c-architecture-migration.md) | Architecture Migration | Protobuf + Agent SDK | Connect-RPC + gRPC + Python Agent Manager |
| [Phase 3](./phase3-orchestration-insights.md) | Orchestration + Insights + Knowledge | Autonomous operation | Auto-scaling, analytics, Knowledge UI |
| [Phase 4](./phase4-knowledge-autonomy.md) | Knowledge & Autonomy | Autonomous growth | Researcher agents, self-improvement |

## Dependency Chain

```
Phase 1 (Foundation)
  └── Phase 2A (Core Pipeline)
        ├── Phase 2B (Pipeline Hardening)
        │     └── Phase 2C (Architecture Migration)
        │           └── Phase 3 (Orchestration + Insights + Knowledge)
        │                 └── Phase 4 (Knowledge & Autonomy)
        └── "Flux builds Flux" transition point
```

## Tech Stack

### Phase 1–2B (Current)
- **Language**: Go 1.22+
- **Execution**: `claude -p` subprocess (Claude Code CLI)
- **API**: REST (Go `http.ServeMux`)
- **Real-time**: WebSocket
- **Database**: SQLite (WAL mode, `modernc.org/sqlite`)
- **Frontend**: React 18 + TypeScript + Tailwind CSS + Vite + Zustand
- **Integrations**: Claude Code CLI, GitHub API, Discord Webhooks, ccusage

### Phase 2C+ (Target)
- **Language**: Go 1.22+ (backend) + Python 3.11+ (agent manager)
- **Execution**: Python Agent SDK via gRPC streaming
- **API**: Connect-RPC (protobuf, browser-native HTTP)
- **Real-time**: Connect-RPC server-side streaming (SSE)
- **Proto**: Single `flux.proto` → Go + TypeScript + Python code generation
- **Knowledge**: Obsidian Vault + Obsidian CLI (v1.12+)
- **Deployment**: macOS launchd (Go + Python processes)

## Conventions

- All optional TEXT fields: `''` default (never NULL)
- JSON array fields: `'[]'` default
- Quality: minimum Sonnet model (never Haiku)
- QA: tests mandatory for coding tasks
- Git: worktree-based parallel execution

## Self-Contained Phase Docs

Each phase document (2C, 3, 4) is fully self-contained with all code samples, API schemas, architecture diagrams, and technical decisions inline. No external reference documents needed.

| Phase | Content Merged From |
|-------|---------------------|
| Phase 2C | Proto migration plan (full proto definition, code samples, architecture) |
| Phase 3 | Insight analytics plan (API schemas, SQL strategies, chart library decisions) + Obsidian integration plan (CLI mapping, VaultFacade, Knowledge API/UI) |
| Phase 4 | Already self-contained |

## How to Use These Plans

1. Start with Phase 1 and complete all tasks sequentially within the phase
2. Each task lists its files, dependencies, and acceptance criteria
3. Do not skip phases — each builds on the previous
4. Phase 2A experiments (sandbox, rate limit, Agent Teams) inform later phases
5. "Flux builds Flux" is the Phase 2A completion milestone
6. Phase 2C is the major architecture migration — plan for a dedicated sprint

## Testing Strategy

- **Unit Tests Required**: Every Medium+ complexity task must include unit tests
- **Phase 1**: Establish test infrastructure (testutil package, mock helpers)
- **Phase 2A**: Add integration tests for pipeline components
- **Phase 2B**: E2E smoke test for complete task lifecycle
- **Phase 2C**: Integration tests for Connect-RPC + gRPC + Agent SDK flow
- **Phase 3**: Unit tests for insights queries, knowledge API handlers
- **Coverage**: Focus on business logic, error paths, and edge cases

## Security Principles

1. **Sandbox Evaluation**: Phase 2A blocker — must validate before unattended operation
2. **Authentication**: Internal API uses shared secret authentication (not just localhost check)
3. **Session Management**: Persisted in SQLite (not in-memory), with optional expiry
4. **Rate Limiting**: Login attempts limited to 5/hour
5. **Self-Improvement PRs**: NEVER auto-merged — always require human approval
6. **Input Validation**: Prompt validation on all task inputs to prevent injection
7. **Proto Contract**: `flux.proto` is immutable in self-improvement (Phase 4)
