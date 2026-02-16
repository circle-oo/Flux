# Phase 2C: Architecture Migration — "Protobuf unification + Agent SDK"

## Goal

Replace `claude -p` subprocess execution with Python Agent SDK via gRPC, unify all API communication through a single `flux.proto` definition. Frontend uses Connect-RPC, Go<>Python uses gRPC streaming. Obsidian data access happens through `obsidian-cli` called by agents via Bash tool.

## Deliverable

Single `.proto` -> TypeScript client (Connect-RPC) + Go server (Connect-RPC + gRPC client) + Python Agent Manager (gRPC server + Agent SDK). Real-time task event streaming replaces WebSocket polling.

## Prerequisites

- Phase 2B complete (reliable pipeline with rate limit handling, Vault recording, launchd)
- Python 3.11+ installed
- Claude Agent SDK (`claude-agent-sdk`) Python package available
- `buf` CLI installed for protobuf code generation
- `obsidian-cli` installed on the Mac Mini

---

## Architecture Overview

### Communication Model

- **Frontend <> Go Backend**: Connect-RPC (HTTP, browser-native)
- **Go Backend -> Python Agent Manager**: gRPC server-side streaming
- **Python -> Go**: ExecuteTask RPC streaming response

A single proto definition generates TypeScript client, Go server, and Python client/server code.

When an agent needs Obsidian data, it calls `obsidian-cli` directly via Bash tool. Since everything runs on the same Mac Mini, no separate callback RPC is needed.

### Architecture Diagram

```
+-----------------------------------------------------------------+
|                       Mac Mini M4 Pro                            |
|                                                                  |
|  +---------------+                                               |
|  |   Frontend    |  Connect-RPC (HTTP POST, JSON/binary)         |
|  |   React/Next  |-----------------+                             |
|  |               |                 |                              |
|  |  * Dashboard  |                 |                              |
|  |  * Insights   |                 |                              |
|  |  * Task Mgmt  |                 |                              |
|  +---------------+                 |                              |
|                                    v                              |
|                         +----------------------+                  |
|                         |   Go API Backend     |                  |
|                         |       :8080          |                  |
|                         |                      |                  |
|                         |  ConnectRPC Handler -- Frontend         |
|                         |  gRPC Client --------- Agent           |
|                         |                      |                  |
|                         |  * Task Scheduler    |                  |
|                         |  * Orchestrator      |                  |
|                         |  * State Machine     |                  |
|                         |  * REST API          |                  |
|                         +----------+-----------+                  |
|                                    |                              |
|                           gRPC :50051                             |
|                      +--- request ---->+                          |
|                      |<-- stream back -|                          |
|                                    |                              |
|                         +----------v-----------+                  |
|                         |  Python Agent Mgr    |                  |
|                         |                      |                  |
|                         |  * Agent SDK         |                  |
|                         |  * Bash              |                  |
|                         |    +- obsidian-cli   |                  |
|                         |  * Read/Edit/Glob    |                  |
|                         |  * Hooks             |                  |
|                         +----------------------+                  |
|                                                                   |
|  +-------------------+              +---------------------+       |
|  | SQLite / Postgres |              |  Agent Sessions     |       |
|  | (task state)      |              |  (Claude contexts)  |       |
|  +-------------------+              +---------------------+       |
+-----------------------------------------------------------------+
```

### Before / After

```
BEFORE (Phase 2B):
  Frontend -> REST API (fetch) -> Go Backend -> claude -p subprocess
  WebSocket for real-time updates

AFTER (Phase 2C):
  Frontend -> Connect-RPC (HTTP POST) -> Go Backend -> gRPC stream -> Python Agent Manager
  Connect-RPC SSE for real-time streaming
  Agent -> Bash("obsidian-cli ...") for knowledge access
```

### Data Flow

```
 User clicks "Create Task"
      |
      v  Connect-RPC POST /flux.v1.FluxService/CreateTask
 Go Backend: store task, return task ID
      |
      v  goroutine -> gRPC ExecuteTask to Python :50051
 Python Agent Manager: run Claude Agent SDK
      |
      +- Agent needs Obsidian data?
      |  Bash("obsidian-cli search '...'") -> direct filesystem access
      |
      v  gRPC stream: TaskEvent messages (Python -> Go)
 Go Backend: store events + broadcast to subscribers
      |
      v  Connect-RPC SSE /flux.v1.FluxService/StreamTaskEvents
 Frontend: useTaskStream() receives real-time updates
```

### Why Connect-RPC?

| Feature                         | Connect-RPC        | gRPC-Web            | REST + OpenAPI     |
| ------------------------------- | ------------------ | ------------------- | ------------------ |
| Browser native                  | Yes, HTTP POST     | No, Envoy proxy     | Yes                |
| Same `.proto`                   | Yes                | Yes                 | No, separate schema|
| Server streaming                | Yes                | Yes                 | No, WebSocket      |
| JSON support                    | Yes, automatic     | No, binary only     | Yes                |
| Go server codegen               | Yes, `connect-go`  | Yes                 | No                 |
| TS client codegen               | Yes, `@connectrpc` | Yes, `grpc-web`     | No, codegen needed |
| No proxy required               | Yes                | No                  | Yes                |
| Share port with gRPC            | Yes                | No                  | N/A                |

---

## Task Breakdown

### Task 2C.1: Proto Definitions + Buf Setup + Code Generation

**Description**: Define `flux.proto` with both FluxService (Frontend<>Go) and AgentExecutionService (Go<>Python). Configure Buf for 3-language code generation.

**Files to create**:
```
proto/flux/v1/flux.proto
buf.yaml
buf.gen.yaml
```

**Implementation details**:
- `FluxService` (Connect-RPC, Frontend<>Go):
  - `CreateTask`, `GetTask`, `ListTasks`, `CancelTask`
  - `StreamTaskEvents` (server-side streaming for real-time updates)
  - `GetDashboard`, `GetInsights`, `GetAgentStatus`
- `AgentExecutionService` (gRPC, Go<>Python):
  - `ExecuteTask` (server-side streaming: Python streams `TaskEvent` back to Go)
  - `CancelAgentTask`
  - `GetPodStatus`
- Message definitions: `Task`, `TaskEvent`, `TaskStatus`, `TaskPriority`, `AgentPod`, etc.
- `TaskEvent.TaskEventType`: `ASSISTANT_MESSAGE`, `TOOL_USE`, `TOOL_RESULT`, `TASK_COMPLETE`, `TASK_ERROR`, `PROGRESS`
- Buf plugins: `protocolbuffers/go`, `grpc/go`, `connectrpc/go`, `bufbuild/es`, `connectrpc/es`, `protocolbuffers/python`, `grpc/python`
- Generated output: `gen/go/`, `gen/ts/`, `gen/python/`
- Add `gen/` to `.gitignore`
- Add `make proto` target to Makefile

#### Full `flux.proto` Definition

```protobuf
syntax = "proto3";
package flux.v1;

option go_package = "flux/gen/flux/v1;fluxv1";

import "google/protobuf/timestamp.proto";

// ========================================================
// Frontend <> Go Backend (Connect-RPC)
// ========================================================

service FluxService {
  // Task CRUD
  rpc CreateTask(CreateTaskRequest) returns (CreateTaskResponse);
  rpc GetTask(GetTaskRequest) returns (GetTaskResponse);
  rpc ListTasks(ListTasksRequest) returns (ListTasksResponse);
  rpc CancelTask(CancelTaskRequest) returns (CancelTaskResponse);

  // Real-time task event streaming (SSE via Connect)
  rpc StreamTaskEvents(StreamTaskEventsRequest) returns (stream TaskEvent);

  // Dashboard
  rpc GetDashboard(GetDashboardRequest) returns (GetDashboardResponse);
  rpc GetInsights(GetInsightsRequest) returns (GetInsightsResponse);

  // Agent pod status
  rpc GetAgentStatus(GetAgentStatusRequest) returns (GetAgentStatusResponse);
}

// --- Task Messages ---

message CreateTaskRequest {
  string agent_type = 1;          // "dev", "qa", "devops", "rnd"
  string prompt = 2;
  string working_directory = 3;
  map<string, string> metadata = 4;
  TaskPriority priority = 5;
}

message CreateTaskResponse {
  Task task = 1;
}

message GetTaskRequest {
  string task_id = 1;
}

message GetTaskResponse {
  Task task = 1;
  repeated TaskEvent recent_events = 2;
}

message ListTasksRequest {
  TaskStatus status_filter = 1;
  string agent_type_filter = 2;
  int32 page_size = 3;
  string page_token = 4;
}

message ListTasksResponse {
  repeated Task tasks = 1;
  string next_page_token = 2;
  int32 total_count = 3;
}

message CancelTaskRequest {
  string task_id = 1;
}

message CancelTaskResponse {
  bool success = 1;
}

message StreamTaskEventsRequest {
  string task_id = 1;
}

message Task {
  string id = 1;
  string agent_type = 2;
  string prompt = 3;
  TaskStatus status = 4;
  TaskPriority priority = 5;
  string working_directory = 6;
  map<string, string> metadata = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
  google.protobuf.Timestamp completed_at = 10;
  string result_summary = 11;
  string error_message = 12;
}

enum TaskStatus {
  TASK_STATUS_UNSPECIFIED = 0;
  TASK_STATUS_PENDING = 1;
  TASK_STATUS_RUNNING = 2;
  TASK_STATUS_COMPLETED = 3;
  TASK_STATUS_FAILED = 4;
  TASK_STATUS_CANCELLED = 5;
}

enum TaskPriority {
  TASK_PRIORITY_UNSPECIFIED = 0;
  TASK_PRIORITY_LOW = 1;
  TASK_PRIORITY_NORMAL = 2;
  TASK_PRIORITY_HIGH = 3;
  TASK_PRIORITY_CRITICAL = 4;
}

// --- Dashboard Messages ---

message GetDashboardRequest {}

message GetDashboardResponse {
  int32 total_tasks_today = 1;
  int32 completed_tasks_today = 2;
  int32 failed_tasks_today = 3;
  int32 active_agents = 4;
  repeated AgentSummary agent_summaries = 5;
}

message AgentSummary {
  string agent_type = 1;
  int32 tasks_completed = 2;
  int32 tasks_failed = 3;
  double avg_duration_seconds = 4;
}

message GetInsightsRequest {
  string time_range = 1;         // "24h", "7d", "30d"
}

message GetInsightsResponse {
  repeated DailyMetric daily_metrics = 1;
  repeated ToolUsageStat tool_usage = 2;
  repeated AgentPerformance agent_performance = 3;
}

message DailyMetric {
  string date = 1;
  int32 tasks_completed = 2;
  int32 tasks_failed = 3;
  double total_duration_seconds = 4;
}

message ToolUsageStat {
  string tool_name = 1;
  int32 invocation_count = 2;
  double avg_duration_ms = 3;
}

message AgentPerformance {
  string agent_type = 1;
  double success_rate = 2;
  double avg_turns = 3;
  double avg_duration_seconds = 4;
}

// --- Agent Status Messages ---

message GetAgentStatusRequest {}

message GetAgentStatusResponse {
  repeated AgentPod pods = 1;
}

message AgentPod {
  string agent_type = 1;
  string status = 2;              // "idle", "running", "error"
  string current_task_id = 3;
  int64 uptime_seconds = 4;
  int32 tasks_completed = 5;
}

// ========================================================
// Go Backend -> Python Agent Manager (gRPC)
// Response streams from Python -> Go direction
// ========================================================

service AgentExecutionService {
  // Go calls, Python returns TaskEvent as streaming response
  rpc ExecuteTask(ExecuteTaskRequest) returns (stream TaskEvent);
  rpc CancelAgentTask(CancelAgentTaskRequest) returns (CancelAgentTaskResponse);
  rpc GetPodStatus(GetPodStatusRequest) returns (GetPodStatusResponse);
}

message ExecuteTaskRequest {
  string task_id = 1;
  string agent_type = 2;
  string prompt = 3;
  string working_directory = 4;
  repeated string allowed_tools = 5;
  string system_prompt = 6;
  int32 max_turns = 7;
  map<string, string> metadata = 8;
}

message TaskEvent {
  string task_id = 1;
  TaskEventType type = 2;
  string content = 3;
  google.protobuf.Timestamp timestamp = 4;
  map<string, string> metadata = 5;

  enum TaskEventType {
    TASK_EVENT_TYPE_UNSPECIFIED = 0;
    ASSISTANT_MESSAGE = 1;
    TOOL_USE = 2;
    TOOL_RESULT = 3;
    TASK_COMPLETE = 4;
    TASK_ERROR = 5;
    PROGRESS = 6;
  }
}

message CancelAgentTaskRequest {
  string task_id = 1;
}

message CancelAgentTaskResponse {
  bool success = 1;
}

message GetPodStatusRequest {}

message GetPodStatusResponse {
  repeated AgentPod pods = 1;
}
```

#### `buf.yaml`

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

#### `buf.gen.yaml`

```yaml
version: v2
plugins:
  # Go: protobuf messages
  - remote: buf.build/protocolbuffers/go
    out: gen/go
    opt: paths=source_relative

  # Go: gRPC server/client (for Python communication)
  - remote: buf.build/grpc/go
    out: gen/go
    opt: paths=source_relative

  # Go: Connect-RPC handlers (for Frontend communication)
  - remote: buf.build/connectrpc/go
    out: gen/go
    opt: paths=source_relative

  # TypeScript: protobuf messages
  - remote: buf.build/bufbuild/es
    out: gen/ts
    opt: target=ts

  # TypeScript: Connect-RPC client
  - remote: buf.build/connectrpc/es
    out: gen/ts
    opt: target=ts

  # Python: protobuf messages + gRPC
  - remote: buf.build/protocolbuffers/python
    out: gen/python

  - remote: buf.build/grpc/python
    out: gen/python
```

#### Code Generation

```bash
buf generate proto

# Output structure:
# gen/
# +-- go/flux/v1/
# |   +-- flux.pb.go                (protobuf messages)
# |   +-- flux_grpc.pb.go           (gRPC server/client)
# |   +-- fluxv1connect/
# |       +-- flux.connect.go       (Connect-RPC handlers)
# +-- ts/flux/v1/
# |   +-- flux_pb.ts                (message types)
# |   +-- flux_connect.ts           (Connect client)
# +-- python/flux/v1/
#     +-- flux_pb2.py
#     +-- flux_pb2_grpc.py
```

**Acceptance criteria**:
- [ ] `buf lint` passes
- [ ] `buf generate proto` produces code for all 3 languages
- [ ] Go types compile without errors
- [ ] TypeScript types import correctly
- [ ] Python types import correctly
- [ ] Makefile `proto` target works

**Complexity**: Medium

---

### Task 2C.2: Python Agent Manager -- gRPC Server

**Description**: Implement the Python gRPC server that receives task execution requests from Go and runs them via Claude Agent SDK, streaming events back.

**Files to create**:
```
agent_manager/server.py
agent_manager/config.py
agent_manager/requirements.txt
agent_manager/pyproject.toml
```

**Implementation details**:
- `AgentExecutionServicer` implements:
  - `ExecuteTask(request) -> stream[TaskEvent]`: Run Agent SDK `query()`, stream events back
  - `CancelAgentTask(request) -> CancelAgentTaskResponse`: Set cancellation flag
  - `GetPodStatus(request) -> GetPodStatusResponse`: Return active agent states
- Agent type configs in `config.py`:
  - `dev`: Senior Go/Python engineer, tools=[Read, Edit, Bash, Glob, Grep], max_turns=100
  - `qa`: QA engineer (read-only source), tools=[Read, Bash, Glob, Grep], max_turns=50
  - `devops`: DevOps engineer, tools=[Read, Edit, Bash], max_turns=30
  - `rnd`: R&D researcher, tools=[Read, Edit, Bash, Glob, Grep], max_turns=200
- `permission_mode="acceptEdits"` for headless 24/7 operation
- Cancellation via `asyncio.Event` per task
- System prompt includes obsidian-cli usage instructions:
  ```
  Use obsidian-cli to search and read project notes when needed:
  $ obsidian-cli search "query" --vault /path/to/vault
  $ obsidian-cli read "Path/To/Note.md" --vault /path/to/vault
  ```
- Message-to-TaskEvent conversion: parse `content` blocks for text/tool_use, handle `result` for tool_result
- Graceful shutdown: drain active tasks on SIGTERM

#### `agent_manager/config.py`

```python
# agent_manager/config.py

AGENT_CONFIGS = {
    "dev": {
        "system_prompt": (
            "You are a senior Go/Python engineer working on the Flux project. "
            "Write clean, tested code. Follow existing patterns in the codebase. "
            "Use obsidian-cli to search project notes when needed."
        ),
        "allowed_tools": ["Read", "Edit", "Bash", "Glob", "Grep"],
        "max_turns": 100,
    },
    "qa": {
        "system_prompt": (
            "You are a QA engineer. Run tests, analyze failures, "
            "and verify fixes. Never modify source code directly."
        ),
        "allowed_tools": ["Read", "Bash", "Glob", "Grep"],
        "max_turns": 50,
    },
    "devops": {
        "system_prompt": (
            "You are a DevOps engineer. Manage deployments, "
            "monitor services, and handle infrastructure tasks."
        ),
        "allowed_tools": ["Read", "Edit", "Bash"],
        "max_turns": 30,
    },
    "rnd": {
        "system_prompt": (
            "You are an R&D researcher. Explore new approaches, "
            "prototype ideas, and document findings thoroughly. "
            "Use obsidian-cli to read and write research notes."
        ),
        "allowed_tools": ["Read", "Edit", "Bash", "Glob", "Grep"],
        "max_turns": 200,
    },
}
```

#### `agent_manager/server.py`

```python
# agent_manager/server.py

import asyncio
import grpc
from google.protobuf.timestamp_pb2 import Timestamp
from claude_agent_sdk import query, ClaudeAgentOptions

from gen.python.flux.v1 import flux_pb2, flux_pb2_grpc
from config import AGENT_CONFIGS


class AgentExecutionServicer(flux_pb2_grpc.AgentExecutionServiceServicer):
    def __init__(self):
        self.cancellation_flags: dict[str, asyncio.Event] = {}
        self.active_agents: dict[str, str] = {}

    async def ExecuteTask(self, request, context):
        """Go calls -> Agent SDK executes -> stream TaskEvent back"""
        config = AGENT_CONFIGS.get(request.agent_type, AGENT_CONFIGS["dev"])
        cancel_event = asyncio.Event()
        self.cancellation_flags[request.task_id] = cancel_event
        self.active_agents[request.task_id] = request.agent_type

        options = ClaudeAgentOptions(
            system_prompt=request.system_prompt or config["system_prompt"],
            allowed_tools=list(request.allowed_tools) or config["allowed_tools"],
            max_turns=request.max_turns or config["max_turns"],
            cwd=request.working_directory,
            permission_mode="acceptEdits",
        )

        try:
            async for message in query(prompt=request.prompt, options=options):
                if cancel_event.is_set():
                    yield _event(request.task_id, "TASK_ERROR", "Cancelled")
                    return

                event = self._to_event(request.task_id, message)
                if event:
                    yield event

            yield _event(request.task_id, "TASK_COMPLETE", "Done")

        except Exception as e:
            yield _event(request.task_id, "TASK_ERROR", str(e))

        finally:
            self.cancellation_flags.pop(request.task_id, None)
            self.active_agents.pop(request.task_id, None)

    async def CancelAgentTask(self, request, context):
        flag = self.cancellation_flags.get(request.task_id)
        if flag:
            flag.set()
            return flux_pb2.CancelAgentTaskResponse(success=True)
        return flux_pb2.CancelAgentTaskResponse(success=False)

    async def GetPodStatus(self, request, context):
        pods = []
        for agent_type in AGENT_CONFIGS:
            task_id = next(
                (t for t, a in self.active_agents.items() if a == agent_type), ""
            )
            pods.append(flux_pb2.AgentPod(
                agent_type=agent_type,
                status="running" if task_id else "idle",
                current_task_id=task_id,
            ))
        return flux_pb2.GetPodStatusResponse(pods=pods)

    def _to_event(self, task_id, message):
        if hasattr(message, "content"):
            for block in message.content:
                if hasattr(block, "text"):
                    return _event(task_id, "ASSISTANT_MESSAGE", block.text)
                if hasattr(block, "name"):
                    return _event(task_id, "TOOL_USE", block.name)
        if hasattr(message, "result"):
            return _event(task_id, "TOOL_RESULT", str(message.result))
        return None


def _event(task_id, event_type, content):
    ts = Timestamp()
    ts.GetCurrentTime()
    return flux_pb2.TaskEvent(
        task_id=task_id,
        type=flux_pb2.TaskEvent.TaskEventType.Value(event_type),
        content=content,
        timestamp=ts,
    )


async def serve():
    server = grpc.aio.server()
    flux_pb2_grpc.add_AgentExecutionServiceServicer_to_server(
        AgentExecutionServicer(), server
    )
    server.add_insecure_port("[::]:50051")
    await server.start()
    print("Agent Manager listening on :50051")
    await server.wait_for_termination()


if __name__ == "__main__":
    asyncio.run(serve())
```

**Acceptance criteria**:
- [ ] gRPC server starts on `:50051`
- [ ] `grpcurl` can call `ExecuteTask` and receive streamed events
- [ ] Agent SDK `query()` executes with correct options
- [ ] Cancellation stops running agent
- [ ] `GetPodStatus` returns accurate state
- [ ] Different agent types use correct configs
- [ ] obsidian-cli instructions in system prompts

**Complexity**: High

---

### Task 2C.3: Go gRPC Client -> Python Agent Manager

**Description**: Implement the Go-side gRPC client that connects to the Python Agent Manager and relays task execution requests.

**Files to create**:
```
go/src/internal/agent/client.go
```

**Implementation details**:
- `Client` struct: gRPC connection, `AgentExecutionServiceClient`, logger
- `NewClient(addr, logger)`: Connect with keepalive (30s/10s), insecure credentials (localhost), max recv 10MB
- `ExecuteTask(ctx, req, onEvent func(*TaskEvent)) error`: Stream events, call `onEvent` callback for each, stop on TASK_COMPLETE/TASK_ERROR
- `CancelTask(ctx, taskID) error`
- `PodStatus(ctx) (*GetPodStatusResponse, error)`
- `Close() error`: Close gRPC connection
- Connection retry with exponential backoff on startup

#### `internal/agent/client.go`

```go
// internal/agent/client.go
package agent

import (
    "context"
    "io"
    "log/slog"
    "time"

    fluxv1 "flux/gen/go/flux/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/keepalive"
)

type Client struct {
    conn   *grpc.ClientConn
    agent  fluxv1.AgentExecutionServiceClient
    logger *slog.Logger
}

func NewClient(addr string, logger *slog.Logger) (*Client, error) {
    conn, err := grpc.NewClient(addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
        grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(10<<20)),
        grpc.WithKeepaliveParams(keepalive.ClientParameters{
            Time:    30 * time.Second,
            Timeout: 10 * time.Second,
        }),
    )
    if err != nil {
        return nil, err
    }
    return &Client{
        conn:   conn,
        agent:  fluxv1.NewAgentExecutionServiceClient(conn),
        logger: logger,
    }, nil
}

func (c *Client) ExecuteTask(
    ctx context.Context,
    req *fluxv1.ExecuteTaskRequest,
    onEvent func(*fluxv1.TaskEvent),
) error {
    stream, err := c.agent.ExecuteTask(ctx, req)
    if err != nil {
        return err
    }
    for {
        event, err := stream.Recv()
        if err == io.EOF {
            return nil
        }
        if err != nil {
            return err
        }
        c.logger.Info("agent event",
            "task_id", event.TaskId,
            "type", event.Type.String(),
        )
        onEvent(event)
        if event.Type == fluxv1.TaskEvent_TASK_COMPLETE ||
            event.Type == fluxv1.TaskEvent_TASK_ERROR {
            return nil
        }
    }
}

func (c *Client) CancelTask(ctx context.Context, taskID string) error {
    _, err := c.agent.CancelAgentTask(ctx,
        &fluxv1.CancelAgentTaskRequest{TaskId: taskID})
    return err
}

func (c *Client) PodStatus(ctx context.Context) (*fluxv1.GetPodStatusResponse, error) {
    return c.agent.GetPodStatus(ctx, &fluxv1.GetPodStatusRequest{})
}

func (c *Client) Close() error {
    return c.conn.Close()
}
```

**Acceptance criteria**:
- [ ] Go client connects to Python gRPC server
- [ ] Events streamed correctly from Python to Go
- [ ] Cancellation propagates to Python
- [ ] Pod status retrieved
- [ ] Connection retry on transient failure
- [ ] Graceful close

**Complexity**: Medium

**Depends on**: Task 2C.1, 2C.2

---

### Task 2C.4: Go Backend Connect-RPC Migration

**Description**: Replace existing REST API handlers with Connect-RPC handlers generated from `flux.proto`. Maintain backward compatibility during migration.

**Files to create/modify**:
```
go/src/internal/handler/flux_service.go    (new)
go/src/internal/store/task_store.go        (new -- event pubsub)
go/src/cmd/flux/main.go                    (modify -- h2c server)
```

**Implementation details**:
- `FluxServiceHandler` implements all `FluxService` RPCs:
  - `CreateTask`: Store task, fire-and-forget goroutine to execute via Python agent
  - `GetTask`: Return task + recent events
  - `ListTasks`: Paginated with status/agent_type filters
  - `CancelTask`: Propagate to Python agent
  - `StreamTaskEvents`: Subscribe to event channel, stream to client
  - `GetDashboard`: Aggregate daily stats + pod status
  - `GetInsights`: Time-range metrics (daily metrics, tool usage, agent performance)
  - `GetAgentStatus`: Delegate to Python `GetPodStatus`
- Event pubsub in `task_store.go`: Subscribe/Unsubscribe per task_id, Broadcast events
- Server setup: `h2c.NewHandler(withCORS(mux), &http2.Server{})` for HTTP/2 cleartext
- CORS headers: `Access-Control-Allow-Origin: *`, methods POST, headers `Content-Type, Connect-Protocol-Version`
- Keep existing REST endpoints during migration (gradual cutover), remove after frontend migration
- Background executor: `executeInBackground(task)` calls `agent.ExecuteTask()`, records events, broadcasts to subscribers

#### `cmd/flux/main.go` (Go Server Setup with h2c and CORS)

```go
// cmd/flux/main.go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"

    "golang.org/x/net/http2"
    "golang.org/x/net/http2/h2c"

    "flux/gen/go/flux/v1/fluxv1connect"
    "flux/internal/agent"
    "flux/internal/handler"
)

func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Python Agent Manager gRPC client
    agentClient, err := agent.NewClient("localhost:50051", logger)
    if err != nil {
        logger.Error("failed to connect to agent manager", "error", err)
        os.Exit(1)
    }
    defer agentClient.Close()

    // Connect-RPC handler (Frontend)
    fluxHandler := handler.NewFluxServiceHandler(agentClient, logger)
    path, connectHandler := fluxv1connect.NewFluxServiceHandler(fluxHandler)

    // HTTP mux
    mux := http.NewServeMux()
    mux.Handle(path, connectHandler)

    // h2c: HTTP/2 cleartext -- serves Connect-RPC
    server := &http.Server{
        Addr:    ":8080",
        Handler: h2c.NewHandler(withCORS(mux), &http2.Server{}),
    }

    // Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    go func() {
        logger.Info("flux server starting", "addr", ":8080")
        if err := server.ListenAndServe(); err != http.ErrServerClosed {
            logger.Error("server error", "error", err)
        }
    }()

    <-ctx.Done()
    server.Shutdown(context.Background())
}

func withCORS(h http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "POST")
        w.Header().Set("Access-Control-Allow-Headers",
            "Content-Type, Connect-Protocol-Version")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }
        h.ServeHTTP(w, r)
    })
}
```

#### `internal/handler/flux_service.go` (Connect-RPC Handler)

```go
// internal/handler/flux_service.go
package handler

import (
    "context"
    "log/slog"

    "connectrpc.com/connect"

    fluxv1 "flux/gen/go/flux/v1"
    "flux/internal/agent"
    "flux/internal/store"
)

type FluxServiceHandler struct {
    agent  *agent.Client
    store  *store.TaskStore
    logger *slog.Logger
}

func NewFluxServiceHandler(
    agent *agent.Client,
    logger *slog.Logger,
) *FluxServiceHandler {
    return &FluxServiceHandler{
        agent:  agent,
        store:  store.New(),
        logger: logger,
    }
}

func (h *FluxServiceHandler) CreateTask(
    ctx context.Context,
    req *connect.Request[fluxv1.CreateTaskRequest],
) (*connect.Response[fluxv1.CreateTaskResponse], error) {
    task := h.store.Create(req.Msg)

    // Async execution via Python Agent Manager
    go h.executeInBackground(task)

    return connect.NewResponse(&fluxv1.CreateTaskResponse{
        Task: task,
    }), nil
}

func (h *FluxServiceHandler) GetTask(
    ctx context.Context,
    req *connect.Request[fluxv1.GetTaskRequest],
) (*connect.Response[fluxv1.GetTaskResponse], error) {
    task, events := h.store.Get(req.Msg.TaskId)
    return connect.NewResponse(&fluxv1.GetTaskResponse{
        Task:         task,
        RecentEvents: events,
    }), nil
}

func (h *FluxServiceHandler) ListTasks(
    ctx context.Context,
    req *connect.Request[fluxv1.ListTasksRequest],
) (*connect.Response[fluxv1.ListTasksResponse], error) {
    tasks, nextToken, total := h.store.List(req.Msg)
    return connect.NewResponse(&fluxv1.ListTasksResponse{
        Tasks:         tasks,
        NextPageToken: nextToken,
        TotalCount:    total,
    }), nil
}

func (h *FluxServiceHandler) CancelTask(
    ctx context.Context,
    req *connect.Request[fluxv1.CancelTaskRequest],
) (*connect.Response[fluxv1.CancelTaskResponse], error) {
    err := h.agent.CancelTask(ctx, req.Msg.TaskId)
    return connect.NewResponse(&fluxv1.CancelTaskResponse{
        Success: err == nil,
    }), nil
}

func (h *FluxServiceHandler) StreamTaskEvents(
    ctx context.Context,
    req *connect.Request[fluxv1.StreamTaskEventsRequest],
    stream *connect.ServerStream[fluxv1.TaskEvent],
) error {
    eventCh := h.store.Subscribe(req.Msg.TaskId)
    defer h.store.Unsubscribe(req.Msg.TaskId, eventCh)

    for {
        select {
        case event, ok := <-eventCh:
            if !ok {
                return nil
            }
            if err := stream.Send(event); err != nil {
                return err
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

func (h *FluxServiceHandler) GetDashboard(
    ctx context.Context,
    req *connect.Request[fluxv1.GetDashboardRequest],
) (*connect.Response[fluxv1.GetDashboardResponse], error) {
    stats := h.store.GetDailyStats()
    podStatus, _ := h.agent.PodStatus(ctx)

    return connect.NewResponse(&fluxv1.GetDashboardResponse{
        TotalTasksToday:     stats.Total,
        CompletedTasksToday: stats.Completed,
        FailedTasksToday:    stats.Failed,
        ActiveAgents:        int32(len(podStatus.Pods)),
        AgentSummaries:      stats.Summaries,
    }), nil
}

func (h *FluxServiceHandler) GetInsights(
    ctx context.Context,
    req *connect.Request[fluxv1.GetInsightsRequest],
) (*connect.Response[fluxv1.GetInsightsResponse], error) {
    insights := h.store.GetInsights(req.Msg.TimeRange)
    return connect.NewResponse(insights), nil
}

func (h *FluxServiceHandler) GetAgentStatus(
    ctx context.Context,
    req *connect.Request[fluxv1.GetAgentStatusRequest],
) (*connect.Response[fluxv1.GetAgentStatusResponse], error) {
    status, err := h.agent.PodStatus(ctx)
    if err != nil {
        return nil, err
    }
    return connect.NewResponse(&fluxv1.GetAgentStatusResponse{
        Pods: status.Pods,
    }), nil
}

func (h *FluxServiceHandler) executeInBackground(task *fluxv1.Task) {
    ctx := context.Background()
    req := &fluxv1.ExecuteTaskRequest{
        TaskId:           task.Id,
        AgentType:        task.AgentType,
        Prompt:           task.Prompt,
        WorkingDirectory: task.WorkingDirectory,
    }

    h.agent.ExecuteTask(ctx, req, func(event *fluxv1.TaskEvent) {
        h.store.RecordEvent(task.Id, event)
        h.store.Broadcast(task.Id, event) // -> StreamTaskEvents subscribers
    })
}
```

**Acceptance criteria**:
- [ ] Connect-RPC endpoints respond correctly
- [ ] `StreamTaskEvents` delivers real-time events via SSE
- [ ] h2c server serves both Connect-RPC and legacy REST
- [ ] Event pubsub correctly manages subscribers
- [ ] CORS allows browser Connect-RPC requests
- [ ] Existing REST endpoints still work (backward compatibility)

**Complexity**: High

**Depends on**: Task 2C.1, 2C.3

---

### Task 2C.5: Frontend Connect-RPC Migration

**Description**: Replace REST `fetch()` API calls with typed Connect-RPC client. Replace WebSocket with `StreamTaskEvents` SSE.

**Files to create/modify**:
```
frontend/src/lib/flux-client.ts              (new)
frontend/src/hooks/useTaskStream.ts          (new)
frontend/src/lib/api.ts                      (modify -> wrap or replace)
frontend/src/stores/taskStore.ts             (modify)
frontend/src/stores/wsStore.ts               (modify -> remove or replace)
frontend/src/pages/Dashboard.tsx             (modify)
frontend/src/pages/Tasks.tsx                 (modify)
frontend/src/pages/TaskDetail.tsx            (modify)
```

**Implementation details**:
- Install: `@connectrpc/connect`, `@connectrpc/connect-web`, `@bufbuild/protobuf`
- `flux-client.ts`: `createClient(FluxService, createConnectTransport({ baseUrl }))` -- fully typed
- `useTaskStream(taskId)` hook:
  - `fluxClient.streamTaskEvents({ taskId })` with `AbortController`
  - Accumulate events in state
  - Detect `TASK_COMPLETE`/`TASK_ERROR` for completion
- Migrate stores to use typed proto messages
- Gradual migration: convert one page at a time, verify, then next
- Migration order: Dashboard -> Tasks -> TaskDetail -> PRs -> Projects -> Goals
- Remove `wsStore.ts` after full migration (Connect-RPC SSE replaces WebSocket)

#### `frontend/src/lib/flux-client.ts`

```typescript
// lib/flux-client.ts
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { FluxService } from "../gen/ts/flux/v1/flux_connect";

const transport = createConnectTransport({
  baseUrl: "http://localhost:8080",
});

export const fluxClient = createClient(FluxService, transport);
```

#### Task Creation Example (fully typed)

```typescript
// app/tasks/create.ts
import { fluxClient } from "@/lib/flux-client";
import { TaskPriority } from "../gen/ts/flux/v1/flux_pb";

const response = await fluxClient.createTask({
  agentType: "dev",
  prompt: "Fix the failing test in auth_service.go",
  workingDirectory: "/opt/flux/projects/main",
  priority: TaskPriority.HIGH,
});

console.log(response.task?.id); // fully typed
```

#### `frontend/src/hooks/useTaskStream.ts`

```typescript
// hooks/use-task-stream.ts
import { useEffect, useState } from "react";
import { fluxClient } from "@/lib/flux-client";
import type { TaskEvent } from "../gen/ts/flux/v1/flux_pb";
import { TaskEvent_TaskEventType } from "../gen/ts/flux/v1/flux_pb";

export function useTaskStream(taskId: string) {
  const [events, setEvents] = useState<TaskEvent[]>([]);
  const [isRunning, setIsRunning] = useState(true);

  useEffect(() => {
    const controller = new AbortController();

    (async () => {
      try {
        for await (const event of fluxClient.streamTaskEvents(
          { taskId },
          { signal: controller.signal }
        )) {
          setEvents((prev) => [...prev, event]);
          if (
            event.type === TaskEvent_TaskEventType.TASK_COMPLETE ||
            event.type === TaskEvent_TaskEventType.TASK_ERROR
          ) {
            setIsRunning(false);
          }
        }
      } catch (e) {
        if (!controller.signal.aborted) setIsRunning(false);
      }
    })();

    return () => controller.abort();
  }, [taskId]);

  return { events, isRunning };
}
```

#### Dashboard Page Example

```typescript
// app/dashboard/page.tsx
import { fluxClient } from "@/lib/flux-client";

export default async function Dashboard() {
  const dashboard = await fluxClient.getDashboard({});
  const agents = await fluxClient.getAgentStatus({});

  return (
    <div>
      <StatsCards
        total={dashboard.totalTasksToday}
        completed={dashboard.completedTasksToday}
        failed={dashboard.failedTasksToday}
      />
      <AgentGrid pods={agents.pods} />
    </div>
  );
}
```

**Acceptance criteria**:
- [ ] All pages use Connect-RPC typed client
- [ ] Task creation fully typed (no `any`)
- [ ] Real-time streaming works (TaskDetail shows live events)
- [ ] WebSocket code removed
- [ ] No runtime type errors
- [ ] All existing UI functionality preserved

**Complexity**: High

**Depends on**: Task 2C.4

---

### Task 2C.6: Obsidian CLI Setup + Agent Integration

**Description**: Install and configure `obsidian-cli` for agent access to the Obsidian Vault. Verify agents can search, read, and write notes.

**Files to create/modify**:
```
agent_manager/config.py       (modify -- add obsidian-cli paths)
docs/experiments/obsidian-cli-evaluation.md  (new)
```

**Implementation details**:
- Install `obsidian-cli` (or implement a minimal CLI wrapper if not available):
  - `obsidian-cli search "query" --vault /path/to/vault` -- search notes
  - `obsidian-cli read "Path/To/Note.md" --vault /path/to/vault` -- read note content
  - `obsidian-cli write "Path/To/Note.md" --content "..." --vault /path/to/vault` -- write note
- Add vault path to agent system prompts
- Test with each agent type: can `dev` agent read project notes? Can `rnd` agent write research findings?
- Verify obsidian-cli doesn't conflict with Vault Writer (both can write, but Vault Writer handles Flux-system writes, obsidian-cli handles agent-initiated writes)
- Document access patterns and any limitations

#### Obsidian Access Pattern

Agents call `obsidian-cli` directly via Bash tool. Since everything runs on the same Mac Mini filesystem, no separate RPC is needed.

```bash
# Example agent-initiated commands
$ obsidian-cli search "flux architecture" --vault /path/to/vault
$ obsidian-cli read "Flux/Architecture.md" --vault /path/to/vault
$ obsidian-cli write "Flux/Logs/2026-02-16.md" --content "..." --vault /path/to/vault
```

System prompt includes obsidian-cli usage instructions so the agent calls it when needed.

**Acceptance criteria**:
- [ ] obsidian-cli installed and accessible in agent Bash environment
- [ ] All agent types can search vault content
- [ ] Agents can read specific notes
- [ ] rnd agents can write research findings
- [ ] No conflict with Vault Writer
- [ ] Results documented

**Complexity**: Medium

**Depends on**: Task 2C.2

---

### Task 2C.7: Executor Migration -- Go Executor -> Python Agent Delegation

**Description**: Refactor the Go Executor to delegate code execution to the Python Agent Manager via gRPC, while keeping orchestration logic (worktree management, PR creation, auto-merge) in Go.

**Files to modify**:
```
go/src/internal/executor/executor.go
go/src/internal/executor/prompt.go
```

**Implementation details**:
- `executeOnce()` changes:
  - Steps 1-4 remain in Go (task fetch, model decision, system prompt, worktree)
  - Step 6 changes: Instead of `claude.Run()`, call `agent.ExecuteTask()` via gRPC
  - Build `ExecuteTaskRequest` with: task_id, agent_type (derive from task type), prompt, working_directory (worktree path), allowed_tools, system_prompt, max_turns
  - Receive streamed `TaskEvent` messages -- store in DB for UI streaming
  - Steps 7-16 remain in Go (rate limit check, verification, QA, PR, auto-merge)
- Agent type mapping from task type:
  - CODING/BUGFIX/MAINTENANCE -> "dev"
  - RESEARCH -> "rnd"
  - DEPLOY -> "devops"
  - DOCUMENT/PLANNING -> "dev"
- Remove `internal/claudecli/` package (replaced by gRPC agent client)
- Keep guardrails in Go (timeout via gRPC context deadline)

**Acceptance criteria**:
- [ ] Executor delegates to Python Agent Manager
- [ ] Task events stored and streamable via frontend
- [ ] Worktree management still works (Go side)
- [ ] PR creation still works (Go side)
- [ ] Auto-merge decisions still in Go
- [ ] Guardrails enforced (timeout via context)
- [ ] Old `claudecli` package removed
- [ ] Unit tests updated

**Complexity**: High

**Depends on**: Tasks 2C.3, 2C.4

---

### Task 2C.8: Process Management -- Go + Python

**Description**: Update launchd and startup to manage both the Go backend and Python Agent Manager as coordinated processes.

**Files to modify**:
```
deploy/com.circle-oo.flux.plist             (modify)
deploy/com.circle-oo.flux-agent.plist       (new)
deploy/install-launchd.sh                   (modify)
go/src/cmd/flux/main.go                     (modify)
```

**Implementation details**:
- Option A (recommended): Separate launchd plists for Go and Python
  - `com.circle-oo.flux.plist`: Go backend (`:8080`)
  - `com.circle-oo.flux-agent.plist`: Python Agent Manager (`:50051`)
  - Both with KeepAlive=true, RunAtLoad=true
  - Go backend retries gRPC connection on startup (Python may start later)
- Option B: Go manages Python as child process
  - `exec.Command("python", "agent_manager/server.py")` started from `main.go`
  - Pros: single plist; Cons: process management complexity
- Health check: Go verifies Python gRPC connectivity before accepting tasks
- Graceful shutdown: Go sends SIGTERM to Python, waits for drain
- Python dependencies: `requirements.txt` with `grpcio`, `grpcio-tools`, `claude-agent-sdk`, `protobuf`

**Acceptance criteria**:
- [ ] Both processes start on boot via launchd
- [ ] Go retries Python connection on startup
- [ ] Health check verifies Python availability
- [ ] Graceful shutdown coordinates both processes
- [ ] Crash recovery works (both processes restart independently)

**Complexity**: Medium

**Depends on**: Task 2C.7

---

### Task 2C.9: Legacy REST Cleanup + Integration Testing

**Description**: Remove legacy REST API handlers and WebSocket code after full Connect-RPC migration. Run integration tests for the complete flow.

**Files to modify**:
```
go/src/internal/server/server.go            (remove legacy routes)
go/src/internal/server/websocket.go         (remove)
go/src/internal/server/api_*.go             (remove or keep internal-only)
go/src/internal/claudecli/                  (remove entire package)
frontend/src/stores/wsStore.ts              (remove)
frontend/src/lib/api.ts                     (remove legacy fetch, keep as thin wrapper if needed)
```

**Implementation details**:
- Remove all `/api/` REST handlers replaced by Connect-RPC
- Keep `/internal/` REST handlers (Pod<>Manager communication) -- may migrate to gRPC later
- Keep `/health` endpoint as plain HTTP
- Remove WebSocket handler and store
- Remove `claudecli` package entirely
- Integration test: Frontend -> Connect-RPC -> Go -> gRPC -> Python -> Agent SDK -> events -> Frontend
- Burn-in test: 24-hour stability test with mixed task types

**Acceptance criteria**:
- [ ] All legacy REST code removed
- [ ] WebSocket code removed
- [ ] `claudecli` package removed
- [ ] Integration test passes end-to-end
- [ ] 24-hour burn-in stable
- [ ] No regression in existing functionality

**Complexity**: Medium

**Depends on**: Tasks 2C.5, 2C.7, 2C.8

---

## Phase 2C Completion Checklist

- [ ] `flux.proto` defines all services and messages
- [ ] `buf generate` produces Go, TypeScript, Python code
- [ ] Python Agent Manager runs gRPC server with Agent SDK
- [ ] Go Backend serves Connect-RPC (Frontend) + gRPC client (Python)
- [ ] Frontend uses fully-typed Connect-RPC client
- [ ] Real-time task streaming via Connect-RPC SSE
- [ ] Obsidian CLI accessible to agents
- [ ] Executor delegates to Python Agent Manager
- [ ] Process management handles Go + Python lifecycle
- [ ] Legacy REST + WebSocket + `claudecli` removed
- [ ] 24-hour burn-in test stable

## File Count Summary

| Category | New Files | Modified Files | Removed Files |
|----------|-----------|----------------|---------------|
| Proto/Codegen | ~4 files | -- | -- |
| Python Agent | ~4 files | -- | -- |
| Go backend | ~3 files | ~6 files | ~8 files |
| Frontend | ~2 files | ~8 files | ~2 files |
| Deploy | ~1 file | ~2 files | -- |
| Docs | ~1 file | -- | -- |
| **Total** | **~15 files** | **~16 files** | **~10 files** |

---

## Appendix A: Directory Structure

```
flux/
+-- proto/
|   +-- flux/v1/
|       +-- flux.proto
+-- buf.yaml
+-- buf.gen.yaml
|
+-- gen/                          # auto-generated (gitignore)
|   +-- go/flux/v1/
|   +-- ts/flux/v1/
|   +-- python/flux/v1/
|
+-- cmd/
|   +-- flux/
|       +-- main.go               # Go server entrypoint
|
+-- internal/
|   +-- agent/
|   |   +-- client.go             # gRPC client -> Python
|   +-- handler/
|   |   +-- flux_service.go       # Connect-RPC handlers
|   +-- store/
|   |   +-- task_store.go         # Task state + event pubsub
|   +-- orchestrator/
|       +-- orchestrator.go
|
+-- agent_manager/                # Python
|   +-- server.py
|   +-- config.py
|   +-- requirements.txt
|
+-- frontend/                     # React / Next.js
|   +-- lib/
|   |   +-- flux-client.ts
|   +-- hooks/
|   |   +-- use-task-stream.ts
|   +-- app/
|       +-- dashboard/
|       +-- tasks/
|       +-- insights/
|
+-- Makefile
+-- README.md
```

---

## Appendix B: Makefile

```makefile
.PHONY: proto build run clean setup

proto:
	buf generate proto
	@echo "Generated: gen/go, gen/ts, gen/python"

build: proto
	go build -o bin/flux ./cmd/flux

run: build
	@echo "Starting Python Agent Manager..."
	python agent_manager/server.py &
	@sleep 2
	@echo "Starting Go Backend..."
	./bin/flux &
	@echo "Starting Frontend..."
	cd frontend && npm run dev

setup:
	cd agent_manager && pip install -r requirements.txt
	cd frontend && npm install
	go mod tidy

clean:
	rm -rf bin/ gen/
```

---

## Appendix C: Key Decisions

| Decision                          | Rationale                                                             |
| --------------------------------- | --------------------------------------------------------------------- |
| Single `.proto`                   | Eliminates type mismatches across Frontend/Backend/Agent               |
| Connect-RPC for frontend          | Browser native HTTP, no proxy needed, shares port with gRPC           |
| gRPC streaming for Go <> Python   | Single RPC handles request + streaming response                       |
| Bash tool for Obsidian            | Same machine, no callback RPC needed, simplifies architecture          |
| Buf for codegen                   | `buf generate` once produces all 3 languages                          |
| `h2c` (HTTP/2 cleartext)         | HTTP/2 + Connect serving on localhost without TLS                      |
| `permission_mode="acceptEdits"`   | Required for 24/7 headless operation                                   |

---

## Appendix D: Timeline

| Day    | Task                                       | Deliverable                                     |
| ------ | ------------------------------------------ | ----------------------------------------------- |
| 1-2    | Proto definition + Buf setup + codegen     | `flux.proto`, 3-language generated code          |
| 3-4    | Python Agent Manager (gRPC server)         | Agent SDK integration, standalone `grpcurl` test |
| 5-6    | Go Backend (Connect-RPC + gRPC client)     | Frontend RPCs + Python integration test          |
| 7-8    | Frontend (Connect-RPC client + UI)         | Dashboard, Task management, live streaming       |
| 9      | Stability, process management              | launchd, health check, graceful shutdown         |
| 10     | Burn-in test                               | 24-hour stability test, documentation            |

---

## Appendix E: Migration Checklist

### Proto & Codegen
- [ ] `flux.proto` written and reviewed
- [ ] `buf.yaml`, `buf.gen.yaml` configured
- [ ] `buf generate` produces 3-language code
- [ ] Makefile `proto` target works

### Python Agent Manager
- [ ] gRPC server implemented (`AgentExecutionServicer`)
- [ ] Agent SDK `query()` integration
- [ ] Message-to-TaskEvent conversion logic
- [ ] Standalone `grpcurl` test passes

### Go Backend
- [ ] Connect-RPC handler implemented (`FluxServiceHandler`)
- [ ] gRPC client -> Python Agent Manager
- [ ] Task store + event pubsub
- [ ] `h2c` server setup + CORS
- [ ] Legacy `claude -p` subprocess / REST code removed
- [ ] Integration test: Frontend -> Go -> Python -> Agent SDK

### Frontend
- [ ] Connect-RPC transport + client configured
- [ ] Dashboard page (`getDashboard`, `getAgentStatus`)
- [ ] Task CRUD pages
- [ ] Real-time streaming (`useTaskStream` hook)
- [ ] Insights page (`getInsights`)

### Operations
- [ ] launchd plist (Go + Python)
- [ ] Health check endpoints
- [ ] Graceful shutdown
- [ ] 24-hour burn-in test
- [ ] Migration completion documented
