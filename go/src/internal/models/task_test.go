package models

import (
	"testing"

	"github.com/circle-oo/flux/internal/testutil"
)

func TestTask_NeedsOpus(t *testing.T) {
	tests := []struct {
		name     string
		task     Task
		expected bool
	}{
		{
			name:     "high priority (<= 5)",
			task:     Task{Priority: 3, Source: TaskSourceSystem},
			expected: true,
		},
		{
			name:     "operator with complex keyword",
			task:     Task{Priority: 10, Source: TaskSourceOperator, Title: "Refactor auth module"},
			expected: true,
		},
		{
			name:     "operator without complex keyword",
			task:     Task{Priority: 10, Source: TaskSourceOperator, Title: "Add button"},
			expected: false,
		},
		{
			name:     "initial-design tag",
			task:     Task{Priority: 50, Source: TaskSourceSystem, Tags: []string{"initial-design"}},
			expected: true,
		},
		{
			name:     "goal-strategy tag",
			task:     Task{Priority: 50, Source: TaskSourceSystem, Tags: []string{"goal-strategy"}},
			expected: true,
		},
		{
			name:     "normal task",
			task:     Task{Priority: 50, Source: TaskSourceSystem},
			expected: false,
		},
		{
			name:     "complex keyword in description",
			task:     Task{Priority: 10, Source: TaskSourceOperator, Description: "security audit needed"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.NeedsOpus()
			if got != tt.expected {
				t.Errorf("NeedsOpus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTask_RequiresTest(t *testing.T) {
	tests := []struct {
		taskType string
		expected bool
	}{
		{TaskTypeCoding, true},
		{TaskTypeBugfix, true},
		{TaskTypeMaintenance, true},
		{TaskTypeResearch, false},
		{TaskTypeDocument, false},
		{TaskTypeDeploy, false},
		{TaskTypePlanning, false},
	}

	for _, tt := range tests {
		t.Run(tt.taskType, func(t *testing.T) {
			task := Task{Type: tt.taskType}
			got := task.RequiresTest()
			if got != tt.expected {
				t.Errorf("RequiresTest() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTask_hasComplexKeywords(t *testing.T) {
	tests := []struct {
		name     string
		task     Task
		expected bool
	}{
		{"architect", Task{Title: "Architect the system"}, true},
		{"refactor", Task{Description: "Needs refactoring"}, true},
		{"redesign", Task{Title: "Redesign the UI"}, true},
		{"migration", Task{Title: "Database migration"}, true},
		{"security", Task{Description: "Security review"}, true},
		{"overhaul", Task{Title: "Complete overhaul"}, true},
		{"none", Task{Title: "Add button", Description: "Simple change"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.hasComplexKeywords()
			if got != tt.expected {
				t.Errorf("hasComplexKeywords() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTaskStore_CreateAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewTaskStore(db)

	task := &Task{
		Title:    "Implement feature",
		Type:     TaskTypeCoding,
		Priority: 50,
		Source:   TaskSourceSystem,
		Tags:     []string{"backend", "api"},
		DependsOn: []string{},
	}

	if err := store.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if task.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := store.GetByID(task.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Implement feature" {
		t.Errorf("expected title, got %s", got.Title)
	}
	if len(got.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(got.Tags))
	}
}

func TestTaskStore_OperatorStaysPendingForTriage(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewTaskStore(db)

	task := &Task{
		Title:  "Operator task",
		Type:   TaskTypeCoding,
		Source: TaskSourceOperator,
	}

	if err := store.Create(task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Operator tasks stay PENDING until triage promotes them to READY
	if task.Status != TaskPending {
		t.Errorf("expected PENDING for operator task (awaiting triage), got %s", task.Status)
	}
}

func TestTaskStore_List_WithFilter(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewTaskStore(db)

	store.Create(&Task{Title: "A", Type: TaskTypeCoding, Status: TaskReady, Priority: 10})
	store.Create(&Task{Title: "B", Type: TaskTypeCoding, Status: TaskCompleted, Priority: 20})
	store.Create(&Task{Title: "C", Type: TaskTypeCoding, Status: TaskReady, Priority: 5})

	tasks, err := store.List(ListFilter{Status: TaskReady})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 READY tasks, got %d", len(tasks))
	}
	// Should be ordered by priority ASC
	if tasks[0].Priority > tasks[1].Priority {
		t.Errorf("expected ascending priority order")
	}
}

func TestTaskStore_Cancel(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewTaskStore(db)

	task := &Task{Title: "Cancel me", Type: TaskTypeCoding}
	store.Create(task)

	if err := store.Cancel(task.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	got, _ := store.GetByID(task.ID)
	if got.Status != TaskCancelled {
		t.Errorf("expected CANCELLED, got %s", got.Status)
	}
	if got.ErrorLog != "cancelled by operator" {
		t.Errorf("expected 'cancelled by operator', got %s", got.ErrorLog)
	}
}

func TestTaskStore_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewTaskStore(db)

	task := &Task{Title: "Delete me", Type: TaskTypeCoding}
	store.Create(task)

	if err := store.Delete(task.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.GetByID(task.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestTaskStore_CountByParent(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewTaskStore(db)

	parent := &Task{Title: "Parent", Type: TaskTypeCoding}
	store.Create(parent)

	for i := 0; i < 3; i++ {
		store.Create(&Task{
			Title:    "Child",
			Type:     TaskTypeCoding,
			ParentID: parent.ID,
			Depth:    1,
		})
	}

	count, err := store.CountByParent(parent.ID)
	if err != nil {
		t.Fatalf("CountByParent: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}
