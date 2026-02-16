package executor

import (
	"fmt"
	"log/slog"

	"github.com/circle-oo/flux/internal/executor/prompts"
	"github.com/circle-oo/flux/internal/models"
)

// BuildAutopilotPrompt creates an enriched prompt that includes triage analysis,
// guiding Claude Code to analyze → plan → execute in one autopilot session.
func BuildAutopilotPrompt(task *models.Task, projectName, projectDesc, projectTech string) string {
	result, err := prompts.Render("autopilot.txt", prompts.AutopilotData{
		Title:              task.Title,
		Description:        task.Description,
		TriageDescription:  task.TriageDescription,
		TriageAnalysis:     task.TriageAnalysis,
		Prompt:             task.Prompt,
		ProjectName:        projectName,
		ProjectDescription: projectDesc,
		ProjectTechStack:   projectTech,
		TaskType:           task.Type,
	})
	if err != nil {
		slog.Warn("failed to render autopilot prompt template, using fallback", "error", err)
		return fmt.Sprintf("# Task: %s\n\n%s\n\nImplement this task. Follow existing conventions.", task.Title, task.Description)
	}
	return result
}
