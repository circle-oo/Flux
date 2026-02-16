package github

import (
	"strings"
	"testing"

	"github.com/circle-oo/flux/internal/models"
)

func TestPRDescriptionBuilder_Build(t *testing.T) {
	t.Run("basic PR description with result", func(t *testing.T) {
		task := &models.Task{
			ID:           "task-123",
			Title:        "Add user authentication",
			Description:  "Implement JWT-based authentication for the API",
			
			Priority:     5,
			Source:       models.TaskSourceOperator,
			DiffLines:    150,
			FilesChanged: 3,
			Result:       "Successfully implemented JWT authentication with token validation and user login endpoints.",
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
			"Successfully implemented JWT authentication", // Task result
			"👀 Review Points",
			"**Priority:** 5",
			"*Automated by Flux executor `executor-1`*",
			"*Task ID: `task-123`*",
		}

		for _, section := range requiredSections {
			if !strings.Contains(body, section) {
				t.Errorf("expected body to contain '%s', but it's missing.\nFull body:\n%s", section, body)
			}
		}

		// Verify that commits and file changes are NOT present
		unwantedSections := []string{
			"### Commits",
			"### Files Changed",
			"### Summary",
		}

		for _, section := range unwantedSections {
			if strings.Contains(body, section) {
				t.Errorf("expected body NOT to contain '%s', but it's present.\nFull body:\n%s", section, body)
			}
		}
	})

	t.Run("handles missing result", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-456",
			Title:       "Fix login bug",
			Description: "Fix issue where users can't log in",
			
			Priority:    3,
			Result:      "", // No result
		}

		builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-2", "main")
		_, body := builder.Build()

		if !strings.Contains(body, "*No task result available*") {
			t.Errorf("expected body to contain fallback message for missing result")
		}
	})

	t.Run("includes test status when available", func(t *testing.T) {
		testPassed := true
		task := &models.Task{
			ID:          "task-456",
			Title:       "Fix login bug",
			Description: "Fix issue where users can't log in",
			
			Priority:    3,
			TestPassed:  &testPassed,
			Result:      "Fixed authentication issue",
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
			
			Priority:    10,
			Tags:        []string{"refactor", "database", "performance"},
			GoalID:      "goal-1",
			AlertID:     "alert-5",
			Model:       "opus",
			Result:      "Refactored database connection pooling",
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
			
			Priority:     5,
			DiffLines:    2500,
			FilesChanged: 25,
			Result:       "Completed large refactoring",
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
	task := &models.Task{
		ID:          "task-test",
		Title:       "Test task",
		Description: "Test description",
		
		Priority:    5,
	}

	builder := NewPRDescriptionBuilder(task, "/tmp/test", "executor-test", "main")
	points := builder.generateReviewPoints()

	// All tasks should have common review points
	commonPoints := []string{
		"Code follows existing conventions and style",
		"Changes are focused and minimal",
		"No unintended side effects",
		"Implementation meets requirements",
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
}


func TestPRDescriptionBuilder_BaseBranch(t *testing.T) {
	t.Run("uses provided base branch", func(t *testing.T) {
		task := &models.Task{
			ID:          "task-base",
			Title:       "Test base branch",
			Description: "Testing base branch parameter",
			
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
