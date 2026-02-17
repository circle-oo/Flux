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

	fluxv1 "github.com/circle-oo/flux/gen/flux/v1"
)

// AgentScaler is an interface for pod scaling operations.
type AgentScaler interface {
	ScalePods(ctx context.Context, executorCount, triagerCount, researcherCount int) error
}

// ScaleManager scales executor pods based on READY queue depth and daily cost budget.
// Triager and researcher pods are always held at their configured min.
type ScaleManager struct {
	db      *sql.DB
	config  *config.OrchestratorConfig
	scaler  AgentScaler
	discord *notifier.Discord

	mu             sync.RWMutex
	executorPods   int
	triagerPods    int
	researcherPods int
	readyCount     int
	dailyCost      float64
	budgetExceeded bool
	lastScaleTime  time.Time
}

// NewScaleManager creates a new ScaleManager.
func NewScaleManager(
	db *sql.DB,
	cfg *config.OrchestratorConfig,
	discord *notifier.Discord,
	scaler AgentScaler,
) *ScaleManager {
	return &ScaleManager{
		db:      db,
		config:  cfg,
		scaler:  scaler,
		discord: discord,
	}
}

func (s *ScaleManager) Name() string { return "scale_manager" }

// clamp restricts v to [min, max].
func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// Tick evaluates READY queue depth and daily cost, then scales executor pods.
func (s *ScaleManager) Tick(ctx context.Context) error {
	pods := s.config.ResolvePods()

	// 1. Count READY tasks (work waiting for executors)
	readyCount, err := s.countReady(ctx)
	if err != nil {
		return fmt.Errorf("count ready: %w", err)
	}

	// 2. Count RUNNING tasks (already being executed)
	runningCount, err := s.countRunning(ctx)
	if err != nil {
		return fmt.Errorf("count running: %w", err)
	}

	// 3. Query today's cost
	dailyCost, err := s.todayCost(ctx)
	if err != nil {
		return fmt.Errorf("today cost: %w", err)
	}

	// 4. Check daily budget
	budget := s.config.DailyCostBudget
	if budget <= 0 {
		budget = 20.0 // default $20/day
	}
	budgetExceeded := dailyCost >= budget

	// 5. Determine desired executor count
	//    - One executor per READY task (up to max), minus already running
	//    - If budget exceeded, scale to min only
	var executorDesired int
	if budgetExceeded {
		executorDesired = pods.Executor.Min
	} else {
		// Need enough executors to drain the READY queue
		// Subtract running count since those executors are already busy
		executorDesired = readyCount + runningCount
	}
	executorPods := clamp(executorDesired, pods.Executor.Min, pods.Executor.Max)

	// Triager and researcher: always at min (config controls availability)
	triagerPods := pods.Triager.Min
	researcherPods := pods.Researcher.Min

	// 6. Check if anything changed
	s.mu.Lock()
	changed := s.executorPods != executorPods || s.triagerPods != triagerPods || s.researcherPods != researcherPods
	prevExecutor := s.executorPods
	lastScale := s.lastScaleTime

	// Always update tracked metrics for Status()
	s.readyCount = readyCount
	s.dailyCost = dailyCost
	s.budgetExceeded = budgetExceeded
	s.mu.Unlock()

	if !changed {
		return nil
	}

	// 7. Apply cooldown for scale-down only; scale-up is always immediate
	cooldown := s.config.ScaleCooldown
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	isScaleUp := executorPods > prevExecutor
	coolingDown := !lastScale.IsZero() && time.Since(lastScale) < cooldown

	if coolingDown && !isScaleUp {
		return nil
	}

	// 8. Apply
	s.mu.Lock()
	s.executorPods = executorPods
	s.triagerPods = triagerPods
	s.researcherPods = researcherPods
	s.lastScaleTime = time.Now()
	s.mu.Unlock()

	slog.Info("scale adjusted",
		"executor_pods", executorPods,
		"ready_tasks", readyCount,
		"running_tasks", runningCount,
		"daily_cost", fmt.Sprintf("$%.2f/$%.2f", dailyCost, budget),
		"budget_exceeded", budgetExceeded,
	)

	if s.discord != nil {
		msg := fmt.Sprintf("Scale: executor=%d (ready=%d, running=%d, cost=$%.2f/$%.2f)",
			executorPods, readyCount, runningCount, dailyCost, budget)
		if budgetExceeded {
			msg += " [BUDGET EXCEEDED]"
		}
		s.discord.Send(notifier.LevelInfo, msg)
	}

	if s.scaler != nil {
		if err := s.scaler.ScalePods(ctx, executorPods, triagerPods, researcherPods); err != nil {
			slog.Error("failed to scale pods", "error", err)
			if s.discord != nil {
				s.discord.Send(notifier.LevelWarning, fmt.Sprintf("Failed to scale pods: %v", err))
			}
			return fmt.Errorf("scale pods: %w", err)
		}
	}

	return nil
}

// countReady returns the number of READY tasks in the queue.
func (s *ScaleManager) countReady(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status = 'READY'`,
	).Scan(&count)
	return count, err
}

// countRunning returns the number of currently RUNNING tasks.
func (s *ScaleManager) countRunning(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status = 'RUNNING'`,
	).Scan(&count)
	return count, err
}

// todayCost returns total cost_usd for tasks completed today.
func (s *ScaleManager) todayCost(ctx context.Context) (float64, error) {
	var cost float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM tasks
		 WHERE date(completed_at) = date('now')`,
	).Scan(&cost)
	return cost, err
}

// Status returns the current scale status for the orchestrator status RPC.
func (s *ScaleManager) Status() *fluxv1.ScaleStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pods := s.config.ResolvePods()

	var lastScale string
	if !s.lastScaleTime.IsZero() {
		lastScale = s.lastScaleTime.Format(time.RFC3339)
	}

	queueState := fmt.Sprintf("ready=%d cost=$%.2f", s.readyCount, s.dailyCost)
	if s.budgetExceeded {
		queueState += " [budget exceeded]"
	}

	return &fluxv1.ScaleStatus{
		ExecutorPods:      int32(s.executorPods),
		TriagerPods:       int32(s.triagerPods),
		ResearcherPods:    int32(s.researcherPods),
		MaxExecutorPods:   int32(pods.Executor.Max),
		MaxTriagerPods:    int32(pods.Triager.Max),
		MaxResearcherPods: int32(pods.Researcher.Max),
		QueueState:        queueState,
		LastScaleTime:     lastScale,
	}
}
