package github

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/circle-oo/flux/internal/models"
)

func TestPRDescriptionBuilder_Build(t *testing.T) {
	t.Run("basic PR description", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-123",
			Title:       "Add user authentication",
			Description: "Implement JWT-based authentication for the API",
			Type:        models.TaskTypeCoding,
			Priority:    5,
			Source:      models.TaskSourceOperator,
			DiffLines:   150,
			FilesChanged: 3,
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-1", "main")
		title, body := builder.Build()

		if title != "[flux] Add user authentication" {
			t.Errorf("expected title '[flux] Add user authentication', got %s", title)
		}

		// Check that all main sections are present
		requiredSections := []string{
			"📋 Requirements / Problem / Issues",
			"🔨 What Was Done in This PR",
			"### Summary",
			"👀 Review Points",
			"**Task Type:** CODING",
			"**Priority:** 5",
			"*Automated by Flux executor `executor-1`*",
			"*Task ID: `task-123`*",
		}

		for _, section := range requiredSections {
			if !strings.Contains(body, section) {
				t.Errorf("expected body to contain '%s', but it's missing.\nFull body:\n%s", section, body)
			}
		}
	})

	t.Run("includes test status when available", func(t *testing.T) {
		testPassed := true
		task := &models.Task{
			ID:          "task-456",
			Title:       "Fix login bug",
			Description: "Fix issue where users can't log in",
			Type:        models.TaskTypeBugfix,
			Priority:    3,
			TestPassed:  &testPassed,
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-2", "main")
		_, body := builder.Build()

		if !strings.Contains(body, "✅ All tests passed") {
			t.Errorf("expected body to contain test status")
		}
	})

	t.Run("includes tags and metadata", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-789",
			Title:       "Refactor database layer",
			Description: "Improve database connection pooling",
			Type:        models.TaskTypeMaintenance,
			Priority:    10,
			Tags:        []string{"refactor", "database", "performance"},
			GoalID:      "goal-1",
			AlertID:     "alert-5",
			Model:       "opus",
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-3", "main")
		_, body := builder.Build()

		expectedContent := []string{
			"**Tags:** refactor, database, performance",
			"**Goal ID:** goal-1",
			"**Alert ID:** alert-5",
			"*Model: `opus`*",
		}

		for _, content := range expectedContent {
			if !strings.Contains(body, content) {
				t.Errorf("expected body to contain '%s'", content)
			}
		}
	})

	t.Run("shows change size warnings", func(t *testing.T) {
		task := &models.Task{
			ID:           "task-large",
			Title:        "Large refactor",
			Description:  "Major refactoring of core modules",
			Type:         models.TaskTypeCoding,
			Priority:     5,
			DiffLines:    2500,
			FilesChanged: 25,
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-4", "main")
		_, body := builder.Build()

		if !strings.Contains(body, "**Size:** Large") {
			t.Errorf("expected body to contain 'Large' size indicator")
		}

		if !strings.Contains(body, "⚠️ This is a large change") {
			t.Errorf("expected body to contain warning for large changes")
		}
	})
}

func TestPRDescriptionBuilder_GenerateReviewPoints(t *testing.T) {
	tests := []struct {
		name          string
		taskType      string
		expectedPoint string
	}{
		{
			name:          "coding task",
			taskType:      models.TaskTypeCoding,
			expectedPoint: "New functionality works as expected",
		},
		{
			name:          "bugfix task",
			taskType:      models.TaskTypeBugfix,
			expectedPoint: "Bug is fixed and root cause addressed",
		},
		{
			name:          "document task",
			taskType:      models.TaskTypeDocument,
			expectedPoint: "Documentation is clear and accurate",
		},
		{
			name:          "maintenance task",
			taskType:      models.TaskTypeMaintenance,
			expectedPoint: "Dependencies are up to date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:          "task-test",
				Title:       "Test task",
				Description: "Test description",
				Type:        tt.taskType,
				Priority:    5,
			}

			builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-test", "main")
			points := builder.generateReviewPoints()

			found := false
			for _, point := range points {
				if strings.Contains(point, tt.expectedPoint) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("expected review points to contain '%s' for task type %s", tt.expectedPoint, tt.taskType)
			}

			// All tasks should have common review points
			commonPoints := []string{
				"Code follows existing conventions and style",
				"Changes are focused and minimal",
			}

			for _, common := range commonPoints {
				found := false
				for _, point := range points {
					if strings.Contains(point, common) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected review points to contain common point: '%s'", common)
				}
			}
		})
	}
}

func TestPRDescriptionBuilder_GetCommitSummary(t *testing.T) {
	t.Run("parses commit messages", func(t *testing.T) {
		// Create a temporary git repo for testing
		tmpDir := t.TempDir()

		// Initialize git repo
		initCmd := exec.Command("git", "init")
		initCmd.Dir = tmpDir
		if err := initCmd.Run(); err != nil {
			t.Skip("git not available, skipping test")
		}

		// Configure git user
		configCmd1 := exec.Command("git", "config", "user.name", "Test User")
		configCmd1.Dir = tmpDir
		configCmd1.Run()
		configCmd2 := exec.Command("git", "config", "user.email", "test@example.com")
		configCmd2.Dir = tmpDir
		configCmd2.Run()

		// Create main branch with initial commit
		readmePath := filepath.Join(tmpDir, "README.md")
		os.WriteFile(readmePath, []byte("# Test"), 0644)
		addCmd1 := exec.Command("git", "add", "README.md")
		addCmd1.Dir = tmpDir
		addCmd1.Run()
		commitCmd1 := exec.Command("git", "commit", "-m", "Initial commit")
		commitCmd1.Dir = tmpDir
		commitCmd1.Run()
		branchCmd := exec.Command("git", "branch", "-M", "main")
		branchCmd.Dir = tmpDir
		branchCmd.Run()

		// Create a new branch
		checkoutCmd := exec.Command("git", "checkout", "-b", "feature")
		checkoutCmd.Dir = tmpDir
		checkoutCmd.Run()

		// Add commits to feature branch
		file1Path := filepath.Join(tmpDir, "file1.txt")
		os.WriteFile(file1Path, []byte("content1"), 0644)
		addCmd2 := exec.Command("git", "add", "file1.txt")
		addCmd2.Dir = tmpDir
		addCmd2.Run()
		commitCmd2 := exec.Command("git", "commit", "-m", "Add authentication module")
		commitCmd2.Dir = tmpDir
		commitCmd2.Run()

		file2Path := filepath.Join(tmpDir, "file2.txt")
		os.WriteFile(file2Path, []byte("content2"), 0644)
		addCmd3 := exec.Command("git", "add", "file2.txt")
		addCmd3.Dir = tmpDir
		addCmd3.Run()
		commitCmd3 := exec.Command("git", "commit", "-m", "Add JWT token validation")
		commitCmd3.Dir = tmpDir
		commitCmd3.Run()

		// Test the commit summary
		task := &models.Task{
			ID:          "task-git",
			Title:       "Test git operations",
			Description: "Testing git commit parsing",
			Type:        models.TaskTypeCoding,
			Priority:    5,
		}

		builder := NewPRDescriptionBuilder(task, tmpDir, "executor-test", "main")
		commits := builder.getCommitSummary()

		if len(commits) != 2 {
			t.Errorf("expected 2 commits, got %d", len(commits))
		}

		expectedCommits := []string{
			"Add JWT token validation",
			"Add authentication module",
		}

		for i, expected := range expectedCommits {
			if i < len(commits) && commits[i] != expected {
				t.Errorf("expected commit[%d] to be '%s', got '%s'", i, expected, commits[i])
			}
		}
	})
}

func TestPRDescriptionBuilder_GetFileChanges(t *testing.T) {
	t.Run("parses file changes with status", func(t *testing.T) {
		// Create a temporary git repo for testing
		tmpDir := t.TempDir()

		// Initialize git repo
		initCmd := exec.Command("git", "init")
		initCmd.Dir = tmpDir
		if err := initCmd.Run(); err != nil {
			t.Skip("git not available, skipping test")
		}

		// Configure git user
		configCmd1 := exec.Command("git", "config", "user.name", "Test User")
		configCmd1.Dir = tmpDir
		configCmd1.Run()
		configCmd2 := exec.Command("git", "config", "user.email", "test@example.com")
		configCmd2.Dir = tmpDir
		configCmd2.Run()

		// Create main branch with initial commit
		readmePath := filepath.Join(tmpDir, "README.md")
		os.WriteFile(readmePath, []byte("# Test"), 0644)
		existingPath := filepath.Join(tmpDir, "existing.txt")
		os.WriteFile(existingPath, []byte("existing content"), 0644)
		addCmd1 := exec.Command("git", "add", ".")
		addCmd1.Dir = tmpDir
		addCmd1.Run()
		commitCmd1 := exec.Command("git", "commit", "-m", "Initial commit")
		commitCmd1.Dir = tmpDir
		commitCmd1.Run()
		branchCmd := exec.Command("git", "branch", "-M", "main")
		branchCmd.Dir = tmpDir
		branchCmd.Run()

		// Create a new branch
		checkoutCmd := exec.Command("git", "checkout", "-b", "feature")
		checkoutCmd.Dir = tmpDir
		checkoutCmd.Run()

		// Add new file
		newFilePath := filepath.Join(tmpDir, "new.txt")
		os.WriteFile(newFilePath, []byte("new content"), 0644)
		addCmd2 := exec.Command("git", "add", "new.txt")
		addCmd2.Dir = tmpDir
		addCmd2.Run()

		// Modify existing file
		os.WriteFile(existingPath, []byte("modified content"), 0644)
		addCmd3 := exec.Command("git", "add", "existing.txt")
		addCmd3.Dir = tmpDir
		addCmd3.Run()

		commitCmd2 := exec.Command("git", "commit", "-m", "Add and modify files")
		commitCmd2.Dir = tmpDir
		commitCmd2.Run()

		// Test file changes
		task := &models.Task{
			ID:          "task-files",
			Title:       "Test file changes",
			Description: "Testing file change parsing",
			Type:        models.TaskTypeCoding,
			Priority:    5,
		}

		builder := NewPRDescriptionBuilder(task, tmpDir, "executor-test", "main")
		changes := builder.getFileChanges()

		if len(changes) < 2 {
			t.Errorf("expected at least 2 file changes, got %d", len(changes))
		}

		// Check that changes contain expected patterns
		changesStr := strings.Join(changes, " ")
		if !strings.Contains(changesStr, "new.txt") {
			t.Errorf("expected changes to contain new.txt")
		}
		if !strings.Contains(changesStr, "existing.txt") {
			t.Errorf("expected changes to contain existing.txt")
		}
	})
}

func TestPRDescriptionBuilder_GenerateImplementationSummary(t *testing.T) {
	t.Run("generates complete summary", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-summary",
			Title:       "Add user authentication",
			Description: "Implement JWT-based authentication for the API to secure user access and protect sensitive endpoints",
			Type:        models.TaskTypeCoding,
			Priority:    5,
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-test", "main")
		summary := builder.generateImplementationSummary()

		// Check that summary contains key elements (Key Changes is optional if no commits/files)
		expectedElements := []string{
			"Add user authentication",
			"**Why:**",
			"**Impact:**",
		}

		for _, element := range expectedElements {
			if !strings.Contains(summary, element) {
				t.Errorf("expected summary to contain '%s', but it's missing.\nFull summary:\n%s", element, summary)
			}
		}

		// Impact should mention functionality
		if !strings.Contains(summary, "functionality") {
			t.Errorf("expected summary to mention functionality for CODING task")
		}
	})

	t.Run("includes task-specific impact", func(t *testing.T) {
		tests := []struct {
			taskType      string
			expectedImpact string
		}{
			{models.TaskTypeCoding, "Adds new functionality"},
			{models.TaskTypeBugfix, "Fixes a bug"},
			{models.TaskTypeDocument, "Improves documentation"},
			{models.TaskTypeMaintenance, "Maintains code quality"},
		}

		for _, tt := range tests {
			t.Run(tt.taskType, func(t *testing.T) {
				task := &models.Task{
					ID:          "task-test",
					Title:       "Test task",
					Description: "Test description",
					Type:        tt.taskType,
					Priority:    5,
				}

				builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-test", "main")
				summary := builder.generateImplementationSummary()

				if !strings.Contains(summary, tt.expectedImpact) {
					t.Errorf("expected summary to contain impact '%s' for task type %s", tt.expectedImpact, tt.taskType)
				}
			})
		}
	})
}

func TestPRDescriptionBuilder_BaseBranch(t *testing.T) {
	t.Run("uses provided base branch", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-base",
			Title:       "Test base branch",
			Description: "Testing base branch parameter",
			Type:        models.TaskTypeCoding,
			Priority:    5,
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-test", "develop")
		if builder.baseBranch != "develop" {
			t.Errorf("expected base branch to be 'develop', got '%s'", builder.baseBranch)
		}
	})

	t.Run("defaults to main when empty", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-default",
			Title:       "Test default branch",
			Description: "Testing default base branch",
			Type:        models.TaskTypeCoding,
			Priority:    5,
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-test", "")
		if builder.baseBranch != "main" {
			t.Errorf("expected base branch to default to 'main', got '%s'", builder.baseBranch)
		}
	})
}

func TestPRDescriptionBuilder_BuildFooter(t *testing.T) {
	task := &models.Task{
		ID:          "task-footer",
		Title:       "Test footer",
		Description: "Testing footer generation",
		Type:        models.TaskTypeCoding,
		Priority:    5,
		Model:       "sonnet",
	}

	builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-42", "main")
	footer := builder.buildFooter()

	if !strings.Contains(footer, "executor-42") {
		t.Errorf("expected footer to contain executor ID")
	}

	if !strings.Contains(footer, "task-footer") {
		t.Errorf("expected footer to contain task ID")
	}

	if !strings.Contains(footer, "sonnet") {
		t.Errorf("expected footer to contain model name")
	}
}
