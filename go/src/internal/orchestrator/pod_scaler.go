package orchestrator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/circle-oo/flux/internal/agent"
	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/shutdown"
	"github.com/circle-oo/flux/internal/triager"
	"github.com/circle-oo/flux/internal/vault"
)

// PodScaler implements AgentScaler and dynamically manages executor/triager/researcher pod lifecycle.
type PodScaler struct {
	cfg         *config.Config
	discord     *notifier.Discord
	vaultWriter *vault.Writer
	agentClient *agent.Client

	mu          sync.Mutex
	executors   []*executor.Executor
	triagers    []*triager.Triager
	researchers []*triager.Triager // Uses triager constructor as placeholder
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPodScaler creates a new PodScaler. Call Start() to initialize the context.
func NewPodScaler(cfg *config.Config, discord *notifier.Discord, vaultWriter *vault.Writer, agentClient *agent.Client) *PodScaler {
	ctx, cancel := context.WithCancel(context.Background())
	return &PodScaler{
		cfg:         cfg,
		discord:     discord,
		vaultWriter: vaultWriter,
		agentClient: agentClient,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ScalePods adjusts executor, triager, and researcher counts to match desired levels.
func (ps *PodScaler) ScalePods(ctx context.Context, executorCount, triagerCount, researcherCount int) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if err := ps.scaleExecutors(executorCount); err != nil {
		return fmt.Errorf("scale executors: %w", err)
	}

	if err := ps.scaleTriagers(triagerCount); err != nil {
		return fmt.Errorf("scale triagers: %w", err)
	}

	if err := ps.scaleResearchers(researcherCount); err != nil {
		return fmt.Errorf("scale researchers: %w", err)
	}

	return nil
}

func (ps *PodScaler) scaleExecutors(desired int) error {
	current := len(ps.executors)

	if desired == current {
		return nil
	}

	if desired > current {
		// Scale up: create new executors
		for i := current; i < desired; i++ {
			execID := fmt.Sprintf("executor-%02d", i+1)
			exec := executor.NewExecutor(execID, ps.cfg, ps.discord, ps.vaultWriter, ps.agentClient)
			ps.executors = append(ps.executors, exec)
			go func(e *executor.Executor, id string) {
				slog.Info("executor pod started (dynamic)", "id", id)
				e.Run(ps.ctx)
			}(exec, execID)
		}
		slog.Info("scaled up executors", "from", current, "to", desired)
	} else {
		// Scale down: stop excess executors (from the end)
		for i := desired; i < current; i++ {
			exec := ps.executors[i]
			slog.Info("stopping executor pod (scale down)", "id", fmt.Sprintf("executor-%02d", i+1))
			exec.Stop()
		}
		ps.executors = ps.executors[:desired]
		slog.Info("scaled down executors", "from", current, "to", desired)
	}

	return nil
}

func (ps *PodScaler) scaleTriagers(desired int) error {
	current := len(ps.triagers)

	if !ps.cfg.Triager.Enabled {
		// If triager is disabled, don't create any
		desired = 0
	}

	if desired == current {
		return nil
	}

	if desired > current {
		// Scale up: create new triagers
		for i := current; i < desired; i++ {
			triagerID := fmt.Sprintf("triager-%02d", i+1)
			t := triager.New(triagerID, ps.cfg, ps.discord, ps.agentClient)
			ps.triagers = append(ps.triagers, t)
			go func(tr *triager.Triager, id string) {
				slog.Info("triager pod started (dynamic)", "id", id)
				tr.Run(ps.ctx)
			}(t, triagerID)
		}
		slog.Info("scaled up triagers", "from", current, "to", desired)
	} else {
		// Scale down: stop excess triagers (from the end)
		for i := desired; i < current; i++ {
			t := ps.triagers[i]
			slog.Info("stopping triager pod (scale down)", "id", fmt.Sprintf("triager-%02d", i+1))
			t.Stop()
		}
		ps.triagers = ps.triagers[:desired]
		slog.Info("scaled down triagers", "from", current, "to", desired)
	}

	return nil
}

func (ps *PodScaler) scaleResearchers(desired int) error {
	current := len(ps.researchers)

	if desired == current {
		return nil
	}

	if desired > current {
		// Scale up: create new researchers (using triager constructor as placeholder)
		for i := current; i < desired; i++ {
			researcherID := fmt.Sprintf("researcher-%02d", i+1)
			r := triager.New(researcherID, ps.cfg, ps.discord, ps.agentClient)
			ps.researchers = append(ps.researchers, r)
			go func(res *triager.Triager, id string) {
				slog.Info("researcher pod started (dynamic)", "id", id)
				res.Run(ps.ctx)
			}(r, researcherID)
		}
		slog.Info("scaled up researchers", "from", current, "to", desired)
	} else {
		// Scale down: stop excess researchers (from the end)
		for i := desired; i < current; i++ {
			r := ps.researchers[i]
			slog.Info("stopping researcher pod (scale down)", "id", fmt.Sprintf("researcher-%02d", i+1))
			r.Stop()
		}
		ps.researchers = ps.researchers[:desired]
		slog.Info("scaled down researchers", "from", current, "to", desired)
	}

	return nil
}

// Pods returns all current pods for graceful shutdown.
func (ps *PodScaler) Pods() []shutdown.Pod {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	pods := make([]shutdown.Pod, 0, len(ps.executors)+len(ps.triagers)+len(ps.researchers))
	for _, e := range ps.executors {
		pods = append(pods, e)
	}
	for _, t := range ps.triagers {
		pods = append(pods, t)
	}
	for _, r := range ps.researchers {
		pods = append(pods, r)
	}
	return pods
}

// Stop cancels the context and stops all pods.
func (ps *PodScaler) Stop() {
	ps.cancel()

	ps.mu.Lock()
	defer ps.mu.Unlock()

	for _, e := range ps.executors {
		e.Stop()
	}
	for _, t := range ps.triagers {
		t.Stop()
	}
	for _, r := range ps.researchers {
		r.Stop()
	}
}
