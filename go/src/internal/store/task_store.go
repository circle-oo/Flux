// Package store provides an in-memory task store for the Connect-RPC FluxService.
//
// This is a lightweight staging layer for tasks created via the Connect-RPC API
// (Python Agent Manager path). It is NOT a replacement for the SQLite-backed
// models.TaskStore used by the REST API and executor/triager orchestration.
//
// The two stores serve different purposes:
//   - models.TaskStore (SQLite): persistent, used by REST API, executors, triagers
//   - store.TaskStore (in-memory): ephemeral, used by Connect-RPC FluxService handler
//
// TODO(phase3): Unify by wiring Connect-RPC handler to SQLite-backed stores.
package store

import (
	"sync"
	"time"

	fluxv1 "github.com/circle-oo/flux/gen/flux/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TaskStore manages in-memory task state and event pubsub for Connect-RPC.
type TaskStore struct {
	mu     sync.RWMutex
	tasks  map[string]*fluxv1.Task
	events map[string][]*fluxv1.TaskEvent

	subMu       sync.RWMutex
	subscribers map[string]map[chan *fluxv1.TaskEvent]struct{}
}

// New creates a new TaskStore.
func New() *TaskStore {
	return &TaskStore{
		tasks:       make(map[string]*fluxv1.Task),
		events:      make(map[string][]*fluxv1.TaskEvent),
		subscribers: make(map[string]map[chan *fluxv1.TaskEvent]struct{}),
	}
}

// Create stores a new task from a CreateTaskRequest and returns it.
func (s *TaskStore) Create(req *fluxv1.CreateTaskRequest) *fluxv1.Task {
	now := timestamppb.Now()
	task := &fluxv1.Task{
		Id:               uuid.New().String(),
		AgentType:        req.GetAgentType(),
		Prompt:           req.GetPrompt(),
		Status:           fluxv1.TaskStatus_TASK_STATUS_PENDING,
		Priority:         req.GetPriority(),
		WorkingDirectory: req.GetWorkingDirectory(),
		Metadata:         req.GetMetadata(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	s.mu.Lock()
	s.tasks[task.Id] = task
	s.mu.Unlock()

	return task
}

// Get returns a task and its recent events.
func (s *TaskStore) Get(taskID string) (*fluxv1.Task, []*fluxv1.TaskEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[taskID], s.events[taskID]
}

// List returns tasks matching filters with pagination.
func (s *TaskStore) List(req *fluxv1.ListTasksRequest) ([]*fluxv1.Task, string, int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*fluxv1.Task
	for _, t := range s.tasks {
		if req.GetStatusFilter() != fluxv1.TaskStatus_TASK_STATUS_UNSPECIFIED &&
			t.GetStatus() != req.GetStatusFilter() {
			continue
		}
		if req.GetAgentTypeFilter() != "" && t.GetAgentType() != req.GetAgentTypeFilter() {
			continue
		}
		filtered = append(filtered, t)
	}

	total := int32(len(filtered))
	pageSize := req.GetPageSize()
	if pageSize <= 0 {
		pageSize = 50
	}

	// Simple offset-based pagination using page_token as offset index.
	start := 0
	if req.GetPageToken() != "" {
		for i, t := range filtered {
			if t.GetId() == req.GetPageToken() {
				start = i
				break
			}
		}
	}

	end := start + int(pageSize)
	if end > len(filtered) {
		end = len(filtered)
	}

	page := filtered[start:end]
	nextToken := ""
	if end < len(filtered) {
		nextToken = filtered[end].GetId()
	}

	return page, nextToken, total
}

// UpdateStatus updates a task's status.
func (s *TaskStore) UpdateStatus(taskID string, status fluxv1.TaskStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.tasks[taskID]; ok {
		t.Status = status
		t.UpdatedAt = timestamppb.Now()
		if status == fluxv1.TaskStatus_TASK_STATUS_COMPLETED ||
			status == fluxv1.TaskStatus_TASK_STATUS_FAILED ||
			status == fluxv1.TaskStatus_TASK_STATUS_CANCELLED {
			t.CompletedAt = timestamppb.Now()
		}
	}
}

// RecordEvent appends an event to a task's event history.
func (s *TaskStore) RecordEvent(taskID string, event *fluxv1.TaskEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events[taskID] = append(s.events[taskID], event)
}

// Subscribe creates an event channel for a task.
func (s *TaskStore) Subscribe(taskID string) chan *fluxv1.TaskEvent {
	ch := make(chan *fluxv1.TaskEvent, 64)
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if s.subscribers[taskID] == nil {
		s.subscribers[taskID] = make(map[chan *fluxv1.TaskEvent]struct{})
	}
	s.subscribers[taskID][ch] = struct{}{}
	return ch
}

// Unsubscribe removes and closes an event channel.
func (s *TaskStore) Unsubscribe(taskID string, ch chan *fluxv1.TaskEvent) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if subs, ok := s.subscribers[taskID]; ok {
		delete(subs, ch)
		if len(subs) == 0 {
			delete(s.subscribers, taskID)
		}
	}
	close(ch)
}

// Broadcast sends an event to all subscribers for a task.
func (s *TaskStore) Broadcast(taskID string, event *fluxv1.TaskEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()
	for ch := range s.subscribers[taskID] {
		select {
		case ch <- event:
		default:
			// Drop if subscriber is slow.
		}
	}
}

// DailyStats holds aggregated daily task statistics.
type DailyStats struct {
	Total     int32
	Completed int32
	Failed    int32
	Summaries []*fluxv1.AgentSummary
}

// GetDailyStats returns aggregated stats for tasks created today.
func (s *TaskStore) GetDailyStats() DailyStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	today := time.Now().Truncate(24 * time.Hour)
	agentStats := make(map[string]*fluxv1.AgentSummary)

	var stats DailyStats
	for _, t := range s.tasks {
		if t.GetCreatedAt() == nil || t.GetCreatedAt().AsTime().Before(today) {
			continue
		}
		stats.Total++
		switch t.GetStatus() {
		case fluxv1.TaskStatus_TASK_STATUS_COMPLETED:
			stats.Completed++
		case fluxv1.TaskStatus_TASK_STATUS_FAILED:
			stats.Failed++
		}

		as, ok := agentStats[t.GetAgentType()]
		if !ok {
			as = &fluxv1.AgentSummary{AgentType: t.GetAgentType()}
			agentStats[t.GetAgentType()] = as
		}
		switch t.GetStatus() {
		case fluxv1.TaskStatus_TASK_STATUS_COMPLETED:
			as.TasksCompleted++
		case fluxv1.TaskStatus_TASK_STATUS_FAILED:
			as.TasksFailed++
		}
	}

	for _, as := range agentStats {
		stats.Summaries = append(stats.Summaries, as)
	}
	return stats
}

// GetInsights returns insight metrics for the given time range.
func (s *TaskStore) GetInsights(timeRange string) *fluxv1.GetInsightsResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cutoff time.Time
	switch timeRange {
	case "7d":
		cutoff = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		cutoff = time.Now().Add(-30 * 24 * time.Hour)
	default: // "24h"
		cutoff = time.Now().Add(-24 * time.Hour)
	}

	dailyMap := make(map[string]*fluxv1.DailyMetric)
	agentMap := make(map[string]*struct {
		completed int
		failed    int
		turns     float64
		duration  float64
		count     int
	})

	for _, t := range s.tasks {
		if t.GetCreatedAt() == nil || t.GetCreatedAt().AsTime().Before(cutoff) {
			continue
		}
		date := t.GetCreatedAt().AsTime().Format("2006-01-02")
		dm, ok := dailyMap[date]
		if !ok {
			dm = &fluxv1.DailyMetric{Date: date}
			dailyMap[date] = dm
		}
		switch t.GetStatus() {
		case fluxv1.TaskStatus_TASK_STATUS_COMPLETED:
			dm.TasksCompleted++
		case fluxv1.TaskStatus_TASK_STATUS_FAILED:
			dm.TasksFailed++
		}

		if t.GetCompletedAt() != nil && t.GetCreatedAt() != nil {
			dur := t.GetCompletedAt().AsTime().Sub(t.GetCreatedAt().AsTime()).Seconds()
			dm.TotalDurationSeconds += dur

			ap := agentMap[t.GetAgentType()]
			if ap == nil {
				ap = &struct {
					completed int
					failed    int
					turns     float64
					duration  float64
					count     int
				}{}
				agentMap[t.GetAgentType()] = ap
			}
			ap.duration += dur
			ap.count++
			if t.GetStatus() == fluxv1.TaskStatus_TASK_STATUS_COMPLETED {
				ap.completed++
			} else if t.GetStatus() == fluxv1.TaskStatus_TASK_STATUS_FAILED {
				ap.failed++
			}
		}
	}

	resp := &fluxv1.GetInsightsResponse{}
	for _, dm := range dailyMap {
		resp.DailyMetrics = append(resp.DailyMetrics, dm)
	}
	for agentType, ap := range agentMap {
		total := ap.completed + ap.failed
		var successRate float64
		if total > 0 {
			successRate = float64(ap.completed) / float64(total)
		}
		var avgDur float64
		if ap.count > 0 {
			avgDur = ap.duration / float64(ap.count)
		}
		resp.AgentPerformance = append(resp.AgentPerformance, &fluxv1.AgentPerformance{
			AgentType:           agentType,
			SuccessRate:         successRate,
			AvgDurationSeconds:  avgDur,
		})
	}
	return resp
}
