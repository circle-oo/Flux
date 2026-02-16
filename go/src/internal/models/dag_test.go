package models

import (
	"testing"
)

func TestValidateDAG_NoDependencies(t *testing.T) {
	subtasks := []string{"task1", "task2", "task3"}
	deps := []SubtaskDependency{}

	err := ValidateDAG(subtasks, deps)
	if err != nil {
		t.Errorf("expected no error for empty dependencies, got: %v", err)
	}
}

func TestValidateDAG_ValidDAG(t *testing.T) {
	subtasks := []string{"task1", "task2", "task3"}
	deps := []SubtaskDependency{
		{DependentID: "task2", DependencyID: "task1"},
		{DependentID: "task3", DependencyID: "task2"},
	}

	err := ValidateDAG(subtasks, deps)
	if err != nil {
		t.Errorf("expected no error for valid DAG, got: %v", err)
	}
}

func TestValidateDAG_SimpleCycle(t *testing.T) {
	subtasks := []string{"task1", "task2"}
	deps := []SubtaskDependency{
		{DependentID: "task1", DependencyID: "task2"},
		{DependentID: "task2", DependencyID: "task1"},
	}

	err := ValidateDAG(subtasks, deps)
	if err == nil {
		t.Error("expected error for circular dependency, got nil")
	}
}

func TestValidateDAG_SelfLoop(t *testing.T) {
	subtasks := []string{"task1"}
	deps := []SubtaskDependency{
		{DependentID: "task1", DependencyID: "task1"},
	}

	err := ValidateDAG(subtasks, deps)
	if err == nil {
		t.Error("expected error for self-loop, got nil")
	}
}

func TestValidateDAG_ComplexCycle(t *testing.T) {
	subtasks := []string{"task1", "task2", "task3", "task4"}
	deps := []SubtaskDependency{
		{DependentID: "task2", DependencyID: "task1"},
		{DependentID: "task3", DependencyID: "task2"},
		{DependentID: "task4", DependencyID: "task3"},
		{DependentID: "task1", DependencyID: "task4"}, // Creates cycle
	}

	err := ValidateDAG(subtasks, deps)
	if err == nil {
		t.Error("expected error for complex cycle, got nil")
	}
}

func TestValidateDAG_InvalidDependentID(t *testing.T) {
	subtasks := []string{"task1", "task2"}
	deps := []SubtaskDependency{
		{DependentID: "task3", DependencyID: "task1"}, // task3 doesn't exist
	}

	err := ValidateDAG(subtasks, deps)
	if err == nil {
		t.Error("expected error for invalid dependent ID, got nil")
	}
}

func TestValidateDAG_InvalidDependencyID(t *testing.T) {
	subtasks := []string{"task1", "task2"}
	deps := []SubtaskDependency{
		{DependentID: "task1", DependencyID: "task3"}, // task3 doesn't exist
	}

	err := ValidateDAG(subtasks, deps)
	if err == nil {
		t.Error("expected error for invalid dependency ID, got nil")
	}
}

func TestValidateDAG_MultipleValidPaths(t *testing.T) {
	// Diamond pattern: task1 -> task2 -> task4
	//                  task1 -> task3 -> task4
	subtasks := []string{"task1", "task2", "task3", "task4"}
	deps := []SubtaskDependency{
		{DependentID: "task2", DependencyID: "task1"},
		{DependentID: "task3", DependencyID: "task1"},
		{DependentID: "task4", DependencyID: "task2"},
		{DependentID: "task4", DependencyID: "task3"},
	}

	err := ValidateDAG(subtasks, deps)
	if err != nil {
		t.Errorf("expected no error for diamond DAG pattern, got: %v", err)
	}
}
