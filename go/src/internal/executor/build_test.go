package executor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/circle-oo/flux/internal/models"
)

func TestRunBuild_GoProject(t *testing.T) {
	// Create a temporary directory with a go.mod file and valid Go code
	tmpDir := t.TempDir()

	// Write a valid go.mod
	goMod := `module testmod

go 1.22
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Write valid Go source
	goSrc := `package main

func main() {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	e := &Executor{}
	task := &models.Task{Type: models.TaskTypeCoding}

	passed, output := e.runBuild(tmpDir, task)
	if !passed {
		t.Errorf("expected build to pass, got failure: %s", output)
	}
}

func TestRunBuild_GoProject_Failure(t *testing.T) {
	// Create a temporary directory with a go.mod file and invalid Go code
	tmpDir := t.TempDir()

	goMod := `module testmod

go 1.22
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Write invalid Go source that won't compile
	goSrc := `package main

func main() {
	undefinedVariable
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(goSrc), 0644); err != nil {
		t.Fatal(err)
	}

	e := &Executor{}
	task := &models.Task{Type: models.TaskTypeCoding}

	passed, output := e.runBuild(tmpDir, task)
	if passed {
		t.Error("expected build to fail for invalid Go code")
	}
	if output == "" {
		t.Error("expected non-empty build output on failure")
	}
}

func TestRunBuild_NoBuildSystem(t *testing.T) {
	// Empty directory — no build system detected
	tmpDir := t.TempDir()

	e := &Executor{}
	task := &models.Task{Type: models.TaskTypeCoding}

	passed, output := e.runBuild(tmpDir, task)
	if !passed {
		t.Errorf("expected pass when no build system detected, got failure: %s", output)
	}
	if output != "" {
		t.Errorf("expected empty output, got: %s", output)
	}
}

func TestRunBuild_MakefileProject(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a simple Makefile with a build target
	makefile := `build:
	@echo "build ok"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	e := &Executor{}
	task := &models.Task{Type: models.TaskTypeCoding}

	passed, output := e.runBuild(tmpDir, task)
	if !passed {
		t.Errorf("expected Makefile build to pass, got failure: %s", output)
	}
}

func TestRunBuild_MakefileProject_Failure(t *testing.T) {
	tmpDir := t.TempDir()

	// Makefile with a build target that fails
	makefile := `build:
	@exit 1
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	e := &Executor{}
	task := &models.Task{Type: models.TaskTypeCoding}

	passed, _ := e.runBuild(tmpDir, task)
	if passed {
		t.Error("expected Makefile build to fail")
	}
}

func TestRegisterBuildFailureTask_Fields(t *testing.T) {
	// Verify the bugfix task is constructed with correct fields.
	// We use a mock ManagerClient to capture the created task.
	failedTask := &models.Task{
		ID:         "task-123",
		Title:      "Original Task",
		Type:       models.TaskTypeCoding,
		Priority:   30,
		Source:     models.TaskSourceOperator,
		ProjectID:  "project-abc",
		GoalID:     "goal-xyz",
		BranchName: "task/task-123",
	}

	buildOutput := "main.go:5:2: undefined: foo"

	// Test the task construction logic directly
	bugfixTask := buildFailureTask(failedTask, buildOutput)

	if bugfixTask.Type != models.TaskTypeBugfix {
		t.Errorf("expected type BUGFIX, got %s", bugfixTask.Type)
	}
	if bugfixTask.Priority != 10 {
		t.Errorf("expected priority 10 (capped), got %d", bugfixTask.Priority)
	}
	if bugfixTask.Source != models.TaskSourceSystem {
		t.Errorf("expected source SYSTEM, got %s", bugfixTask.Source)
	}
	if bugfixTask.ProjectID != failedTask.ProjectID {
		t.Errorf("expected project_id %s, got %s", failedTask.ProjectID, bugfixTask.ProjectID)
	}
	if bugfixTask.GoalID != failedTask.GoalID {
		t.Errorf("expected goal_id %s, got %s", failedTask.GoalID, bugfixTask.GoalID)
	}
	if bugfixTask.BranchName != failedTask.BranchName {
		t.Errorf("expected branch_name %s, got %s", failedTask.BranchName, bugfixTask.BranchName)
	}
	if len(bugfixTask.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(bugfixTask.Tags))
	}

	// Check that build output is included in description
	if bugfixTask.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestRegisterBuildFailureTask_PriorityAlreadyHigh(t *testing.T) {
	failedTask := &models.Task{
		ID:       "task-456",
		Title:    "High Priority Task",
		Priority: 5, // Already higher than 10
	}

	bugfixTask := buildFailureTask(failedTask, "error")

	if bugfixTask.Priority != 5 {
		t.Errorf("expected priority 5 (preserved), got %d", bugfixTask.Priority)
	}
}

func TestRegisterBuildFailureTask_OutputTruncation(t *testing.T) {
	failedTask := &models.Task{
		ID:    "task-789",
		Title: "Task With Long Output",
	}

	// Create output longer than maxOutputLen (2000)
	longOutput := ""
	for i := 0; i < 300; i++ {
		longOutput += "error: something went wrong on this line\n"
	}

	bugfixTask := buildFailureTask(failedTask, longOutput)

	// Description should contain truncated output, not the full thing
	if len(bugfixTask.Description) > 3000 {
		t.Errorf("description too long (%d chars), expected truncation", len(bugfixTask.Description))
	}
}
