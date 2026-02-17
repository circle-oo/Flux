"""Flux Agent Manager — async gRPC server that executes tasks via Claude Code SDK."""

import asyncio
import logging
import os
import signal
import sys
import time

import grpc
from grpc import aio as grpc_aio

# Add gen/python to path so we can import the generated proto stubs.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "gen", "python"))

from flux.v1 import flux_pb2, flux_pb2_grpc  # noqa: E402
from google.protobuf.timestamp_pb2 import Timestamp  # noqa: E402

from claude_code_sdk import query, ClaudeCodeOptions  # noqa: E402
from claude_code_sdk.types import (  # noqa: E402
    AssistantMessage,
    ResultMessage,
    TextBlock,
    ToolUseBlock,
    ToolResultBlock,
)

from agent_manager.config import get_agent_config, AGENT_CONFIGS  # noqa: E402

logger = logging.getLogger(__name__)

_start_time = time.time()


def _now_timestamp() -> Timestamp:
    ts = Timestamp()
    ts.GetCurrentTime()
    return ts


def _make_event(
    task_id: str,
    event_type: str,
    content: str,
    metadata: dict[str, str] | None = None,
) -> flux_pb2.ExecuteTaskResponse:
    """Create an ExecuteTaskResponse wrapping a TaskEvent."""
    enum_name = f"TASK_EVENT_TYPE_{event_type}"
    event = flux_pb2.TaskEvent(
        task_id=task_id,
        type=flux_pb2.TaskEvent.TaskEventType.Value(enum_name),
        content=content,
        timestamp=_now_timestamp(),
    )
    if metadata:
        for k, v in metadata.items():
            event.metadata[k] = v
    return flux_pb2.ExecuteTaskResponse(event=event)


class AgentExecutionServicer(flux_pb2_grpc.AgentExecutionServiceServicer):
    """Implements the AgentExecutionService using Claude Code SDK."""

    # Maximum task execution time before a cancellation flag is considered stale.
    _STALE_FLAG_SECONDS = 35 * 60  # 35 minutes

    def __init__(self):
        self.cancellation_flags: dict[str, tuple[asyncio.Event, float]] = {}  # task_id -> (event, created_at)
        self.active_agents: dict[str, str] = {}  # task_id -> agent_type
        self.completed_count = 0

    def _cleanup_stale_flags(self):
        """Remove cancellation flags for tasks that exceeded the max execution time.

        If ExecuteTask crashes before reaching its finally block (e.g. gRPC transport
        error), the cancellation_flags entry leaks. This sweep prevents unbounded growth.
        """
        now = time.time()
        stale = [
            tid for tid, (_, created_at) in self.cancellation_flags.items()
            if now - created_at > self._STALE_FLAG_SECONDS
        ]
        for tid in stale:
            logger.warning("Cleaning up stale cancellation flag for task %s", tid)
            self.cancellation_flags.pop(tid, None)
            self.active_agents.pop(tid, None)

    async def ExecuteTask(self, request, context):
        """Execute a task via Claude Code SDK and stream events back."""
        task_id = request.task_id
        agent_type = request.agent_type
        prompt = request.prompt

        logger.info("ExecuteTask: task_id=%s agent_type=%s", task_id, agent_type)

        # Sweep stale entries from prior crashes before adding a new one
        self._cleanup_stale_flags()

        # Validate agent type
        try:
            agent_cfg = get_agent_config(agent_type)
        except KeyError as e:
            yield _make_event(task_id, "TASK_ERROR", str(e))
            return

        # Set up cancellation
        cancel_event = asyncio.Event()
        self.cancellation_flags[task_id] = (cancel_event, time.time())
        self.active_agents[task_id] = agent_type

        try:
            yield _make_event(task_id, "PROGRESS", f"Starting {agent_type} agent")

            # Build SDK options
            model = request.metadata.get("model") if request.metadata else None
            options = ClaudeCodeOptions(
                system_prompt=request.system_prompt or agent_cfg.system_prompt,
                allowed_tools=list(request.allowed_tools) or agent_cfg.allowed_tools,
                max_turns=request.max_turns or agent_cfg.max_turns,
                cwd=request.working_directory or None,
                model=model,
                permission_mode="bypassPermissions",
            )

            has_result = False
            async for message in query(prompt=prompt, options=options):
                if cancel_event.is_set():
                    yield _make_event(task_id, "TASK_ERROR", "Cancelled by operator")
                    return

                # Convert SDK messages to gRPC events
                for event in self._convert_message(task_id, message):
                    yield event
                    if isinstance(message, ResultMessage):
                        has_result = True

            # If SDK didn't yield a ResultMessage, send a fallback TASK_COMPLETE
            if not has_result:
                yield _make_event(task_id, "TASK_COMPLETE", "")

            self.completed_count += 1

        except Exception as e:
            logger.exception("Error executing task %s", task_id)
            yield _make_event(task_id, "TASK_ERROR", f"Execution error: {e}")
        finally:
            self.cancellation_flags.pop(task_id, None)
            self.active_agents.pop(task_id, None)

    def _convert_message(self, task_id: str, message):
        """Convert a Claude Code SDK message to gRPC TaskEvent responses."""
        if isinstance(message, AssistantMessage):
            for block in message.content:
                if isinstance(block, TextBlock):
                    yield _make_event(task_id, "ASSISTANT_MESSAGE", block.text)
                elif isinstance(block, ToolUseBlock):
                    yield _make_event(
                        task_id, "TOOL_USE", block.name,
                        {"tool": block.name, "tool_use_id": block.id},
                    )
                elif isinstance(block, ToolResultBlock):
                    content = block.content if isinstance(block.content, str) else str(block.content)
                    yield _make_event(task_id, "TOOL_RESULT", content[:2000])
        elif isinstance(message, ResultMessage):
            # ResultMessage is the final SDK output. Store metadata on it and
            # use TASK_COMPLETE (if success) or TASK_ERROR (if error) so the Go
            # client can distinguish the final result from intermediate messages.
            # Extract token counts from usage dict if available
            usage = message.usage or {}
            total_tokens = (
                usage.get("input_tokens", 0)
                + usage.get("output_tokens", 0)
                + usage.get("cache_creation_input_tokens", 0)
                + usage.get("cache_read_input_tokens", 0)
            )
            meta = {
                "num_turns": str(message.num_turns),
                "session_id": message.session_id or "",
                "is_result": "true",
                "total_tokens": str(total_tokens),
            }
            if message.total_cost_usd:
                meta["cost_usd"] = f"{message.total_cost_usd:.4f}"
            if message.is_error:
                yield _make_event(
                    task_id, "TASK_ERROR",
                    message.result or "Agent returned error",
                    meta,
                )
            else:
                # Yield as TASK_COMPLETE with the actual result text.
                # The Go client uses this as the authoritative output.
                yield _make_event(
                    task_id, "TASK_COMPLETE",
                    message.result or "",
                    meta,
                )

    async def CancelAgentTask(self, request, context):
        """Cancel a running task."""
        task_id = request.task_id
        logger.info("CancelAgentTask: task_id=%s", task_id)

        entry = self.cancellation_flags.get(task_id)
        if entry:
            flag, _ = entry
            flag.set()
            return flux_pb2.CancelAgentTaskResponse(success=True)

        context.set_code(grpc.StatusCode.NOT_FOUND)
        context.set_details(f"Task {task_id} not found or already completed")
        return flux_pb2.CancelAgentTaskResponse(success=False)

    async def GetPodStatus(self, request, context):
        """Return status of agent pods."""
        pods = []
        for agent_type in AGENT_CONFIGS:
            current_task = next(
                (t for t, a in self.active_agents.items() if a == agent_type), ""
            )
            pods.append(
                flux_pb2.AgentPod(
                    agent_type=agent_type,
                    status="running" if current_task else "idle",
                    current_task_id=current_task,
                    uptime_seconds=int(time.time() - _start_time),
                    tasks_completed=self.completed_count,
                )
            )
        return flux_pb2.GetPodStatusResponse(pods=pods)


async def serve(port: int = 50051) -> None:
    """Start the async gRPC server with graceful shutdown."""
    server = grpc_aio.server(options=[
        # Allow the Go client's 30-second keepalive pings.
        # Default enforcement minimum is 5 minutes, which causes GOAWAY errors.
        ("grpc.keepalive_permit_without_calls", True),
        ("grpc.http2.min_ping_interval_without_data_ms", 10_000),  # 10s minimum
    ])
    flux_pb2_grpc.add_AgentExecutionServiceServicer_to_server(
        AgentExecutionServicer(), server
    )
    server.add_insecure_port(f"[::]:{port}")

    loop = asyncio.get_running_loop()
    stop_event = asyncio.Event()

    def _handle_signal():
        logger.info("Received shutdown signal, stopping gracefully...")
        stop_event.set()

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, _handle_signal)

    await server.start()
    logger.info("Agent Manager gRPC server listening on :%d", port)

    await stop_event.wait()
    await server.stop(grace=5)
    logger.info("Server stopped.")


if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    port = int(os.environ.get("AGENT_MANAGER_PORT", "50051"))
    asyncio.run(serve(port))
