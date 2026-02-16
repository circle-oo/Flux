package manager

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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
		{Title: "Low Priority", Status: models.TaskReady, Priority: 50},
		{Title: "High Priority", Status: models.TaskReady, Priority: 10},
		{Title: "Medium Priority", Status: models.TaskReady, Priority: 30},
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

func TestTransitionTask_ValidTransitions(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Test Task",
		
		Status:   models.TaskPending,
		Source:   models.TaskSourceOperator, // stays PENDING (awaiting triage)
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

func TestTransitionTask_RunningToRetry(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Rate Limited Task",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// READY -> RUNNING
	if err := mgr.TransitionTask(task.ID, models.TaskRunning); err != nil {
		t.Fatalf("transition to RUNNING: %v", err)
	}

	// RUNNING -> RETRY (e.g. rate limited)
	if err := mgr.TransitionTask(task.ID, models.TaskRetry); err != nil {
		t.Errorf("transition RUNNING -> RETRY: %v", err)
	}

	updated, _ := mgr.GetTask(task.ID)
	if updated.Status != models.TaskRetry {
		t.Errorf("expected status RETRY, got %s", updated.Status)
	}
	if updated.RetryCount != 1 {
		t.Errorf("expected retry_count 1, got %d", updated.RetryCount)
	}
}

func TestTransitionTask_InvalidTransitions(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Test Task",
		
		Status:   models.TaskPending,
		Source:   models.TaskSourceOperator, // stays PENDING (awaiting triage)
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
		
		Status:   models.TaskCompleted,
		Priority: 50,
		PRStatus: "PENDING",
	}
	task2 := &models.Task{
		Title:    "Task 2",
		
		Status:   models.TaskCompleted,
		Priority: 50,
		PRStatus: "PENDING",
	}
	task3 := &models.Task{
		Title:    "Task 3",
		
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

func TestPopNextTask_GoalBoost(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create goal as PROPOSED first
	goal := &models.Goal{
		Title:       "Test Goal",
		Description: "Test goal for boost",
	}
	if err := mgr.goals.Create(goal); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// Activate the goal using proper Activate method
	if err := mgr.goals.Activate(goal.ID); err != nil {
		t.Fatalf("activate goal: %v", err)
	}

	// Verify goal is active
	currentGoal, err := mgr.GetCurrentGoal()
	if err != nil {
		t.Fatalf("get current goal: %v", err)
	}
	if currentGoal == nil {
		t.Fatal("expected active goal, got nil")
	}
	if currentGoal.ID != goal.ID {
		t.Fatalf("expected goal ID %s, got %s", goal.ID, currentGoal.ID)
	}

	// Create non-goal task first (should have earlier created_at)
	task1 := &models.Task{
		Title:    "Non-Goal Task",
		
		Status:   models.TaskReady,
		Priority: 50,
		GoalID:   "",
	}
	if err := mgr.CreateTask(task1); err != nil {
		t.Fatalf("create task1: %v", err)
	}

	// Add slight delay to ensure different created_at timestamps
	time.Sleep(10 * time.Millisecond)

	// Create goal task second (later created_at, but should still win due to boost)
	task2 := &models.Task{
		Title:    "Goal Task",
		
		Status:   models.TaskReady,
		Priority: 50,
		GoalID:   goal.ID,
	}
	if err := mgr.CreateTask(task2); err != nil {
		t.Fatalf("create task2: %v", err)
	}

	// Pop task - should get goal-related task first despite being created later
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "Goal Task" {
		t.Errorf("expected Goal Task due to boost, got %s (goal_id=%s, expected=%s)", next.Title, next.GoalID, goal.ID)
	}
}

func TestPopNextTask_GoalBoostNoCrossPriority(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create and activate a goal
	goal := &models.Goal{
		Title:       "Test Goal",
		Description: "Test goal for boost",
		Status:      models.GoalActive,
	}
	if err := mgr.goals.Create(goal); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	// Create high-priority non-goal task and lower-priority goal task
	task1 := &models.Task{
		Title:    "High Priority Non-Goal",
		
		Status:   models.TaskReady,
		Priority: 48,
		GoalID:   "",
	}
	task2 := &models.Task{
		Title:    "Lower Priority Goal Task",
		
		Status:   models.TaskReady,
		Priority: 50,
		GoalID:   goal.ID,
	}

	if err := mgr.CreateTask(task1); err != nil {
		t.Fatalf("create task1: %v", err)
	}
	if err := mgr.CreateTask(task2); err != nil {
		t.Fatalf("create task2: %v", err)
	}

	// Pop task - should get high priority task (goal boost doesn't cross priority boundaries)
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "High Priority Non-Goal" {
		t.Errorf("expected high priority task to win despite goal boost, got %s", next.Title)
	}
}

func TestPopNextTask_DependencyBlocking(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create dependency task (not completed)
	depTask := &models.Task{
		Title:    "Dependency Task",
		
		Status:   models.TaskRunning,
		Priority: 50,
	}
	if err := mgr.CreateTask(depTask); err != nil {
		t.Fatalf("create dep task: %v", err)
	}

	// Create task that depends on the first
	blockedTask := &models.Task{
		Title:     "Blocked Task",
		
		Status:    models.TaskReady,
		Priority:  40,
		DependsOn: []string{depTask.ID},
	}
	if err := mgr.CreateTask(blockedTask); err != nil {
		t.Fatalf("create blocked task: %v", err)
	}

	// Create independent task with lower priority
	independentTask := &models.Task{
		Title:    "Independent Task",
		
		Status:   models.TaskReady,
		Priority: 60,
	}
	if err := mgr.CreateTask(independentTask); err != nil {
		t.Fatalf("create independent task: %v", err)
	}

	// Pop task - should skip blocked task and get independent one
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "Independent Task" {
		t.Errorf("expected Independent Task, got %s", next.Title)
	}
}

func TestPopNextTask_DependencyResolved(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create dependency task and complete it
	depTask := &models.Task{
		Title:    "Dependency Task",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(depTask); err != nil {
		t.Fatalf("create dep task: %v", err)
	}

	// Transition to completed
	mgr.TransitionTask(depTask.ID, models.TaskRunning)
	mgr.TransitionTask(depTask.ID, models.TaskCompleted)

	// Create task that depends on the completed task
	dependentTask := &models.Task{
		Title:     "Dependent Task",
		
		Status:    models.TaskReady,
		Priority:  40,
		DependsOn: []string{depTask.ID},
	}
	if err := mgr.CreateTask(dependentTask); err != nil {
		t.Fatalf("create dependent task: %v", err)
	}

	// Pop task - should get dependent task since dependency is resolved
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "Dependent Task" {
		t.Errorf("expected Dependent Task, got %s", next.Title)
	}
}

func TestTransitionTask_DecomposedTransitions(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Decompose me",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// READY -> RUNNING
	if err := mgr.TransitionTask(task.ID, models.TaskRunning); err != nil {
		t.Fatalf("transition to RUNNING: %v", err)
	}

	// RUNNING -> DECOMPOSED
	if err := mgr.TransitionTask(task.ID, models.TaskDecomposed); err != nil {
		t.Errorf("transition RUNNING -> DECOMPOSED: %v", err)
	}

	updated, _ := mgr.GetTask(task.ID)
	if updated.Status != models.TaskDecomposed {
		t.Errorf("expected DECOMPOSED, got %s", updated.Status)
	}

	// DECOMPOSED -> COMPLETED (when all subtasks done)
	if err := mgr.TransitionTask(task.ID, models.TaskCompleted); err != nil {
		t.Errorf("transition DECOMPOSED -> COMPLETED: %v", err)
	}
}

func TestTransitionTask_DecomposedInvalidTransition(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	task := &models.Task{
		Title:    "Decompose me",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	mgr.TransitionTask(task.ID, models.TaskRunning)
	mgr.TransitionTask(task.ID, models.TaskDecomposed)

	// DECOMPOSED -> RUNNING is invalid
	err := mgr.TransitionTask(task.ID, models.TaskRunning)
	if err == nil {
		t.Error("expected error for invalid transition DECOMPOSED -> RUNNING")
	}
}

func TestCheckParentCompletion_AllCompleted(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent task and transition to DECOMPOSED
	parent := &models.Task{
		Title:    "Parent",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	mgr.TransitionTask(parent.ID, models.TaskRunning)
	mgr.TransitionTask(parent.ID, models.TaskDecomposed)

	// Create subtasks as COMPLETED
	for i := 0; i < 3; i++ {
		sub := &models.Task{
			Title:    "Subtask",
			
			Status:   models.TaskCompleted,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
		}
		if err := mgr.CreateTask(sub); err != nil {
			t.Fatalf("create subtask: %v", err)
		}
	}

	// Check parent completion
	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}

	updated, _ := mgr.GetTask(parent.ID)
	if updated.Status != models.TaskCompleted {
		t.Errorf("expected parent COMPLETED, got %s", updated.Status)
	}
}

func TestCheckParentCompletion_SomeFailed(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	parent := &models.Task{
		Title:    "Parent",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	mgr.TransitionTask(parent.ID, models.TaskRunning)
	mgr.TransitionTask(parent.ID, models.TaskDecomposed)

	// One completed, one failed with exhausted retries
	mgr.CreateTask(&models.Task{
		Title: "Sub OK", Status: models.TaskCompleted,
		Priority: 50, ParentID: parent.ID, Depth: 1,
	})
	mgr.CreateTask(&models.Task{
		Title: "Sub Fail", Status: models.TaskFailed,
		Priority: 50, ParentID: parent.ID, Depth: 1,
		RetryCount: 3, MaxRetries: 3, // Retries exhausted
	})

	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}

	updated, _ := mgr.GetTask(parent.ID)
	if updated.Status != models.TaskFailed {
		t.Errorf("expected parent FAILED, got %s", updated.Status)
	}
}

func TestCheckParentCompletion_StillRunning(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	parent := &models.Task{
		Title:    "Parent",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	mgr.TransitionTask(parent.ID, models.TaskRunning)
	mgr.TransitionTask(parent.ID, models.TaskDecomposed)

	mgr.CreateTask(&models.Task{
		Title: "Sub Done", Status: models.TaskCompleted,
		Priority: 50, ParentID: parent.ID, Depth: 1,
	})
	mgr.CreateTask(&models.Task{
		Title: "Sub Running", Status: models.TaskRunning,
		Priority: 50, ParentID: parent.ID, Depth: 1,
	})

	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}

	updated, _ := mgr.GetTask(parent.ID)
	if updated.Status != models.TaskDecomposed {
		t.Errorf("expected parent still DECOMPOSED, got %s", updated.Status)
	}
}

func TestPopNextPending(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create PENDING operator task
	task := &models.Task{
		Title:    "Needs triage",
		
		Status:   models.TaskPending,
		Source:   models.TaskSourceOperator,
		Priority: 50,
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Pop pending task
	pending, err := mgr.PopNextPending("triager-01")
	if err != nil {
		t.Fatalf("pop next pending: %v", err)
	}
	if pending == nil {
		t.Fatal("expected pending task, got nil")
	}
	if pending.ExecutorID != "triager-01" {
		t.Errorf("expected executor_id triager-01, got %s", pending.ExecutorID)
	}

	// Second pop should return nil (task already claimed)
	pending2, err := mgr.PopNextPending("triager-02")
	if err != nil {
		t.Fatalf("pop next pending 2: %v", err)
	}
	if pending2 != nil {
		t.Error("expected nil for already-claimed task")
	}
}

func TestAreDependenciesMet(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create completed dependency
	completedDep := &models.Task{
		Title:    "Completed Dep",
		
		Status:   models.TaskCompleted,
		Priority: 50,
	}
	if err := mgr.CreateTask(completedDep); err != nil {
		t.Fatalf("create completed dep: %v", err)
	}

	// Create running dependency
	runningDep := &models.Task{
		Title:    "Running Dep",
		
		Status:   models.TaskRunning,
		Priority: 50,
	}
	if err := mgr.CreateTask(runningDep); err != nil {
		t.Fatalf("create running dep: %v", err)
	}

	// Test 1: No dependencies
	taskNoDeps := &models.Task{
		DependsOn: []string{},
	}
	tx, _ := db.Begin()
	met, err := mgr.areDependenciesMet(tx, taskNoDeps)
	tx.Rollback()
	if err != nil {
		t.Errorf("areDependenciesMet with no deps: %v", err)
	}
	if !met {
		t.Error("expected dependencies met with no deps")
	}

	// Test 2: All dependencies completed
	taskAllCompleted := &models.Task{
		DependsOn: []string{completedDep.ID},
	}
	tx, _ = db.Begin()
	met, err = mgr.areDependenciesMet(tx, taskAllCompleted)
	tx.Rollback()
	if err != nil {
		t.Errorf("areDependenciesMet with completed deps: %v", err)
	}
	if !met {
		t.Error("expected dependencies met with completed deps")
	}

	// Test 3: Some dependencies not completed
	taskSomeRunning := &models.Task{
		DependsOn: []string{completedDep.ID, runningDep.ID},
	}
	tx, _ = db.Begin()
	met, err = mgr.areDependenciesMet(tx, taskSomeRunning)
	tx.Rollback()
	if err != nil {
		t.Errorf("areDependenciesMet with mixed deps: %v", err)
	}
	if met {
		t.Error("expected dependencies not met with running deps")
	}
}

func TestPopNextTask_RetryTasksAutomaticallyPicked(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create a RETRY task (simulating crash recovery or rate limit)
	retryTask := &models.Task{
		Title:         "Retry Task",
		
		Status:        models.TaskRetry,
		Priority:      30,
		RetryCount:    1,
		CrashRecovery: true,
	}
	if err := mgr.CreateTask(retryTask); err != nil {
		t.Fatalf("create retry task: %v", err)
	}

	// Create a READY task with lower priority
	readyTask := &models.Task{
		Title:    "Ready Task",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(readyTask); err != nil {
		t.Fatalf("create ready task: %v", err)
	}

	// Pop next task - should get RETRY task due to higher priority
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "Retry Task" {
		t.Errorf("expected Retry Task, got %s", next.Title)
	}

	if next.Status != models.TaskRunning {
		t.Errorf("expected status RUNNING after pop, got %s", next.Status)
	}

	// Verify the task was transitioned from RETRY to RUNNING
	updated, err := mgr.GetTask(retryTask.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updated.Status != models.TaskRunning {
		t.Errorf("expected task status RUNNING in DB, got %s", updated.Status)
	}
	if updated.StartedAt == "" {
		t.Error("expected started_at to be set")
	}
}

func TestPopNextTask_MixedReadyAndRetryTasks(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create multiple READY and RETRY tasks with different priorities
	tasks := []*models.Task{
		{Title: "Retry High", Status: models.TaskRetry, Priority: 10, RetryCount: 1},
		{Title: "Ready High", Status: models.TaskReady, Priority: 20},
		{Title: "Retry Low", Status: models.TaskRetry, Priority: 60, RetryCount: 2},
		{Title: "Ready Low", Status: models.TaskReady, Priority: 70},
	}

	for _, task := range tasks {
		if err := mgr.CreateTask(task); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	// Pop tasks one by one, verify priority ordering regardless of READY/RETRY status
	expectedOrder := []string{"Retry High", "Ready High", "Retry Low", "Ready Low"}
	for i, expectedTitle := range expectedOrder {
		next, err := mgr.PopNextTask("EXECUTOR")
		if err != nil {
			t.Fatalf("pop task %d: %v", i, err)
		}
		if next == nil {
			t.Fatalf("pop task %d: expected task, got nil", i)
		}
		if next.Title != expectedTitle {
			t.Errorf("pop task %d: expected %s, got %s", i, expectedTitle, next.Title)
		}
		if next.Status != models.TaskRunning {
			t.Errorf("pop task %d: expected status RUNNING, got %s", i, next.Status)
		}
	}

	// Verify no more tasks
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop final task: %v", err)
	}
	if next != nil {
		t.Errorf("expected no more tasks, got %s", next.Title)
	}
}

func TestAggregateSubtaskResults_AllCompleted(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create completed subtasks with results
	subtasks := []*models.Task{
		{
			Title:       "Subtask 1",
			Description: "First subtask description",
			
			Status:      models.TaskCompleted,
			Priority:    50,
			ParentID:    parent.ID,
			Depth:       1,
			Result:      "Successfully implemented feature A",
		},
		{
			Title:       "Subtask 2",
			Description: "Second subtask description",
			
			Status:      models.TaskCompleted,
			Priority:    50,
			ParentID:    parent.ID,
			Depth:       1,
			Result:      "Successfully added tests for feature B",
		},
		{
			Title:    "Subtask 3",
			
			Status:   models.TaskCompleted,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			Result:   "Documentation updated",
		},
	}

	for _, sub := range subtasks {
		if err := mgr.CreateTask(sub); err != nil {
			t.Fatalf("create subtask: %v", err)
		}
	}

	// Aggregate results
	if err := mgr.AggregateSubtaskResults(parent.ID); err != nil {
		t.Fatalf("aggregate results: %v", err)
	}

	// Verify parent result is populated
	updated, err := mgr.GetTask(parent.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}

	if updated.Result == "" {
		t.Fatal("expected parent result to be populated")
	}

	// Verify result contains key information
	result := updated.Result
	if !strings.Contains(result, "Subtask Results Summary") {
		t.Error("result should contain summary header")
	}
	if !strings.Contains(result, "Total subtasks: 3") {
		t.Error("result should contain subtask count")
	}
	if !strings.Contains(result, "Subtask 1") {
		t.Error("result should contain first subtask")
	}
	if !strings.Contains(result, "Successfully implemented feature A") {
		t.Error("result should contain first subtask result")
	}
	if !strings.Contains(result, "First subtask description") {
		t.Error("result should contain subtask description")
	}
	if !strings.Contains(result, "Completed: 3") {
		t.Error("result should contain completion count")
	}
	if !strings.Contains(result, "Failed: 0") {
		t.Error("result should contain failed count")
	}
}

func TestAggregateSubtaskResults_MixedStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create subtasks with mixed statuses
	subtasks := []*models.Task{
		{
			Title:    "Completed Sub",
			
			Status:   models.TaskCompleted,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			Result:   "Completed successfully",
		},
		{
			Title:    "Failed Sub",
			
			Status:   models.TaskFailed,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			Result:   "Attempted but failed",
			ErrorLog: "Build error: syntax issue",
		},
		{
			Title:    "Cancelled Sub",
			
			Status:   models.TaskCancelled,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			ErrorLog: "cancelled by operator",
		},
	}

	for _, sub := range subtasks {
		if err := mgr.CreateTask(sub); err != nil {
			t.Fatalf("create subtask: %v", err)
		}
	}

	if err := mgr.AggregateSubtaskResults(parent.ID); err != nil {
		t.Fatalf("aggregate results: %v", err)
	}

	updated, err := mgr.GetTask(parent.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}

	result := updated.Result
	if !strings.Contains(result, "Completed: 1") {
		t.Error("result should show 1 completed")
	}
	if !strings.Contains(result, "Failed: 1") {
		t.Error("result should show 1 failed")
	}
	if !strings.Contains(result, "Cancelled: 1") {
		t.Error("result should show 1 cancelled")
	}
	if !strings.Contains(result, "Build error: syntax issue") {
		t.Error("result should contain error log")
	}
}

func TestAggregateSubtaskResults_NoSubtasks(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Aggregate with no subtasks should not error
	if err := mgr.AggregateSubtaskResults(parent.ID); err != nil {
		t.Errorf("aggregate with no subtasks: %v", err)
	}

	// Parent result should remain empty
	updated, err := mgr.GetTask(parent.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}
	if updated.Result != "" {
		t.Error("parent result should remain empty with no subtasks")
	}
}

func TestCheckParentCompletion_WithAggregation(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent and transition to DECOMPOSED
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	mgr.TransitionTask(parent.ID, models.TaskRunning)
	mgr.TransitionTask(parent.ID, models.TaskDecomposed)

	// Create completed subtasks with results
	for i := 1; i <= 2; i++ {
		sub := &models.Task{
			Title:    fmt.Sprintf("Subtask %d", i),
			
			Status:   models.TaskCompleted,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			Result:   fmt.Sprintf("Result for subtask %d", i),
		}
		if err := mgr.CreateTask(sub); err != nil {
			t.Fatalf("create subtask %d: %v", i, err)
		}
	}

	// CheckParentCompletion should aggregate results and transition parent
	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}

	// Verify parent is completed
	updated, err := mgr.GetTask(parent.ID)
	if err != nil {
		t.Fatalf("get parent: %v", err)
	}

	if updated.Status != models.TaskCompleted {
		t.Errorf("expected parent COMPLETED, got %s", updated.Status)
	}

	// Verify aggregated result is present
	if updated.Result == "" {
		t.Fatal("expected aggregated result in parent")
	}

	if !strings.Contains(updated.Result, "Subtask Results Summary") {
		t.Error("parent result should contain aggregated summary")
	}
	if !strings.Contains(updated.Result, "Result for subtask 1") {
		t.Error("parent result should contain subtask 1 result")
	}
	if !strings.Contains(updated.Result, "Result for subtask 2") {
		t.Error("parent result should contain subtask 2 result")
	}
}

func TestPopNextTask_AutoRetryFailed(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create a FAILED task that is still retryable
	task := &models.Task{
		Title:      "Failed Task",
		
		Status:     models.TaskFailed,
		Priority:   50,
		RetryCount: 1,
		MaxRetries: 3,
		ErrorLog:   "previous failure",
	}
	if err := mgr.CreateTask(task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// PopNextTask should pick it up and auto-retry
	popped, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}
	if popped == nil {
		t.Fatal("expected to pop failed retryable task")
	}
	if popped.ID != task.ID {
		t.Errorf("expected to pop task %s, got %s", task.ID, popped.ID)
	}
	if popped.Status != models.TaskRunning {
		t.Errorf("expected status RUNNING, got %s", popped.Status)
	}

	// Verify retry count was incremented
	updated, _ := mgr.GetTask(task.ID)
	if updated.RetryCount != 2 {
		t.Errorf("expected retry_count 2, got %d", updated.RetryCount)
	}
	if updated.ErrorLog != "" {
		t.Error("expected error_log to be cleared")
	}
}

func TestPopNextTask_SkipExhaustedRetries(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create a FAILED task with exhausted retries
	task1 := &models.Task{
		Title:      "Exhausted Task",
		
		Status:     models.TaskFailed,
		Priority:   50,
		RetryCount: 3,
		MaxRetries: 3,
	}
	if err := mgr.CreateTask(task1); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// Create a READY task
	task2 := &models.Task{
		Title:    "Ready Task",
		
		Status:   models.TaskReady,
		Priority: 51,
	}
	if err := mgr.CreateTask(task2); err != nil {
		t.Fatalf("create task: %v", err)
	}

	// PopNextTask should skip the exhausted task and pick the ready one
	popped, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop next task: %v", err)
	}
	if popped == nil {
		t.Fatal("expected to pop ready task")
	}
	if popped.ID != task2.ID {
		t.Errorf("expected to pop ready task %s, got %s", task2.ID, popped.ID)
	}
}

func TestCheckParentCompletion_DeferFailureForRetryable(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			WorkspaceBase: t.TempDir(),
		},
	}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create completed subtask
	sub1 := &models.Task{
		Title:    "Completed Subtask",
		
		Status:   models.TaskCompleted,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1); err != nil {
		t.Fatalf("create subtask 1: %v", err)
	}

	// Create failed but retryable subtask
	sub2 := &models.Task{
		Title:      "Failed Retryable Subtask",
		
		Status:     models.TaskFailed,
		Priority:   50,
		ParentID:   parent.ID,
		Depth:      1,
		RetryCount: 1,
		MaxRetries: 3,
	}
	if err := mgr.CreateTask(sub2); err != nil {
		t.Fatalf("create subtask 2: %v", err)
	}

	// Check parent completion - should NOT transition parent to FAILED
	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}

	updated, _ := mgr.GetTask(parent.ID)
	if updated.Status != models.TaskDecomposed {
		t.Errorf("expected parent to stay DECOMPOSED, got %s", updated.Status)
	}
}

func TestCheckParentCompletion_FailWhenRetriesExhausted(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			WorkspaceBase: t.TempDir(),
		},
	}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create completed subtask
	sub1 := &models.Task{
		Title:    "Completed Subtask",
		
		Status:   models.TaskCompleted,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1); err != nil {
		t.Fatalf("create subtask 1: %v", err)
	}

	// Create failed subtask with exhausted retries
	sub2 := &models.Task{
		Title:      "Failed Exhausted Subtask",
		
		Status:     models.TaskFailed,
		Priority:   50,
		ParentID:   parent.ID,
		Depth:      1,
		RetryCount: 3,
		MaxRetries: 3,
	}
	if err := mgr.CreateTask(sub2); err != nil {
		t.Fatalf("create subtask 2: %v", err)
	}

	// Check parent completion - should transition parent to FAILED
	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}

	updated, _ := mgr.GetTask(parent.ID)
	if updated.Status != models.TaskFailed {
		t.Errorf("expected parent to be FAILED, got %s", updated.Status)
	}
}

func TestCheckParentCompletion_RevalidateAfterRetry(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			WorkspaceBase: t.TempDir(),
		},
	}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create two subtasks
	sub1 := &models.Task{
		Title:    "Subtask 1",
		
		Status:   models.TaskCompleted,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1); err != nil {
		t.Fatalf("create subtask 1: %v", err)
	}

	sub2 := &models.Task{
		Title:      "Subtask 2",
		
		Status:     models.TaskFailed,
		Priority:   50,
		ParentID:   parent.ID,
		Depth:      1,
		RetryCount: 1,
		MaxRetries: 3,
	}
	if err := mgr.CreateTask(sub2); err != nil {
		t.Fatalf("create subtask 2: %v", err)
	}

	// First check - parent should stay DECOMPOSED
	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("check parent completion: %v", err)
	}
	updated, _ := mgr.GetTask(parent.ID)
	if updated.Status != models.TaskDecomposed {
		t.Errorf("expected parent to stay DECOMPOSED, got %s", updated.Status)
	}

	// Simulate retry success - mark subtask 2 as completed
	sub2.Status = models.TaskCompleted
	if err := mgr.tasks.Update(sub2); err != nil {
		t.Fatalf("update subtask 2: %v", err)
	}

	// Re-check parent - should now transition to COMPLETED
	if err := mgr.CheckParentCompletion(parent.ID); err != nil {
		t.Fatalf("re-check parent completion: %v", err)
	}
	updated, _ = mgr.GetTask(parent.ID)
	if updated.Status != models.TaskCompleted {
		t.Errorf("expected parent to be COMPLETED, got %s", updated.Status)
	}
}

func TestPopNextTask_SubtaskDependencies(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create subtask 1 (no dependencies)
	sub1 := &models.Task{
		Title:    "Subtask 1",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1); err != nil {
		t.Fatalf("create subtask 1: %v", err)
	}

	// Create subtask 2 (depends on subtask 1)
	sub2 := &models.Task{
		Title:     "Subtask 2",
		
		Status:    models.TaskReady,
		Priority:  50,
		ParentID:  parent.ID,
		Depth:     1,
		DependsOn: []string{sub1.ID},
	}
	if err := mgr.CreateTask(sub2); err != nil {
		t.Fatalf("create subtask 2: %v", err)
	}

	// Create subtask 3 (depends on subtask 2)
	sub3 := &models.Task{
		Title:     "Subtask 3",
		
		Status:    models.TaskReady,
		Priority:  50,
		ParentID:  parent.ID,
		Depth:     1,
		DependsOn: []string{sub2.ID},
	}
	if err := mgr.CreateTask(sub3); err != nil {
		t.Fatalf("create subtask 3: %v", err)
	}

	// Pop task - should get subtask 1 (no dependencies)
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 1: %v", err)
	}
	if next == nil {
		t.Fatal("expected task, got nil")
	}
	if next.ID != sub1.ID {
		t.Errorf("expected subtask 1, got %s", next.Title)
	}

	// Try to pop again - should get nothing (subtask 2 blocked by subtask 1)
	next, err = mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 2: %v", err)
	}
	if next != nil {
		t.Errorf("expected no task (sub2 blocked), got %s", next.Title)
	}

	// Complete subtask 1
	mgr.TransitionTask(sub1.ID, models.TaskCompleted)

	// Pop task - should now get subtask 2
	next, err = mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 3: %v", err)
	}
	if next == nil {
		t.Fatal("expected task, got nil")
	}
	if next.ID != sub2.ID {
		t.Errorf("expected subtask 2, got %s", next.Title)
	}

	// Try to pop again - should get nothing (subtask 3 blocked by subtask 2)
	next, err = mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 4: %v", err)
	}
	if next != nil {
		t.Errorf("expected no task (sub3 blocked), got %s", next.Title)
	}

	// Complete subtask 2
	mgr.TransitionTask(sub2.ID, models.TaskCompleted)

	// Pop task - should now get subtask 3
	next, err = mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 5: %v", err)
	}
	if next == nil {
		t.Fatal("expected task, got nil")
	}
	if next.ID != sub3.ID {
		t.Errorf("expected subtask 3, got %s", next.Title)
	}
}

func TestPopNextTask_SubtaskDependencies_TopologicalOrder(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create subtask A (no dependencies, priority 50)
	subA := &models.Task{
		Title:    "Subtask A",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(subA); err != nil {
		t.Fatalf("create subtask A: %v", err)
	}

	// Small delay to ensure different created_at timestamps
	time.Sleep(10 * time.Millisecond)

	// Create subtask B (no dependencies, priority 50, created later)
	subB := &models.Task{
		Title:    "Subtask B",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(subB); err != nil {
		t.Fatalf("create subtask B: %v", err)
	}

	// Pop task - should get subtask A (created first, same priority)
	next, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 1: %v", err)
	}
	if next == nil {
		t.Fatal("expected task, got nil")
	}
	if next.ID != subA.ID {
		t.Errorf("expected subtask A (topological order), got %s", next.Title)
	}

	// Pop task - should get subtask B
	next, err = mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 2: %v", err)
	}
	if next == nil {
		t.Fatal("expected task, got nil")
	}
	if next.ID != subB.ID {
		t.Errorf("expected subtask B, got %s", next.Title)
	}
}

func TestPopNextTask_SubtaskDependencies_BackwardCompatibility(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}

	// Create subtasks without dependencies (backward compatibility)
	sub1 := &models.Task{
		Title:    "Subtask 1",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1); err != nil {
		t.Fatalf("create subtask 1: %v", err)
	}

	sub2 := &models.Task{
		Title:    "Subtask 2",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub2); err != nil {
		t.Fatalf("create subtask 2: %v", err)
	}

	// Both subtasks should be pickable (no dependencies)
	next1, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 1: %v", err)
	}
	if next1 == nil {
		t.Fatal("expected task 1, got nil")
	}

	next2, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 2: %v", err)
	}
	if next2 == nil {
		t.Fatal("expected task 2, got nil")
	}

	// Verify both subtasks were picked up
	pickedIDs := map[string]bool{next1.ID: true, next2.ID: true}
	if !pickedIDs[sub1.ID] || !pickedIDs[sub2.ID] {
		t.Error("expected both subtasks to be picked up without dependencies")
	}
}

func TestPopNextTask_SubtaskDependencies_MixedParents(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent 1
	parent1 := &models.Task{
		Title:    "Parent 1",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent1); err != nil {
		t.Fatalf("create parent 1: %v", err)
	}

	// Create parent 2
	parent2 := &models.Task{
		Title:    "Parent 2",
		
		Status:   models.TaskDecomposed,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent2); err != nil {
		t.Fatalf("create parent 2: %v", err)
	}

	// Create subtask 1 for parent 1 (no dependencies)
	sub1p1 := &models.Task{
		Title:    "P1 Subtask 1",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent1.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1p1); err != nil {
		t.Fatalf("create p1 subtask 1: %v", err)
	}

	// Create subtask 2 for parent 1 (depends on sub1p1)
	sub2p1 := &models.Task{
		Title:     "P1 Subtask 2",
		
		Status:    models.TaskReady,
		Priority:  50,
		ParentID:  parent1.ID,
		Depth:     1,
		DependsOn: []string{sub1p1.ID},
	}
	if err := mgr.CreateTask(sub2p1); err != nil {
		t.Fatalf("create p1 subtask 2: %v", err)
	}

	// Create subtask 1 for parent 2 (no dependencies)
	sub1p2 := &models.Task{
		Title:    "P2 Subtask 1",
		
		Status:   models.TaskReady,
		Priority: 50,
		ParentID: parent2.ID,
		Depth:    1,
	}
	if err := mgr.CreateTask(sub1p2); err != nil {
		t.Fatalf("create p2 subtask 1: %v", err)
	}

	// Pop tasks - should get sub1p1 and sub1p2 (no dependencies)
	// Order depends on created_at, but both should be available
	next1, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 1: %v", err)
	}
	if next1 == nil {
		t.Fatal("expected task 1, got nil")
	}

	next2, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 2: %v", err)
	}
	if next2 == nil {
		t.Fatal("expected task 2, got nil")
	}

	// Verify we got the two independent subtasks
	pickedIDs := map[string]bool{next1.ID: true, next2.ID: true}
	if !pickedIDs[sub1p1.ID] || !pickedIDs[sub1p2.ID] {
		t.Error("expected to pick both independent subtasks from different parents")
	}

	// Try to pop again - should get nothing (sub2p1 blocked)
	next3, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 3: %v", err)
	}
	if next3 != nil {
		t.Errorf("expected no task (sub2p1 still blocked), got %s", next3.Title)
	}

	// Complete sub1p1
	mgr.TransitionTask(sub1p1.ID, models.TaskCompleted)

	// Pop task - should now get sub2p1
	next4, err := mgr.PopNextTask("EXECUTOR")
	if err != nil {
		t.Fatalf("pop task 4: %v", err)
	}
	if next4 == nil {
		t.Fatal("expected task 4, got nil")
	}
	if next4.ID != sub2p1.ID {
		t.Errorf("expected sub2p1 after dependency resolved, got %s", next4.Title)
	}
}
