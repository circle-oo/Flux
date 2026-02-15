package orchestrator

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/models"
)

// Orchestrator manages task execution, model selection, and goal context injection.
// This is a minimal implementation for Phase 2B — full orchestration loop comes in Phase 3.
type Orchestrator struct {
	config           *config.Config
	rateLimitHandler *RateLimitHandler
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator(cfg *config.Config, rlh *RateLimitHandler) *Orchestrator {
	o := &Orchestrator{
		config:           cfg,
		rateLimitHandler: rlh,
	}
	slog.Debug("orchestrator created")
	return o
}

// SelectModel determines which model to use for a given task.
// Logic:
// - If recently rate limited → always return Sonnet (cost-saving mode)
// - If task.NeedsOpus() → return Opus
// - Otherwise → return Sonnet
// TODO: This method is currently only used in tests. Consider removing if not needed in production code.
func (o *Orchestrator) SelectModel(task *models.Task) string {
	slog.Debug("selecting model for task", "task_id", task.ID, "task_type", task.Type, "task_priority", task.Priority)

	// Priority 1: Rate limit protection - always use cheaper model
	if o.rateLimitHandler.RecentlyLimited() {
		model := o.config.Orchestrator.Models.Sonnet
		slog.Info("model selection: using sonnet due to recent rate limit", "task_id", task.ID)
		return model
	}

	// Priority 2: Task complexity analysis
	if task.NeedsOpus() {
		model := o.config.Orchestrator.Models.Opus
		slog.Info("model selection: using opus for complex task", "task_id", task.ID)
		return model
	}

	// Default: Sonnet for standard tasks
	model := o.config.Orchestrator.Models.Sonnet
	slog.Info("model selection: using sonnet", "task_id", task.ID)
	return model
}

// BuildGoalSystemPrompt constructs a system prompt section from a goal.
// Returns empty string if goal is nil.
// Format:
//   Current Goal: {title}
//   Description: {description}
//   Priorities: {p1, p2, ...}
//   Metrics: {m1, m2, ...}
//
//   All your work should align with this Goal.
// TODO: This method is currently only used in tests. Consider removing if not needed in production code.
func (o *Orchestrator) BuildGoalSystemPrompt(goal *models.Goal) string {
	slog.Debug("building goal system prompt", "has_goal", goal != nil)

	if goal == nil {
		return ""
	}

	slog.Debug("goal context injected", "goal_id", goal.ID, "goal_title", goal.Title, "priorities", len(goal.Priorities), "metrics", len(goal.Metrics))

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Current Goal: %s\n", goal.Title))
	sb.WriteString(fmt.Sprintf("Description: %s\n", goal.Description))

	// Format priorities
	if len(goal.Priorities) > 0 {
		sb.WriteString("Priorities: ")
		sb.WriteString(strings.Join(goal.Priorities, ", "))
		sb.WriteString("\n")
	} else {
		sb.WriteString("Priorities: (none)\n")
	}

	// Format metrics
	if len(goal.Metrics) > 0 {
		sb.WriteString("Metrics: ")
		sb.WriteString(strings.Join(goal.Metrics, ", "))
		sb.WriteString("\n")
	} else {
		sb.WriteString("Metrics: (none)\n")
	}

	sb.WriteString("\nAll your work should align with this Goal.")

	return sb.String()
}
