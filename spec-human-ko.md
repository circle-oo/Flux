# Flux — 자율 엔지니어링 시스템

## 개요

Flux는 24/7 운영되는 **자율 엔지니어링 시스템**입니다. Operator(사용자)가 방향만 잡으면, Flux가 Claude Code 에이전트를 오케스트레이션하여 개발·QA·DevOps·R&D를 자율 수행합니다.

코딩, 테스트, 배포, 서비스 모니터링, 버그 자동 탐지/수정, 기술 리서치, 지식 축적, 새 프로젝트 발굴 및 제안까지 수행하며, 모든 지식과 문서는 Obsidian Vault로 관리됩니다.

## 시스템 구조

```
Operator (사용자)
  ├── 방향: Goal 수립, Orchestrator와 전략 논의
  ├── 승인: 새 프로젝트 승인/거절
  ├── 리뷰: PR (Approve / Request Changes + GitHub 코멘트)
  ├── 피드백: 태스크 생성 ("훈수")
  └── 알림: Discord

Flux
  ├── Orchestrator: Pod 수·모델 결정, 스케일링, 데일리 서머리
  │     ├── ScaleManager: Pod 수/비율 결정, 쿨다운 관리
  │     ├── UsageCollector: ccusage 시계열 수집, 스냅샷 DB 저장
  │     ├── DailySummary: 매일 자정 Discord 서머리 전송
  │     ├── RateLimitHandler: 감지 → 정지 → 동적 대기 → 재개
  │     └── GoalAdvisor: Goal 제안 (PROPOSED → Operator 선택)
  ├── Manager: 태스크 큐, 우선순위 배정, 상태 관리
  ├── Executor Pods: 코딩, 테스트, PR, 배포
  ├── Researcher Pods: 리서치, 지식 축적, 프로젝트 발굴
  ├── Vault Writer: Obsidian 쓰기 직렬화
  ├── Notifier: Discord 알림
  └── Obsidian Vault: 지식 저장소
```

## 핵심 철학

- **Goal 중심**: 현재 Goal이 모든 의사결정의 기반.
- **최대 자율성**: Operator는 방향만. 나머지는 Flux가 자율.
- **질 우선**: Haiku 금지. 예산 부족 시 Pod을 줄이되 최소 Sonnet.
- **QA 필수**: 테스트 통과 후 커밋. 테스트 없으면 작성. PR 기반.
- **지식 축적**: 리서치 → 지식 → 작업 → 새 지식 → 반복.
- **예산 인식**: ccusage 사용량 추적 + 속도 제한 감지.
- **간결한 설계**: Go + SQLite + Obsidian + 파일 시스템.

## 기술 스택

Go를 선택한 이유: Operator 주력 언어(자기 개선 시 Go 코드를 다룸), 프로세스 오케스트레이션(goroutine + exec.Command), 단일 바이너리(`go build` → launchd 간편), Web UI 임베드(`embed` 패키지), CGO 없는 SQLite(`modernc.org/sqlite`).

## 컨벤션

모든 optional TEXT 필드는 빈 문자열(`''`)을 기본값으로 사용. NULL 미사용. 조회 시 `WHERE field != ''`로 통일. JSON 배열 필드는 `'[]'` 기본값.

---

## 상태 머신

**태스크**: `PENDING → READY → RUNNING → COMPLETED/FAILED → ARCHIVED`. FAILED는 RETRY 가능(최대 3회). 취소 시 FAILED로 전이 + `error_log: "cancelled by operator"`.

**프로젝트**: `PROPOSED → ACTIVE → ARCHIVED` (또는 `REJECTED`).

**Goal**: `PROPOSED → ACTIVE → COMPLETED / SUPERSEDED`.

**PR**: `OPEN → APPROVED → MERGED` (또는 `CHANGES_REQUESTED → 수정 → OPEN`).

---

## 내부 통신

Pod과 Manager는 내부 HTTP API(`/internal/...`)로 통신. 현재 같은 프로세스 내 localhost 호출이지만, 향후 프로세스 분리 시 인터페이스 변경 없이 확장 가능.

외부 Web UI API(`/api/...`)와 내부 Pod API(`/internal/...`)는 prefix로 구분. `/internal/...`은 외부 노출 금지.

---

## 연동

### GitHub

레포 자동 생성, PR 생성/머지, 코멘트 1회 fetch. 폴링 없음.

### Claude Code (정액제)

CLI non-interactive 모드(`-p` 플래그) 사용. 주요 플래그: `--max-turns 30`, `--output-format json`, `--append-system-prompt`(Goal 주입), `--dangerously-skip-permissions`(Phase 2A sandbox 평가 후 결정), `--model sonnet/opus`.

### Discord

웹훅 기반 알림. 대상: 서비스 장애(CRITICAL/HIGH), 프로젝트 제안, 플랜 한도, Goal 제안, PR 리뷰 요청, 태스크 실패, 인증 만료, 데일리 서머리.

---

## Goal 시스템

현재 ACTIVE Goal 1개. 모든 Pod은 작업 전 Goal을 읽어 `--append-system-prompt`로 Claude Code에 전달. Operator 직접 설정 → 즉시 ACTIVE. Orchestrator(GoalAdvisor) 제안 → PROPOSED → Operator 승인. Goal은 `Goals/_current.md`에 저장.

---

## 사용량 추적 (ccusage)

ccusage에 전적으로 위임. Flux는 JSONL을 직접 파싱하지 않음. 각 태스크는 별도 Git worktree에서 실행되어 Claude Code가 독립 프로젝트로 인식. Flux는 worktree 절대경로를 인코딩하여 ccusage 프로젝트명을 미리 계산. UsageCollector가 매시간 스냅샷을 DB에 축적. 30일 경과 JSONL 원본은 삭제(DB 스냅샷으로 이력 보존).

---

## 속도 제한 대응

2단계 감지: (1) exit code 429, (2) stderr 패턴 매칭. Phase 2A에서 실험으로 동작 패턴 확인.

**Phase 2B**: 기본 대응 — 감지 → 전 Pod 정지 → 고정 5시간 대기 → 재개.

**Phase 3**: 동적 대기 — `ccusage blocks --json`으로 정확한 리셋 시점 확인. 조회 실패 시 5시간 fallback.

한도를 "추정"하려는 시도는 하지 않음. 단순 정지/재개 방식.

---

## 모델 선택

기본 Sonnet. Opus는 최근 rate limit 없음 + 복잡한 태스크(긴급, Operator 복잡 일감, 새 프로젝트, Goal 전략)에만. 예산 부족 → Pod 줄이기, 모델 하락 금지.

---

## QA & 브랜치 전략

코딩 태스크 테스트 필수. 없으면 작성. RESEARCH/DOCUMENT 면제. 최대 3회 재시도.

Git worktree로 병렬 작업. bare repo + 태스크별 worktree. 레포 Lock 불필요.

**정리 정책**: COMPLETED+MERGED → 즉시 삭제. PR 리뷰 대기 → 보존. FAILED → 24시간 보존. CHANGES_REQUESTED → 기존 worktree에서 수정.

**외부 변경 감지**: Post-execution 검증으로 worktree 외부 변경 스캔. 감지 시 FAILED + Discord 알림.

---

## PR 리뷰

**자동 머지 조건**: 시스템/자체 태스크, 유지보수, 저우선순위 버그, 소규모 PR(≤3파일, <100줄). 가드레일 초과(>2000줄 or >20파일)는 Operator 리뷰 강제.

**Operator 리뷰**: Web UI + Discord 알림 → Approve(머지 + worktree 삭제) 또는 Request Changes(GitHub 코멘트 1회 fetch → 수정 태스크 생성 P:6 → 기존 worktree에서 수정 → 같은 PR 푸시). GitHub API 폴링 없음.

---

## 오케스트레이션

### Orchestrator 구조

5개 하위 컴포넌트 조율: ScaleManager(Pod 수/비율), UsageCollector(ccusage 스냅샷), DailySummary(자정 Discord), RateLimitHandler(감지 → 정지 → 동적 대기 → 재개), GoalAdvisor(Goal 제안).

### Orchestrator vs Manager

**Orchestrator — "얼마나"**: Pod 수, 모델 선택, rate limit 대응, 사용량 수집, 데일리 서머리, Goal 제안.

**Manager — "무엇을"**: 태스크 큐(Priority Queue), 일감 배정(최고 우선순위 pop), 서브태스크 생성 API(depth 1, 최대 5개), 상태 전이, Goal 부스트, 의존성 체크.

### Pod 스케일링

정상: `max_total_pods`까지 가동. Rate limit: 전부 정지 → 빌링 윈도우 리셋 → 재개.

**Executor:Researcher 비율**: 긴급 Operator 일감(9:1) → 보통(8:2) → 시스템만(7:3) → 큐 거의 비어있음(3:7) → 큐 비어있음(0:10) → 서비스 장애(10:0). R&D 보호: 최소 20%. 쿨다운: 15분.

---

## 핵심 컴포넌트

### Executor Pod

전체 파이프라인: 일감 요청 → 모델 결정 → Goal+지식 로드 → worktree → Claude Code(-p, 가드레일) → post-execution 검증 → QA → 커밋 → diff 검사 → PR → 자동 머지/Operator 리뷰 → ccusage 추적 → worktree 정리.

**가드레일**: 30분 timeout, 10MB output, 30 max-turns, 2000줄 diff / 20파일 제한.

**서브태스크 분해**: Claude Code 자율 판단(프롬프트 지시). 너무 크면 분해 계획 JSON 출력 → Executor 파싱 → Manager API로 생성. depth 1 제한, 태스크당 최대 5개.

### Researcher Pod

리서치 타입과 스케줄링 자율 판단. github-scan, dependency-check, industry-research, project-ideas 등. Pod별 독립 workspace로 병렬 충돌 방지.

### Manager

우선순위: 서비스 장애(P:1-5) > Operator 일감(P:6-20) > 유지보수(P:21-40) > 개선(P:41-60) > 리서치 파생(P:61-80) > 새 프로젝트(P:81-100). Goal 관련 태스크 부스트.

### Vault Writer

싱글 고루틴 채널 기반 순차 처리. 파일 락 불필요. ms 단위 쓰기로 병목 없음.

### Obsidian Vault

`~/ObsidianVault/Flux/` 하위: Goals/(현재, 완료, 제안), Projects/{이름}/(인덱스, 아키텍처, 결정, 학습), Research/(Industry, Tools, Ideas), Tasks/completed/, Templates/.

---

## 자율 운영

| 행동 | 자율 | Operator 승인 |
|------|:---:|:---:|
| Goal 제안 | ✅ | |
| **Goal 확정** | | ✅ |
| 태스크 생성/코딩/커밋 | ✅ | |
| 단순 PR 자동 머지 | ✅ | |
| 복잡 PR / 가드레일 초과 PR | | ✅ |
| **새 프로젝트** | | ✅ |
| Flux 자기 개선 + PR | ✅ | |
| **Flux 재시작** | ⏰ 유휴 시 | |
| 리서치/문서 | ✅ | |
| **플랜 변경** | 제안만 | ✅ |

---

## 부트스트랩

DB 없음 감지 → SQLite 스키마(WAL) → Obsidian Vault 구조 → Notifier 시작 → Flux 프로젝트 등록 → Claude Code 인증 확인 → Smoke Test → ccusage 확인(경고만) → Web UI 시작 → Discord: "초기화 완료. Goal 설정해주세요." → Operator Goal 설정 → Pod 시작(2-3개).

---

## 자기 개선

개선점 발견 → worktree → `flux-safe-{ts}` 태그 → 수정 + 전체 테스트 → 통과: PR + 자동 머지(재시작은 유휴 시) / 실패: `git revert` + FAILED. **DB 스키마 변경은 범위 제외.**

---

## 에러 복구

Mac 재부팅 → launchd → flux → WAL SQLite 복구 → RUNNING 태스크 RETRY(`crash_recovery=true`, retry_count 미증가) → Pod 재시작 → Discord 알림.

**크래시 복구 vs 실행 실패**: 크래시 복구 RETRY는 `crash_recovery=true`로 표시되며 retry_count를 소진하지 않음. 실행 실패만 카운트.

매일 새벽 4시 백업: SQLite `.backup` + Vault `tar.gz`. 7일 보관.

---

## Graceful Shutdown

SIGTERM → 새 태스크 배정 중단 → 실행 중 Pod "현재 태스크 끝나면 정지" 시그널 → 10분 대기 → 미완료 Pod SIGKILL + 태스크 RETRY(crash_recovery=true) → DB flush, Vault Writer 드레인 → 종료. Phase 3에서 12분 force kill로 고도화.

---

## Web UI

Go `embed`로 React 내장. Tailscale 접근. 비밀번호 인증(세션 만료 없음 — Tailscale 네트워크 인증 의존, 명시적 로그아웃만).

페이지: Dashboard, Goals, Tasks, PRs, Projects, Research, Usage, Settings.

기술: React + TypeScript + Tailwind CSS + Vite, WebSocket, Zustand.

---

## 데이터베이스 (SQLite)

WAL 모드. 테이블: `goals`, `projects`, `tasks`(`crash_recovery` boolean 포함), `alerts`, `usage_snapshots`, `rate_limit_events`, `service_metrics`. 주요 인덱스: status+priority, project, PR status, parent task, usage type+time, service+time.

---

## API

외부(`/api/...`): Goals CRUD + activate + orchestrator 제안, Tasks CRUD + cancel, PRs pending + approve + request-changes, Projects CRUD + approve/reject, Services + alerts, Usage(daily/monthly/blocks/timeseries/rate-limits), Orchestrator status + pods.

내부(`/internal/...`): tasks/next, tasks/:id/done, subtasks, model/:task_id.

WebSocket: `/ws/events` 이벤트 스트림.

---

## 구현 순서

### Phase 1: 기반 — "뼈대"

**목표**: Flux 부팅, Web UI로 Goal/Task/Project CRUD 가능.

**결과물**: `go build` → 단일 바이너리 → Web UI CRUD.

항목: Go 프로젝트 초기화 + 설정 로더, SQLite 스키마 + 부트스트랩, CRUD API, 내부 API 프레임워크, Discord Notifier, GitHub 클라이언트(레포 생성만), Web UI(Dashboard, Goals, Tasks, Projects).

### Phase 2A: 핵심 파이프라인 — "태스크 하나가 PR까지 간다"

**목표**: Operator 태스크 등록 → Executor 코딩 → PR 생성 → 자동/수동 머지.

항목: Claude Code CLI 통합, JSON 파싱 전략 확정, **Sandbox 평가**, Smoke Test, Git worktree 관리, GitHub PR 클라이언트, Manager 기본, Executor Pod + 가드레일, post-execution 검증, QA, PR + 리뷰, Web UI PRs 페이지, rate limit 감지 **실험**, **Claude Code Agent Teams 호환성 실험**(OMC 에이전트 정의 패턴 포함).

**Phase 2A 완료 = "Flux가 Flux를 만든다" 전환점.**

### Phase 2B: 파이프라인 강화 — "안정적으로 돌아간다"

**목표**: 안전장치, 사용량 추적, 지식 기록 추가.

항목: Rate limit 감지 구현, 기본 대응(고정 5시간 대기), 모델 선택, Goal 프롬프트, 서브태스크 분해, Manager 고도화, Vault Writer, 최소 Vault 기록, ccusage 매핑 검증, 최소 Graceful Shutdown, launchd 등록, 에러 복구 기본.

### Phase 3: 오케스트레이션 — "자율 운영"

**목표**: Pod 자동 관리, 사용량 추적, 매일 보고.

항목: Orchestrator 프레임워크 + 하위 컴포넌트, ScaleManager, RateLimitHandler 고도화(동적 대기), UsageCollector, 시계열 스냅샷, 태스크별 사용량, DailySummary, JSONL 정리, 데일리 백업, Graceful Shutdown 고도화, 데이터 정리, Usage UI.

### Phase 4: 지식 & 자율 — "자율적으로 성장"

**목표**: Researcher 자율 리서치, 지식 축적, 자기 개선.

항목: Researcher Pod + workspace, 자율 리서치 스케줄링, Vault Writer 고도화, 새 프로젝트 제안, 배포 자동화, 자기 개선(flux-safe + revert), Research UI.

---

## Future Work

### 원래 설계에서 미룬 기능

Monitor HTTP 헬스체크, 인시던트 리포트, Services UI, 복수 Goal + Rank, REFINING 상태, Issue 시스템, 한도 학습, 점진적 Pod 스케일링, 상세 데일리 서머리, 로그 분석 모니터링, 프로세스/리소스 모니터링, Watchdog, Obsidian 대시보드 자동 갱신, 메트릭 집계, JSONL gzip, 알림 필터, 플랜 추천, 라이브러리 분석, Research 품질 검증, DB 마이그레이션, PR 테이블 분리, Rate Limit 3순위 감지.

### 경쟁 분석에서 도출

실행 환경 격리(Sandbox), 코드 리뷰 피드백 학습, 외부 이벤트 트리거(GitHub webhook, Discord 봇), CI/CD 통합, 멀티 리포 태스크, 세션 지속성/컨텍스트 공유, 모바일 접근성.

### oh-my-claudecode(OMC) 참고

에이전트 정의 패턴(`.claude/agents/`), CLAUDE.md 자동 갱신(3티어 노트패드), 네이티브 Agent Teams 활용(서브에이전트 병렬 실행).

**참고**: OMC는 인터랙티브 세션용 오케스트레이터이므로 Flux 플러그인으로 사용 부적합(이중 오케스트레이션). 아이디어만 차용.
