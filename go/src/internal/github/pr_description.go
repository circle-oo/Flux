package github

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/circle-oo/flux/internal/models"
)

// PRDescriptionBuilder builds rich PR descriptions with structured sections.
type PRDescriptionBuilder struct {
	task        *models.Task
	worktreePath string
	executorID  string
}

// NewPRDescriptionBuilder creates a new PR description builder.
func NewPRDescriptionBuilder(task *models.Task, worktreePath, executorID string) *PRDescriptionBuilder {
	return &PRDescriptionBuilder{
		task:        task,
		worktreePath: worktreePath,
		executorID:  executorID,
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
	sb.WriteString(fmt.Sprintf("**Task Type:** %s\n", b.task.Type))
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

	// Get commit information
	commits := b.getCommitSummary()
	if len(commits) > 0 {
		sb.WriteString("### Commits\n\n")
		for _, commit := range commits {
			sb.WriteString(fmt.Sprintf("- %s\n", commit))
		}
		sb.WriteString("\n")
	}

	// Get file changes
	fileChanges := b.getFileChanges()
	if len(fileChanges) > 0 {
		sb.WriteString("### Files Changed\n\n")
		for _, change := range fileChanges {
			sb.WriteString(fmt.Sprintf("- %s\n", change))
		}
		sb.WriteString("\n")
	}

	// Statistics
	if b.task.DiffLines > 0 || b.task.FilesChanged > 0 {
		sb.WriteString("### Statistics\n\n")
		if b.task.FilesChanged > 0 {
			sb.WriteString(fmt.Sprintf("- **Files Changed:** %d\n", b.task.FilesChanged))
		}
		if b.task.DiffLines > 0 {
			sb.WriteString(fmt.Sprintf("- **Lines Changed:** %d\n", b.task.DiffLines))
		}
	}

	// Additional prompt if provided
	if b.task.Prompt != "" {
		sb.WriteString("\n### Additional Implementation Notes\n\n")
		sb.WriteString(b.task.Prompt)
		sb.WriteString("\n")
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

// generateReviewPoints creates task-specific review points.
func (b *PRDescriptionBuilder) generateReviewPoints() []string {
	points := []string{
		"Code follows existing conventions and style",
		"Changes are focused and minimal",
		"No unintended side effects",
	}

	switch b.task.Type {
	case models.TaskTypeCoding:
		points = append(points,
			"New functionality works as expected",
			"Error handling is appropriate",
			"Tests cover new code paths",
		)
	case models.TaskTypeBugfix:
		points = append(points,
			"Bug is fixed and root cause addressed",
			"Fix doesn't introduce new issues",
			"Regression test added",
		)
	case models.TaskTypeDocument:
		points = append(points,
			"Documentation is clear and accurate",
			"Examples are helpful",
			"Links and references are valid",
		)
	case models.TaskTypeMaintenance:
		points = append(points,
			"Dependencies are up to date",
			"No breaking changes",
			"Backwards compatibility maintained",
		)
	case models.TaskTypeResearch:
		points = append(points,
			"Research findings are documented",
			"Sources are cited",
			"Recommendations are actionable",
		)
	case models.TaskTypeDeploy:
		points = append(points,
			"Deployment steps are validated",
			"Rollback plan is in place",
			"No service disruption expected",
		)
	case models.TaskTypePlanning:
		points = append(points,
			"Plan is comprehensive",
			"Dependencies are identified",
			"Timeline is realistic",
		)
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

// getCommitSummary retrieves commit messages from the current branch.
func (b *PRDescriptionBuilder) getCommitSummary() []string {
	cmd := exec.Command("git", "log", "--format=%s", "main..HEAD")
	cmd.Dir = b.worktreePath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var commits []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			commits = append(commits, line)
		}
	}

	return commits
}

// getFileChanges retrieves the list of files changed with their change type.
func (b *PRDescriptionBuilder) getFileChanges() []string {
	cmd := exec.Command("git", "diff", "--name-status", "main..HEAD")
	cmd.Dir = b.worktreePath
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var changes []string

	changeTypeMap := map[string]string{
		"A": "Added",
		"M": "Modified",
		"D": "Deleted",
		"R": "Renamed",
		"C": "Copied",
	}

	// Regex to match git diff --name-status output (e.g., "M\tfile.go" or "R100\told.go\tnew.go")
	re := regexp.MustCompile(`^([AMDRC])(\d*)\s+(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) >= 4 {
			changeType := matches[1]
			fileName := matches[3]

			// Handle renamed files (format: "old\tnew")
			if changeType == "R" && strings.Contains(fileName, "\t") {
				parts := strings.Split(fileName, "\t")
				if len(parts) == 2 {
					fileName = fmt.Sprintf("%s → %s", parts[0], parts[1])
				}
			}

			typeLabel := changeTypeMap[changeType]
			if typeLabel == "" {
				typeLabel = "Modified"
			}

			changes = append(changes, fmt.Sprintf("**%s**: `%s`", typeLabel, fileName))
		}
	}

	return changes
}
