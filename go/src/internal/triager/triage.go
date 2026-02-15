package triager

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/models"
)

//go:embed triage.txt
var triagePromptFS embed.FS

var triageTemplate *template.Template

func init() {
	var err error
	triageTemplate, err = template.ParseFS(triagePromptFS, "triage.txt")
	if err != nil {
		panic(fmt.Sprintf("failed to parse triage prompt template: %v", err))
	}
}

// triageData holds data for triage.txt template.
type triageData struct {
	Title       string
	Type        string
	Priority    int
	Description string
	Tags        string
	ProjectName string
}

// TriageResult contains the output of task triage analysis.
type TriageResult struct {
	Analysis    string // Structured analysis of the task
	Priority    int    // Suggested priority (1-100)
	Description string // Rewritten description with clear requirements
}

// TriageTask uses Claude to analyze a task, rewrite its description with clear
// requirements, and suggest a priority level. Called asynchronously after task creation.
func TriageTask(ctx context.Context, runner *executor.ClaudeCodeRunner, task *models.Task) (*TriageResult, error) {
	slog.Info("triaging task", "task_id", task.ID, "title", task.Title)

	prompt := buildTriagePrompt(task)

	triageCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	result, err := runner.Run(triageCtx, executor.ClaudeCodeOpts{
		Prompt:   prompt,
		Model:    "haiku",
		MaxTurns: 1,
		WorkDir:  "/tmp",
	})
	if err != nil {
		return nil, fmt.Errorf("triage execution failed: %w", err)
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("triage exited with code %d", result.ExitCode)
	}

	// Parse the response
	parsed, err := executor.ParseResponse(result.Stdout)
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

	var buf strings.Builder
	err := triageTemplate.ExecuteTemplate(&buf, "triage.txt", triageData{
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
	return buf.String()
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
