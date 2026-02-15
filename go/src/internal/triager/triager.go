package triager

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// component tag for structured logging.
const component = "triager"

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

// Triager is a standalone component that polls for PENDING tasks,
// runs triage (Claude analysis), and promotes them to READY.
// Triage determines priority, analysis, rewritten description, and
// the recommended model for task execution.
type Triager struct {
	id       string
	config   *config.Config
	claude   *executor.ClaudeCodeRunner
	client   *executor.ManagerClient
	notifier *notifier.Discord
	stopCh   chan struct{}
}

// New creates a new Triager.
func New(id string, cfg *config.Config, discord *notifier.Discord) *Triager {
	return &Triager{
		id:       id,
		config:   cfg,
		claude:   executor.NewClaudeCodeRunner(&cfg.Executor),
		client:   executor.NewManagerClient(fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)),
		notifier: discord,
		stopCh:   make(chan struct{}),
	}
}

// Run is the main loop. It polls for PENDING tasks and triages them.
func (t *Triager) Run(ctx context.Context) {
	slog.Info("triager started", "id", t.id, "component", component)

	// Smoke test
	if err := t.smokeTest(); err != nil {
		slog.Error("triager smoke test failed", "error", err, "component", component)
		_ = t.notifier.Send(notifier.LevelCritical,
			fmt.Sprintf("Triager %s: smoke test failed: %v", t.id, err))
		return
	}
	slog.Info("triager smoke test passed", "id", t.id, "component", component)

	for {
		select {
		case <-ctx.Done():
			slog.Info("triager stopping (context cancelled)", "id", t.id, "component", component)
			return
		case <-t.stopCh:
			slog.Info("triager stopping (stop signal)", "id", t.id, "component", component)
			return
		default:
			t.processNext(ctx)
			time.Sleep(10 * time.Second)
		}
	}
}

// Stop signals the triager to stop.
func (t *Triager) Stop() {
	close(t.stopCh)
}

// processNext polls for one PENDING task and triages it.
func (t *Triager) processNext(ctx context.Context) {
	task, err := t.client.NextPending(t.id)
	if err != nil {
		slog.Error("failed to get next pending task", "error", err, "component", component)
		return
	}
	if task == nil {
		return
	}

	slog.Info("triaging task", "task_id", task.ID, "title", task.Title, "component", component)

	result, err := TriageTask(ctx, t.claude, task)
	if err != nil {
		slog.Warn("triage failed, promoting with original description",
			"task_id", task.ID, "error", err, "component", component)
		// Even on failure, promote to READY so the task doesn't get stuck
		if reportErr := t.client.ReportTriaged(task.ID, "", "", 0, ""); reportErr != nil {
			slog.Error("failed to promote task after triage failure",
				"task_id", task.ID, "error", reportErr, "component", component)
		}
		return
	}

	if reportErr := t.client.ReportTriaged(task.ID, result.Analysis, result.Description, result.Priority, result.Model); reportErr != nil {
		slog.Error("failed to report triage results",
			"task_id", task.ID, "error", reportErr, "component", component)
		return
	}

	slog.Info("task triaged and promoted to READY",
		"task_id", task.ID, "priority", result.Priority, "model", result.Model, "component", component)
}

// smokeTest verifies the Claude CLI is available.
func (t *Triager) smokeTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	model := t.config.Triager.Model
	if model == "" {
		model = "haiku"
	}

	result, err := t.claude.Run(ctx, executor.ClaudeCodeOpts{
		Prompt:   "respond with exactly: SMOKE_TEST_OK",
		Model:    model,
		MaxTurns: 1,
		WorkDir:  "/tmp",
	})
	if err != nil {
		return fmt.Errorf("smoke test failed: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("smoke test exited with code %d", result.ExitCode)
	}

	return nil
}

// --- Triage execution logic ---

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
	Model       string // Recommended model for execution (opus or sonnet)
}

// TriageTask uses Claude to analyze a task, rewrite its description with clear
// requirements, suggest a priority level, and recommend an execution model.
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
	slog.Info("triage complete",
		"task_id", task.ID, "suggested_priority", triage.Priority, "suggested_model", triage.Model)

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
		return fmt.Sprintf("Analyze this task and suggest priority, model, and rewrite description:\n\nTitle: %s\nType: %s\nDescription: %s", task.Title, task.Type, task.Description)
	}
	return buf.String()
}

// parseSections extracts named sections from markdown-style response text.
// Sections are delimited by ### or ## headings.
func parseSections(text string) map[string]string {
	sections := map[string]string{}
	currentSection := ""
	var currentLines []string

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for known section headers
		sectionName := ""
		for _, name := range []string{"ANALYSIS", "PRIORITY", "DESCRIPTION", "MODEL"} {
			if strings.Contains(upper, "### "+name) || strings.Contains(upper, "## "+name) {
				sectionName = strings.ToLower(name)
				break
			}
		}

		if sectionName != "" {
			if currentSection != "" {
				sections[currentSection] = strings.TrimSpace(strings.Join(currentLines, "\n"))
			}
			currentSection = sectionName
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

	return sections
}

func parseTriageResponse(text string, task *models.Task) *TriageResult {
	result := &TriageResult{
		Priority:    task.Priority, // default to existing
		Description: task.Description,
		Model:       "sonnet", // default model
	}

	sections := parseSections(text)

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

	// Extract model recommendation
	if modelText, ok := sections["model"]; ok && modelText != "" {
		m := strings.ToLower(strings.TrimSpace(modelText))
		if m == "opus" || m == "sonnet" {
			result.Model = m
		}
	}

	return result
}
