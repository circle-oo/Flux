package executor

import (
	"regexp"
	"strings"

	"github.com/circle-oo/flux/internal/models"
)

// DecompositionSignal represents a detected signal that suggests task decomposition.
type DecompositionSignal struct {
	Name        string
	Description string
	Strength    int // 1-3: weak, moderate, strong
}

// DecompositionAnalysis contains the results of analyzing whether a task should be decomposed.
type DecompositionAnalysis struct {
	ShouldDecompose bool
	Score           int
	Signals         []DecompositionSignal
	Reasoning       string
}

// ComplexityFactors represents scoring factors for task complexity.
type ComplexityFactors struct {
	MultiLayerArchitecture int
	ExternalIntegration    int
	DataMigration          int
	SecurityAuthChanges    int
	PerformanceOptimization int
	TestingRequirements    int
	BreakingChanges        int
	MultiRepoChanges       int
	AmbiguousRequirements  int
}

// Keyword patterns for detecting complexity factors
var (
	multiLayerKeywords = []string{"frontend", "backend", "database", "api", "ui", "full-stack", "end-to-end"}
	integrationKeywords = []string{"webhook", "third-party", "external", "integration", "api client", "sdk"}
	migrationKeywords = []string{"migration", "schema", "migrate", "upgrade", "transform data"}
	securityKeywords = []string{"auth", "authentication", "authorization", "security", "encrypt", "jwt", "oauth", "permission"}
	perfKeywords = []string{"optimize", "performance", "benchmark", "profile", "slow", "latency", "cache"}
	breakingKeywords = []string{"breaking", "backward", "compatibility", "deprecate", "major version"}
	multiRepoKeywords = []string{"multiple repo", "cross-repo", "monorepo", "submodule"}
	ambiguousKeywords = []string{"improve", "enhance", "refactor", "overhaul", "redesign", "research", "investigate", "explore"}

	// Regex patterns for detecting multiple verbs or conjunctions
	multipleVerbsRe = regexp.MustCompile(`\b(add|create|implement|update|fix|refactor|optimize)\b.*\b(and|also|then|additionally)\b.*\b(add|create|implement|update|fix|refactor|optimize)\b`)
)

// AnalyzeDecomposition determines if a task should be decomposed based on complexity and signals.
func AnalyzeDecomposition(task *models.Task) *DecompositionAnalysis {
	analysis := &DecompositionAnalysis{
		Signals: []DecompositionSignal{},
	}

	// Tasks at max depth cannot be decomposed further
	if task.Depth >= 2 {
		analysis.Reasoning = "Maximum decomposition depth (2) reached"
		return analysis
	}

	// Calculate complexity score
	factors := calculateComplexityFactors(task)
	analysis.Score = sumComplexityScore(factors)

	// Detect signals
	detectPrimarySignals(task, analysis)
	detectSecondarySignals(task, analysis)

	// Decomposition decision
	if analysis.Score >= 8 {
		analysis.ShouldDecompose = true
		analysis.Reasoning = "High complexity score indicates multi-step work"
	} else if analysis.Score >= 5 && len(analysis.Signals) >= 2 {
		analysis.ShouldDecompose = true
		analysis.Reasoning = "Moderate complexity with multiple decomposition signals"
	} else if hasStrongPrimarySignal(analysis) {
		analysis.ShouldDecompose = true
		analysis.Reasoning = "Strong primary signal detected"
	} else {
		analysis.Reasoning = "Task is focused and does not require decomposition"
	}

	return analysis
}

// calculateComplexityFactors analyzes task text to score complexity factors.
func calculateComplexityFactors(task *models.Task) ComplexityFactors {
	factors := ComplexityFactors{}
	combined := strings.ToLower(task.Title + " " + task.Description + " " + task.TriageAnalysis)

	// Multi-layer architecture (+3)
	if containsMultipleKeywords(combined, multiLayerKeywords, 2) {
		factors.MultiLayerArchitecture = 3
	}

	// External integration (+2)
	if containsAnyKeyword(combined, integrationKeywords) {
		factors.ExternalIntegration = 2
	}

	// Data migration (+3)
	if containsAnyKeyword(combined, migrationKeywords) {
		factors.DataMigration = 3
	}

	// Security/auth changes (+2)
	if containsAnyKeyword(combined, securityKeywords) {
		factors.SecurityAuthChanges = 2
	}

	// Performance optimization (+2)
	if containsAnyKeyword(combined, perfKeywords) {
		factors.PerformanceOptimization = 2
	}

	// Testing requirements (+1)
	if strings.Contains(combined, "test") || strings.Contains(combined, "e2e") || strings.Contains(combined, "integration test") {
		factors.TestingRequirements = 1
	}

	// Breaking changes (+2)
	if containsAnyKeyword(combined, breakingKeywords) {
		factors.BreakingChanges = 2
	}

	// Multi-repo changes (+3)
	if containsAnyKeyword(combined, multiRepoKeywords) {
		factors.MultiRepoChanges = 3
	}

	// Ambiguous requirements (+2)
	if containsMultipleKeywords(combined, ambiguousKeywords, 2) {
		factors.AmbiguousRequirements = 2
	}

	return factors
}

// sumComplexityScore totals all complexity factor points.
func sumComplexityScore(factors ComplexityFactors) int {
	return factors.MultiLayerArchitecture +
		factors.ExternalIntegration +
		factors.DataMigration +
		factors.SecurityAuthChanges +
		factors.PerformanceOptimization +
		factors.TestingRequirements +
		factors.BreakingChanges +
		factors.MultiRepoChanges +
		factors.AmbiguousRequirements
}

// detectPrimarySignals checks for primary decomposition signals.
func detectPrimarySignals(task *models.Task, analysis *DecompositionAnalysis) {
	combined := strings.ToLower(task.Title + " " + task.Description)

	// Multiple independent components
	if containsMultipleKeywords(combined, multiLayerKeywords, 3) {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Multiple Independent Components",
			Description: "Task spans 3+ unrelated subsystems",
			Strength:    3,
		})
	}

	// Sequential multi-phase work
	if containsPhaseKeywords(combined) {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Sequential Multi-Phase Work",
			Description: "Task requires distinct phases that build on each other",
			Strength:    3,
		})
	}

	// High complexity score (primary signal if score >= 8)
	if analysis.Score >= 8 {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "High Complexity Score",
			Description: "Combined complexity factors exceed threshold",
			Strength:    3,
		})
	}

	// Large scope (estimating from keywords)
	if estimateLargeScope(combined) {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Large Scope",
			Description: "Task likely touches 10+ files or produces 500+ lines",
			Strength:    2,
		})
	}

	// Cross-cutting concerns
	if containsMultipleKeywords(combined, multiLayerKeywords, 2) && containsAnyKeyword(combined, []string{"feature", "system", "platform"}) {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Cross-Cutting Concerns",
			Description: "Task requires coordination between multiple tech stack layers",
			Strength:    2,
		})
	}
}

// detectSecondarySignals checks for secondary decomposition indicators.
func detectSecondarySignals(task *models.Task, analysis *DecompositionAnalysis) {
	combined := strings.ToLower(task.Title + " " + task.Description)

	// Multiple verbs or conjunctions
	if multipleVerbsRe.MatchString(combined) {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Multiple Verbs/Conjunctions",
			Description: "Task description contains 'and', 'also', 'additionally' with multiple actions",
			Strength:    2,
		})
	}

	// Multiple acceptance criteria (check for bullet points or numbered lists)
	if countAcceptanceCriteria(task.Description) >= 3 {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Multiple Acceptance Criteria",
			Description: "Task has 3+ distinct acceptance criteria",
			Strength:    1,
		})
	}

	// High-value coding tasks
	if task.Type == models.TaskTypeCoding && task.Priority <= 25 {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "High-Value Feature",
			Description: "High-priority coding task may benefit from phased approach",
			Strength:    1,
		})
	}

	// Ambiguous description
	if len(strings.TrimSpace(task.Description)) < 50 || containsAnyKeyword(combined, ambiguousKeywords) {
		analysis.Signals = append(analysis.Signals, DecompositionSignal{
			Name:        "Ambiguous Requirements",
			Description: "Task description is vague or exploratory",
			Strength:    1,
		})
	}
}

// hasStrongPrimarySignal returns true if any signal has strength 3.
func hasStrongPrimarySignal(analysis *DecompositionAnalysis) bool {
	for _, signal := range analysis.Signals {
		if signal.Strength >= 3 {
			return true
		}
	}
	return false
}

// Helper functions

// containsAnyKeyword returns true if text contains any of the keywords.
func containsAnyKeyword(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}

// containsMultipleKeywords returns true if text contains at least minCount keywords.
func containsMultipleKeywords(text string, keywords []string, minCount int) bool {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			count++
			if count >= minCount {
				return true
			}
		}
	}
	return false
}

// containsPhaseKeywords detects sequential phase language.
func containsPhaseKeywords(text string) bool {
	// Look for explicit phase/step numbering
	phasePatterns := []string{"phase 1", "phase 2", "step 1", "step 2", "stage 1", "stage 2"}
	for _, pattern := range phasePatterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}

	// Or multiple sequential keywords
	phaseWords := []string{"first", "then", "next", "finally"}
	return containsMultipleKeywords(text, phaseWords, 2)
}

// estimateLargeScope estimates if task has large scope based on keywords.
func estimateLargeScope(text string) bool {
	largeKeywords := []string{"multiple", "several", "across", "entire", "all", "comprehensive", "complete"}
	fileKeywords := []string{"files", "modules", "components", "services", "packages"}
	return containsAnyKeyword(text, largeKeywords) && containsAnyKeyword(text, fileKeywords)
}

// countAcceptanceCriteria estimates acceptance criteria count from description.
// Looks for bullet points, numbered lists, or "must/should" statements.
func countAcceptanceCriteria(description string) int {
	count := 0
	lines := strings.Split(description, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Bullet points: -, *, •
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "•") {
			count++
		}
		// Numbered lists: 1., 2., etc.
		if matched, _ := regexp.MatchString(`^\d+\.`, trimmed); matched {
			count++
		}
		// Must/should statements
		if strings.Contains(strings.ToLower(trimmed), "must") || strings.Contains(strings.ToLower(trimmed), "should") {
			count++
		}
	}
	return count
}

// ShouldSkipDecomposition returns true if task should NOT be decomposed regardless of signals.
func ShouldSkipDecomposition(task *models.Task) bool {
	// Already at max depth
	if task.Depth >= 2 {
		return true
	}

	// Quick wins
	if isQuickWin(task) {
		return true
	}

	// Already small
	if isAlreadySmall(task) {
		return true
	}

	return false
}

// isQuickWin checks if task is a trivial quick win.
func isQuickWin(task *models.Task) bool {
	if task.Type == models.TaskTypeDocument {
		return true
	}

	combined := strings.ToLower(task.Title + " " + task.Description)
	quickWinKeywords := []string{"typo", "fix readme", "update comment", "config change", "version bump"}
	return containsAnyKeyword(combined, quickWinKeywords)
}

// isAlreadySmall checks if task is already small and focused.
func isAlreadySmall(task *models.Task) bool {
	// Very short description suggests focused task
	if len(strings.TrimSpace(task.Description)) < 100 {
		return true
	}

	// Single file/function mention
	combined := strings.ToLower(task.Title + " " + task.Description)
	singleScopeKeywords := []string{"in file", "function ", "method ", "one file", "single"}
	return containsAnyKeyword(combined, singleScopeKeywords)
}
