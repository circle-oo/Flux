package vault

import (
	"fmt"
	"strings"
)

// TaskSummaryData holds data for task summary template
type TaskSummaryData struct {
	ID           string
	Title        string
	Type         string
	Status       string
	Priority     int
	Model        string
	Duration     string
	CreatedAt    string
	CompletedAt  string
	PRUrl        string
	PRStatus     string
	DiffLines    int
	FilesChanged int
	Result       string
}

// TaskSummaryTemplate generates markdown for task summary
func TaskSummaryTemplate(task TaskSummaryData) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Task: %s\n\n", task.Title))
	b.WriteString(fmt.Sprintf("**ID**: %s\n", task.ID))
	b.WriteString(fmt.Sprintf("**Type**: %s\n", task.Type))
	b.WriteString(fmt.Sprintf("**Status**: %s\n", task.Status))
	b.WriteString(fmt.Sprintf("**Priority**: %d\n", task.Priority))
	b.WriteString(fmt.Sprintf("**Model**: %s\n", task.Model))
	b.WriteString(fmt.Sprintf("**Duration**: %s\n", task.Duration))
	b.WriteString(fmt.Sprintf("**Created**: %s\n", task.CreatedAt))
	b.WriteString(fmt.Sprintf("**Completed**: %s\n\n", task.CompletedAt))

	b.WriteString("## PR\n")
	b.WriteString(fmt.Sprintf("- URL: %s\n", task.PRUrl))
	b.WriteString(fmt.Sprintf("- Status: %s\n", task.PRStatus))
	b.WriteString(fmt.Sprintf("- Diff: +%d lines, %d files\n\n", task.DiffLines, task.FilesChanged))

	b.WriteString("## Result\n")
	b.WriteString(task.Result)
	b.WriteString("\n")

	return b.String()
}

// DecisionTemplate generates markdown for decision record
func DecisionTemplate(title, context, decision, rationale string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Decision: %s\n\n", title))

	b.WriteString("## Context\n")
	b.WriteString(context)
	b.WriteString("\n\n")

	b.WriteString("## Decision\n")
	b.WriteString(decision)
	b.WriteString("\n\n")

	b.WriteString("## Rationale\n")
	b.WriteString(rationale)
	b.WriteString("\n")

	return b.String()
}

// ProjectIndexTemplate generates markdown for project overview
func ProjectIndexTemplate(name, projectType, repoURL, description string, techStack []string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Project: %s\n\n", name))
	b.WriteString(fmt.Sprintf("**Type**: %s\n", projectType))
	b.WriteString(fmt.Sprintf("**Repository**: %s\n\n", repoURL))

	b.WriteString("## Description\n")
	b.WriteString(description)
	b.WriteString("\n\n")

	if len(techStack) > 0 {
		b.WriteString("## Tech Stack\n")
		for _, tech := range techStack {
			b.WriteString(fmt.Sprintf("- %s\n", tech))
		}
		b.WriteString("\n")
	}

	return b.String()
}
