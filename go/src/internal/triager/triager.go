package triager

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"text/template"
	"time"

	fluxv1 "github.com/circle-oo/flux/gen/flux/v1"
	"github.com/circle-oo/flux/internal/agent"
	"github.com/circle-oo/flux/internal/apiclient"
	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// component tag for structured logging.
const component = "triager"

//go:embed triage.txt
var triagePromptFS embed.FS

var triageTemplate = template.Must(template.ParseFS(triagePromptFS, "triage.txt"))

// Triager is a standalone component that polls for PENDING tasks,
// runs triage (Claude analysis), and promotes them to READY.
// Triage determines priority, analysis, rewritten description, and
// the recommended model for task execution.
type Triager struct {
	id          string
	config      *config.Config
	agentClient *agent.Client
	client      *apiclient.Client
	notifier    *notifier.Discord
	stopCh      chan struct{}
	stopOnce    sync.Once

	mu            sync.Mutex // guards currentTaskID and running
	currentTaskID string
	running       bool
}

// New creates a new Triager.
func New(id string, cfg *config.Config, discord *notifier.Discord, ac *agent.Client) *Triager {
	return &Triager{
		id:            id,
		config:        cfg,
		agentClient:   ac,
		client:        apiclient.NewClient(fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)),
		notifier:      discord,
		stopCh:        make(chan struct{}),
		currentTaskID: "",
		running:       false,
	}
}

// Run is the main loop. It polls for PENDING tasks and triages them.
func (t *Triager) Run(ctx context.Context) {
	slog.Info("triager started", "id", t.id, "component", component)
	t.mu.Lock()
	t.running = true
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.running = false
		t.mu.Unlock()
	}()

	// Smoke test: verify agent manager is reachable
	if err := t.smokeTest(ctx); err != nil {
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

			// Interruptible wait — respond to stop signals immediately
			select {
			case <-ctx.Done():
				slog.Info("triager stopping (context cancelled)", "id", t.id, "component", component)
				return
			case <-t.stopCh:
				slog.Info("triager stopping (stop signal)", "id", t.id, "component", component)
				return
			case <-time.After(10 * time.Second):
			}
		}
	}
}

// Stop signals the triager to stop. Safe to call multiple times.
func (t *Triager) Stop() {
	t.stopOnce.Do(func() { close(t.stopCh) })
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

	t.mu.Lock()
	t.currentTaskID = task.ID
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.currentTaskID = ""
		t.mu.Unlock()
	}()

	model := t.config.Triager.Model
	if model == "" {
		model = "haiku"
	}

	slog.Info("triaging task", "task_id", task.ID, "title", task.Title, "model", model, "component", component)

	start := time.Now()
	result, err := t.triageTask(ctx, task, model)
	if err != nil {
		slog.Warn("triage failed, promoting with original description",
			"task_id", task.ID, "title", task.Title, "error", err, "component", component)
		// Even on failure, promote to READY so the task doesn't get stuck
		if reportErr := t.client.ReportTriaged(task.ID, "", "", "", 0); reportErr != nil {
			slog.Error("failed to promote task after triage failure",
				"task_id", task.ID, "error", reportErr, "component", component)
		}
		return
	}

	slog.Info("triage completed",
		"task_id", task.ID, "duration", time.Since(start), "component", component)

	if reportErr := t.client.ReportTriaged(task.ID, result.Analysis, result.Description, result.Title, result.Priority); reportErr != nil {
		slog.Error("failed to report triage results",
			"task_id", task.ID, "error", reportErr, "component", component)
		return
	}

	slog.Info("task triaged and promoted to READY",
		"task_id", task.ID, "priority", result.Priority, "component", component)
}

// smokeTest verifies the Python Agent Manager is reachable via gRPC.
// Retries up to 15 times (30s total) to allow the agent manager time to start.
func (t *Triager) smokeTest(ctx context.Context) error {
	if t.agentClient == nil {
		return fmt.Errorf("agent client not available")
	}

	const maxAttempts = 15
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		testCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		_, lastErr = t.agentClient.PodStatus(testCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		slog.Warn("triager waiting for agent manager",
			"attempt", attempt, "max", maxAttempts, "error", lastErr, "component", component)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.stopCh:
			return fmt.Errorf("triager stopped during smoke test")
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("agent manager health check failed after %d attempts: %w", maxAttempts, lastErr)
}

// --- Triage execution logic ---

// triageData holds data for triage.txt template.
type triageData struct {
	Title              string
	Priority           int
	Description        string
	Tags               string
	ProjectName        string
	ProjectDescription string
	ProjectType        string
	TechStack          string
	GoalTitle          string
	GoalDescription    string
	GoalPriorities     string
}

// TriageResult contains the output of task triage analysis.
type TriageResult struct {
	Analysis    string // Structured analysis of the task
	Priority    int    // Suggested priority (1-100)
	Title       string // Suggested improved title (optional)
	Description string // Rewritten description with clear requirements
}

// triageJSON is the expected JSON output from the triage prompt.
type triageJSON struct {
	Analysis    string `json:"analysis"`
	Priority    int    `json:"priority"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// triageTask uses the Agent Manager (Claude Code SDK) to analyze a task via gRPC.
func (t *Triager) triageTask(ctx context.Context, task *models.Task, model string) (*TriageResult, error) {
	if t.agentClient == nil {
		return nil, fmt.Errorf("agent client not available")
	}

	// Fetch project context if available
	var project *models.Project
	var goal *models.Goal
	if task.ProjectID != "" && t.client != nil {
		var err error
		project, err = t.client.GetProject(task.ProjectID)
		if err != nil {
			slog.Warn("failed to fetch project context for triage",
				"task_id", task.ID, "project_id", task.ProjectID, "error", err)
		}
	}

	// Fetch goal context if task or project has a goal_id
	goalID := task.GoalID
	if goalID == "" && project != nil {
		goalID = project.GoalID
	}
	_ = goalID // Goal fetching requires GetGoal in apiclient — deferred

	prompt := buildTriagePromptWithContext(task, project, goal)

	triageCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	// Execute via gRPC — triage uses a lightweight agent type with max_turns=1
	req := &fluxv1.ExecuteTaskRequest{
		TaskId:           task.ID,
		AgentType:        "qa",
		Prompt:           prompt,
		WorkingDirectory: "/tmp",
		MaxTurns:         1,
		Metadata: map[string]string{
			"model": model,
			"mode":  "triage",
		},
	}

	// Collect only ASSISTANT_MESSAGE content (skip PROGRESS, TOOL_USE, etc.)
	var output strings.Builder
	err := t.agentClient.ExecuteTask(triageCtx, req, func(event *fluxv1.TaskEvent) {
		if event.GetType() == fluxv1.TaskEvent_TASK_EVENT_TYPE_ASSISTANT_MESSAGE {
			output.WriteString(event.GetContent())
		}
	})

	if err != nil {
		return nil, fmt.Errorf("triage execution failed: %w", err)
	}

	resultText := strings.TrimSpace(output.String())
	if resultText == "" {
		return nil, fmt.Errorf("triage returned empty output")
	}

	slog.Info("triage raw response",
		"task_id", task.ID,
		"result_text_len", len(resultText),
		"result_text_prefix", truncate(resultText, 300))

	triage := parseTriageResponse(resultText, task)
	return triage, nil
}

func buildTriagePrompt(task *models.Task) string {
	return buildTriagePromptWithContext(task, nil, nil)
}

func buildTriagePromptWithContext(task *models.Task, project *models.Project, goal *models.Goal) string {
	tags := ""
	if len(task.Tags) > 0 {
		tags = strings.Join(task.Tags, ", ")
	}

	data := triageData{
		Title:       task.Title,
		Priority:    task.Priority,
		Description: task.Description,
		Tags:        tags,
		ProjectName: task.ProjectID,
	}

	// Enrich with project context
	if project != nil {
		data.ProjectName = project.Name
		data.ProjectDescription = project.Description
		data.ProjectType = project.Type
		if len(project.TechStack) > 0 {
			data.TechStack = strings.Join(project.TechStack, ", ")
		}
	}

	// Enrich with goal context
	if goal != nil {
		data.GoalTitle = goal.Title
		data.GoalDescription = goal.Description
		if len(goal.Priorities) > 0 {
			data.GoalPriorities = strings.Join(goal.Priorities, ", ")
		}
	}

	var buf strings.Builder
	err := triageTemplate.ExecuteTemplate(&buf, "triage.txt", data)
	if err != nil {
		slog.Warn("failed to render triage prompt template, using fallback", "error", err)
		return fmt.Sprintf("Analyze this task and suggest priority and rewrite description:\n\nTitle: %s\nDescription: %s", task.Title, task.Description)
	}
	return buf.String()
}

// parseSections extracts named sections from markdown-style response text.
// Sections are delimited by ### or ## headings.
// Kept as fallback for non-JSON triage responses.
func parseSections(text string) map[string]string {
	sections := map[string]string{}
	currentSection := ""
	var currentLines []string

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		upper := strings.ToUpper(trimmed)

		// Check for known section headers
		sectionName := ""
		for _, name := range []string{"ANALYSIS", "PRIORITY", "DESCRIPTION"} {
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

// extractJSON attempts to extract a JSON object from text that may contain
// markdown code fences or narrative wrapping around the JSON.
func extractJSON(text string) string {
	text = strings.TrimSpace(text)

	// Strip markdown code fences
	if strings.HasPrefix(text, "```") {
		lines := strings.SplitN(text, "\n", 2)
		if len(lines) > 1 {
			text = lines[1]
		}
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	// Find JSON object boundaries in narrative text
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		text = text[start : end+1]
	}

	return strings.TrimSpace(text)
}

func parseTriageResponse(text string, task *models.Task) *TriageResult {
	result := &TriageResult{
		Priority:    task.Priority, // default to existing
		Description: task.Description,
	}

	text = strings.TrimSpace(text)

	// Handle empty result text explicitly
	if text == "" {
		slog.Warn("triage response text is empty, returning defaults", "task_id", task.ID)
		return result
	}

	// Try JSON first, extracting from narrative/code-fenced wrapping
	extracted := extractJSON(text)
	var tj triageJSON
	if err := json.Unmarshal([]byte(extracted), &tj); err == nil {
		if tj.Analysis != "" {
			result.Analysis = tj.Analysis
		}
		if tj.Priority >= 1 && tj.Priority <= 100 {
			result.Priority = tj.Priority
		}
		if tj.Title != "" {
			result.Title = tj.Title
		}
		if tj.Description != "" {
			result.Description = tj.Description
		}
	} else {
		// Fallback to markdown section parsing
		sections := parseSections(text)

		if analysis, ok := sections["analysis"]; ok && analysis != "" {
			result.Analysis = analysis
		}
		if priorityText, ok := sections["priority"]; ok && priorityText != "" {
			var p int
			if _, err := fmt.Sscanf(strings.TrimSpace(priorityText), "%d", &p); err == nil && p >= 1 && p <= 100 {
				result.Priority = p
			}
		}
		if desc, ok := sections["description"]; ok && desc != "" {
			result.Description = desc
		}

		if result.Analysis == "" {
			slog.Warn("triage parsing produced no analysis from JSON or markdown",
				"task_id", task.ID,
				"text_len", len(text),
				"text_prefix", truncate(text, 100),
			)
		}
	}

	// Sanity guard: if the task input is very short and priority is suspiciously high,
	// override to default. This catches garbage/test inputs that the model may
	// incorrectly triage as critical.
	inputLen := len(strings.TrimSpace(task.Title)) + len(strings.TrimSpace(task.Description))
	if inputLen < 10 && result.Priority < 20 {
		result.Priority = 40
	}

	return result
}

// truncate returns s truncated to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// IsRunning returns whether the triager is currently running.
// Implements the shutdown.Pod interface.
func (t *Triager) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// CurrentTaskID returns the ID of the task currently being triaged.
// Returns empty string if no task is active.
// Implements the shutdown.Pod interface.
func (t *Triager) CurrentTaskID() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentTaskID
}
