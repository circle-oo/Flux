package orchestrator

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/testutil"
)

func TestNewOrchestrator(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{}
	orc := NewOrchestrator(cfg, db, discord)
	if orc == nil {
		t.Fatal("Expected non-nil orchestrator")
	}
}

// stubComponent is a test SubComponent that records ticks.
type stubComponent struct {
	name      string
	tickCount atomic.Int64
	err       error
}

func (s *stubComponent) Name() string { return s.name }
func (s *stubComponent) Tick(_ context.Context) error {
	s.tickCount.Add(1)
	return s.err
}

// panicComponent panics on every tick.
type panicComponent struct {
	name string
}

func (p *panicComponent) Name() string                 { return p.name }
func (p *panicComponent) Tick(_ context.Context) error { panic("test panic") }

func TestOrchestratorTickLoop(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			CheckInterval: 50 * time.Millisecond,
		},
	}

	orc := NewOrchestrator(cfg, db, discord)

	comp := &stubComponent{name: "test-comp"}
	orc.Register(comp)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		orc.Start(ctx)
		close(done)
	}()

	// Wait for at least 3 ticks (initial + 2 from ticker)
	time.Sleep(200 * time.Millisecond)
	cancel()
	<-done

	ticks := comp.tickCount.Load()
	if ticks < 3 {
		t.Errorf("expected at least 3 ticks, got %d", ticks)
	}

	status := orc.GetStatus()
	if status.Running {
		t.Error("orchestrator should not be running after cancel")
	}
	if status.TickCount < 3 {
		t.Errorf("expected at least 3 ticks in status, got %d", status.TickCount)
	}
}

func TestOrchestratorPanicRecovery(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			CheckInterval: 50 * time.Millisecond,
		},
	}

	orc := NewOrchestrator(cfg, db, discord)

	pc := &panicComponent{name: "panicker"}
	good := &stubComponent{name: "good-comp"}
	orc.Register(pc)
	orc.Register(good)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		orc.Start(ctx)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done

	// Good component should still have been ticked
	if good.tickCount.Load() < 1 {
		t.Error("good component should have been ticked despite panicking sibling")
	}

	// Panicking component should be marked unhealthy
	status := orc.GetStatus()
	for _, ch := range status.Components {
		if ch.Name == "panicker" {
			if ch.Healthy {
				t.Error("panicking component should be unhealthy")
			}
			if ch.LastError == "" {
				t.Error("panicking component should have a last error")
			}
		}
	}
}

func TestOrchestratorComponentError(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			CheckInterval: 50 * time.Millisecond,
		},
	}

	orc := NewOrchestrator(cfg, db, discord)

	errComp := &stubComponent{name: "err-comp", err: errors.New("test error")}
	orc.Register(errComp)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		orc.Start(ctx)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()
	<-done

	status := orc.GetStatus()
	for _, ch := range status.Components {
		if ch.Name == "err-comp" {
			if ch.Healthy {
				t.Error("erroring component should be unhealthy")
			}
			if ch.LastError != "test error" {
				t.Errorf("expected 'test error', got %q", ch.LastError)
			}
		}
	}
}

func TestOrchestratorGetStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{}

	orc := NewOrchestrator(cfg, db, discord)

	comp := &stubComponent{name: "comp1"}
	orc.Register(comp)

	status := orc.GetStatus()
	if status.Running {
		t.Error("should not be running before Start")
	}
	if len(status.Components) != 1 {
		t.Errorf("expected 1 component, got %d", len(status.Components))
	}
	if status.Components[0].Name != "comp1" {
		t.Errorf("expected component name 'comp1', got %q", status.Components[0].Name)
	}
}

func TestOrchestratorStop(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			CheckInterval: 50 * time.Millisecond,
		},
	}

	orc := NewOrchestrator(cfg, db, discord)

	done := make(chan struct{})
	go func() {
		orc.Start(context.Background())
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	orc.Stop()
	<-done

	if orc.GetStatus().Running {
		t.Error("should not be running after Stop")
	}
}

func TestOrchestratorRateLimitStatus(t *testing.T) {
	db := testutil.NewTestDB(t)
	discord := notifier.NewDiscord("")
	cfg := &config.Config{}

	orc := NewOrchestrator(cfg, db, discord)

	rlh := NewRateLimitHandler(db, discord, "")
	orc.SetRateLimitHandler(rlh)

	// Manually set rate limited
	rlh.mu.Lock()
	rlh.isLimited = true
	rlh.rateLimitUntil = time.Now().Add(1 * time.Hour)
	rlh.mu.Unlock()

	status := orc.GetStatus()
	if !status.RateLimited {
		t.Error("status should reflect rate limited state")
	}
	if status.RateLimitUntil.IsZero() {
		t.Error("rate limit until should be set")
	}
}
