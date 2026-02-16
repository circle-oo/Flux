package executor

import (
	"os"
	"path/filepath"
	"testing"
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

	passed, output := e.runBuild(tmpDir)
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

	passed, output := e.runBuild(tmpDir)
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

	passed, output := e.runBuild(tmpDir)
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

	passed, output := e.runBuild(tmpDir)
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

	passed, _ := e.runBuild(tmpDir)
	if passed {
		t.Error("expected Makefile build to fail")
	}
}
