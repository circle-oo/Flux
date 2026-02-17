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

// ScaleManager scales executor pods based on READY queue depth and budget.
// Budget mode depends on billing:
//   - API billing: daily cost budget in USD
//   - Max/Pro plan: token budget per 5-hour window (resets every 5h)
type ScaleManager struct {
	db         *sql.DB
	config     *config.OrchestratorConfig
	planConfig *config.ClaudeCodeConfig
	scaler     AgentScaler
	discord    *notifier.Discord

	mu             sync.RWMutex
	executorPods   int
	triagerPods    int
	researcherPods int
	readyCount     int
	dailyCost      float64
	windowTokens   int
	budgetExceeded bool
	lastScaleTime  time.Time
}

// NewScaleManager creates a new ScaleManager.
func NewScaleManager(
	db *sql.DB,
	cfg *config.OrchestratorConfig,
	planCfg *config.ClaudeCodeConfig,
	discord *notifier.Discord,
	scaler AgentScaler,
) *ScaleManager {
	return &ScaleManager{
		db:         db,
		config:     cfg,
		planConfig: planCfg,
		scaler:     scaler,
		discord:    discord,
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

// Tick evaluates READY queue depth and budget, then scales executor pods.
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

	// 3. Check budget based on billing mode
	var budgetExceeded bool
	var budgetLabel string

	isAPI := s.planConfig == nil || s.planConfig.IsAPIBilling()

	if isAPI {
		// API billing: daily cost budget
		dailyCost, err := s.todayCost(ctx)
		if err != nil {
			return fmt.Errorf("today cost: %w", err)
		}
		budget := s.config.DailyCostBudget
		if budget <= 0 {
			budget = 20.0 // default $20/day
		}
		budgetExceeded = dailyCost >= budget
		budgetLabel = fmt.Sprintf("$%.2f/$%.2f", dailyCost, budget)

		s.mu.Lock()
		s.dailyCost = dailyCost
		s.mu.Unlock()
	} else {
		// Max/Pro plan: token budget per 5-hour window
		windowTokens, err := s.currentWindowTokens(ctx)
		if err != nil {
			return fmt.Errorf("window tokens: %w", err)
		}
		tokenBudget := s.config.WindowTokenBudget
		if tokenBudget <= 0 {
			tokenBudget = 5_000_000 // default 5M tokens per window
		}
		budgetExceeded = windowTokens >= tokenBudget
		budgetLabel = fmt.Sprintf("%dk/%dk tokens", windowTokens/1000, tokenBudget/1000)

		s.mu.Lock()
		s.windowTokens = windowTokens
		s.mu.Unlock()
	}

	// 4. Determine desired executor count
	//    - One executor per READY task (up to max), minus already running
	//    - If budget exceeded, scale to min only
	var executorDesired int
	if budgetExceeded {
		executorDesired = pods.Executor.Min
	} else {
		executorDesired = readyCount + runningCount
	}
	executorPods := clamp(executorDesired, pods.Executor.Min, pods.Executor.Max)

	// Triager and researcher: always at min
	triagerPods := pods.Triager.Min
	researcherPods := pods.Researcher.Min

	// 5. Check if anything changed
	s.mu.Lock()
	changed := s.executorPods != executorPods || s.triagerPods != triagerPods || s.researcherPods != researcherPods
	prevExecutor := s.executorPods
	lastScale := s.lastScaleTime

	s.readyCount = readyCount
	s.budgetExceeded = budgetExceeded
	s.mu.Unlock()

	if !changed {
		return nil
	}

	// 6. Apply cooldown for scale-down only; scale-up is always immediate
	cooldown := s.config.ScaleCooldown
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	isScaleUp := executorPods > prevExecutor
	coolingDown := !lastScale.IsZero() && time.Since(lastScale) < cooldown

	if coolingDown && !isScaleUp {
		return nil
	}

	// 7. Apply
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
		"budget", budgetLabel,
		"budget_exceeded", budgetExceeded,
	)

	if s.discord != nil {
		msg := fmt.Sprintf("Scale: executor=%d (ready=%d, running=%d, budget=%s)",
			executorPods, readyCount, runningCount, budgetLabel)
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

// todayCost returns total cost_usd for tasks completed today (API billing mode).
func (s *ScaleManager) todayCost(ctx context.Context) (float64, error) {
	var cost float64
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM task_usage_events
		 WHERE date(recorded_at) = date('now')`,
	).Scan(&cost)
	return cost, err
}

// currentWindowTokens returns total tokens used in the current 5-hour window.
// Max/Pro plans reset usage at 0h, 5h, 10h, 15h, 20h UTC.
func (s *ScaleManager) currentWindowTokens(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	windowHour := (now.Hour() / 5) * 5
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), windowHour, 0, 0, 0, time.UTC)

	var tokens int
	err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(tokens), 0) FROM task_usage_events
		 WHERE recorded_at >= ?`,
		windowStart.Format(time.RFC3339),
	).Scan(&tokens)
	return tokens, err
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

	isAPI := s.planConfig == nil || s.planConfig.IsAPIBilling()
	var queueState string
	if isAPI {
		queueState = fmt.Sprintf("ready=%d cost=$%.2f", s.readyCount, s.dailyCost)
	} else {
		queueState = fmt.Sprintf("ready=%d tokens=%dk", s.readyCount, s.windowTokens/1000)
	}
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
