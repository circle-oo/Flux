"""Flux Agent Manager — async gRPC server that executes tasks via Claude Agent SDK."""

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

from config import get_agent_config, AGENT_CONFIGS  # noqa: E402

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
    """Create an ExecuteTaskResponse wrapping a TaskEvent.

    event_type is a short name like "ASSISTANT_MESSAGE" which gets prefixed
    with TASK_EVENT_TYPE_ for the enum lookup.
    """
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
    """Implements the AgentExecutionService for Go -> Python task execution."""

    def __init__(self):
        self.cancellation_flags: dict[str, asyncio.Event] = {}
        self.active_agents: dict[str, str] = {}  # task_id -> agent_type
        self.completed_count = 0

    async def ExecuteTask(self, request, context):
        """Execute a task and stream events back.

        TODO: Replace placeholder simulation with real Claude Agent SDK integration.
        The placeholder streams realistic events so the gRPC pipeline can be tested
        end-to-end before the SDK is wired in.
        """
        task_id = request.task_id
        agent_type = request.agent_type
        prompt = request.prompt

        logger.info("ExecuteTask called: task_id=%s agent_type=%s", task_id, agent_type)

        # Validate agent type
        try:
            agent_cfg = get_agent_config(agent_type)
        except KeyError as e:
            yield _make_event(task_id, "TASK_ERROR", str(e))
            return

        # Set up cancellation
        cancel_event = asyncio.Event()
        self.cancellation_flags[task_id] = cancel_event
        self.active_agents[task_id] = agent_type

        try:
            # --- PLACEHOLDER: Simulate Claude Agent SDK execution ---
            # TODO: Replace this block with real Claude Agent SDK call:
            #
            #   from claude_agent_sdk import query, ClaudeAgentOptions
            #   options = ClaudeAgentOptions(
            #       system_prompt=request.system_prompt or agent_cfg.system_prompt,
            #       allowed_tools=list(request.allowed_tools) or agent_cfg.allowed_tools,
            #       max_turns=request.max_turns or agent_cfg.max_turns,
            #       cwd=request.working_directory,
            #       permission_mode="acceptEdits",
            #   )
            #   async for message in query(prompt=prompt, options=options):
            #       if cancel_event.is_set():
            #           yield _make_event(task_id, "TASK_ERROR", "Cancelled")
            #           return
            #       event = self._to_event(task_id, message)
            #       if event:
            #           yield event
            #   yield _make_event(task_id, "TASK_COMPLETE", "Done")

            # 1. Progress: starting
            yield _make_event(
                task_id,
                "PROGRESS",
                f"Starting {agent_type} agent for task {task_id}",
                {"agent_type": agent_type, "max_turns": str(agent_cfg.max_turns)},
            )
            await asyncio.sleep(0.1)

            if cancel_event.is_set():
                yield _make_event(task_id, "TASK_ERROR", "Cancelled")
                return

            # 2. Assistant message: analyzing the prompt
            yield _make_event(
                task_id,
                "ASSISTANT_MESSAGE",
                f"I'll work on this task: {prompt[:200]}",
            )
            await asyncio.sleep(0.1)

            if cancel_event.is_set():
                yield _make_event(task_id, "TASK_ERROR", "Cancelled")
                return

            # 3. Tool use: simulated file read
            yield _make_event(
                task_id,
                "TOOL_USE",
                "Reading project structure...",
                {"tool": "Glob", "pattern": "**/*.go"},
            )
            await asyncio.sleep(0.1)

            if cancel_event.is_set():
                yield _make_event(task_id, "TASK_ERROR", "Cancelled")
                return

            # 4. Tool result
            yield _make_event(
                task_id,
                "TOOL_RESULT",
                "Found 42 Go files in the project.",
                {"tool": "Glob", "files_found": "42"},
            )
            await asyncio.sleep(0.1)

            if cancel_event.is_set():
                yield _make_event(task_id, "TASK_ERROR", "Cancelled")
                return

            # 5. Assistant message: summary
            yield _make_event(
                task_id,
                "ASSISTANT_MESSAGE",
                "I've analyzed the codebase and completed the requested changes.",
            )

            # 6. Task complete
            yield _make_event(
                task_id,
                "TASK_COMPLETE",
                "Task completed successfully (placeholder simulation).",
                {"turns_used": "3"},
            )
            # --- END PLACEHOLDER ---

            self.completed_count += 1

        except Exception as e:
            logger.exception("Error executing task %s", task_id)
            yield _make_event(task_id, "TASK_ERROR", f"Internal error: {e}")
        finally:
            self.cancellation_flags.pop(task_id, None)
            self.active_agents.pop(task_id, None)

    def _to_event(self, task_id: str, message):
        """Convert a Claude Agent SDK message to a TaskEvent response.

        TODO: Wire this up once the real SDK is integrated. The SDK message
        format has content blocks (text, tool_use) and result fields.
        """
        if hasattr(message, "content"):
            for block in message.content:
                if hasattr(block, "text"):
                    return _make_event(task_id, "ASSISTANT_MESSAGE", block.text)
                if hasattr(block, "name"):
                    return _make_event(task_id, "TOOL_USE", block.name)
        if hasattr(message, "result"):
            return _make_event(task_id, "TOOL_RESULT", str(message.result))
        return None

    async def CancelAgentTask(self, request, context):
        """Cancel a running task."""
        task_id = request.task_id
        logger.info("CancelAgentTask called: task_id=%s", task_id)

        flag = self.cancellation_flags.get(task_id)
        if flag:
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
    server = grpc_aio.server()
    flux_pb2_grpc.add_AgentExecutionServiceServicer_to_server(
        AgentExecutionServicer(), server
    )
    server.add_insecure_port(f"[::]:{port}")

    # Graceful shutdown on SIGTERM/SIGINT
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
