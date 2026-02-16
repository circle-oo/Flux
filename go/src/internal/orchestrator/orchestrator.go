// Package orchestrator manages the background tick loop that coordinates
// sub-components: rate-limit recovery, usage collection, daily summaries,
// and goal advisory.
package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/notifier"
)

const defaultCheckInterval = 5 * time.Minute

// ComponentHealth tracks the runtime health of a single SubComponent.
type ComponentHealth struct {
	Name      string    `json:"name"`
	Healthy   bool      `json:"healthy"`
	LastTick  time.Time `json:"last_tick"`
	LastError string    `json:"last_error,omitempty"`
}

// OrchestratorStatus is a snapshot of the orchestrator's state.
type OrchestratorStatus struct {
	Running        bool              `json:"running"`
	StartedAt      time.Time         `json:"started_at"`
	TickCount      int64             `json:"tick_count"`
	Components     []ComponentHealth `json:"components"`
	RateLimited    bool              `json:"rate_limited"`
	RateLimitUntil time.Time         `json:"rate_limit_until"`
}

// Orchestrator runs a periodic tick loop, dispatching to registered SubComponents.
type Orchestrator struct {
	config           *config.Config
	db               *sql.DB
	discord          *notifier.Discord
	rateLimitHandler *RateLimitHandler

	components []SubComponent
	health     map[string]*ComponentHealth
	mu         sync.RWMutex

	running   bool
	startedAt time.Time
	tickCount int64
	stop      chan struct{}
}

// NewOrchestrator creates a new Orchestrator instance.
func NewOrchestrator(cfg *config.Config, db *sql.DB, discord *notifier.Discord) *Orchestrator {
	o := &Orchestrator{
		config:  cfg,
		db:      db,
		discord: discord,
		health:  make(map[string]*ComponentHealth),
		stop:    make(chan struct{}),
	}
	slog.Debug("orchestrator created")
	return o
}

// SetRateLimitHandler sets the rate limit handler on the orchestrator.
func (o *Orchestrator) SetRateLimitHandler(rlh *RateLimitHandler) {
	o.rateLimitHandler = rlh
}

// Register adds a SubComponent to the tick loop. Must be called before Start.
func (o *Orchestrator) Register(c SubComponent) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.components = append(o.components, c)
	o.health[c.Name()] = &ComponentHealth{Name: c.Name(), Healthy: true}
	slog.Info("registered orchestrator component", "name", c.Name())
}

// Status is an alias for GetStatus, kept for backward compatibility with
// existing handler/server callers.
func (o *Orchestrator) Status() OrchestratorStatus {
	return o.GetStatus()
}

// GetStatus returns a snapshot of the orchestrator's runtime state.
func (o *Orchestrator) GetStatus() OrchestratorStatus {
	o.mu.RLock()
	defer o.mu.RUnlock()

	status := OrchestratorStatus{
		Running:   o.running,
		StartedAt: o.startedAt,
		TickCount: o.tickCount,
	}

	for _, c := range o.components {
		if h, ok := o.health[c.Name()]; ok {
			status.Components = append(status.Components, *h)
		}
	}

	if o.rateLimitHandler != nil {
		status.RateLimited = o.rateLimitHandler.IsLimited()
		o.rateLimitHandler.mu.RLock()
		status.RateLimitUntil = o.rateLimitHandler.rateLimitUntil
		o.rateLimitHandler.mu.RUnlock()
	}

	return status
}

// Start begins the orchestrator tick loop. It blocks until ctx is cancelled
// or Stop is called.
func (o *Orchestrator) Start(ctx context.Context) {
	interval := o.config.Orchestrator.CheckInterval
	if interval <= 0 {
		interval = defaultCheckInterval
	}

	o.mu.Lock()
	o.running = true
	o.startedAt = time.Now()
	o.stop = make(chan struct{})
	o.mu.Unlock()

	slog.Info("orchestrator started", "interval", interval, "components", len(o.components))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run an initial tick immediately.
	o.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			o.shutdown()
			return
		case <-o.stop:
			o.shutdown()
			return
		case <-ticker.C:
			o.tick(ctx)
		}
	}
}

// Stop signals the orchestrator to shut down gracefully.
func (o *Orchestrator) Stop() {
	o.mu.RLock()
	running := o.running
	o.mu.RUnlock()

	if running {
		close(o.stop)
	}
}

// tick runs one full cycle: rate-limit recovery + all components.
func (o *Orchestrator) tick(ctx context.Context) {
	o.mu.Lock()
	o.tickCount++
	tick := o.tickCount
	o.mu.Unlock()

	slog.Debug("orchestrator tick", "tick", tick)

	// Always check rate-limit recovery.
	if o.rateLimitHandler != nil {
		o.rateLimitHandler.CheckAndRecover()
	}

	// Dispatch to each component with panic recovery.
	o.mu.RLock()
	components := make([]SubComponent, len(o.components))
	copy(components, o.components)
	o.mu.RUnlock()

	for _, c := range components {
		o.runComponent(ctx, c)
	}
}

// runComponent calls a single SubComponent with panic recovery.
func (o *Orchestrator) runComponent(ctx context.Context, c SubComponent) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic: %v", r)
			slog.Error("sub-component panicked", "name", c.Name(), "error", errMsg)

			o.mu.Lock()
			if h, ok := o.health[c.Name()]; ok {
				h.Healthy = false
				h.LastError = errMsg
				h.LastTick = time.Now()
			}
			o.mu.Unlock()
		}
	}()

	err := c.Tick(ctx)

	o.mu.Lock()
	if h, ok := o.health[c.Name()]; ok {
		h.LastTick = time.Now()
		if err != nil {
			h.Healthy = false
			h.LastError = err.Error()
			slog.Warn("component tick error", "component", c.Name(), "error", err)
		} else {
			h.Healthy = true
			h.LastError = ""
		}
	}
	o.mu.Unlock()
}

func (o *Orchestrator) shutdown() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.running = false
	slog.Info("orchestrator stopped", "total_ticks", o.tickCount)
}
