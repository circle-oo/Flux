package triager

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/notifier"
)

// component tag for structured logging.
const component = "triager"

// Triager is a standalone component that polls for PENDING tasks,
// runs triage (Claude haiku analysis), and promotes them to READY.
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

	result, err := executor.TriageTask(ctx, t.claude, task)
	if err != nil {
		slog.Warn("triage failed, promoting with original description",
			"task_id", task.ID, "error", err, "component", component)
		// Even on failure, promote to READY so the task doesn't get stuck
		if reportErr := t.client.ReportTriaged(task.ID, "", "", 0); reportErr != nil {
			slog.Error("failed to promote task after triage failure",
				"task_id", task.ID, "error", reportErr, "component", component)
		}
		return
	}

	if reportErr := t.client.ReportTriaged(task.ID, result.Analysis, result.Description, result.Priority); reportErr != nil {
		slog.Error("failed to report triage results",
			"task_id", task.ID, "error", reportErr, "component", component)
		return
	}

	slog.Info("task triaged and promoted to READY",
		"task_id", task.ID, "priority", result.Priority, "component", component)
}

// smokeTest verifies the Claude CLI is available.
func (t *Triager) smokeTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := t.claude.Run(ctx, executor.ClaudeCodeOpts{
		Prompt:   "respond with exactly: SMOKE_TEST_OK",
		Model:    "haiku",
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
