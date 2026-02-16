package orchestrator

import (
	"testing"

	"github.com/circle-oo/flux/internal/config"
)

func TestNewOrchestrator(t *testing.T) {
	cfg := &config.Config{}
	rlh := &RateLimitHandler{}
	orc := NewOrchestrator(cfg, rlh)
	if orc == nil {
		t.Fatal("Expected non-nil orchestrator")
	}
}
