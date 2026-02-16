package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
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

// ScaleManager implements SubComponent and manages executor/triager/researcher pod scaling
// based on queue state.
type ScaleManager struct {
	db      *sql.DB
	config  *config.OrchestratorConfig
	scaler  AgentScaler
	discord *notifier.Discord

	mu             sync.RWMutex
	queueState     string
	executorPods   int
	triagerPods    int
	researcherPods int
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
		db:         db,
		config:     cfg,
		scaler:     scaler,
		discord:    discord,
		queueState: "empty",
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

// Tick evaluates the current queue state and adjusts pod allocation if needed.
func (s *ScaleManager) Tick(ctx context.Context) error {
	// Always evaluate queue state (even during cooldown) so we detect transitions.
	state, err := s.evaluateQueue(ctx)
	if err != nil {
		return fmt.Errorf("evaluate queue: %w", err)
	}

	pods := s.config.ResolvePods()

	// Calculate ratio
	executorRatio, _ := s.ratioForState(state)

	totalBudget := pods.Executor.Max + pods.Triager.Max + pods.Researcher.Max

	// Allocate researcher first (currently 0 since max=0 by default)
	researcherPods := clamp(0, pods.Researcher.Min, pods.Researcher.Max)

	// Remaining budget for executor + triager
	remaining := totalBudget - researcherPods

	executorDesired := int(math.Round(float64(remaining) * executorRatio))
	triagerDesired := remaining - executorDesired

	// Clamp per type
	executorPods := clamp(executorDesired, pods.Executor.Min, pods.Executor.Max)
	triagerPods := clamp(triagerDesired, pods.Triager.Min, pods.Triager.Max)

	s.mu.Lock()
	changed := s.queueState != state || s.executorPods != executorPods || s.triagerPods != triagerPods || s.researcherPods != researcherPods
	prevState := s.queueState
	prevExecutor := s.executorPods
	prevTriager := s.triagerPods
	lastScale := s.lastScaleTime

	// Always update the tracked state so Status() reflects reality.
	s.queueState = state
	s.mu.Unlock()

	if !changed {
		return nil
	}

	// Apply cooldown only to the actual ScalePods call, not to state evaluation.
	// Exception: always allow scaling up from zero (startup / recovery).
	cooldown := s.config.ScaleCooldown
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}
	isScaleUp := executorPods > prevExecutor || triagerPods > prevTriager
	coolingDown := !lastScale.IsZero() && time.Since(lastScale) < cooldown

	if coolingDown && !isScaleUp {
		return nil
	}

	s.mu.Lock()
	s.executorPods = executorPods
	s.triagerPods = triagerPods
	s.researcherPods = researcherPods
	s.lastScaleTime = time.Now()
	s.mu.Unlock()

	slog.Info("scale adjusted",
		"state", state,
		"prev_state", prevState,
		"executor_pods", executorPods,
		"triager_pods", triagerPods,
		"researcher_pods", researcherPods,
	)

	// Notify Discord on state transition
	if s.discord != nil {
		msg := fmt.Sprintf("Scale state: %s -> %s (executor=%d, triager=%d, researcher=%d, total=%d)",
			prevState, state, executorPods, triagerPods, researcherPods, totalBudget)
		s.discord.Send(notifier.LevelInfo, msg)
	}

	// Call the scaler to apply the new pod counts
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

// evaluateQueue queries the task queue and determines the current queue state.
func (s *ScaleManager) evaluateQueue(ctx context.Context) (string, error) {
	// Count urgent operator tasks (priority >= 80)
	var urgentCount int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status IN ('PENDING','READY') AND source = 'OPERATOR' AND priority >= 80`,
	).Scan(&urgentCount)
	if err != nil {
		return "", fmt.Errorf("count urgent: %w", err)
	}
	if urgentCount > 0 {
		return "urgent", nil
	}

	// Count operator tasks (source = 'OPERATOR')
	var operatorCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status IN ('PENDING','READY') AND source = 'OPERATOR'`,
	).Scan(&operatorCount)
	if err != nil {
		return "", fmt.Errorf("count operator: %w", err)
	}
	if operatorCount > 0 {
		return "operator", nil
	}

	// Count system/researcher tasks
	var systemCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status IN ('PENDING','READY') AND source IN ('SYSTEM','RESEARCHER')`,
	).Scan(&systemCount)
	if err != nil {
		return "", fmt.Errorf("count system: %w", err)
	}
	if systemCount > 0 {
		return "system", nil
	}

	// Count total pending/ready tasks
	var totalCount int
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tasks WHERE status IN ('PENDING','READY')`,
	).Scan(&totalCount)
	if err != nil {
		return "", fmt.Errorf("count total: %w", err)
	}

	if totalCount >= 3 {
		return "system", nil
	}
	if totalCount > 0 {
		return "near_empty", nil
	}

	return "empty", nil
}

// ratioForState returns (executorRatio, triagerRatio) for a given queue state.
func (s *ScaleManager) ratioForState(state string) (float64, float64) {
	switch state {
	case "urgent":
		return 0.90, 0.10
	case "operator":
		return 0.80, 0.20
	case "system":
		return 0.70, 0.30
	case "near_empty":
		return 0.30, 0.70
	case "empty":
		return 0.00, 1.00
	default:
		return 0.50, 0.50
	}
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

	return &fluxv1.ScaleStatus{
		ExecutorPods:     int32(s.executorPods),
		TriagerPods:      int32(s.triagerPods),
		ResearcherPods:   int32(s.researcherPods),
		MaxExecutorPods:  int32(pods.Executor.Max),
		MaxTriagerPods:   int32(pods.Triager.Max),
		MaxResearcherPods: int32(pods.Researcher.Max),
		QueueState:       s.queueState,
		LastScaleTime:    lastScale,
	}
}
