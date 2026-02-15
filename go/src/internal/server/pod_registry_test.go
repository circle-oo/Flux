package server

import (
	"testing"
	"time"
)

func TestPodRegistry_RegisterAndList(t *testing.T) {
	pr := NewPodRegistry()
	startTime := time.Now()

	// Register a pod
	pr.Register("executor-01", startTime)

	// List should return one pod
	pods := pr.List()
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}

	pod := pods[0]
	if pod.ID != "executor-01" {
		t.Errorf("expected ID 'executor-01', got '%s'", pod.ID)
	}
	if pod.Status != "idle" {
		t.Errorf("expected status 'idle', got '%s'", pod.Status)
	}
	if pod.TaskCount != 0 {
		t.Errorf("expected task count 0, got %d", pod.TaskCount)
	}
}

func TestPodRegistry_UpdateStatus(t *testing.T) {
	pr := NewPodRegistry()
	startTime := time.Now()

	pr.Register("executor-01", startTime)

	// Update to busy
	pr.UpdateStatus("executor-01", "busy", "task-123", "Test Task")

	pod, exists := pr.Get("executor-01")
	if !exists {
		t.Fatal("pod not found")
	}

	if pod.Status != "busy" {
		t.Errorf("expected status 'busy', got '%s'", pod.Status)
	}
	if pod.CurrentTask != "task-123" {
		t.Errorf("expected current task 'task-123', got '%s'", pod.CurrentTask)
	}
	if pod.TaskTitle != "Test Task" {
		t.Errorf("expected task title 'Test Task', got '%s'", pod.TaskTitle)
	}
	if pod.TaskCount != 1 {
		t.Errorf("expected task count 1, got %d", pod.TaskCount)
	}

	// Update back to idle
	pr.SetIdle("executor-01")

	pod, _ = pr.Get("executor-01")
	if pod.Status != "idle" {
		t.Errorf("expected status 'idle', got '%s'", pod.Status)
	}
	if pod.CurrentTask != "" {
		t.Errorf("expected empty current task, got '%s'", pod.CurrentTask)
	}
	// Task count should remain the same
	if pod.TaskCount != 1 {
		t.Errorf("expected task count 1, got %d", pod.TaskCount)
	}
}

func TestPodRegistry_CleanStale(t *testing.T) {
	pr := NewPodRegistry()
	startTime := time.Now()

	// Register two pods
	pr.Register("executor-01", startTime)
	pr.Register("executor-02", startTime)

	// Manually set one pod's last seen to be very old
	pr.pods["executor-01"].LastSeen = time.Now().Add(-2 * time.Hour)

	// Clean stale pods (anything not seen in last hour)
	pr.CleanStale(1 * time.Hour)

	// Should only have executor-02 left
	pods := pr.List()
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod after cleanup, got %d", len(pods))
	}

	if pods[0].ID != "executor-02" {
		t.Errorf("expected remaining pod to be 'executor-02', got '%s'", pods[0].ID)
	}
}
