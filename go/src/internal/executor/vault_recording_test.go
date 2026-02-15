package executor

import (
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/models"
)

func TestBuildTaskSummaryData(t *testing.T) {
	task := &models.Task{
		ID:           "task-abc123",
		Title:        "Implement feature X",
		Type:         models.TaskTypeCoding,
		Status:       models.TaskCompleted,
		Priority:     5,
		Model:        "sonnet",
		CreatedAt:    "2026-02-15T10:00:00Z",
		CompletedAt:  "2026-02-15T10:30:00Z",
		PRUrl:        "https://github.com/org/repo/pull/123",
		PRStatus:     "merged",
		DiffLines:    150,
		FilesChanged: 5,
		Result:       "Feature implemented successfully",
	}

	result := &ClaudeCodeResult{
		Duration: 30 * time.Minute,
	}

	data := buildTaskSummaryData(task, result)

	if data.ID != task.ID {
		t.Errorf("ID mismatch: got %s, want %s", data.ID, task.ID)
	}
	if data.Title != task.Title {
		t.Errorf("Title mismatch: got %s, want %s", data.Title, task.Title)
	}
	if data.Type != task.Type {
		t.Errorf("Type mismatch: got %s, want %s", data.Type, task.Type)
	}
	if data.Status != task.Status {
		t.Errorf("Status mismatch: got %s, want %s", data.Status, task.Status)
	}
	if data.Priority != task.Priority {
		t.Errorf("Priority mismatch: got %d, want %d", data.Priority, task.Priority)
	}
	if data.Model != task.Model {
		t.Errorf("Model mismatch: got %s, want %s", data.Model, task.Model)
	}
	if data.Duration != "30m0s" {
		t.Errorf("Duration mismatch: got %s, want 30m0s", data.Duration)
	}
	if data.CreatedAt != task.CreatedAt {
		t.Errorf("CreatedAt mismatch: got %s, want %s", data.CreatedAt, task.CreatedAt)
	}
	if data.CompletedAt != task.CompletedAt {
		t.Errorf("CompletedAt mismatch: got %s, want %s", data.CompletedAt, task.CompletedAt)
	}
	if data.PRUrl != task.PRUrl {
		t.Errorf("PRUrl mismatch: got %s, want %s", data.PRUrl, task.PRUrl)
	}
	if data.PRStatus != task.PRStatus {
		t.Errorf("PRStatus mismatch: got %s, want %s", data.PRStatus, task.PRStatus)
	}
	if data.DiffLines != task.DiffLines {
		t.Errorf("DiffLines mismatch: got %d, want %d", data.DiffLines, task.DiffLines)
	}
	if data.FilesChanged != task.FilesChanged {
		t.Errorf("FilesChanged mismatch: got %d, want %d", data.FilesChanged, task.FilesChanged)
	}
	if data.Result != task.Result {
		t.Errorf("Result mismatch: got %s, want %s", data.Result, task.Result)
	}
}

func TestBuildTaskSummaryDataWithDefaults(t *testing.T) {
	task := &models.Task{
		ID:       "task-def456",
		Title:    "Task with defaults",
		Type:     models.TaskTypeResearch,
		Status:   models.TaskCompleted,
		Priority: 10,
		Model:    "opus",
	}

	result := &ClaudeCodeResult{}

	data := buildTaskSummaryData(task, result)

	if data.Duration != "unknown" {
		t.Errorf("Duration should be 'unknown', got %s", data.Duration)
	}
	if data.PRStatus != "not created" {
		t.Errorf("PRStatus should be 'not created', got %s", data.PRStatus)
	}
	if data.PRUrl != "N/A" {
		t.Errorf("PRUrl should be 'N/A', got %s", data.PRUrl)
	}
	if data.Result != "No result recorded" {
		t.Errorf("Result should be 'No result recorded', got %s", data.Result)
	}
}

func TestBuildTaskSummaryDataUsesResultStdout(t *testing.T) {
	task := &models.Task{
		ID:       "task-ghi789",
		Title:    "Task with stdout",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskCompleted,
		Priority: 5,
		Model:    "sonnet",
		Result:   "", // Empty Result field
	}

	result := &ClaudeCodeResult{
		Stdout:   "Claude Code output here",
		Duration: 10 * time.Minute,
	}

	data := buildTaskSummaryData(task, result)

	if data.Result != result.Stdout {
		t.Errorf("Result should use stdout when Result is empty, got %s", data.Result)
	}
}
