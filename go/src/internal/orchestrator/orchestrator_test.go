package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/models"
)

func TestSelectModel_NormalTask(t *testing.T) {
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			Models: config.ModelsConfig{
				Opus:   "claude-opus-4",
				Sonnet: "claude-sonnet-3.5",
			},
		},
	}
	rlh := &RateLimitHandler{} // Not rate limited
	orc := NewOrchestrator(cfg, rlh)

	task := &models.Task{
		Title:       "Normal coding task",
		Description: "Implement a simple feature",
		Priority:    10,
		
	}

	model := orc.SelectModel(task)
	if model != cfg.Orchestrator.Models.Sonnet {
		t.Errorf("Expected Sonnet for normal task, got %s", model)
	}
}

func TestSelectModel_OpusForHighPriority(t *testing.T) {
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			Models: config.ModelsConfig{
				Opus:   "claude-opus-4",
				Sonnet: "claude-sonnet-3.5",
			},
		},
	}
	rlh := &RateLimitHandler{} // Not rate limited
	orc := NewOrchestrator(cfg, rlh)

	task := &models.Task{
		Title:       "Critical task",
		Description: "High priority work",
		Priority:    5, // Priority <= 5 triggers Opus
		
	}

	model := orc.SelectModel(task)
	if model != cfg.Orchestrator.Models.Opus {
		t.Errorf("Expected Opus for priority <= 5, got %s", model)
	}
}

func TestSelectModel_OpusForComplexKeywords(t *testing.T) {
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			Models: config.ModelsConfig{
				Opus:   "claude-opus-4",
				Sonnet: "claude-sonnet-3.5",
			},
		},
	}
	rlh := &RateLimitHandler{} // Not rate limited
	orc := NewOrchestrator(cfg, rlh)

	testCases := []struct {
		title       string
		description string
		keyword     string
	}{
		{"Architect the system", "Design new architecture", "architect"},
		{"Refactor the codebase", "Large scale refactor", "refactor"},
		{"Redesign the API", "Complete redesign needed", "redesign"},
		{"Database migration", "Migrate to new schema", "migration"},
		{"Security audit", "Review security", "security"},
		{"System overhaul", "Complete overhaul", "overhaul"},
	}

	for _, tc := range testCases {
		t.Run(tc.keyword, func(t *testing.T) {
			task := &models.Task{
				Title:       tc.title,
				Description: tc.description,
				Priority:    10,
				Source:      models.TaskSourceOperator,
				
			}

			model := orc.SelectModel(task)
			if model != cfg.Orchestrator.Models.Opus {
				t.Errorf("Expected Opus for keyword '%s', got %s", tc.keyword, model)
			}
		})
	}
}

func TestSelectModel_OpusForInitialDesignTag(t *testing.T) {
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			Models: config.ModelsConfig{
				Opus:   "claude-opus-4",
				Sonnet: "claude-sonnet-3.5",
			},
		},
	}
	rlh := &RateLimitHandler{} // Not rate limited
	orc := NewOrchestrator(cfg, rlh)

	task := &models.Task{
		Title:       "Design new feature",
		Description: "Initial design phase",
		Priority:    10,
		Tags:        []string{"initial-design"},
		
	}

	model := orc.SelectModel(task)
	if model != cfg.Orchestrator.Models.Opus {
		t.Errorf("Expected Opus for initial-design tag, got %s", model)
	}
}

func TestSelectModel_OpusForGoalStrategyTag(t *testing.T) {
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			Models: config.ModelsConfig{
				Opus:   "claude-opus-4",
				Sonnet: "claude-sonnet-3.5",
			},
		},
	}
	rlh := &RateLimitHandler{} // Not rate limited
	orc := NewOrchestrator(cfg, rlh)

	task := &models.Task{
		Title:       "Develop goal strategy",
		Description: "Strategic planning",
		Priority:    10,
		Tags:        []string{"goal-strategy"},
		
	}

	model := orc.SelectModel(task)
	if model != cfg.Orchestrator.Models.Opus {
		t.Errorf("Expected Opus for goal-strategy tag, got %s", model)
	}
}

func TestSelectModel_SonnetWhenRecentlyLimited(t *testing.T) {
	cfg := &config.Config{
		Orchestrator: config.OrchestratorConfig{
			Models: config.ModelsConfig{
				Opus:   "claude-opus-4",
				Sonnet: "claude-sonnet-3.5",
			},
		},
	}

	// Create a rate limit handler and manually set state to simulate recent rate limit
	rlh := &RateLimitHandler{
		isLimited:      true,
		rateLimitUntil: time.Now().Add(1 * time.Hour), // Currently limited
	}

	orc := NewOrchestrator(cfg, rlh)

	// Even with high priority task that would normally use Opus
	task := &models.Task{
		Title:       "Critical architecture task",
		Description: "Architect the system with security focus",
		Priority:    3, // Would normally trigger Opus
		Source:      models.TaskSourceOperator,
		Tags:        []string{"initial-design", "goal-strategy"},
		
	}

	model := orc.SelectModel(task)
	if model != cfg.Orchestrator.Models.Sonnet {
		t.Errorf("Expected Sonnet when recently limited (cost-saving mode), got %s", model)
	}
}

func TestBuildGoalSystemPrompt_ValidGoal(t *testing.T) {
	cfg := &config.Config{}
	rlh := &RateLimitHandler{}
	orc := NewOrchestrator(cfg, rlh)

	goal := &models.Goal{
		Title:       "Improve system reliability",
		Description: "Reduce errors and increase uptime",
		Priorities:  []string{"Error handling", "Monitoring", "Testing"},
		Metrics:     []string{"Uptime %", "Error rate", "MTTR"},
	}

	prompt := orc.BuildGoalSystemPrompt(goal)

	// Verify all components are present
	expectedParts := []string{
		"Current Goal: Improve system reliability",
		"Description: Reduce errors and increase uptime",
		"Priorities: Error handling, Monitoring, Testing",
		"Metrics: Uptime %, Error rate, MTTR",
		"All your work should align with this Goal.",
	}

	for _, part := range expectedParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("Expected prompt to contain '%s', got:\n%s", part, prompt)
		}
	}
}

func TestBuildGoalSystemPrompt_NilGoal(t *testing.T) {
	cfg := &config.Config{}
	rlh := &RateLimitHandler{}
	orc := NewOrchestrator(cfg, rlh)

	prompt := orc.BuildGoalSystemPrompt(nil)
	if prompt != "" {
		t.Errorf("Expected empty string for nil goal, got: %s", prompt)
	}
}

func TestBuildGoalSystemPrompt_EmptyPriorities(t *testing.T) {
	cfg := &config.Config{}
	rlh := &RateLimitHandler{}
	orc := NewOrchestrator(cfg, rlh)

	goal := &models.Goal{
		Title:       "Test goal",
		Description: "Testing empty priorities",
		Priorities:  []string{},
		Metrics:     []string{"Metric 1"},
	}

	prompt := orc.BuildGoalSystemPrompt(goal)

	if !strings.Contains(prompt, "Priorities: (none)") {
		t.Errorf("Expected 'Priorities: (none)' for empty priorities, got:\n%s", prompt)
	}
}

func TestBuildGoalSystemPrompt_EmptyMetrics(t *testing.T) {
	cfg := &config.Config{}
	rlh := &RateLimitHandler{}
	orc := NewOrchestrator(cfg, rlh)

	goal := &models.Goal{
		Title:       "Test goal",
		Description: "Testing empty metrics",
		Priorities:  []string{"Priority 1"},
		Metrics:     []string{},
	}

	prompt := orc.BuildGoalSystemPrompt(goal)

	if !strings.Contains(prompt, "Metrics: (none)") {
		t.Errorf("Expected 'Metrics: (none)' for empty metrics, got:\n%s", prompt)
	}
}

func TestBuildGoalSystemPrompt_EmptyPrioritiesAndMetrics(t *testing.T) {
	cfg := &config.Config{}
	rlh := &RateLimitHandler{}
	orc := NewOrchestrator(cfg, rlh)

	goal := &models.Goal{
		Title:       "Minimal goal",
		Description: "Goal with no priorities or metrics",
		Priorities:  []string{},
		Metrics:     []string{},
	}

	prompt := orc.BuildGoalSystemPrompt(goal)

	expectedParts := []string{
		"Current Goal: Minimal goal",
		"Description: Goal with no priorities or metrics",
		"Priorities: (none)",
		"Metrics: (none)",
		"All your work should align with this Goal.",
	}

	for _, part := range expectedParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("Expected prompt to contain '%s', got:\n%s", part, prompt)
		}
	}
}

