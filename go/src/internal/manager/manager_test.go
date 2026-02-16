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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
		Status:   models.TaskReady,
		Priority: 48,
		GoalID:   "",
	}
	task2 := &models.Task{
		Title:    "Lower Priority Goal Task",
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
		Status:   models.TaskRunning,
		Priority: 50,
	}
	if err := mgr.CreateTask(depTask); err != nil {
		t.Fatalf("create dep task: %v", err)
	}

	// Create task that depends on the first
	blockedTask := &models.Task{
		Title:     "Blocked Task",
		Type:      models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:      models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
			Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	mgr.TransitionTask(parent.ID, models.TaskRunning)
	mgr.TransitionTask(parent.ID, models.TaskDecomposed)

	// One completed, one failed
	mgr.CreateTask(&models.Task{
		Title: "Sub OK", Type: models.TaskTypeCoding, Status: models.TaskCompleted,
		Priority: 50, ParentID: parent.ID, Depth: 1,
	})
	mgr.CreateTask(&models.Task{
		Title: "Sub Fail", Type: models.TaskTypeCoding, Status: models.TaskFailed,
		Priority: 50, ParentID: parent.ID, Depth: 1,
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
		Type:     models.TaskTypeCoding,
		Status:   models.TaskReady,
		Priority: 50,
	}
	if err := mgr.CreateTask(parent); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	mgr.TransitionTask(parent.ID, models.TaskRunning)
	mgr.TransitionTask(parent.ID, models.TaskDecomposed)

	mgr.CreateTask(&models.Task{
		Title: "Sub Done", Type: models.TaskTypeCoding, Status: models.TaskCompleted,
		Priority: 50, ParentID: parent.ID, Depth: 1,
	})
	mgr.CreateTask(&models.Task{
		Title: "Sub Running", Type: models.TaskTypeCoding, Status: models.TaskRunning,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
		Status:   models.TaskCompleted,
		Priority: 50,
	}
	if err := mgr.CreateTask(completedDep); err != nil {
		t.Fatalf("create completed dep: %v", err)
	}

	// Create running dependency
	runningDep := &models.Task{
		Title:    "Running Dep",
		Type:     models.TaskTypeCoding,
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
		Type:          models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		{Title: "Retry High", Type: models.TaskTypeCoding, Status: models.TaskRetry, Priority: 10, RetryCount: 1},
		{Title: "Ready High", Type: models.TaskTypeCoding, Status: models.TaskReady, Priority: 20},
		{Title: "Retry Low", Type: models.TaskTypeCoding, Status: models.TaskRetry, Priority: 60, RetryCount: 2},
		{Title: "Ready Low", Type: models.TaskTypeCoding, Status: models.TaskReady, Priority: 70},
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

func TestPopNextTask_ResearcherIncludesRetry(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create RETRY research task
	retryResearch := &models.Task{
		Title:      "Retry Research",
		Type:       models.TaskTypeResearch,
		Status:     models.TaskRetry,
		Priority:   30,
		RetryCount: 1,
	}
	if err := mgr.CreateTask(retryResearch); err != nil {
		t.Fatalf("create retry research task: %v", err)
	}

	// Create READY coding task (should be ignored by RESEARCHER)
	readyCoding := &models.Task{
		Title:    "Ready Coding",
		Type:     models.TaskTypeCoding,
		Status:   models.TaskReady,
		Priority: 10,
	}
	if err := mgr.CreateTask(readyCoding); err != nil {
		t.Fatalf("create ready coding task: %v", err)
	}

	// RESEARCHER pod should get the RETRY research task
	next, err := mgr.PopNextTask("RESEARCHER")
	if err != nil {
		t.Fatalf("pop researcher task: %v", err)
	}

	if next == nil {
		t.Fatal("expected task, got nil")
	}

	if next.Title != "Retry Research" {
		t.Errorf("expected Retry Research, got %s", next.Title)
	}

	if next.Type != models.TaskTypeResearch {
		t.Errorf("researcher should get research task, got %s", next.Type)
	}

	if next.Status != models.TaskRunning {
		t.Errorf("expected status RUNNING, got %s", next.Status)
	}
}

func TestAggregateSubtaskResults_AllCompleted(t *testing.T) {
	db := testutil.NewTestDB(t)
	cfg := &config.Config{}
	mgr := NewManager(db, cfg)

	// Create parent task
	parent := &models.Task{
		Title:    "Parent Task",
		Type:     models.TaskTypeCoding,
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
			Type:        models.TaskTypeCoding,
			Status:      models.TaskCompleted,
			Priority:    50,
			ParentID:    parent.ID,
			Depth:       1,
			Result:      "Successfully implemented feature A",
		},
		{
			Title:       "Subtask 2",
			Description: "Second subtask description",
			Type:        models.TaskTypeCoding,
			Status:      models.TaskCompleted,
			Priority:    50,
			ParentID:    parent.ID,
			Depth:       1,
			Result:      "Successfully added tests for feature B",
		},
		{
			Title:    "Subtask 3",
			Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
			Type:     models.TaskTypeCoding,
			Status:   models.TaskCompleted,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			Result:   "Completed successfully",
		},
		{
			Title:    "Failed Sub",
			Type:     models.TaskTypeCoding,
			Status:   models.TaskFailed,
			Priority: 50,
			ParentID: parent.ID,
			Depth:    1,
			Result:   "Attempted but failed",
			ErrorLog: "Build error: syntax issue",
		},
		{
			Title:    "Cancelled Sub",
			Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
		Type:     models.TaskTypeCoding,
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
			Type:     models.TaskTypeCoding,
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
