# Flux Implementation Plans

## Overview

This directory contains detailed implementation plans for the Flux Autonomous Engineering System. Each phase builds on the previous one, progressing from a basic skeleton to a fully autonomous system.

## Phase Summary

| Phase | Name | Goal | Key Deliverable |
|-------|------|------|-----------------|
| [Phase 1](./phase1-foundation.md) | Foundation | Flux boots, Web UI CRUD | `go build` → single binary → Web UI |
| [Phase 2A](./phase2a-core-pipeline.md) | Core Pipeline | Task → PR pipeline | End-to-end execution pipeline |
| [Phase 2B](./phase2b-pipeline-hardening.md) | Pipeline Hardening | Reliable unattended ops | Rate limit, Vault, launchd |
| [Phase 3](./phase3-orchestration.md) | Orchestration | Autonomous operation | Auto-scaling, usage tracking |
| [Phase 4](./phase4-knowledge-autonomy.md) | Knowledge & Autonomy | Autonomous growth | Researcher Pods, self-improvement |

## Dependency Chain

```
Phase 1 (Foundation)
  └── Phase 2A (Core Pipeline)
        ├── Phase 2B (Pipeline Hardening)
        │     └── Phase 3 (Orchestration)
        │           └── Phase 4 (Knowledge & Autonomy)
        └── "Flux builds Flux" transition point
```

## Tech Stack

- **Language**: Go 1.22+
- **Database**: SQLite (WAL mode, `modernc.org/sqlite` — no CGO)
- **Frontend**: React 18 + TypeScript + Tailwind CSS + Vite + Zustand
- **Integrations**: Claude Code CLI, GitHub API, Discord Webhooks, ccusage
- **Knowledge**: Obsidian Vault
- **Deployment**: macOS launchd

## Conventions

- All optional TEXT fields: `''` default (never NULL)
- JSON array fields: `'[]'` default
- Quality: minimum Sonnet model (never Haiku)
- QA: tests mandatory for coding tasks
- Git: worktree-based parallel execution

## How to Use These Plans

1. Start with Phase 1 and complete all tasks sequentially within the phase
2. Each task lists its files, dependencies, and acceptance criteria
3. Do not skip phases — each builds on the previous
4. Phase 2A experiments (sandbox, rate limit, Agent Teams) inform later phases
5. "Flux builds Flux" is the Phase 2A completion milestone
