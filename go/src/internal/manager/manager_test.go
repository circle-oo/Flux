package manager

import (
	"sync"
	"testing"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/testutil"
)

func TestPopNextTask_Priority(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create tasks with different priorities
	tasks := []*models.Task{
		{Title: "Low Priority", Type: models.TaskTypeCoding, Status: models.TaskReady, Priority: 50},
		{Title: "High Priority", Type: models.TaskTypeCoding, Status: models.TaskReady, Priority: 10},
		{Title: "Medium Priority", Type: models.TaskTypeCoding, Status: models.TaskReady, Priority: 30},
	}

	for _, task := range tasks {
		if err := mgr.CreateTask(task); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	// Pop next task - should get highest priority (lowest number)
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "High Priority" {
		t.Errorf("expected High Priority task, got %s", next.Title)
	}

	if next.Status != models.TaskRunning {
		t.Errorf("expected status RUNNING, got %s", next.Status)
	}

	if next.StartedAt == "" {
		t.Error("expected started_at to be set")
	}
}

func TestPopNextTask_Concurrent(t *testing.T) {
	// Use a file-based temp DB for concurrent access testing
	// (in-memory SQLite creates separate DBs per connection in Go's pool)
	tmpDir := t.TempDir()
	database, err := testutil.NewTestDBFile(tmpDir)
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	cfg := &config.Config{}
	mgr := NewManager(database, cfg)

	// Create 10 READY tasks
	for i := 0; i < 10; i++ {
		task := &models.Task{
			Title:    "Concurrent Task",
			Type:     models.TaskTypeCoding,
			Status:   models.TaskReady,
			Priority: 50,
		}
		if err := mgr.CreateTask(task); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	// Launch 10 concurrent goroutines to pop tasks
	var wg sync.WaitGroup
	results := make(chan *models.Task, 10)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := mgr.PopNextTask("EXECUTOR")
			if err != nil {
				errors <- err
				return
			}
			if task != nil {
				results <- task
			}
		}()
	}

	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("pop next task error: %v", err)
	}

	// Collect task IDs
	taskIDs := make(map[string]bool)
	for task := range results {
		if taskIDs[task.ID] {
			t.Errorf("duplicate task ID: %s", task.ID)
		}
		taskIDs[task.ID] = true
	}

	// Should have exactly 10 unique tasks
	if len(taskIDs) != 10 {
		t.Errorf("expected 10 unique tasks, got %d", len(taskIDs))
	}
}

func TestPopNextTask_PodTypeFiltering(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create RESEARCH task
	researchTask := &models.Task{
		Title:    "Research Task",
		Type:     models.TaskTypeResearch,
		Status:   models.TaskReady,
		Priority: 10,
	}
	if err := mgr.CreateTask(researchTask); err != nil {
		t.Fatalf("create research task: %v", err)
	}

	// Create CODING task
	codingTask := &models.Task{
		Title:    "Coding Task",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskReady,
		Priority: 20,
	}
	if err := mgr.CreateTask(codingTask); err != nil {
		t.Fatalf("create coding task: %v", err)
	}

	// EXECUTOR should get coding task (not research)
	execTask, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop executor task: %v", err)
	}
	if execTask == nil {
		t.Fatal("expected executor task, got nil")
	}
	if execTask.Type == models.TaskTypeResearch {
		t.Error("executor should not get research tasks")
	}

	// RESEARCHER should get research task
	resTask, err := mgr.PopNextTask("RESEARCHER")
	if err != nil {
		t.Fatalf("pop researcher task: %v", err)
	}
	if resTask == nil {
		t.Fatal("expected researcher task, got nil")
	}
	if resTask.Type != models.TaskTypeResearch {
		t.Errorf("researcher should get research tasks, got %s", resTask.Type)
	}
}

func TestTransitionTask_ValidTransitions(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Test Task",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskPending,
		Priority: 50,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// PENDING -> READY
	if err := mgr.TransitionTask(task.ID, models.TaskReady); err != nil {
		t.Errorf("transition to READY: %v", err)
	}

	// READY -> RUNNING
	if err := mgr.TransitionTask(task.ID, models.TaskRunning); err != nil {
		t.Errorf("transition to RUNNING: %v", err)
	}

	// Verify started_at is set
	updated, _ := mgr.GetTask(task.ID)
	if updated.StartedAt == "" {
		t.Error("expected started_at to be set")
	}

	// RUNNING -> COMPLETED
	if err := mgr.TransitionTask(task.ID, models.TaskCompleted); err != nil {
		t.Errorf("transition to COMPLETED: %v", err)
	}

	// Verify completed_at is set
	updated, _ = mgr.GetTask(task.ID)
	if updated.CompletedAt == "" {
		t.Error("expected completed_at to be set")
	}
}

func TestTransitionTask_InvalidTransitions(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Test Task",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskPending,
		Priority: 50,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// PENDING -> RUNNING (invalid, must go through READY)
	err := mgr.TransitionTask(task.ID, models.TaskRunning)
	if err == nil {
		t.Error("expected error for invalid transition PENDING -> RUNNING")
	}

	// PENDING -> COMPLETED (invalid)
	err = mgr.TransitionTask(task.ID, models.TaskCompleted)
	if err == nil {
		t.Error("expected error for invalid transition PENDING -> COMPLETED")
	}
}

func TestTransitionTask_RetryLimit(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:      "Test Task",
		Type:       models.TaskTypeCoding,
		Status:     models.TaskReady,
		Priority:   50,
		RetryCount: 0,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Transition to RUNNING then FAILED
	mgr.TransitionTask(task.ID, models.TaskRunning)
	mgr.TransitionTask(task.ID, models.TaskFailed)

	// Retry 1
	if err := mgr.TransitionTask(task.ID, models.TaskRetry); err != nil {
		t.Errorf("retry 1: %v", err)
	}
	updated, _ := mgr.GetTask(task.ID)
	if updated.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", updated.RetryCount)
	}

	// Complete cycle for retry 2 and 3
	mgr.TransitionTask(task.ID, models.TaskRunning)
	mgr.TransitionTask(task.ID, models.TaskFailed)
	mgr.TransitionTask(task.ID, models.TaskRetry) // retry 2

	mgr.TransitionTask(task.ID, models.TaskRunning)
	mgr.TransitionTask(task.ID, models.TaskFailed)
	mgr.TransitionTask(task.ID, models.TaskRetry) // retry 3

	updated, _ = mgr.GetTask(task.ID)
	if updated.RetryCount != 3 {
		t.Errorf("expected retry_count 3, got %d", updated.RetryCount)
	}

	// Attempt retry 4 - should fail
	mgr.TransitionTask(task.ID, models.TaskRunning)
	mgr.TransitionTask(task.ID, models.TaskFailed)
	err := mgr.TransitionTask(task.ID, models.TaskRetry)
	if err == nil {
		t.Error("expected error for retry limit exceeded")
	}
}

func TestTransitionTask_RetryCrashRecovery(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:         "Test Task",
		Type:          models.TaskTypeCoding,
		Status:        models.TaskFailed,
		Priority:      50,
		RetryCount:    2,
		CrashRecovery: true,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Retry with crash_recovery=true should NOT increment retry_count
	if err := mgr.TransitionTask(task.ID, models.TaskRetry); err != nil {
		t.Errorf("retry with crash recovery: %v", err)
	}

	updated, _ := mgr.GetTask(task.ID)
	if updated.RetryCount != 2 {
		t.Errorf("expected retry_count to stay 2, got %d", updated.RetryCount)
	}
	if updated.CrashRecovery {
		t.Error("expected crash_recovery to be reset to false")
	}
}

func TestTransitionTask_CancelledNoRetry(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:      "Test Task",
		Type:       models.TaskTypeCoding,
		Status:     models.TaskFailed,
		Priority:   50,
		ErrorLog:   "cancelled by operator",
		RetryCount: 0,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Attempt to retry cancelled task - should fail
	err := mgr.TransitionTask(task.ID, models.TaskRetry)
	if err == nil {
		t.Error("expected error for retrying cancelled task")
	}
}

func TestCountByStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create tasks with different statuses
	statuses := []string{models.TaskReady, models.TaskReady, models.TaskRunning, models.TaskCompleted}
	for _, status := range statuses {
		task := &models.Task{
			Title:    "Test Task",
			Type:     models.TaskTypeCoding,
			Status:   status,
			Priority: 50,
		}
		mgr.CreateTask(task)
	}

	count, err := mgr.CountByStatus(models.TaskReady)
	if err != nil {
		t.Fatalf("count by status: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 READY tasks, got %d", count)
	}
}

func TestCountByPriority(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create tasks with different priorities
	priorities := []int{5, 10, 15, 50, 100}
	for _, pri := range priorities {
		task := &models.Task{
			Title:    "Test Task",
			Type:     models.TaskTypeCoding,
			Status:   models.TaskReady,
			Priority: pri,
		}
		mgr.CreateTask(task)
	}

	// Count high priority tasks (1-20)
	count, err := mgr.CountByPriority(1, 20)
	if err != nil {
		t.Fatalf("count by priority: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 high priority tasks, got %d", count)
	}
}

func TestListByPRStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create tasks with different PR statuses
	task1 := &models.Task{
		Title:    "Task 1",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskCompleted,
		Priority: 50,
		PRStatus: "PENDING",
	}
	task2 := &models.Task{
		Title:    "Task 2",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskCompleted,
		Priority: 50,
		PRStatus: "PENDING",
	}
	task3 := &models.Task{
		Title:    "Task 3",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskCompleted,
		Priority: 50,
		PRStatus: "MERGED",
	}

	mgr.CreateTask(task1)
	mgr.CreateTask(task2)
	mgr.CreateTask(task3)

	pending, err := mgr.ListByPRStatus("PENDING")
	if err != nil {
		t.Fatalf("list by pr status: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 PENDING tasks, got %d", len(pending))
	}
}
