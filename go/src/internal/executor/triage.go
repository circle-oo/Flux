package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/executor/prompts"
	"github.com/circle-oo/flux/internal/models"
)

// TriageResult contains the output of task triage analysis.
type TriageResult struct {
	Analysis    string // Structured analysis of the task
	Priority    int    // Suggested priority (1-100)
	Description string // Rewritten description with clear requirements
}

// TriageTask uses Claude to analyze a task, rewrite its description with clear
// requirements, and suggest a priority level. Called asynchronously after task creation.
func TriageTask(ctx context.Context, runner *ClaudeCodeRunner, task *models.Task) (*TriageResult, error) {
	slog.Info("triaging task", "task_id", task.ID, "title", task.Title)

	prompt := buildTriagePrompt(task)

	triageCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := runner.Run(triageCtx, ClaudeCodeOpts{
		Prompt: prompt,
		Model:  "haiku",
		WorkDir: "/tmp",
	})
	if err != nil {
		return nil, fmt.Errorf("triage execution failed: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("triage exited with code %d", result.ExitCode)
	}

	// Parse the response
	parsed, err := ParseResponse(result.Stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to parse triage response: %w", err)
	}

	triage := parseTriageResponse(parsed.ResultText, task)
	slog.Info("triage complete", "task_id", task.ID, "suggested_priority", triage.Priority)

	return triage, nil
}

func buildTriagePrompt(task *models.Task) string {
	tags := ""
	if len(task.Tags) > 0 {
		tags = strings.Join(task.Tags, ", ")
	}
	result, err := prompts.Render("triage.txt", prompts.TriageData{
		Title:       task.Title,
		Type:        task.Type,
		Priority:    task.Priority,
		Description: task.Description,
		Tags:        tags,
	})
	if err != nil {
		slog.Warn("failed to render triage prompt template, using fallback", "error", err)
		return fmt.Sprintf("Analyze this task and suggest priority and rewrite description:\n\nTitle: %s\nType: %s\nDescription: %s", task.Title, task.Type, task.Description)
	}
	return result
}

func parseTriageResponse(text string, task *models.Task) *TriageResult {
	result := &TriageResult{
		Priority:    task.Priority, // default to existing
		Description: task.Description,
	}

	// Parse sections
	sections := map[string]string{}
	currentSection := ""
	var currentLines []string

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		if strings.Contains(upper, "### ANALYSIS") || strings.Contains(upper, "## ANALYSIS") {
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentSection = "analysis"
			currentLines = nil
			continue
		}
		if strings.Contains(upper, "### PRIORITY") || strings.Contains(upper, "## PRIORITY") {
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentSection = "priority"
			currentLines = nil
			continue
		}
		if strings.Contains(upper, "### DESCRIPTION") || strings.Contains(upper, "## DESCRIPTION") {
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentSection = "description"
			currentLines = nil
			continue
		}

		if currentSection != "" {
			currentLines = append(currentLines, line)
		}
	}
	if currentSection != "" {
		sections[currentSection] = strings.TrimSpace(strings.Join(currentLines, "\n"))
	}

	// Extract analysis
	if analysis, ok := sections["analysis"]; ok && analysis != "" {
		result.Analysis = analysis
	}

	// Extract priority
	if priorityText, ok := sections["priority"]; ok && priorityText != "" {
		var p int
		if _, err := fmt.Sscanf(strings.TrimSpace(priorityText), "%d", &p); err == nil && p >= 1 && p <= 100 {
			result.Priority = p
		}
	}

	// Extract rewritten description
	if desc, ok := sections["description"]; ok && desc != "" {
		result.Description = desc
	}

	return result
}

// BuildAutopilotPrompt creates an enriched prompt that includes triage analysis,
// guiding Claude Code to analyze → plan → execute in one autopilot session.
func BuildAutopilotPrompt(task *models.Task, projectName, projectDesc, projectTech string) string {
	result, err := prompts.Render("autopilot.txt", prompts.AutopilotData{
		Title:              task.Title,
		Description:        task.Description,
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
