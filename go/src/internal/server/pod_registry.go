package server

import (
	"sync"
	"time"
)

// PodInfo represents the runtime status of an executor pod.
type PodInfo struct {
	ID          string    `json:"id"`
	PodType     string    `json:"pod_type"` // "executor" or "researcher"
	Status      string    `json:"status"`   // "idle" or "busy"
	CurrentTask string    `json:"current_task"`
	TaskTitle   string    `json:"task_title"`
	StartedAt   time.Time `json:"started_at"`
	LastSeen    time.Time `json:"last_seen"`
	TaskCount   int       `json:"task_count"` // total tasks completed by this pod
}

// PodRegistry tracks active executor pods and their current status.
type PodRegistry struct {
	mu   sync.RWMutex
	pods map[string]*PodInfo
}

// NewPodRegistry creates a new PodRegistry.
func NewPodRegistry() *PodRegistry {
	return &PodRegistry{
		pods: make(map[string]*PodInfo),
	}
}

// Register adds or updates a pod's registration.
func (pr *PodRegistry) Register(id string, startedAt time.Time, podType string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if podType == "" {
		podType = "executor"
	}

	if _, exists := pr.pods[id]; !exists {
		pr.pods[id] = &PodInfo{
			ID:        id,
			PodType:   podType,
			Status:    "idle",
			StartedAt: startedAt,
			LastSeen:  time.Now(),
			TaskCount: 0,
		}
	} else {
		pr.pods[id].LastSeen = time.Now()
		pr.pods[id].PodType = podType
	}
}

// UpdateStatus updates a pod's current task status.
func (pr *PodRegistry) UpdateStatus(id string, status string, taskID string, taskTitle string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	if pod, exists := pr.pods[id]; exists {
		pod.Status = status
		pod.CurrentTask = taskID
		pod.TaskTitle = taskTitle
		pod.LastSeen = time.Now()

		// Increment task count when a task starts
		if status == "busy" && taskID != "" {
			pod.TaskCount++
		}
	}
}

// SetIdle marks a pod as idle (no current task).
func (pr *PodRegistry) SetIdle(id string) {
	pr.UpdateStatus(id, "idle", "", "")
}

// List returns all registered pods.
func (pr *PodRegistry) List() []PodInfo {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	result := make([]PodInfo, 0, len(pr.pods))
	for _, pod := range pr.pods {
		result = append(result, *pod)
	}
	return result
}

// Get returns a specific pod's info.
func (pr *PodRegistry) Get(id string) (*PodInfo, bool) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	pod, exists := pr.pods[id]
	if !exists {
		return nil, false
	}
	// Return a copy to avoid race conditions
	podCopy := *pod
	return &podCopy, true
}

// CleanStale removes pods that haven't sent a heartbeat in the specified duration.
func (pr *PodRegistry) CleanStale(staleDuration time.Duration) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	now := time.Now()
	for id, pod := range pr.pods {
		if now.Sub(pod.LastSeen) > staleDuration {
			delete(pr.pods, id)
		}
	}
}
