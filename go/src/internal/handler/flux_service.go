// Package handler implements the Connect-RPC FluxService, providing a typed
// gRPC/Connect API for agent task creation and streaming. This is the Connect-RPC
// path — separate from the REST API in internal/server which serves the full
// orchestration UI. The two API surfaces will converge in Phase 3 when the
// in-memory store (internal/store) is unified with the SQLite-backed models.
package handler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"

	fluxv1 "github.com/circle-oo/flux/gen/flux/v1"
	"github.com/circle-oo/flux/internal/agent"
	"github.com/circle-oo/flux/internal/orchestrator"
	"github.com/circle-oo/flux/internal/store"
)

// FluxServiceHandler implements the Connect-RPC FluxService.
type FluxServiceHandler struct {
	agent        *agent.Client
	store        *store.TaskStore
	logger       *slog.Logger
	orchestrator *orchestrator.Orchestrator
	scaleManager *orchestrator.ScaleManager
}

// NewFluxServiceHandler creates a new handler with the given agent client and logger.
func NewFluxServiceHandler(
	agentClient *agent.Client,
	logger *slog.Logger,
) *FluxServiceHandler {
	return &FluxServiceHandler{
		agent:  agentClient,
		store:  store.New(),
		logger: logger,
	}
}

// SetOrchestrator sets the orchestrator for status queries.
func (h *FluxServiceHandler) SetOrchestrator(o *orchestrator.Orchestrator) {
	h.orchestrator = o
}

// SetScaleManager sets the scale manager for status queries.
func (h *FluxServiceHandler) SetScaleManager(sm *orchestrator.ScaleManager) {
	h.scaleManager = sm
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
	task, events := h.store.Get(req.Msg.GetTaskId())
	if task == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
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
	err := h.agent.CancelTask(ctx, req.Msg.GetTaskId())
	if err != nil {
		h.logger.Error("cancel task failed", "task_id", req.Msg.GetTaskId(), "error", err)
	}
	h.store.UpdateStatus(req.Msg.GetTaskId(), fluxv1.TaskStatus_TASK_STATUS_CANCELLED)
	return connect.NewResponse(&fluxv1.CancelTaskResponse{
		Success: err == nil,
	}), nil
}

func (h *FluxServiceHandler) StreamTaskEvents(
	ctx context.Context,
	req *connect.Request[fluxv1.StreamTaskEventsRequest],
	stream *connect.ServerStream[fluxv1.StreamTaskEventsResponse],
) error {
	eventCh := h.store.Subscribe(req.Msg.GetTaskId())
	defer h.store.Unsubscribe(req.Msg.GetTaskId(), eventCh)

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return nil
			}
			if err := stream.Send(&fluxv1.StreamTaskEventsResponse{
				Event: event,
			}); err != nil {
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

	resp := &fluxv1.GetDashboardResponse{
		TotalTasksToday:     stats.Total,
		CompletedTasksToday: stats.Completed,
		FailedTasksToday:    stats.Failed,
		AgentSummaries:      stats.Summaries,
	}

	podStatus, err := h.agent.PodStatus(ctx)
	if err == nil && podStatus != nil {
		resp.ActiveAgents = int32(len(podStatus.GetPods()))
	}

	return connect.NewResponse(resp), nil
}

func (h *FluxServiceHandler) GetInsights(
	ctx context.Context,
	req *connect.Request[fluxv1.GetInsightsRequest],
) (*connect.Response[fluxv1.GetInsightsResponse], error) {
	insights := h.store.GetInsights(req.Msg.GetTimeRange())
	return connect.NewResponse(insights), nil
}

func (h *FluxServiceHandler) GetAgentStatus(
	ctx context.Context,
	req *connect.Request[fluxv1.GetAgentStatusRequest],
) (*connect.Response[fluxv1.GetAgentStatusResponse], error) {
	status, err := h.agent.PodStatus(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&fluxv1.GetAgentStatusResponse{
		Pods: status.GetPods(),
	}), nil
}

func (h *FluxServiceHandler) GetOrchestratorStatus(
	ctx context.Context,
	req *connect.Request[fluxv1.GetOrchestratorStatusRequest],
) (*connect.Response[fluxv1.GetOrchestratorStatusResponse], error) {
	resp := &fluxv1.GetOrchestratorStatusResponse{}

	if h.orchestrator != nil {
		status := h.orchestrator.Status()
		resp.Running = status.Running
		resp.TickCount = int32(status.TickCount)

		if !status.StartedAt.IsZero() {
			resp.Uptime = fmt.Sprintf("%s", time.Since(status.StartedAt).Round(time.Second))
		}

		for _, ch := range status.Components {
			var lastTick string
			if !ch.LastTick.IsZero() {
				lastTick = ch.LastTick.Format(time.RFC3339)
			}
			resp.SubComponents = append(resp.SubComponents, &fluxv1.SubComponentStatus{
				Name:      ch.Name,
				Healthy:   ch.Healthy,
				LastTick:  lastTick,
				LastError: ch.LastError,
			})
		}
	}

	if h.scaleManager != nil {
		resp.ScaleStatus = h.scaleManager.Status()
	}

	return connect.NewResponse(resp), nil
}

func (h *FluxServiceHandler) executeInBackground(task *fluxv1.Task) {
	ctx := context.Background()

	h.store.UpdateStatus(task.GetId(), fluxv1.TaskStatus_TASK_STATUS_RUNNING)

	req := &fluxv1.ExecuteTaskRequest{
		TaskId:           task.GetId(),
		AgentType:        task.GetAgentType(),
		Prompt:           task.GetPrompt(),
		WorkingDirectory: task.GetWorkingDirectory(),
		Metadata:         task.GetMetadata(),
	}

	result, err := h.agent.ExecuteTask(ctx, req, func(event *fluxv1.TaskEvent) {
		h.store.RecordEvent(task.GetId(), event)
		h.store.Broadcast(task.GetId(), event)
	})

	if err != nil {
		h.logger.Error("task execution failed", "task_id", task.GetId(), "error", err)
		h.store.UpdateStatus(task.GetId(), fluxv1.TaskStatus_TASK_STATUS_FAILED)
	} else if result != nil && result.IsError {
		h.store.UpdateStatus(task.GetId(), fluxv1.TaskStatus_TASK_STATUS_FAILED)
	} else {
		h.store.UpdateStatus(task.GetId(), fluxv1.TaskStatus_TASK_STATUS_COMPLETED)
	}
}
