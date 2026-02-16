# Flux — Obsidian CLI Integration & Knowledge UI/UX Plan

> 공식 Obsidian CLI (1.12+) 기반 지식 시스템 통합 + Web UI

## 요약

이 문서는 세 가지를 정의한다:

1. **Obsidian CLI 통합**: 기존 `internal/vault/writer.go` (파일 I/O 기반 VaultWriter)를 공식 Obsidian CLI로 교체/확장
2. **Knowledge Backend API**: Vault 데이터를 Web UI에 노출하는 API 레이어
3. **Knowledge UI/UX**: Vault 콘텐츠 탐색, 검색, 리서치 이력, 대시보드를 포함한 프론트엔드

Phase 2B (Vault Writer)와 Phase 4 (Task 4.3 Vault Writer 고도화, Task 4.7 Research UI)를 Obsidian CLI 기반으로 재설계하며, Knowledge 전용 UI를 추가한다.

---

## 1. 공식 Obsidian CLI 개요

### Obsidian 1.12 (2026-02-10 릴리즈)

```
obsidian <command> [param=value ...] [flag ...]

COMMANDS:
  files         파일 CRUD (read, write, list)
  search        전문 검색 (Obsidian 인덱스 기반)
  properties    frontmatter CRUD
  tags          태그 조회/필터
  tasks         체크박스 태스크 관리
  links         아웃링크
  backlinks     백링크
  orphans       고아 노트
  unresolved    깨진 링크
  deadends      아웃링크 없는 노트
  daily         데일리 노트
  templates     템플릿 적용
  outline       헤딩 구조
  dev           JavaScript 실행 (dev:eval)
  vault         Vault 정보
```

### 요구사항

| 항목 | 내용 |
|------|------|
| Obsidian 앱 | v1.12+ 백그라운드 실행 (LaunchAgent) |
| Catalyst License | $25 일회성 (Early Access, 추후 무료) |
| CLI 활성화 | Settings → Command line interface → ON |

### 이점 (기존 VaultWriter 대비)

| 역량 | 기존 VaultWriter | Obsidian CLI |
|------|-----------------|-------------|
| 파일 쓰기 | ✅ atomic write | ✅ `obsidian create/append/prepend` |
| 파일 읽기 | ✅ 직접 파일 읽기 | ✅ `obsidian read` |
| 전문 검색 | ❌ (grep 수준) | ✅ Obsidian 인덱스 기반 |
| Frontmatter | ❌ | ✅ `obsidian property:set/read` |
| 백링크/링크 | ❌ | ✅ `obsidian backlinks/links` |
| 고아 노트 | ❌ | ✅ `obsidian orphans` |
| 태그 관리 | ❌ | ✅ `obsidian tags all counts` |
| 태스크 관리 | ❌ | ✅ `obsidian tasks all todo/done` |
| 데일리 노트 | ❌ | ✅ `obsidian daily/daily:append` |
| 템플릿 렌더링 | ✅ Go template | ✅ `obsidian create template=` (Obsidian 네이티브) |
| 헤딩 구조 | ❌ | ✅ `obsidian outline` |

---

## 2. 아키텍처 변경

### Before (현재 구현)

```
Flux Process
  └── internal/vault/
        ├── writer.go       // channel → atomicWrite (os.WriteFile)
        └── templates.go    // Go text/template → markdown
```

### After (CLI 통합)

```
┌──────────────────────────────────────────────────────┐
│  Mac Mini                                             │
│                                                        │
│  ┌──────────────┐     IPC     ┌──────────────────┐   │
│  │ Obsidian App  │◄───────────│  obsidian CLI     │   │
│  │ (백그라운드)   │            └────────▲─────────┘   │
│  │ • 인덱싱      │                     │              │
│  │ • 검색 엔진   │            ┌────────┴─────────┐   │
│  │ • 링크 그래프 │            │  Flux Process     │   │
│  └──────┬───────┘            │                   │   │
│         ▼                     │  internal/vault/  │   │
│  ~/ObsidianVault/Flux/       │  ├── client.go    │   │
│                               │  ├── writer.go   │   │
│                               │  └── fallback.go │   │
│                               │                   │   │
│                               │  internal/server/ │   │
│                               │  └── api_knowledge.go│ │
│                               │                   │   │
│                               │  frontend/        │   │
│                               │  └── pages/       │   │
│                               │      └── Knowledge.tsx│ │
│                               └──────────────────┘   │
└──────────────────────────────────────────────────────┘
```

---

## 3. Go 패키지: `internal/vault`

### 3.1 `client.go` — Obsidian CLI 래퍼 (신규)

```go
package vault

import (
    "context"
    "encoding/json"
    "fmt"
    "os/exec"
    "strings"
    "time"
)

// Client wraps the official Obsidian CLI (1.12+).
// Requires Obsidian app running in background.
type Client struct {
    vaultName string
    timeout   time.Duration
    available bool // CLI health check result
}

func NewClient(vaultName string) *Client {
    c := &Client{vaultName: vaultName, timeout: 30 * time.Second}
    c.available = c.ping() == nil
    return c
}

func (c *Client) exec(ctx context.Context, args ...string) (string, error) {
    if c.vaultName != "" {
        args = append([]string{fmt.Sprintf(`vault=%s`, c.vaultName)}, args...)
    }
    ctx, cancel := context.WithTimeout(ctx, c.timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, "obsidian", args...)
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("obsidian %s: %w", strings.Join(args, " "), err)
    }
    return strings.TrimSpace(string(out)), nil
}

// IsAvailable returns whether Obsidian CLI is connected.
func (c *Client) IsAvailable() bool { return c.available }

// --- Read/Write ---
func (c *Client) Read(ctx context.Context, path string) (string, error)
func (c *Client) Create(ctx context.Context, name, content string) error // always adds "silent"
func (c *Client) CreateFromTemplate(ctx context.Context, name, template string) error
func (c *Client) Append(ctx context.Context, path, content string) error
func (c *Client) Prepend(ctx context.Context, path, content string) error
func (c *Client) Move(ctx context.Context, from, to string) error
func (c *Client) Delete(ctx context.Context, path string) error

// --- Search ---
type SearchResult struct {
    File    string        `json:"file"`
    Matches []SearchMatch `json:"matches"`
}
type SearchMatch struct {
    Line int    `json:"line"`
    Text string `json:"text"`
}
func (c *Client) Search(ctx context.Context, query string, folder string, limit int) ([]SearchResult, error)

// --- Properties (Frontmatter) ---
func (c *Client) PropertySet(ctx context.Context, path, key, value string) error
func (c *Client) PropertyRead(ctx context.Context, path, key string) (string, error)
func (c *Client) Properties(ctx context.Context, path string) (map[string]string, error)

// --- Tags ---
func (c *Client) TagsAll(ctx context.Context) (map[string]int, error)
func (c *Client) FilesByTag(ctx context.Context, tag string) ([]string, error)

// --- Tasks ---
func (c *Client) TasksAllTodo(ctx context.Context) (string, error)
func (c *Client) TaskDone(ctx context.Context, ref string) error

// --- Links & Graph ---
func (c *Client) Backlinks(ctx context.Context, path string) ([]string, error)
func (c *Client) Links(ctx context.Context, path string) ([]string, error)
func (c *Client) Orphans(ctx context.Context) ([]string, error)
func (c *Client) Unresolved(ctx context.Context) ([]string, error)

// --- Daily Notes ---
func (c *Client) DailyRead(ctx context.Context) (string, error)
func (c *Client) DailyAppend(ctx context.Context, content string) error
func (c *Client) DailyPrepend(ctx context.Context, content string) error

// --- Structure ---
func (c *Client) ListFiles(ctx context.Context, folder string) ([]string, error)
func (c *Client) Folders(ctx context.Context) ([]string, error)
func (c *Client) Outline(ctx context.Context, path string) (string, error)
func (c *Client) VaultInfo(ctx context.Context) (string, error)

// --- Diagnostics ---
func (c *Client) ping() error {
    _, err := c.exec(context.Background(), "vault")
    return err
}
```

### 3.2 `writer.go` — 기존 VaultWriter 유지 (Fallback)

기존 `vault/writer.go` (`Writer` + channel 기반)는 CLI 불가 시 fallback으로 유지. `client.go`가 1차, `writer.go`가 2차.

```go
// Facade: CLI 우선, 실패 시 Writer fallback
type VaultFacade struct {
    cli      *Client
    writer   *Writer  // 기존 channel-based writer
    mu       sync.Mutex // DailyNote 동시 접근 보호
}

func NewFacade(vaultName string, basePath string) *VaultFacade {
    return &VaultFacade{
        cli:    NewClient(vaultName),
        writer: NewWriter(basePath),
    }
}

func (f *VaultFacade) Write(ctx context.Context, path, content string, mode WriteMode) error {
    if f.cli.IsAvailable() {
        switch mode {
        case ModeCreate:
            return f.cli.Create(ctx, path, content)
        case ModeAppend:
            return f.cli.Append(ctx, path, content)
        case ModeReplace:
            // CLI에는 explicit replace가 없으므로 create with overwrite
            return f.cli.Create(ctx, path, content) // overwrite flag 추가
        }
    }
    // Fallback to file-based writer
    return f.writer.Write(path, content, mode)
}

func (f *VaultFacade) DailyAppend(ctx context.Context, content string) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    if f.cli.IsAvailable() {
        return f.cli.DailyAppend(ctx, content)
    }
    // Fallback: 직접 daily note 경로 계산 + file append
    return f.writer.Write(dailyNotePath(), content, ModeAppend)
}
```

### 3.3 `fallback.go` — CLI 불가 시 최소 기능 (신규)

```go
package vault

// FallbackSearch uses grep when CLI is unavailable
func FallbackSearch(basePath, query string) ([]SearchResult, error)

// FallbackProperties parses YAML frontmatter directly
func FallbackProperties(basePath, path string) (map[string]string, error)

// FallbackTagsFromGrep finds tags via grep
func FallbackTagsFromGrep(basePath string) (map[string]int, error)
```

---

## 4. Knowledge Backend API: `internal/server/api_knowledge.go`

기존 API 패턴 (`api_tasks.go`, `api_projects.go`)을 따른다.

### 4.1 API 엔드포인트

```
GET  /api/knowledge/search?q={query}&folder={folder}&limit={n}
GET  /api/knowledge/note?path={path}
GET  /api/knowledge/outline?path={path}
GET  /api/knowledge/tags
GET  /api/knowledge/tags/{tag}/files
GET  /api/knowledge/backlinks?path={path}
GET  /api/knowledge/links?path={path}
GET  /api/knowledge/orphans
GET  /api/knowledge/unresolved
GET  /api/knowledge/files?folder={folder}
GET  /api/knowledge/folders
GET  /api/knowledge/stats
GET  /api/knowledge/daily
GET  /api/knowledge/research/history
GET  /api/knowledge/research/stats
```

### 4.2 구현

```go
package server

func (s *Server) registerKnowledgeRoutes() {
    r := s.router.Group("/api/knowledge")
    r.Use(s.authMiddleware)

    r.GET("/search", s.knowledgeSearch)
    r.GET("/note", s.knowledgeGetNote)
    r.GET("/outline", s.knowledgeOutline)
    r.GET("/tags", s.knowledgeTags)
    r.GET("/tags/:tag/files", s.knowledgeTagFiles)
    r.GET("/backlinks", s.knowledgeBacklinks)
    r.GET("/links", s.knowledgeLinks)
    r.GET("/orphans", s.knowledgeOrphans)
    r.GET("/unresolved", s.knowledgeUnresolved)
    r.GET("/files", s.knowledgeFiles)
    r.GET("/folders", s.knowledgeFolders)
    r.GET("/stats", s.knowledgeStats)
    r.GET("/daily", s.knowledgeDaily)
    r.GET("/research/history", s.knowledgeResearchHistory)
    r.GET("/research/stats", s.knowledgeResearchStats)
}

func (s *Server) knowledgeSearch(c *gin.Context) {
    q := c.Query("q")
    folder := c.DefaultQuery("folder", "")
    limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

    results, err := s.vault.Search(c.Request.Context(), q, folder, limit)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, results)
}

func (s *Server) knowledgeGetNote(c *gin.Context) {
    path := c.Query("path")
    content, err := s.vault.Read(c.Request.Context(), path)
    if err != nil {
        c.JSON(404, gin.H{"error": err.Error()})
        return
    }
    // Frontmatter도 함께 반환
    props, _ := s.vault.Properties(c.Request.Context(), path)
    backlinks, _ := s.vault.Backlinks(c.Request.Context(), path)
    outlinks, _ := s.vault.Links(c.Request.Context(), path)

    c.JSON(200, gin.H{
        "path":       path,
        "content":    content,
        "properties": props,
        "backlinks":  backlinks,
        "outlinks":   outlinks,
    })
}

func (s *Server) knowledgeStats(c *gin.Context) {
    info, _ := s.vault.VaultInfo(c.Request.Context())
    tags, _ := s.vault.TagsAll(c.Request.Context())
    orphans, _ := s.vault.Orphans(c.Request.Context())
    unresolved, _ := s.vault.Unresolved(c.Request.Context())

    c.JSON(200, gin.H{
        "vault_info":      info,
        "total_tags":      len(tags),
        "top_tags":        topN(tags, 20),
        "orphan_count":    len(orphans),
        "unresolved_count": len(unresolved),
    })
}

// knowledgeResearchStats는 tasks 테이블의 type=RESEARCH 통계.
// Phase 4 Task 4.7에서 정의된 것과 동일 — 별도 테이블 불필요.
func (s *Server) knowledgeResearchStats(c *gin.Context) {
    // SELECT type_tag, COUNT(*) FROM tasks WHERE type='RESEARCH' GROUP BY type_tag
    stats, err := s.db.ResearchTypeDistribution()
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, stats)
}
```

---

## 5. Knowledge UI/UX

### 5.1 Navigation 구조

기존 페이지 (`Dashboard`, `Goals`, `Tasks`, `PRs`, `Projects`, `Settings`)에 추가:

```
기존:
  Dashboard | Goals | Tasks | PRs | Projects | Settings

추가:
  Dashboard | Goals | Tasks | PRs | Projects | Knowledge | Settings
                                                    │
                                                    ├── Browse (파일 트리)
                                                    ├── Search (전문 검색)
                                                    ├── Research (리서치 이력)
                                                    ├── Graph (태그/링크 맵)
                                                    └── Health (고아/깨진 링크)
```

### 5.2 Knowledge 메인 페이지: `frontend/src/pages/Knowledge.tsx`

서브탭 구조. `react-router` nested routes.

```
/knowledge              → Browse (기본)
/knowledge/search       → Search
/knowledge/research     → Research History
/knowledge/graph        → Tag/Link Graph
/knowledge/health       → Vault Health
```

### 5.3 Browse 탭

**기능**: Vault 폴더/파일 트리 + 노트 내용 미리보기.

```
┌───────────────────────────────────────────────────────┐
│  Knowledge > Browse                                    │
├─────────────────────┬─────────────────────────────────┤
│  📁 Goals/          │  # Current Goal: Phase 4 완료    │
│    📄 _current.md ← │                                  │
│    📁 completed/    │  **Status**: ACTIVE              │
│    📁 proposals/    │  **Since**: 2026-02-15           │
│  📁 Projects/       │                                  │
│    📁 flux/         │  ## Description                  │
│    📁 go-mcp/       │  Researcher 자율 리서치...        │
│  📁 Research/       │                                  │
│    📁 Industry/     │  ## Priorities                   │
│    📁 Tools/        │  1. Researcher Pod 구현           │
│    📁 Ideas/        │  2. Vault Writer 고도화           │
│  📁 Tasks/          │                                  │
│    📁 completed/    │  ---                             │
│  📁 Templates/      │  **Backlinks**: 3 notes          │
│                     │  **Outlinks**: 5 notes           │
└─────────────────────┴─────────────────────────────────┘
```

**구현 파일**:
- `frontend/src/pages/knowledge/Browse.tsx`
- 좌측: 폴더 트리 (`GET /api/knowledge/folders` + `GET /api/knowledge/files?folder=`)
- 우측: 마크다운 렌더링 (`GET /api/knowledge/note?path=`) + 메타데이터 (properties, backlinks, outlinks)
- 마크다운 렌더링: `react-markdown` + `remark-gfm` + `remark-frontmatter`

### 5.4 Search 탭

**기능**: Obsidian 인덱스 기반 전문 검색. 결과 클릭 → Browse 탭에서 열기.

```
┌───────────────────────────────────────────────────────┐
│  Knowledge > Search                                    │
│                                                        │
│  🔍 [rate limit handling            ] [Search]         │
│                                                        │
│  Folder: [All ▼]   Tag: [All ▼]   Limit: [20 ▼]      │
│                                                        │
│  ── 8 results ──                                       │
│                                                        │
│  📄 Projects/flux/decisions/2026-02-13-rate-limit.md   │
│     ...rate limit 감지 → 전 Pod 정지 → 동적 대기...     │
│     Line 42: RateLimitHandler가 ccusage blocks 기반... │
│                                                        │
│  📄 Research/Tools/claude-code-rate-limits.md          │
│     ...Claude Code 정액제 플랜의 rate limit 패턴...     │
│     Line 15: 5시간 빌링 윈도우 리셋 확인...             │
│                                                        │
│  📄 Tasks/completed/2026-02-14-rate-limit-handler.md   │
│     ...rate limit handler 구현 완료...                  │
└───────────────────────────────────────────────────────┘
```

**구현 파일**:
- `frontend/src/pages/knowledge/Search.tsx`
- `GET /api/knowledge/search?q=...&folder=...&limit=...`
- 검색 결과에서 매칭 라인 스니펫 하이라이트
- 결과 클릭 → `/knowledge?path=...` (Browse 탭에서 열기)

### 5.5 Research 탭

**기능**: 리서치 이력 타임라인 + Researcher Pod 상태 + 타입별 분포.

```
┌───────────────────────────────────────────────────────┐
│  Knowledge > Research                                  │
│                                                        │
│  ┌──────────────────────┐  ┌────────────────────────┐ │
│  │ Active Researchers    │  │ Research Distribution   │ │
│  │                       │  │                         │ │
│  │ 🟢 researcher-01     │  │ industry    ████████ 12 │ │
│  │   industry-research   │  │ tools       █████   8  │ │
│  │   Running 23m         │  │ goal        ████    6  │ │
│  │                       │  │ ideas       ███     5  │ │
│  │ 🟢 researcher-02     │  │ dependency  ██      3  │ │
│  │   goal-research       │  │ opensource  █       2  │ │
│  │   Running 8m          │  │                         │ │
│  └──────────────────────┘  └────────────────────────┘ │
│                                                        │
│  ── Research Timeline ──                               │
│                                                        │
│  Today                                                 │
│  🔬 14:32  industry-research  "MCP Ecosystem 2026"     │
│            → Research/Industry/mcp-ecosystem-2026.md    │
│  🔬 11:15  goal-research      "Phase 4 dependency map" │
│            → Research/Goals/phase4-deps.md              │
│                                                        │
│  Yesterday                                             │
│  🔬 22:40  opensource-scan    "Go AI agent frameworks" │
│            → Research/Tools/go-ai-agent-frameworks.md   │
│  🔬 18:05  project-ideas     "Obsidian MCP plugin"    │
│            → Research/Ideas/obsidian-mcp-plugin.md      │
│            💡 Project Proposal Created                 │
│  🔬 15:20  dependency-check   "flux go.mod audit"     │
│            → Research/Tools/flux-deps-audit.md          │
└───────────────────────────────────────────────────────┘
```

**구현 파일**:
- `frontend/src/pages/knowledge/Research.tsx`
- `frontend/src/stores/knowledgeStore.ts`
- Researcher Pod 상태: `GET /api/pods` (기존, prefix `researcher-` 필터)
- 리서치 이력: `GET /api/tasks?type=RESEARCH&sort=created_at&order=desc` (기존 task API 재사용)
- 타입별 분포: `GET /api/knowledge/research/stats`
- Vault 링크 클릭 → Browse 탭

### 5.6 Graph 탭

**기능**: 태그 클라우드 + 간단한 링크 관계 시각화.

```
┌───────────────────────────────────────────────────────┐
│  Knowledge > Graph                                     │
│                                                        │
│  ── Tag Cloud ──                                       │
│                                                        │
│  architecture(15)  rate-limit(8)  sqlite(7)           │
│  claude-code(12)   testing(6)    go(5)                │
│  orchestrator(10)  vault(6)      mcp(4)               │
│  executor(9)       github(5)     research(3)          │
│                                                        │
│  ── Note Connections (selected: rate-limit) ──         │
│                                                        │
│  rate-limit-handler.md ←→ executor.md                  │
│  rate-limit-handler.md ←→ orchestrator.md              │
│  rate-limit-handler.md  → ccusage-mapping.md           │
│  claude-code-rate-limits.md → rate-limit-handler.md    │
│                                                        │
│  [View in Obsidian]                                    │
└───────────────────────────────────────────────────────┘
```

**구현 파일**:
- `frontend/src/pages/knowledge/Graph.tsx`
- 태그: `GET /api/knowledge/tags`
- 태그 클릭 → 해당 태그의 파일 목록 + 링크 관계
- 링크: `GET /api/knowledge/backlinks` + `GET /api/knowledge/links`
- 시각화: 간단한 리스트 형태 (Phase 4). 향후 d3.js 그래프로 업그레이드 가능.

### 5.7 Health 탭

**기능**: Vault 건강 상태 모니터링.

```
┌───────────────────────────────────────────────────────┐
│  Knowledge > Health                                    │
│                                                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐│
│  │ Total Notes   │  │ Orphan Notes │  │ Broken Links ││
│  │     142       │  │      3       │  │      1       ││
│  └──────────────┘  └──────────────┘  └──────────────┘│
│                                                        │
│  ── Orphan Notes (no incoming links) ──                │
│  ⚠️  Research/Tools/old-benchmark.md                   │
│  ⚠️  Tasks/completed/2026-02-01-init.md                │
│  ⚠️  Research/Ideas/abandoned-idea.md                  │
│                                                        │
│  ── Unresolved Links ──                                │
│  ❌  Projects/flux/architecture.md → [[monitoring]]    │
│      (monitoring.md does not exist)                    │
│                                                        │
│  ── Dead Ends (no outgoing links) ──                   │
│  📄  Templates/project.md                              │
│  📄  Templates/research.md                             │
│                                                        │
│  Last checked: 2 minutes ago  [Refresh]                │
└───────────────────────────────────────────────────────┘
```

**구현 파일**:
- `frontend/src/pages/knowledge/Health.tsx`
- `GET /api/knowledge/stats`
- `GET /api/knowledge/orphans`
- `GET /api/knowledge/unresolved`

---

## 6. Dashboard 연동

기존 `Dashboard.tsx`에 Knowledge 위젯 추가:

```
┌─ Dashboard ──────────────────────────────────────────┐
│                                                       │
│  [기존: Goal, Pods, Tasks, PRs 카드]                  │
│                                                       │
│  ── Knowledge ──                                      │
│  📝 142 notes  |  🏷️ 38 tags  |  ⚠️ 3 orphans       │
│  Latest: "MCP Ecosystem 2026" (14:32)                │
│  🔬 2 Researchers active                              │
└───────────────────────────────────────────────────────┘
```

**수정 파일**: `frontend/src/pages/Dashboard.tsx`
- Knowledge 통계 카드 추가 (클릭 → `/knowledge`)
- 최근 리서치 1건 표시

---

## 7. Zustand Store

```typescript
// frontend/src/stores/knowledgeStore.ts

interface KnowledgeState {
  // Browse
  folders: string[]
  files: string[]
  currentNote: NoteDetail | null
  currentPath: string

  // Search
  searchResults: SearchResult[]
  searchQuery: string
  searchLoading: boolean

  // Tags
  tags: Record<string, number>

  // Stats
  stats: VaultStats | null

  // Research
  researchHistory: Task[]  // type=RESEARCH tasks
  researchStats: Record<string, number>

  // Health
  orphans: string[]
  unresolved: string[]

  // Actions
  fetchFolders: () => Promise<void>
  fetchFiles: (folder: string) => Promise<void>
  fetchNote: (path: string) => Promise<void>
  search: (query: string, folder?: string, limit?: number) => Promise<void>
  fetchTags: () => Promise<void>
  fetchStats: () => Promise<void>
  fetchResearchHistory: () => Promise<void>
  fetchResearchStats: () => Promise<void>
  fetchOrphans: () => Promise<void>
  fetchUnresolved: () => Promise<void>
}
```

---

## 8. Obsidian 앱 실행 관리

### LaunchAgent

```xml
<!-- ~/Library/LaunchAgents/md.obsidian.app.plist -->
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>md.obsidian.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>open</string>
        <string>-a</string>
        <string>Obsidian</string>
    </array>
    <key>RunAtLoad</key><true/>
</dict>
</plist>
```

### 부트스트랩 연동

기존 부트스트랩 (`bootstrap.go`)에 CLI 헬스체크 추가:

```go
// 기존 부트스트랩 단계 사이에 삽입
// ... Vault 구조 생성 후 ...

// Obsidian CLI health check
vaultClient := vault.NewClient(cfg.Vault.Name)
if !vaultClient.IsAvailable() {
    notifier.Send(LevelWarning, "Obsidian CLI not available. Using file-based fallback.")
    // fallback 모드로 진행 — 검색/링크/태그 기능 비활성
} else {
    notifier.Send(LevelInfo, "Obsidian CLI connected.")
}
```

---

## 9. Claude Code 에이전트 연동

Executor/Researcher Pod의 CLAUDE.md에 CLI 가이드를 포함시킨다. Pod이 직접 `obsidian` 명령어를 실행하여 Vault를 조작한다.

### Executor Pod 프롬프트 추가

```markdown
## Obsidian Vault (Knowledge Base)

Read context before working:
  obsidian read path="Projects/{project}/_index.md"
  obsidian search query="{keyword}" path="Projects/{project}" format=json matches

Save results after completion:
  obsidian create name="Projects/{project}/learnings/{slug}" content="..." silent
  obsidian daily:append content="- ✅ {task}: {summary}"
  obsidian property:set name=status value=completed path="Tasks/{task}.md"

⚠️ Always use `silent` flag with `create`. Use `mkdir -p` before create if directory doesn't exist.
```

### Researcher Pod 프롬프트 추가

```markdown
## Obsidian Vault (Knowledge Base)

Check existing research:
  obsidian search query="{topic}" format=json matches
  obsidian tags all counts

Save research findings:
  obsidian create name="Research/{category}/{slug}" content="..." silent

Log to daily note:
  obsidian daily:append content="- 🔬 Research: {topic}"
```

---

## 10. CLI 주의사항

| 주의사항 | 래퍼에서의 대응 |
|---------|--------------|
| `create` 시 `silent` 필수 | `Client.Create()`가 항상 `silent` 추가 |
| 디렉토리 자동 생성 안 함 | `Client.Create()`에서 `os.MkdirAll()` 선행 |
| `template=` 시 경로 무시 가능 | 생성 후 search로 확인 (향후) |
| `tags all` 누락 → 현재 파일만 | 래퍼에서 `all` 기본 추가 |
| 빈 결과 = 인덱스 미준비 | 3회 재시도 (2초 간격) |
| 22.8% silent failure (exit 0) | 결과 검증 로직 |

---

## 11. 구현 로드맵

### Phase 2B 수정 (기존 Task 2B.7, 2B.8 대체)

| 태스크 | 설명 | 변경 |
|--------|------|------|
| **2B.7a** | Obsidian 앱 LaunchAgent + CLI 활성화 | **신규** |
| **2B.7b** | `vault/client.go` — Obsidian CLI 래퍼 | 기존 2B.7 (VaultWriter) **확장** |
| **2B.7c** | `vault/fallback.go` — CLI 불가 시 최소 기능 | **신규** |
| 2B.7 | `vault/writer.go` — 기존 Writer 유지 (fallback) | **유지** (변경 없음) |
| **2B.8a** | 부트스트랩에 CLI 헬스체크 추가 | **신규** |
| 2B.8 | 최소 Vault 기록 (태스크 완료 → Vault) | **유지** (CLI 또는 Writer 경유) |

### Phase 4 수정

| 태스크 | 설명 | 변경 |
|--------|------|------|
| 4.3 | Vault Writer 고도화 → **VaultFacade (CLI+Writer 통합)** | **재설계** |
| **4.7a** | `api_knowledge.go` — Knowledge API 엔드포인트 | **신규** (기존 4.7 확장) |
| **4.7b** | `knowledgeStore.ts` — Zustand store | **신규** |
| **4.7c** | `Knowledge.tsx` — Browse 탭 | **신규** (기존 4.7 대체) |
| **4.7d** | `Search.tsx` — Search 탭 | **신규** |
| **4.7e** | `Research.tsx` — Research 탭 | 기존 4.7 **확장** |
| **4.7f** | `Graph.tsx` — Tag/Link Graph 탭 | **신규** |
| **4.7g** | `Health.tsx` — Vault Health 탭 | **신규** |
| **4.7h** | `Dashboard.tsx` — Knowledge 위젯 추가 | **수정** |

### 의존성 그래프

```
Phase 2B:
  2B.7a (LaunchAgent) ─┬─→ 2B.7b (CLI Client) ─→ 2B.8a (Bootstrap)
                       └─→ 2B.7c (Fallback)        ↓
                                                  2B.8 (Minimal Vault Recording)

Phase 4:
  4.1 (Researcher Pod) ──→ 4.2 (Autonomous Research)
       │                         │
       ▼                         ▼
  4.3 (VaultFacade)         4.7e (Research UI)
       │
       ▼
  4.7a (Knowledge API)
       │
       ├──→ 4.7b (Store) ──→ 4.7c (Browse)
       │                 ├──→ 4.7d (Search)
       │                 ├──→ 4.7f (Graph)
       │                 └──→ 4.7g (Health)
       └──→ 4.7h (Dashboard widget)
```

### Critical Path

```
2B.7a → 2B.7b → 2B.8a → 2B.8 → ... → 4.3 → 4.7a → 4.7b → 4.7c/d/e/f/g
```

---

## 12. 파일 목록

| 카테고리 | 신규 파일 | 수정 파일 |
|---------|----------|----------|
| Go backend (vault) | `client.go`, `fallback.go` | `writer.go` (VaultFacade 추가) |
| Go backend (server) | `api_knowledge.go` | `server.go` (route 등록) |
| Go infra | | `bootstrap.go` (CLI check) |
| React pages | `Knowledge.tsx`, `Browse.tsx`, `Search.tsx`, `Research.tsx`, `Graph.tsx`, `Health.tsx` | `Dashboard.tsx` (위젯), `App.tsx` (route) |
| React stores | `knowledgeStore.ts` | |
| Deploy | `md.obsidian.app.plist` | |
| **합계** | **~10 파일** | **~5 파일** |

---

## 13. 설계 결정 기록

### D1: 공식 CLI 채택 + 기존 Writer 병존

기존 `vault/writer.go`를 삭제하지 않고 fallback으로 유지한다. CLI가 1차, Writer가 2차. 이유: Obsidian 앱이 죽어도 태스크 완료 기록은 계속되어야 한다.

### D2: Research UI를 Knowledge의 서브탭으로 통합

Phase 4 Task 4.7 원래 계획은 독립 Research 페이지였으나, Knowledge 페이지의 서브탭으로 통합한다. Vault 콘텐츠(Browse, Search)와 Research 이력은 같은 지식 시스템의 일부이다.

### D3: 별도 DB 테이블 없이 CLI + 기존 tasks 테이블 활용

Vault 데이터는 CLI를 통해 실시간 조회한다. 리서치 이력은 기존 `tasks` 테이블 (type=RESEARCH)을 재사용한다. Vault 메타데이터를 DB에 캐싱하지 않는다 — Obsidian 앱이 이미 인덱스를 유지하고 있으므로.

### D4: Graph 시각화는 Phase 4에서 리스트 형태, 향후 d3.js

초기 구현은 태그 클라우드 + 링크 리스트. Obsidian의 그래프 뷰를 대체하려는 것이 아니라, Web UI에서 빠르게 관계를 파악하기 위한 것이다.