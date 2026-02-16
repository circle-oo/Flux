package orchestrator

import "context"

// SubComponent is a pluggable unit of work that runs inside the Orchestrator tick loop.
// Each sub-component is called once per tick and should complete quickly (< 30s).
// Long-running work should be dispatched to goroutines internally.
type SubComponent interface {
	// Name returns a human-readable identifier for logging.
	Name() string

	// Tick is called once per orchestrator tick cycle.
	// The context is cancelled on shutdown. Implementations must not block indefinitely.
	Tick(ctx context.Context) error
}
