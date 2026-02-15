# Flux — 자율 엔지니어링 시스템: 구현 시방서

> **대상 독자**: Claude Code Agent / AI 코딩 에이전트가 이 시스템을 처음부터 구현할 때 사용.
> **목적**: Flux 구현에 필요한 모든 세부 사항을 포함. 문서를 문자 그대로 따를 것. 의심스러우면 더 단순한 방법을 선택.

---

## 목차

1. [프로젝트 셋업](#1-프로젝트-셋업)
2. [디렉토리 구조](#2-디렉토리-구조)
3. [설정 파일](#3-설정-파일)
4. [데이터베이스 스키마](#4-데이터베이스-스키마)
5. [핵심 타입 & 모델](#5-핵심-타입--모델)
6. [상태 머신](#6-상태-머신)
7. [내부 통신](#7-내부-통신)
8. [연동](#8-연동)
9. [Goal 시스템](#9-goal-시스템)
10. [사용량 추적](#10-사용량-추적)
11. [속도 제한 대응](#11-속도-제한-대응)
12. [모델 선택](#12-모델-선택)
13. [QA & 브랜치 전략](#13-qa--브랜치-전략)
14. [PR 리뷰](#14-pr-리뷰)
15. [오케스트레이션](#15-오케스트레이션)
16. [핵심 컴포넌트](#16-핵심-컴포넌트)
17. [Web UI](#17-web-ui)
18. [API 레퍼런스](#18-api-레퍼런스)
19. [부트스트랩](#19-부트스트랩)
20. [자기 개선](#20-자기-개선)
21. [에러 복구](#21-에러-복구)
22. [Graceful Shutdown](#22-graceful-shutdown)
23. [구현 순서](#23-구현-순서)
24. [컨벤션 & 규칙](#24-컨벤션--규칙)
25. [Future Work](#25-future-work)

---

## 1. 프로젝트 셋업

### 1.1 전제 조건

```bash
# macOS
# Go 1.22+
go version  # >= 1.22 필수

# Node.js 20+ (Claude Code CLI, Web UI 빌드용)
node --version  # >= 20 필수

# Claude Code CLI (정액제, max_5x 플랜)
claude --version
claude login  # 최초 1회 인증

# Git
git --version

# ccusage (선택사항, 사용량 추적용)
npx ccusage@latest --version
```

### 1.2 Go 모듈 초기화

```bash
mkdir -p ~/workspaces/flux
cd ~/workspaces/flux
go mod init github.com/circle-oo/flux
```

### 1.3 주요 의존성

```bash
go get modernc.org/sqlite          # CGO 없는 SQLite
go get gopkg.in/yaml.v3            # YAML 설정 파싱
go get github.com/google/uuid      # UUID 생성
go get nhooyr.io/websocket         # WebSocket
go get golang.org/x/crypto/bcrypt  # 비밀번호 해싱
```

### 1.4 빌드 명령

```bash
# React Web UI 내장 단일 바이너리
go build -o flux ./cmd/flux
```

---

## 2. 디렉토리 구조

### 2.1 Go 프로젝트 구조

```
flux/
├── cmd/
│   └── flux/
│       └── main.go                    # 진입점
├── internal/
│   ├── config/
│   │   └── config.go                  # YAML 설정 로더 → struct
│   ├── db/
│   │   ├── db.go                      # SQLite 연결 (WAL 모드)
│   │   ├── schema.go                  # 스키마 생성 + 마이그레이션
│   │   └── queries.go                 # 공통 쿼리 헬퍼
│   ├── models/
│   │   ├── goal.go                    # Goal struct + CRUD
│   │   ├── project.go                 # Project struct + CRUD
│   │   ├── task.go                    # Task struct + CRUD + Priority Queue
│   │   ├── alert.go                   # Alert struct + CRUD
│   │   └── usage.go                   # UsageSnapshot, RateLimitEvent structs
│   ├── manager/
│   │   ├── manager.go                 # 태스크 큐, 배정, 상태 전이
│   │   └── priority.go                # Priority Queue (Goal 부스트 포함)
│   ├── orchestrator/
│   │   ├── orchestrator.go            # 메인 오케스트레이터 루프 (5분 틱)
│   │   ├── scale_manager.go           # Pod 수/비율, 쿨다운
│   │   ├── usage_collector.go         # ccusage 시계열 수집
│   │   ├── daily_summary.go           # Discord 데일리 서머리
│   │   ├── rate_limit_handler.go      # 감지 → 정지 → 동적 대기 → 재개
│   │   └── goal_advisor.go            # Goal 제안
│   ├── executor/
│   │   ├── executor.go                # Executor Pod 메인 루프
│   │   ├── claude_code.go             # Claude Code CLI 래퍼 (-p 모드)
│   │   ├── guardrails.go              # Timeout, output 크기, diff 제한
│   │   ├── subtask.go                 # 서브태스크 분해
│   │   └── worktree.go                # Git worktree 관리
│   ├── researcher/
│   │   ├── researcher.go              # Researcher Pod 메인 루프
│   │   └── types.go                   # 리서치 타입 정의
│   ├── vault/
│   │   ├── writer.go                  # 싱글 고루틴 채널 기반 직렬 쓰기
│   │   └── templates.go               # Obsidian 마크다운 템플릿
│   ├── github/
│   │   ├── client.go                  # GitHub API 클라이언트
│   │   ├── repo.go                    # 레포 생성
│   │   └── pr.go                      # PR 생성/머지/코멘트 fetch
│   ├── notifier/
│   │   └── discord.go                 # Discord 웹훅 알림
│   ├── server/
│   │   ├── server.go                  # HTTP 서버 셋업
│   │   ├── auth.go                    # 비밀번호 인증 (bcrypt + 세션 토큰)
│   │   ├── api_goals.go               # /api/goals 핸들러
│   │   ├── api_tasks.go               # /api/tasks 핸들러
│   │   ├── api_projects.go            # /api/projects 핸들러
│   │   ├── api_prs.go                 # /api/prs 핸들러
│   │   ├── api_usage.go               # /api/usage 핸들러
│   │   ├── api_orchestrator.go        # /api/orchestrator 핸들러
│   │   ├── internal_api.go            # /internal/ 핸들러 (Pod 통신)
│   │   └── websocket.go               # /ws/events 핸들러
│   └── shutdown/
│       └── shutdown.go                # Graceful Shutdown 로직
├── web/                               # React 프론트엔드
│   ├── src/
│   │   ├── App.tsx
│   │   ├── pages/
│   │   │   ├── Dashboard.tsx
│   │   │   ├── Goals.tsx
│   │   │   ├── Tasks.tsx
│   │   │   ├── Projects.tsx
│   │   │   ├── PRs.tsx
│   │   │   ├── Research.tsx
│   │   │   ├── Usage.tsx
│   │   │   └── Settings.tsx
│   │   ├── components/
│   │   ├── stores/                    # Zustand 스토어
│   │   └── lib/
│   ├── package.json
│   ├── vite.config.ts
│   └── tailwind.config.js
├── data/                              # 런타임 데이터 (gitignored)
│   ├── flux.db                        # SQLite 데이터베이스
│   └── backups/                       # 데일리 백업
├── workspaces/                        # Git worktree (gitignored)
│   ├── repos/                         # Bare 레포
│   └── trees/                         # 태스크별 worktree
├── logs/                              # 로그 파일 (gitignored)
├── config.yaml                        # 설정 파일
├── go.mod
├── go.sum
└── Makefile
```

### 2.2 Obsidian Vault 구조

부트스트랩 시 `~/ObsidianVault/Flux/`에 생성:

```
~/ObsidianVault/Flux/
├── Goals/
│   ├── _current.md                    # 현재 ACTIVE Goal
│   ├── completed/                     # 완료된 Goal
│   └── proposals/                     # 제안된 Goal
├── Projects/
│   └── {project-name}/
│       ├── _index.md                  # 프로젝트 개요
│       ├── architecture.md            # 아키텍처 결정
│       ├── decisions/                 # 결정 기록
│       └── learnings/                 # 학습 내용
├── Research/
│   ├── Industry/                      # 산업 리서치
│   ├── Tools/                         # 도구 평가
│   ├── Ideas/                         # 프로젝트 아이디어
│   └── _history.md                    # 리서치 로그
├── Tasks/
│   └── completed/                     # 완료 태스크 요약
├── Services/
│   └── alerts/                        # 서비스 알림 기록
└── Templates/
    ├── project.md                     # 프로젝트 템플릿
    ├── research.md                    # 리서치 템플릿
    ├── task-summary.md                # 태스크 완료 요약
    └── decision.md                    # 결정 기록 템플릿
```

---

## 3. 설정 파일

### 3.1 config.yaml (전체 템플릿)

```yaml
server:
  port: 8080
  auth:
    enabled: true
    password_env: "FLUX_UI_PASSWORD"     # 평문 → Flux가 bcrypt 비교
    session_expiry: "none"               # Tailscale 의존, 명시적 로그아웃만

database:
  path: "./data/flux.db"
  backup_dir: "./data/backups"
  backup_cron: "0 4 * * *"              # 매일 새벽 4시
  backup_retention_days: 7

vault:
  path: "~/ObsidianVault/Flux"

github:
  username: "circle-oo"
  token_env: "GITHUB_TOKEN"
  auto_create_repo: true
  default_visibility: "public"

claude_code:
  plan: "max_5x"                         # pro, max_5x, max_20x

ccusage:
  command: "npx ccusage@latest"
  collection_interval: 1h

orchestrator:
  check_interval: 5m
  scale_cooldown: 15m
  max_total_pods: 5
  min_research_ratio: 0.2
  workspace_base: "./workspaces"
  daily_summary_hour: 0                  # 자정

  models:
    opus: "opus"                         # Claude Code 별칭 (항상 최신)
    sonnet: "sonnet"

executor:
  max_execution_time: 30m
  max_output_size: 10485760              # 10MB
  max_turns: 30                          # Claude Code --max-turns
  max_diff_lines: 2000
  max_files_changed: 20

subtask:
  max_depth: 1
  max_per_task: 5

shutdown:
  pod_grace_period: 10m
  force_kill_after: 12m

notifications:
  discord:
    webhook_url_env: "DISCORD_WEBHOOK_URL"

services: []

cleanup:
  service_metrics_raw_days: 7
  jsonl_retention_days: 30
  usage_snapshots_days: 90
  failed_worktree_hours: 24

projects:
  - name: "flux"
    type: "REPO"
    repo_url: "https://github.com/circle-oo/flux"

logging:
  level: "info"
  file: "./logs/flux.log"
```

### 3.2 Config Go 구조체

```go
package config

import (
    "os"
    "path/filepath"
    "strings"
    "time"

    "gopkg.in/yaml.v3"
)

type Config struct {
    Server        ServerConfig        `yaml:"server"`
    Database      DatabaseConfig      `yaml:"database"`
    Vault         VaultConfig         `yaml:"vault"`
    GitHub        GitHubConfig        `yaml:"github"`
    ClaudeCode    ClaudeCodeConfig    `yaml:"claude_code"`
    CCUsage       CCUsageConfig       `yaml:"ccusage"`
    Orchestrator  OrchestratorConfig  `yaml:"orchestrator"`
    Executor      ExecutorConfig      `yaml:"executor"`
    Subtask       SubtaskConfig       `yaml:"subtask"`
    Shutdown      ShutdownConfig      `yaml:"shutdown"`
    Notifications NotificationsConfig `yaml:"notifications"`
    Services      []ServiceConfig     `yaml:"services"`
    Cleanup       CleanupConfig       `yaml:"cleanup"`
    Projects      []ProjectSeed       `yaml:"projects"`
    Logging       LoggingConfig       `yaml:"logging"`
}

type ServerConfig struct {
    Port int        `yaml:"port"`
    Auth AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
    Enabled       bool   `yaml:"enabled"`
    PasswordEnv   string `yaml:"password_env"`
    SessionExpiry string `yaml:"session_expiry"`
    // 런타임에 환경변수에서 로드
    Password string `yaml:"-"`
}

type DatabaseConfig struct {
    Path                string `yaml:"path"`
    BackupDir           string `yaml:"backup_dir"`
    BackupCron          string `yaml:"backup_cron"`
    BackupRetentionDays int    `yaml:"backup_retention_days"`
}

type VaultConfig struct {
    Path string `yaml:"path"`
}

type GitHubConfig struct {
    Username          string `yaml:"username"`
    TokenEnv          string `yaml:"token_env"`
    AutoCreateRepo    bool   `yaml:"auto_create_repo"`
    DefaultVisibility string `yaml:"default_visibility"`
    // 런타임에 환경변수에서 로드
    Token string `yaml:"-"`
}

type ClaudeCodeConfig struct {
    Plan string `yaml:"plan"`
}

type CCUsageConfig struct {
    Command            string        `yaml:"command"`
    CollectionInterval time.Duration `yaml:"collection_interval"`
}

type OrchestratorConfig struct {
    CheckInterval    time.Duration `yaml:"check_interval"`
    ScaleCooldown    time.Duration `yaml:"scale_cooldown"`
    MaxTotalPods     int           `yaml:"max_total_pods"`
    MinResearchRatio float64       `yaml:"min_research_ratio"`
    WorkspaceBase    string        `yaml:"workspace_base"`
    DailySummaryHour int           `yaml:"daily_summary_hour"`
    Models           ModelsConfig  `yaml:"models"`
}

type ModelsConfig struct {
    Opus   string `yaml:"opus"`
    Sonnet string `yaml:"sonnet"`
}

type ExecutorConfig struct {
    MaxExecutionTime time.Duration `yaml:"max_execution_time"`
    MaxOutputSize    int64         `yaml:"max_output_size"`
    MaxTurns         int           `yaml:"max_turns"`
    MaxDiffLines     int           `yaml:"max_diff_lines"`
    MaxFilesChanged  int           `yaml:"max_files_changed"`
}

type SubtaskConfig struct {
    MaxDepth   int `yaml:"max_depth"`
    MaxPerTask int `yaml:"max_per_task"`
}

type ShutdownConfig struct {
    PodGracePeriod time.Duration `yaml:"pod_grace_period"`
    ForceKillAfter time.Duration `yaml:"force_kill_after"`
}

type NotificationsConfig struct {
    Discord DiscordConfig `yaml:"discord"`
}

type DiscordConfig struct {
    WebhookURLEnv string `yaml:"webhook_url_env"`
    // 런타임에 환경변수에서 로드
    WebhookURL string `yaml:"-"`
}

type ServiceConfig struct {
    Name          string        `yaml:"name"`
    Endpoint      string        `yaml:"endpoint"`
    ProjectID     string        `yaml:"project_id"`
    CheckInterval time.Duration `yaml:"check_interval"`
}

type CleanupConfig struct {
    ServiceMetricsRawDays int `yaml:"service_metrics_raw_days"`
    JSONLRetentionDays    int `yaml:"jsonl_retention_days"`
    UsageSnapshotsDays    int `yaml:"usage_snapshots_days"`
    FailedWorktreeHours   int `yaml:"failed_worktree_hours"`
}

type ProjectSeed struct {
    Name    string `yaml:"name"`
    Type    string `yaml:"type"`
    RepoURL string `yaml:"repo_url"`
}

type LoggingConfig struct {
    Level string `yaml:"level"`
    File  string `yaml:"file"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("설정 파일 읽기 실패: %w", err)
    }

    var cfg Config
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return nil, fmt.Errorf("설정 파싱 실패: %w", err)
    }

    // 환경변수 해석
    cfg.GitHub.Token = os.Getenv(cfg.GitHub.TokenEnv)
    cfg.Notifications.Discord.WebhookURL = os.Getenv(cfg.Notifications.Discord.WebhookURLEnv)
    cfg.Server.Auth.Password = os.Getenv(cfg.Server.Auth.PasswordEnv)

    // ~ 경로 확장
    if strings.HasPrefix(cfg.Vault.Path, "~") {
        home, _ := os.UserHomeDir()
        cfg.Vault.Path = filepath.Join(home, cfg.Vault.Path[2:])
    }

    return &cfg, nil
}
```

---

> **참고**: 이 문서의 4~25번 섹션은 영문 Agent 시방서(`spec-agent-en.md`)와 동일한 구조를 한국어로 제공합니다.
> 전체 SQL 스키마, Go 구조체 정의, 상태 전이 로직, 내부 API 핸들러, Claude Code CLI 래퍼,
> Vault Writer, GitHub 클라이언트, Executor 메인 루프, 서브태스크 분해, Worktree 관리,
> PR 리뷰 플로우, Orchestrator 루프, ScaleManager, 부트스트랩 시퀀스, 자기 개선 로직,
> 에러 복구, Graceful Shutdown 코드는 영문 Agent 시방서를 참조하세요.
> 코드는 언어에 관계없이 동일합니다.

---

## 4. 데이터베이스 스키마

```sql
-- 컨벤션: 모든 optional TEXT 필드는 빈 문자열('') 기본값.
-- NULL 사용하지 않음. 조회 시 WHERE field != '' 로 통일.
-- JSON 배열 필드는 '[]' 기본값.

PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS goals (
    id           TEXT PRIMARY KEY,          -- UUID v4
    title        TEXT NOT NULL,
    description  TEXT DEFAULT '',
    priorities   TEXT DEFAULT '[]',         -- JSON 문자열 배열
    metrics      TEXT DEFAULT '[]',         -- JSON 문자열 배열
    status       TEXT NOT NULL DEFAULT 'PROPOSED',  -- PROPOSED, ACTIVE, COMPLETED, SUPERSEDED
    source       TEXT NOT NULL DEFAULT 'ORCHESTRATOR', -- OPERATOR, ORCHESTRATOR
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    active_since DATETIME DEFAULT ''
);

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT PRIMARY KEY,           -- UUID v4
    name        TEXT NOT NULL UNIQUE,
    type        TEXT NOT NULL,              -- REPO, SERVICE, LIBRARY, TOOL
    repo_url    TEXT DEFAULT '',
    description TEXT DEFAULT '',
    vault_path  TEXT DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'ACTIVE', -- PROPOSED, ACTIVE, ARCHIVED, REJECTED
    tech_stack  TEXT DEFAULT '[]',          -- JSON 문자열 배열
    inspiration TEXT DEFAULT '',
    goal_id     TEXT DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tasks (
    id              TEXT PRIMARY KEY,       -- UUID v4
    title           TEXT NOT NULL,
    description     TEXT DEFAULT '',
    type            TEXT NOT NULL,          -- CODING, RESEARCH, DOCUMENT, MAINTENANCE, DEPLOY, BUGFIX, PLANNING
    status          TEXT NOT NULL DEFAULT 'PENDING', -- PENDING, READY, RUNNING, COMPLETED, FAILED, RETRY, ARCHIVED
    priority        INTEGER NOT NULL DEFAULT 50,     -- 1=최고, 100=최저
    source          TEXT NOT NULL DEFAULT 'SYSTEM',  -- OPERATOR, RESEARCHER, SELF, SYSTEM
    project_id      TEXT DEFAULT '',
    parent_id       TEXT DEFAULT '',        -- 서브태스크의 부모 ID
    depth           INTEGER NOT NULL DEFAULT 0,      -- 0: 루트, 1: 서브태스크, 2+: Manager 거부
    alert_id        TEXT DEFAULT '',
    goal_id         TEXT DEFAULT '',
    depends_on      TEXT DEFAULT '[]',      -- JSON 태스크 ID 배열
    tags            TEXT DEFAULT '[]',      -- JSON 문자열 배열
    prompt          TEXT DEFAULT '',        -- Claude Code에 보낸 프롬프트
    result          TEXT DEFAULT '',        -- Claude Code 출력 요약
    error_log       TEXT DEFAULT '',        -- 실패 원인 (stderr, 테스트 로그, "cancelled by operator" 등)
    executor_id     TEXT DEFAULT '',        -- 실행한 Pod ID
    model           TEXT DEFAULT 'sonnet',  -- sonnet, opus
    branch_name     TEXT DEFAULT '',
    pr_url          TEXT DEFAULT '',
    pr_status       TEXT DEFAULT '',        -- OPEN, APPROVED, CHANGES_REQUESTED, MERGED
    diff_lines      INTEGER DEFAULT 0,
    files_changed   INTEGER DEFAULT 0,
    test_passed     BOOLEAN DEFAULT NULL,
    retry_count     INTEGER DEFAULT 0,
    crash_recovery  BOOLEAN DEFAULT FALSE,  -- TRUE = 크래시 복구 RETRY (retry_count 미소진)
    tokens_used     INTEGER DEFAULT 0,
    cost_usd        REAL DEFAULT 0,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    started_at      DATETIME DEFAULT '',
    completed_at    DATETIME DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_tasks_status_priority ON tasks(status, priority);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_pr_status ON tasks(pr_status);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);

CREATE TABLE IF NOT EXISTS alerts (
    id           TEXT PRIMARY KEY,
    service_name TEXT NOT NULL,
    severity     TEXT NOT NULL,             -- CRITICAL, HIGH, MEDIUM, LOW
    type         TEXT NOT NULL,             -- HEALTH_CHECK, LATENCY, ERROR_RATE
    message      TEXT DEFAULT '',
    task_id      TEXT DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'ACTIVE',
    created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
    resolved_at  DATETIME DEFAULT ''
);

CREATE TABLE IF NOT EXISTS usage_snapshots (
    id            TEXT PRIMARY KEY,
    type          TEXT NOT NULL,            -- HOURLY, BLOCKS, DAILY_SUMMARY
    data          TEXT NOT NULL,            -- ccusage --json 원본
    total_tokens  INTEGER DEFAULT 0,
    total_cost    REAL DEFAULT 0,
    recorded_at   DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_snapshots_type_time ON usage_snapshots(type, recorded_at);

CREATE TABLE IF NOT EXISTS rate_limit_events (
    id          TEXT PRIMARY KEY,
    tokens_used INTEGER DEFAULT 0,
    active_pods INTEGER DEFAULT 0,
    occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS service_metrics (
    id           TEXT PRIMARY KEY,
    service_name TEXT NOT NULL,
    latency_ms   INTEGER DEFAULT 0,
    status_code  INTEGER DEFAULT 0,
    is_healthy   BOOLEAN DEFAULT TRUE,
    recorded_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_metrics_service_time ON service_metrics(service_name, recorded_at);
```

---

## 5~22. 핵심 구현 상세

> **구현 코드 참조**: 5번(핵심 타입)부터 22번(Graceful Shutdown)까지의 전체 Go 구현 코드는
> 영문 Agent 시방서(`spec-agent-en.md`)의 동일 섹션을 참조하세요.
> 코드는 동일하며, 주석과 설명만 한국어/영어 차이가 있습니다.
>
> 핵심 포인트 요약:

### 상태 전이 규칙 (섹션 6)

- PENDING→READY→RUNNING→COMPLETED/FAILED
- FAILED→RETRY (retry_count < 3 이고 취소가 아닐 때만)
- 크래시 복구 RETRY: `crash_recovery=true`, retry_count 미증가
- 취소: RUNNING→FAILED + error_log="cancelled by operator"

### 내부 통신 (섹션 7)

- `POST /internal/tasks/next` — Pod이 다음 일감 요청
- `POST /internal/tasks/:id/done` — 완료 보고
- `POST /internal/subtasks` — 서브태스크 생성 (depth≤1, 최대 5개)
- `GET /internal/model/:task_id` — 모델 결정 질의
- `/internal/...` 엔드포인트는 localhost 전용, 외부 노출 금지

### Claude Code CLI 래퍼 (섹션 8)

- `claude -p "프롬프트" --cwd /worktree --model sonnet --max-turns 30 --output-format json --append-system-prompt "Goal: ..." --dangerously-skip-permissions`
- stdout/stderr 분리 캡처
- 가드레일: 30분 timeout, 10MB output, 30 max-turns
- JSON 응답 파싱: Phase 2A에서 실제 응답 확인 후 확정

### Executor Pod 메인 루프 (섹션 16)

```
일감 요청 → 모델 결정 → Goal 프롬프트 빌드 → worktree 생성/재사용
  → Claude Code 실행 (-p, 가드레일)
  → rate limit 체크 → post-execution 검증 (외부 변경 감지)
  → 서브태스크 분해 응답 확인 → QA (테스트)
  → 커밋 + diff 검사 → PR 생성 → 자동 머지 판단
  → ccusage 사용량 수집 → Vault 기록 → 완료 보고
```

### 서브태스크 분해 프롬프트

모든 Executor 프롬프트에 포함:

```
이 태스크가 한 번에 완료하기 어려울 정도로 크다면,
코드를 작성하지 말고 분해 계획만 JSON으로 출력하라:
{"decompose": true, "subtasks": [{"title": "...", "description": "..."}, ...]}
최대 5개. 각각 독립적으로 완료 가능해야 함.
```

### Worktree 정리 정책

- COMPLETED + PR MERGED → 즉시 삭제
- COMPLETED + PR 리뷰 대기 → 머지/거절까지 보존
- FAILED → 24시간 보존 후 삭제
- CHANGES_REQUESTED → 기존 worktree에서 수정 (새로 만들지 않음)

### launchd plist (Phase 2B)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.circle-oo.flux</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/flux</string>
        <string>--config</string>
        <string>$FLUX_DIR/config.yaml</string>       <!-- $FLUX_DIR을 실제 경로로 교체 -->
    </array>
    <key>WorkingDirectory</key>
    <string>$FLUX_DIR</string>                       <!-- $FLUX_DIR을 실제 경로로 교체 -->
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$FLUX_DIR/logs/flux-stdout.log</string>  <!-- $FLUX_DIR을 실제 경로로 교체 -->
    <key>StandardErrorPath</key>
    <string>$FLUX_DIR/logs/flux-stderr.log</string>  <!-- $FLUX_DIR을 실제 경로로 교체 -->
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin</string>
        <key>HOME</key>
        <string>$HOME</string>                       <!-- $HOME을 실제 홈 경로로 교체 -->
    </dict>
</dict>
</plist>
```

설치: `cp com.circle-oo.flux.plist ~/Library/LaunchAgents/ && launchctl load ~/Library/LaunchAgents/com.circle-oo.flux.plist`

---

## 23. 구현 순서

### Phase 1: 기반 — "뼈대"

**목표**: Flux 부팅, Web UI로 Goal/Task/Project CRUD 가능.
**결과물**: `go build` → 단일 바이너리 → Web UI CRUD.
**Operator 수동 작업**: 전부. 이 단계에서 Flux는 자율 실행하지 않음.

| # | 항목 | 상세 |
|---|------|------|
| 1 | Go 프로젝트 초기화 + 설정 로더 | `go mod init`, YAML→struct, 환경변수 해석, ~ 경로 확장 |
| 2 | SQLite 스키마 + 부트스트랩 | WAL 모드, 전체 테이블, Vault 디렉토리 구조, 시드 프로젝트 |
| 3 | Goal/Task/Project CRUD API | `/api/goals`, `/api/tasks`, `/api/projects` 전체 CRUD |
| 4 | 내부 API 프레임워크 | `/internal/...` 엔드포인트 (스텁 구현, localhost 전용 미들웨어) |
| 5 | Discord Notifier | 웹훅 기반, 모든 Phase에서 사용 |
| 6 | GitHub 클라이언트 (레포 생성만) | `CreateRepo()` 만. PR 메서드는 Phase 2A. |
| 7 | Web UI | React+Vite+Tailwind+Go embed. 인증(bcrypt+쿠키, 만료 없음). Dashboard, Goals, Tasks, Projects. WebSocket 스텁. |

### Phase 2A: 핵심 파이프라인 — "태스크 하나가 PR까지 간다"

**목표**: Operator 태스크 등록 → Executor 코딩 → PR 생성 → 자동/수동 머지.
**결과물**: 전체 실행 파이프라인.
**Operator 수동 작업**: 태스크 등록, PR 리뷰, rate limit 수동 대응.

| # | 항목 | 상세 |
|---|------|------|
| 1 | Claude Code CLI 통합 | `-p`, `--max-turns`, `--output-format json`, `--append-system-prompt`, stderr/stdout 분리 캡처 |
| 2 | JSON 응답 파싱 전략 확정 | 실제 응답 테스트, 최소 필드 결정 |
| 3 | **Sandbox 평가** | Claude Code 네이티브 샌드박싱(`/sandbox`) 테스트. 파일시스템·네트워크 격리가 Flux 워크플로우와 호환되는지 확인. `--dangerously-skip-permissions` 유지 또는 sandbox 전환 결정. |
| 4 | Smoke Test | 부트스트랩에 추가. `claude -p "SMOKE_TEST_OK"` 검증. |
| 5 | Git worktree 관리 | Bare repo + 태스크별 worktree + 정리 정책 (PR 대기 보존, FAILED 24시간, 머지 즉시 삭제) |
| 6 | GitHub PR 클라이언트 | `CreatePR()`, `MergePR()`, `FetchPRComments()` |
| 7 | Manager 기본 | Priority Queue pop → 내부 API Pod 배정 → 상태 전이 |
| 8 | Executor Pod + 가드레일 | 30분 timeout, 10MB output, 30 max-turns, 2000줄 diff, 20파일 |
| 9 | Post-execution 검증 | worktree 외부 변경 감지 (Sandbox 평가 결과에 따라 범위 조정) |
| 10 | QA | 테스트 감지 → 실행 → 없으면 작성 → 최대 3회 재시도 |
| 11 | PR + 자동 머지 + Operator 리뷰 | 자동 머지 조건, Request Changes 플로우 (코멘트 fetch → 수정 태스크 → 기존 worktree) |
| 12 | Web UI: PRs 페이지 | PR 목록, Approve/Request Changes 버튼, GitHub 링크 |
| 13 | Rate limit 감지 **실험** | 의도적으로 rate limit 유발. exit code, stderr 패턴 관찰. 결과 기록. |
| 14 | **Agent Teams 호환성 실험** | `-p` 모드에서 `CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS` 동작 확인. `.claude/agents/` 정의 파일 테스트. |

**Phase 2A 완료 = "Flux가 Flux를 만든다" 전환점.**

### Phase 2B: 파이프라인 강화 — "안정적으로 돌아간다"

**목표**: 안전장치, 사용량 추적, 지식 기록, 무인 운영 기반.
**결과물**: rate limit 기본 대응, 모델 선택, Goal 프롬프트, Vault 기록, launchd.
**Operator 수동 작업**: 태스크 등록, PR 리뷰, Pod 수 조절. Rate limit 대기는 자동.

| # | 항목 | 상세 |
|---|------|------|
| 1 | Rate limit 감지 구현 | Phase 2A 실험 결과 반영. 패턴 확정. |
| 2 | Rate limit 기본 대응 | 감지 → 전 Pod 정지 → **고정 5시간 대기** → 재개 |
| 3 | 모델 선택 | Sonnet 기본, Opus 조건부 (NeedsOpus() 로직) |
| 4 | Goal 프롬프트 | `--append-system-prompt`로 현재 Goal 주입 |
| 5 | 서브태스크 분해 | Claude Code 자율 판단, Manager API, depth 1, 최대 5개 |
| 6 | Manager 고도화 | Goal 부스트, 의존성 체크 (depends_on) |
| 7 | Vault Writer | 싱글 고루틴 채널 기반 |
| 8 | 최소 Vault 기록 | 태스크 완료 → 요약 마크다운 → Obsidian |
| 9 | ccusage 매핑 검증 | 절대경로 인코딩 = ccusage `--project` 일치 확인 |
| 10 | 최소 Graceful Shutdown | SIGTERM → 10분 Pod 대기 → SIGKILL → 태스크 RETRY(crash_recovery) |
| 11 | launchd plist 등록 | KeepAlive — 크래시/재부팅 자동 재시작 |
| 12 | 에러 복구 기본 | Mac 재부팅 → launchd → RUNNING→RETRY → Pod 재시작 |

### Phase 3: 오케스트레이션 — "자율 운영"

**목표**: Pod 자동 관리, 사용량 추적, 매일 보고.
**결과물**: Pod 자동 스케일링, ccusage 시계열 그래프, 동적 rate limit 대기, Discord 데일리 서머리.
**Operator 수동 작업**: Goal 설정, 복잡 PR 리뷰, 새 프로젝트 승인만. 나머지 자율.

| # | 항목 |
|---|------|
| 1 | Orchestrator 프레임워크 + 하위 컴포넌트 (ScaleManager, UsageCollector, DailySummary, RateLimitHandler, GoalAdvisor) |
| 2 | ScaleManager (Pod 수, Executor:Researcher 비율, 15분 쿨다운) |
| 3 | RateLimitHandler 고도화 (ccusage blocks 동적 대기, 5시간 fallback) |
| 4 | UsageCollector (ccusage daily/blocks/instances --json) |
| 5 | 시계열 스냅샷 (매시간 → usage_snapshots DB) |
| 6 | 태스크별 사용량 (`ccusage --project {encoded-path}`) |
| 7 | DailySummary (Discord) |
| 8 | JSONL 정리 (30일 삭제) |
| 9 | 데일리 백업 (SQLite .backup + Vault tar.gz, 7일 보관) |
| 10 | Graceful Shutdown 고도화 (12분 force kill) |
| 11 | 데이터 정리 (메트릭, 스냅샷, 백업 보존 기간) |
| 12 | Usage UI (시계열 그래프) |

### Phase 4: 지식 & 자율 — "자율적으로 성장"

**목표**: Researcher 자율 리서치, 지식 축적, 자기 개선.
**결과물**: Researcher Pod 가동, Obsidian 체계적 지식, 프로젝트 제안, Flux 코드 자기 수정.
**Operator 수동 작업**: Goal 설정, 새 프로젝트 승인, 복잡 PR 리뷰만. 나머지 자율.

| # | 항목 |
|---|------|
| 1 | Researcher Pod + workspace (Pod별 독립 workspace, `--add-dir`, MCP) |
| 2 | 자율 리서치 스케줄링 |
| 3 | Vault Writer 고도화 (모든 Pod → 채널 → Obsidian) |
| 4 | 새 프로젝트 제안 → Operator 승인 → GitHub 레포 |
| 5 | 배포 자동화 (deploy.sh / Makefile deploy) |
| 6 | 자기 개선 (flux-safe 태그 + revert, 스키마 변경 제외) |
| 7 | Research UI |

---

## 24. 컨벤션 & 규칙

1. **NULL 컨벤션**: optional TEXT 필드는 `''` 기본값. NULL 사용 금지. `!= ''`로 조회.
2. **질 우선**: Haiku 금지. Pod 축소 OK, 모델 하락 NO. 최소 Sonnet.
3. **Opus 조건부**: 최근 rate limit 없음 + 복잡한 태스크. 나머지 Sonnet.
4. **QA 필수**: 코딩 태스크 테스트 필수. 없으면 작성. RESEARCH/DOCUMENT 면제.
5. **Git worktree**: 병렬 작업. 레포 Lock 불필요. PR 대기 보존. FAILED 24시간 보존.
6. **Rate limit**: 2단계 감지. Phase 2B: 고정 5시간. Phase 3: ccusage blocks 동적 대기.
7. **PR 리뷰**: 단순→자동 머지. 복잡/가드레일 초과→Operator. Changes→기존 worktree에서 수정.
8. **사용량 추적**: ccusage 위임. 절대경로 인코딩. 매시간 스냅샷.
9. **Sandbox**: Phase 2A에서 Claude Code 네이티브 sandbox 평가. 결과에 따라 결정.
10. **도구 권한**: sandbox 미사용 시 `.claude/settings.json` bypass + post-execution 검증.
11. **Claude Code 인증**: 만료 시 Discord 알림.
12. **JSON 파싱**: Phase 2A에서 실제 응답 확인 후 확정.
13. **macOS**: launchd — Phase 2B에서 등록 (KeepAlive).
14. **자기 개선**: PR OK, 재시작은 유휴 시. **스키마 변경 제외**.
15. **Goal 전파**: `Goals/_current.md` → `--append-system-prompt` 주입.
16. **Obsidian**: Vault Writer 채널 직렬 쓰기. Phase 2B부터 최소 기록.
17. **R&D 보호**: 최소 20% 리서치 (긴급 제외).
18. **Graceful Shutdown**: Phase 2B 기본(10분), Phase 3 고도화(12분). crash_recovery 구분.
19. **가드레일**: 30분 timeout, 10MB output, 30 max-turns, 2000줄 diff, 20파일. post-execution 외부 변경 감지.
20. **서브태스크**: Claude Code 자율 판단 → Manager 내부 API. depth 1, 최대 5개. 직접 DB 접근 금지.
21. **Researcher workspace**: Pod별 독립. 병렬 충돌 방지.
22. **Pod 통신**: 내부 HTTP API (`/internal/...`). 외부 노출 금지.
23. **세션**: 만료 없음. Tailscale 의존. 명시적 로그아웃만.
24. **태스크 취소**: CANCELLED 상태 없음. → FAILED + error_log "cancelled by operator".
25. **Orchestrator 구조**: ScaleManager, UsageCollector, DailySummary, RateLimitHandler, GoalAdvisor.

---

## 25. Future Work

### 25.1 원래 설계에서 미룬 기능

Monitor HTTP 헬스체크, 인시던트 리포트, Services UI, 복수 Goal+Rank, REFINING 상태, Issue 시스템, 한도 학습, 점진적 Pod 스케일링, 상세 데일리 서머리, 로그 분석, 프로세스/리소스 모니터링, Watchdog, Obsidian 대시보드, 메트릭 집계, JSONL gzip, 알림 필터, 플랜 추천, 라이브러리 분석, Research 품질 검증, DB 마이그레이션, PR 테이블 분리, Rate Limit 3순위 감지.

### 25.2 경쟁 분석에서 도출

실행 환경 격리(Sandbox), 코드 리뷰 피드백 학습, 외부 이벤트 트리거(GitHub webhook, Discord 봇), CI/CD 통합, 멀티 리포 태스크, 세션 지속성/컨텍스트 공유, 모바일 접근성.

### 25.3 oh-my-claudecode(OMC) 참고

에이전트 정의 패턴(`.claude/agents/`), CLAUDE.md 자동 갱신(3티어 노트패드), 네이티브 Agent Teams 활용(서브에이전트 병렬 실행).

**중요**: OMC는 인터랙티브 세션용 오케스트레이터. Flux 플러그인으로 설치하지 말 것. 아이디어만 차용. Flux는 Claude Code **밖에서** 오케스트레이션; OMC는 Claude Code **안에서** 오케스트레이션. 겹치면 이중 오케스트레이션 충돌.
