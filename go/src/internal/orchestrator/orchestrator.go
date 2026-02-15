package orchestrator

import (
	"fmt"
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
	return &Orchestrator{
		config:           cfg,
		rateLimitHandler: rlh,
	}
}

// SelectModel determines which model to use for a given task.
// Logic:
// - If recently rate limited → always return Sonnet (cost-saving mode)
// - If task.NeedsOpus() → return Opus
// - Otherwise → return Sonnet
func (o *Orchestrator) SelectModel(task *models.Task) string {
	// Priority 1: Rate limit protection - always use cheaper model
	if o.rateLimitHandler.RecentlyLimited() {
		return o.config.Orchestrator.Models.Sonnet
	}

	// Priority 2: Task complexity analysis
	if task.NeedsOpus() {
		return o.config.Orchestrator.Models.Opus
	}

	// Default: Sonnet for standard tasks
	return o.config.Orchestrator.Models.Sonnet
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
func (o *Orchestrator) BuildGoalSystemPrompt(goal *models.Goal) string {
	if goal == nil {
		return ""
	}

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
