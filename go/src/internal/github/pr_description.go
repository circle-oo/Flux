package github

import (
	"fmt"
	"strings"

	"github.com/circle-oo/flux/internal/models"
)

// PRDescriptionBuilder builds rich PR descriptions with structured sections.
type PRDescriptionBuilder struct {
	task         *models.Task
	worktreePath string
	executorID   string
	baseBranch   string
}

// NewPRDescriptionBuilder creates a new PR description builder.
func NewPRDescriptionBuilder(task *models.Task, worktreePath, executorID, baseBranch string) *PRDescriptionBuilder {
	if baseBranch == "" {
		baseBranch = "main"
	}
	return &PRDescriptionBuilder{
		task:         task,
		worktreePath: worktreePath,
		executorID:   executorID,
		baseBranch:   baseBranch,
	}
}

// Build generates a rich PR description with three main sections:
// 1. Requirements/Problem/Issues
// 2. What was done in this PR
// 3. Review points
func (b *PRDescriptionBuilder) Build() (title, body string) {
	title = fmt.Sprintf("[flux] %s", b.task.Title)

	var sections []string

	// Section 1: Requirements/Problem/Issues
	sections = append(sections, b.buildRequirementsSection())

	// Section 2: What was done in this PR
	sections = append(sections, b.buildImplementationSection())

	// Section 3: Review Points
	sections = append(sections, b.buildReviewPointsSection())

	// Footer
	sections = append(sections, b.buildFooter())

	body = strings.Join(sections, "\n\n")
	return title, body
}

// buildRequirementsSection creates the "Requirements/Problem/Issues" section.
func (b *PRDescriptionBuilder) buildRequirementsSection() string {
	var sb strings.Builder
	sb.WriteString("## 📋 Requirements / Problem / Issues\n\n")

	// Task description
	if b.task.Description != "" {
		sb.WriteString(b.task.Description)
		sb.WriteString("\n\n")
	}

	// Task metadata (moved up for better visibility)
	sb.WriteString(fmt.Sprintf("**Priority:** %d\n", b.task.Priority))

	// Triage analysis (if available) - now includes priority reasoning
	if b.task.TriageAnalysis != "" {
		sb.WriteString("\n### Triage Analysis\n\n")
		sb.WriteString(b.task.TriageAnalysis)
		sb.WriteString("\n\n")
	}

	if b.task.Source != "" {
		sb.WriteString(fmt.Sprintf("**Source:** %s\n", b.task.Source))
	}

	if len(b.task.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("**Tags:** %s\n", strings.Join(b.task.Tags, ", ")))
	}

	if b.task.AlertID != "" {
		sb.WriteString(fmt.Sprintf("**Alert ID:** %s\n", b.task.AlertID))
	}

	if b.task.GoalID != "" {
		sb.WriteString(fmt.Sprintf("**Goal ID:** %s\n", b.task.GoalID))
	}

	return sb.String()
}

// buildImplementationSection creates the "What was done" section.
func (b *PRDescriptionBuilder) buildImplementationSection() string {
	var sb strings.Builder
	sb.WriteString("## 🔨 What Was Done in This PR\n\n")

	// Show only the task result
	if b.task.Result != "" {
		sb.WriteString(b.task.Result)
		sb.WriteString("\n")
	} else {
		sb.WriteString("*No task result available*\n")
	}

	return sb.String()
}

// buildReviewPointsSection creates the "Review Points" section.
func (b *PRDescriptionBuilder) buildReviewPointsSection() string {
	var sb strings.Builder
	sb.WriteString("## 👀 Review Points\n\n")

	// Generate review checklist based on task type
	reviewPoints := b.generateReviewPoints()
	for _, point := range reviewPoints {
		sb.WriteString(fmt.Sprintf("- [ ] %s\n", point))
	}

	// Test status
	if b.task.TestPassed != nil {
		sb.WriteString("\n### Test Status\n\n")
		if *b.task.TestPassed {
			sb.WriteString("✅ All tests passed\n")
		} else {
			sb.WriteString("❌ Tests failed\n")
		}
	}

	// Guardrails check
	if b.task.DiffLines > 0 || b.task.FilesChanged > 0 {
		sb.WriteString("\n### Change Scope\n\n")

		changeSize := "Small"
		if b.task.DiffLines > 500 || b.task.FilesChanged > 10 {
			changeSize = "Large"
		} else if b.task.DiffLines > 200 || b.task.FilesChanged > 5 {
			changeSize = "Medium"
		}

		sb.WriteString(fmt.Sprintf("**Size:** %s (%d lines, %d files)\n",
			changeSize, b.task.DiffLines, b.task.FilesChanged))

		if changeSize == "Large" {
			sb.WriteString("\n⚠️ This is a large change. Please review carefully.\n")
		}
	}

	return sb.String()
}

// generateReviewPoints creates generic review points for all tasks.
func (b *PRDescriptionBuilder) generateReviewPoints() []string {
	points := []string{
		"Code follows existing conventions and style",
		"Changes are focused and minimal",
		"No unintended side effects",
		"Implementation meets requirements",
		"Error handling is appropriate",
		"Tests are adequate",
	}

	return points
}

// buildFooter creates the footer section.
func (b *PRDescriptionBuilder) buildFooter() string {
	var sb strings.Builder
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("*Automated by Flux executor `%s`*\n", b.executorID))
	sb.WriteString(fmt.Sprintf("*Task ID: `%s`*", b.task.ID))

	if b.task.Model != "" {
		sb.WriteString(fmt.Sprintf(" | *Model: `%s`*", b.task.Model))
	}

	return sb.String()
}

