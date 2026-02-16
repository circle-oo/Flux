package orchestrator

import (
	"log/slog"

	"github.com/circle-oo/flux/internal/config"
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

