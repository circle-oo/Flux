package executor

import (
	"testing"

	"github.com/circle-oo/flux/internal/models"
)

func TestAnalyzeDecomposition(t *testing.T) {
	tests := []struct {
		name                string
		task                *models.Task
		wantDecompose       bool
		wantMinScore        int
		wantMinSignals      int
		wantReasonContains  string
	}{
		{
			name: "high complexity full-stack feature",
			task: &models.Task{
				Title:       "Add real-time notifications system",
				Description: "Implement WebSocket backend infrastructure, notification queue, frontend UI component, and subscription management. Include authentication and security for WebSocket connections.",
				Priority:    15,
				Depth:       0,
			},
			wantDecompose:  true,
			wantMinScore:   4, // security (2) + testing (1) + multi-layer signal
			wantMinSignals: 2,
			wantReasonContains: "signal",
		},
		{
			name: "simple bug fix",
			task: &models.Task{
				Title:       "Fix typo in README",
				Description: "Correct spelling mistake in README.md file",
				Priority:    50,
				Depth:       0,
			},
			wantDecompose: false,
			wantMinScore:  0,
		},
		{
			name: "multi-phase database migration",
			task: &models.Task{
				Title:       "Migrate database to PostgreSQL",
				Description: "Phase 1: Create schema. Phase 2: Migrate data. Phase 3: Update app connections and test.",
				Priority:    20,
				Depth:       0,
			},
			wantDecompose:  true,
			wantMinSignals: 1,
			wantReasonContains: "signal",
		},
		{
			name: "already at max depth",
			task: &models.Task{
				Title:       "Add complex feature",
				Description: "This is a complex task with backend, frontend, and database changes",
				Priority:    15,
				Depth:       2, // max depth
			},
			wantDecompose: false,
			wantReasonContains: "Maximum decomposition depth",
		},
		{
			name: "multiple verbs and conjunctions",
			task: &models.Task{
				Title:       "Refactor authentication and optimize performance",
				Description: "Update the auth system and also improve query performance. Additionally, add caching.",
				Priority:    25,
				Depth:       0,
			},
			wantDecompose:  true,
			wantMinSignals: 1,
		},
		{
			name: "high-priority with security and integration",
			task: &models.Task{
				Title:       "Implement OAuth2 authentication",
				Description: "Add OAuth2 authentication with JWT tokens and authorization middleware. Integrate third-party OAuth provider. Build frontend login UI, backend API, and token management.",
				Priority:    10,
				Depth:       0,
			},
			wantDecompose:  true,
			wantMinScore:   4, // security (2) + integration (2) + multi-layer potentially
			wantMinSignals: 1,
		},
		{
			name: "documentation task",
			task: &models.Task{
				Title:       "Update API documentation",
				Description: "Add documentation for new endpoints",
				Priority:    60,
				Depth:       0,
			},
			wantDecompose: false,
		},
		{
			name: "multiple acceptance criteria with multi-layer",
			task: &models.Task{
				Title:       "Implement user profile",
				Description: `Add user profile feature with frontend, backend, and database:
- Must allow profile picture upload
- Must support bio editing
- Should show user statistics
- Should have privacy settings
- Must validate all inputs`,
				Priority: 30,
				Depth:    0,
			},
			wantDecompose:  true,
			wantMinSignals: 1,
		},
		{
			name: "performance optimization with profiling phases",
			task: &models.Task{
				Title:       "Optimize slow query performance",
				Description: "Step 1: Profile queries and identify bottlenecks. Step 2: Add indexes. Finally implement caching.",
				Priority:    25,
				Depth:       0,
			},
			wantDecompose:  true,
			wantMinScore:   2,
			wantMinSignals: 1,
		},
		{
			name: "external integration multi-layer",
			task: &models.Task{
				Title:       "Integrate payment gateway",
				Description: "Add Stripe integration with webhook handlers, backend payment processing, and frontend payment UI",
				Priority:    15,
				Depth:       0,
			},
			wantDecompose:  true,
			wantMinScore:   2,
			wantMinSignals: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := AnalyzeDecomposition(tt.task)

			if analysis.ShouldDecompose != tt.wantDecompose {
				t.Errorf("ShouldDecompose = %v, want %v (score=%d, signals=%d, reason=%q)",
					analysis.ShouldDecompose, tt.wantDecompose,
					analysis.Score, len(analysis.Signals), analysis.Reasoning)
			}

			if tt.wantMinScore > 0 && analysis.Score < tt.wantMinScore {
				t.Errorf("Score = %d, want at least %d", analysis.Score, tt.wantMinScore)
			}

			if tt.wantMinSignals > 0 && len(analysis.Signals) < tt.wantMinSignals {
				t.Errorf("Signals count = %d, want at least %d", len(analysis.Signals), tt.wantMinSignals)
			}

			if tt.wantReasonContains != "" && analysis.Reasoning != "" {
				if !contains(analysis.Reasoning, tt.wantReasonContains) {
					t.Errorf("Reasoning = %q, want to contain %q", analysis.Reasoning, tt.wantReasonContains)
				}
			}
		})
	}
}

func TestCalculateComplexityFactors(t *testing.T) {
	tests := []struct {
		name      string
		task      *models.Task
		wantScore int
		checkFunc func(ComplexityFactors) bool
	}{
		{
			name: "multi-layer architecture",
			task: &models.Task{
				Title:       "Full-stack feature",
				Description: "Add frontend UI, backend API, and database schema",
			},
			wantScore: 3,
			checkFunc: func(f ComplexityFactors) bool {
				return f.MultiLayerArchitecture == 3
			},
		},
		{
			name: "external integration",
			task: &models.Task{
				Title:       "Add webhook",
				Description: "Integrate third-party webhook handlers",
			},
			wantScore: 2,
			checkFunc: func(f ComplexityFactors) bool {
				return f.ExternalIntegration == 2
			},
		},
		{
			name: "data migration",
			task: &models.Task{
				Title:       "Schema migration",
				Description: "Migrate database schema and transform data",
			},
			wantScore: 3,
			checkFunc: func(f ComplexityFactors) bool {
				return f.DataMigration == 3
			},
		},
		{
			name: "security changes",
			task: &models.Task{
				Title:       "Add authentication",
				Description: "Implement JWT authentication and authorization middleware",
			},
			wantScore: 2,
			checkFunc: func(f ComplexityFactors) bool {
				return f.SecurityAuthChanges == 2
			},
		},
		{
			name: "performance optimization",
			task: &models.Task{
				Title:       "Optimize performance",
				Description: "Profile slow queries and add benchmark tests",
			},
			wantScore: 3, // perf + testing
			checkFunc: func(f ComplexityFactors) bool {
				return f.PerformanceOptimization == 2 && f.TestingRequirements == 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factors := calculateComplexityFactors(tt.task)
			score := sumComplexityScore(factors)

			if score < tt.wantScore {
				t.Errorf("Score = %d, want at least %d", score, tt.wantScore)
			}

			if tt.checkFunc != nil && !tt.checkFunc(factors) {
				t.Errorf("Complexity factors check failed: %+v", factors)
			}
		})
	}
}

func TestShouldSkipDecomposition(t *testing.T) {
	tests := []struct {
		name       string
		task       *models.Task
		wantSkip   bool
	}{
		{
			name: "max depth reached",
			task: &models.Task{
				Title: "Subtask at depth 2",
				Depth: 2,
			},
			wantSkip: true,
		},
		{
			name: "quick win - typo fix",
			task: &models.Task{
				Title:       "Fix typo in README",
				Description: "Correct spelling",
			},
			wantSkip: true,
		},
		{
			name: "documentation task",
			task: &models.Task{
				Title: "Update docs",
			},
			wantSkip: true,
		},
		{
			name: "small focused task",
			task: &models.Task{
				Title:       "Add function",
				Description: "Add a single function to handle validation",
			},
			wantSkip: true,
		},
		{
			name: "large complex task",
			task: &models.Task{
				Title:       "Implement authentication system",
				Description: "Add complete authentication with frontend, backend, database, and middleware. Include OAuth2 and JWT support.",
			},
			wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip := ShouldSkipDecomposition(tt.task)
			if skip != tt.wantSkip {
				t.Errorf("ShouldSkipDecomposition() = %v, want %v", skip, tt.wantSkip)
			}
		})
	}
}

func TestCountAcceptanceCriteria(t *testing.T) {
	tests := []struct {
		name        string
		description string
		wantMin     int
	}{
		{
			name: "bullet points",
			description: `Feature requirements:
- Add user login
- Add password reset
- Add OAuth support
- Must validate all inputs`,
			wantMin: 4,
		},
		{
			name: "numbered list",
			description: `Steps:
1. Create schema
2. Migrate data
3. Update app
4. Run tests`,
			wantMin: 4,
		},
		{
			name: "must/should statements",
			description: `The system must authenticate users.
It should support OAuth.
It must validate tokens.`,
			wantMin: 3,
		},
		{
			name:        "no criteria",
			description: "Just implement the feature",
			wantMin:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := countAcceptanceCriteria(tt.description)
			if count < tt.wantMin {
				t.Errorf("countAcceptanceCriteria() = %d, want at least %d", count, tt.wantMin)
			}
		})
	}
}

func TestDetectPrimarySignals(t *testing.T) {
	tests := []struct {
		name           string
		task           *models.Task
		wantSignalName string
	}{
		{
			name: "multiple independent components",
			task: &models.Task{
				Title:       "Full-stack feature",
				Description: "Add frontend UI, backend API, and database schema",
			},
			wantSignalName: "Multiple Independent Components",
		},
		{
			name: "sequential phases",
			task: &models.Task{
				Title:       "Migration",
				Description: "Phase 1: plan schema. Phase 2: migrate data. Finally: test.",
			},
			wantSignalName: "Sequential Multi-Phase Work",
		},
		{
			name: "large scope",
			task: &models.Task{
				Title:       "Comprehensive refactor",
				Description: "Refactor all modules across multiple files and components",
			},
			wantSignalName: "Large Scope",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &DecompositionAnalysis{}
			detectPrimarySignals(tt.task, analysis)

			found := false
			for _, signal := range analysis.Signals {
				if signal.Name == tt.wantSignalName {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected signal %q not found in %+v", tt.wantSignalName, analysis.Signals)
			}
		})
	}
}

func TestDetectSecondarySignals(t *testing.T) {
	tests := []struct {
		name           string
		task           *models.Task
		wantSignalName string
	}{
		{
			name: "multiple verbs",
			task: &models.Task{
				Title:       "Refactor and optimize",
				Description: "Refactor auth system and also optimize queries",
			},
			wantSignalName: "Multiple Verbs/Conjunctions",
		},
		{
			name: "high-priority coding",
			task: &models.Task{
				Priority: 15,
			},
			wantSignalName: "High-Value Feature",
		},
		{
			name: "ambiguous requirements",
			task: &models.Task{
				Title:       "Improve system",
				Description: "Enhance performance",
			},
			wantSignalName: "Ambiguous Requirements",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &DecompositionAnalysis{}
			detectSecondarySignals(tt.task, analysis)

			found := false
			for _, signal := range analysis.Signals {
				if signal.Name == tt.wantSignalName {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected signal %q not found in %+v", tt.wantSignalName, analysis.Signals)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
