package models

import (
	"testing"

	"github.com/circle-oo/flux/internal/testutil"
)

func TestGoalStore_CreateAndGet(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewGoalStore(db)

	goal := &Goal{
		Title:       "Build MVP",
		Description: "Build the minimum viable product",
		Priorities:  []string{"speed", "quality"},
		Metrics:     []string{"tasks completed", "test coverage"},
		Source:      GoalSourceOperator,
	}

	if err := store.Create(goal); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if goal.ID == "" {
		t.Error("expected ID to be set")
	}
	if goal.Status != GoalProposed {
		t.Errorf("expected status PROPOSED, got %s", goal.Status)
	}

	got, err := store.GetByID(goal.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Build MVP" {
		t.Errorf("expected title 'Build MVP', got %s", got.Title)
	}
	if len(got.Priorities) != 2 {
		t.Errorf("expected 2 priorities, got %d", len(got.Priorities))
	}
	if len(got.Metrics) != 2 {
		t.Errorf("expected 2 metrics, got %d", len(got.Metrics))
	}
}

func TestGoalStore_Activate_SingleActive(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewGoalStore(db)

	goal1 := &Goal{Title: "Goal 1", Source: GoalSourceOperator}
	goal2 := &Goal{Title: "Goal 2", Source: GoalSourceOperator}

	store.Create(goal1)
	store.Create(goal2)

	// Activate goal1
	if err := store.Activate(goal1.ID); err != nil {
		t.Fatalf("Activate goal1: %v", err)
	}
	g1, _ := store.GetByID(goal1.ID)
	if g1.Status != GoalActive {
		t.Errorf("expected goal1 ACTIVE, got %s", g1.Status)
	}

	// Activate goal2 — should supersede goal1
	if err := store.Activate(goal2.ID); err != nil {
		t.Fatalf("Activate goal2: %v", err)
	}
	g1, _ = store.GetByID(goal1.ID)
	g2, _ := store.GetByID(goal2.ID)
	if g1.Status != GoalSuperseded {
		t.Errorf("expected goal1 SUPERSEDED, got %s", g1.Status)
	}
	if g2.Status != GoalActive {
		t.Errorf("expected goal2 ACTIVE, got %s", g2.Status)
	}

	// GetCurrent should return goal2
	current, err := store.GetCurrent()
	if err != nil {
		t.Fatalf("GetCurrent: %v", err)
	}
	if current == nil {
		t.Fatal("expected current goal")
	}
	if current.ID != goal2.ID {
		t.Errorf("expected current=goal2, got %s", current.ID)
	}
}

func TestGoalStore_List(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewGoalStore(db)

	store.Create(&Goal{Title: "A", Source: GoalSourceOperator})
	store.Create(&Goal{Title: "B", Source: GoalSourceOperator})

	goals, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(goals) != 2 {
		t.Errorf("expected 2 goals, got %d", len(goals))
	}
}

func TestGoalStore_Update(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewGoalStore(db)

	goal := &Goal{Title: "Original", Source: GoalSourceOperator}
	store.Create(goal)

	goal.Title = "Updated"
	goal.Description = "new desc"
	if err := store.Update(goal); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := store.GetByID(goal.ID)
	if got.Title != "Updated" {
		t.Errorf("expected Updated, got %s", got.Title)
	}
}

func TestGoalStore_Delete(t *testing.T) {
	db := testutil.NewTestDB(t)
	store := NewGoalStore(db)

	goal := &Goal{Title: "Delete me", Source: GoalSourceOperator}
	store.Create(goal)

	if err := store.Delete(goal.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.GetByID(goal.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}
