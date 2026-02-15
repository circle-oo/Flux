package triager

import (
	"testing"

	"github.com/circle-oo/flux/internal/models"
)

func TestParseSections(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "all four sections with ### headers",
			input: `### ANALYSIS
This task fixes a bug in the login flow.
- Affects auth module
- Risk: session handling

### PRIORITY
15

### MODEL
opus

### DESCRIPTION
Fix the login bug.
- Acceptance: users can log in
- Technical: check JWT validation`,
			expected: map[string]string{
				"analysis":    "This task fixes a bug in the login flow.\n- Affects auth module\n- Risk: session handling",
				"priority":    "15",
				"model":       "opus",
				"description": "Fix the login bug.\n- Acceptance: users can log in\n- Technical: check JWT validation",
			},
		},
		{
			name: "sections with ## headers",
			input: `## ANALYSIS
Simple task analysis.

## PRIORITY
50

## MODEL
sonnet

## DESCRIPTION
A simple description.`,
			expected: map[string]string{
				"analysis":    "Simple task analysis.",
				"priority":    "50",
				"model":       "sonnet",
				"description": "A simple description.",
			},
		},
		{
			name:  "empty input",
			input: "",
			expected: map[string]string{},
		},
		{
			name: "only analysis and priority",
			input: `### ANALYSIS
Some analysis here.

### PRIORITY
30`,
			expected: map[string]string{
				"analysis": "Some analysis here.",
				"priority": "30",
			},
		},
		{
			name: "text before first section is ignored",
			input: `Here is some preamble text that should be ignored.

### ANALYSIS
The real analysis.

### PRIORITY
42`,
			expected: map[string]string{
				"analysis": "The real analysis.",
				"priority": "42",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSections(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d sections, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for key, want := range tt.expected {
				got, ok := result[key]
				if !ok {
					t.Errorf("missing section %q", key)
					continue
				}
				if got != want {
					t.Errorf("section %q:\n  want: %q\n  got:  %q", key, want, got)
				}
			}
		})
	}
}

func TestParseTriageResponse(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		task         *models.Task
		wantPriority int
		wantModel    string
		wantAnalysis bool
		wantDesc     bool
	}{
		{
			name: "full response with opus model",
			input: `### ANALYSIS
This is a critical security fix that touches authentication.

### PRIORITY
5

### MODEL
opus

### DESCRIPTION
Fix the authentication bypass vulnerability.
- Validate all JWT tokens
- Add rate limiting`,
			task:         &models.Task{Priority: 50, Description: "fix auth"},
			wantPriority: 5,
			wantModel:    "opus",
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "full response with sonnet model",
			input: `### ANALYSIS
Standard feature addition.

### PRIORITY
35

### MODEL
sonnet

### DESCRIPTION
Add a new button to the UI.`,
			task:         &models.Task{Priority: 50, Description: "add button"},
			wantPriority: 35,
			wantModel:    "sonnet",
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "missing model section defaults to sonnet",
			input: `### ANALYSIS
Some analysis.

### PRIORITY
40

### DESCRIPTION
Some description.`,
			task:         &models.Task{Priority: 50, Description: "original"},
			wantPriority: 40,
			wantModel:    "sonnet",
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "invalid model defaults to sonnet",
			input: `### ANALYSIS
Some analysis.

### PRIORITY
40

### MODEL
gpt-4

### DESCRIPTION
Some description.`,
			task:         &models.Task{Priority: 50, Description: "original"},
			wantPriority: 40,
			wantModel:    "sonnet",
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "invalid priority keeps original",
			input: `### ANALYSIS
Analysis text.

### PRIORITY
invalid

### MODEL
opus

### DESCRIPTION
New description.`,
			task:         &models.Task{Priority: 25, Description: "old desc"},
			wantPriority: 25,
			wantModel:    "opus",
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "priority out of range keeps original",
			input: `### ANALYSIS
Analysis text.

### PRIORITY
150

### MODEL
sonnet

### DESCRIPTION
Desc.`,
			task:         &models.Task{Priority: 30, Description: "old"},
			wantPriority: 30,
			wantModel:    "sonnet",
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "empty response keeps defaults",
			input:        "",
			task:         &models.Task{Priority: 50, Description: "original desc"},
			wantPriority: 50,
			wantModel:    "sonnet",
			wantAnalysis: false,
			wantDesc:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTriageResponse(tt.input, tt.task)

			if result.Priority != tt.wantPriority {
				t.Errorf("priority: want %d, got %d", tt.wantPriority, result.Priority)
			}
			if result.Model != tt.wantModel {
				t.Errorf("model: want %q, got %q", tt.wantModel, result.Model)
			}
			if tt.wantAnalysis && result.Analysis == "" {
				t.Error("expected non-empty analysis")
			}
			if !tt.wantAnalysis && result.Analysis != "" {
				t.Errorf("expected empty analysis, got %q", result.Analysis)
			}
			if tt.wantDesc && result.Description == tt.task.Description {
				t.Error("expected description to be rewritten")
			}
			if !tt.wantDesc && result.Description != tt.task.Description {
				t.Errorf("expected original description %q, got %q", tt.task.Description, result.Description)
			}
		})
	}
}

func TestParseTriageResponse_ModelCaseInsensitive(t *testing.T) {
	task := &models.Task{Priority: 50, Description: "test"}

	tests := []struct {
		modelText string
		want      string
	}{
		{"opus", "opus"},
		{"Opus", "opus"},
		{"OPUS", "opus"},
		{"sonnet", "sonnet"},
		{"Sonnet", "sonnet"},
		{"SONNET", "sonnet"},
		{"haiku", "sonnet"},   // invalid model -> default
		{"gpt-4o", "sonnet"},  // invalid model -> default
	}

	for _, tt := range tests {
		t.Run(tt.modelText, func(t *testing.T) {
			input := "### ANALYSIS\ntest\n\n### PRIORITY\n50\n\n### MODEL\n" + tt.modelText + "\n\n### DESCRIPTION\nnew desc"
			result := parseTriageResponse(input, task)
			if result.Model != tt.want {
				t.Errorf("model %q: want %q, got %q", tt.modelText, tt.want, result.Model)
			}
		})
	}
}

func TestBuildTriagePrompt(t *testing.T) {
	task := &models.Task{
		Title:       "Fix login bug",
		Type:        models.TaskTypeBugfix,
		Priority:    20,
		Description: "Users cannot log in after password reset",
		Tags:        []string{"auth", "urgent"},
	}

	prompt := buildTriagePrompt(task)

	// Verify key elements are present in the prompt
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}

	checks := []string{
		"Fix login bug",
		"BUGFIX",
		"20",
		"Users cannot log in after password reset",
		"auth, urgent",
		"### ANALYSIS",
		"### PRIORITY",
		"### MODEL",
		"### DESCRIPTION",
		"opus",
		"sonnet",
	}

	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected content: %q", check)
		}
	}
}

func TestBuildTriagePrompt_EmptyTags(t *testing.T) {
	task := &models.Task{
		Title:       "Simple task",
		Type:        models.TaskTypeCoding,
		Priority:    50,
		Description: "A simple coding task",
	}

	prompt := buildTriagePrompt(task)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
}

func TestBuildTriagePrompt_NoDescription(t *testing.T) {
	task := &models.Task{
		Title:    "Minimal task",
		Type:     models.TaskTypeCoding,
		Priority: 50,
	}

	prompt := buildTriagePrompt(task)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if contains(prompt, "Minimal task") == false {
		t.Error("prompt should contain task title")
	}
}

func TestParseSections_DuplicateSection(t *testing.T) {
	input := `### ANALYSIS
First analysis.

### PRIORITY
10

### ANALYSIS
Second analysis (should overwrite).

### MODEL
opus`

	result := parseSections(input)

	// Second ANALYSIS section should overwrite the first
	if result["analysis"] != "Second analysis (should overwrite)." {
		t.Errorf("expected second analysis to win, got %q", result["analysis"])
	}
	if result["priority"] != "10" {
		t.Errorf("expected priority 10, got %q", result["priority"])
	}
	if result["model"] != "opus" {
		t.Errorf("expected model opus, got %q", result["model"])
	}
}

func TestParseSections_WhitespaceOnlyContent(t *testing.T) {
	input := `### ANALYSIS


### PRIORITY
50`

	result := parseSections(input)

	if result["analysis"] != "" {
		t.Errorf("expected empty analysis after trim, got %q", result["analysis"])
	}
	if result["priority"] != "50" {
		t.Errorf("expected priority 50, got %q", result["priority"])
	}
}

func TestParseSections_MixedHeaderLevels(t *testing.T) {
	// ### for some, ## for others — both should work
	input := `### ANALYSIS
Mixed analysis.

## PRIORITY
25

### MODEL
sonnet

## DESCRIPTION
Mixed description.`

	result := parseSections(input)
	if len(result) != 4 {
		t.Errorf("expected 4 sections, got %d: %v", len(result), result)
	}
	if result["analysis"] != "Mixed analysis." {
		t.Errorf("analysis: got %q", result["analysis"])
	}
	if result["priority"] != "25" {
		t.Errorf("priority: got %q", result["priority"])
	}
}

func TestParseTriageResponse_PriorityBoundaries(t *testing.T) {
	task := &models.Task{Priority: 50, Description: "test"}

	tests := []struct {
		name         string
		priorityText string
		wantPriority int
	}{
		{"priority 1 (minimum valid)", "1", 1},
		{"priority 100 (maximum valid)", "100", 100},
		{"priority 0 (below range)", "0", 50},     // keeps original
		{"priority -1 (negative)", "-1", 50},       // keeps original
		{"priority 101 (above range)", "101", 50},  // keeps original
		{"priority with leading space", "  42  ", 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "### ANALYSIS\ntest\n\n### PRIORITY\n" + tt.priorityText + "\n\n### MODEL\nsonnet\n\n### DESCRIPTION\nnew desc"
			result := parseTriageResponse(input, task)
			if result.Priority != tt.wantPriority {
				t.Errorf("priority %q: want %d, got %d", tt.priorityText, tt.wantPriority, result.Priority)
			}
		})
	}
}

func TestParseTriageResponse_ModelWithExtraWhitespace(t *testing.T) {
	task := &models.Task{Priority: 50, Description: "test"}

	// Model text with trailing newline/space — trimmed before comparison
	input := "### ANALYSIS\ntest\n\n### PRIORITY\n50\n\n### MODEL\n  opus  \n\n### DESCRIPTION\nnew desc"
	result := parseTriageResponse(input, task)
	if result.Model != "opus" {
		t.Errorf("expected model opus, got %q", result.Model)
	}
}

func TestParseTriageResponse_AnalysisContent(t *testing.T) {
	task := &models.Task{Priority: 50, Description: "original desc"}

	input := `### ANALYSIS
This is a multi-line analysis.
- Risk: high complexity
- Affected: auth module

### PRIORITY
20

### MODEL
opus

### DESCRIPTION
New description with acceptance criteria.`

	result := parseTriageResponse(input, task)

	if result.Analysis == "" {
		t.Error("expected non-empty analysis")
	}
	if !contains(result.Analysis, "multi-line analysis") {
		t.Errorf("analysis should contain 'multi-line analysis', got %q", result.Analysis)
	}
	if !contains(result.Analysis, "Risk: high complexity") {
		t.Errorf("analysis should contain bullet points, got %q", result.Analysis)
	}
	if result.Priority != 20 {
		t.Errorf("expected priority 20, got %d", result.Priority)
	}
	if result.Model != "opus" {
		t.Errorf("expected model opus, got %q", result.Model)
	}
	if result.Description == "original desc" {
		t.Error("expected description to be rewritten")
	}
}

func TestParseTriageResponse_OnlyAnalysisSection(t *testing.T) {
	task := &models.Task{Priority: 30, Description: "keep this"}

	input := `### ANALYSIS
Just an analysis, nothing else.`

	result := parseTriageResponse(input, task)

	if result.Analysis == "" {
		t.Error("expected non-empty analysis")
	}
	// All other fields should keep defaults
	if result.Priority != 30 {
		t.Errorf("expected original priority 30, got %d", result.Priority)
	}
	if result.Model != "sonnet" {
		t.Errorf("expected default model sonnet, got %q", result.Model)
	}
	if result.Description != "keep this" {
		t.Errorf("expected original description, got %q", result.Description)
	}
}

func TestTriageResult_Defaults(t *testing.T) {
	// Verify that parseTriageResponse with completely empty text produces safe defaults
	task := &models.Task{Priority: 42, Description: "original"}
	result := parseTriageResponse("", task)

	if result.Priority != 42 {
		t.Errorf("expected original priority 42, got %d", result.Priority)
	}
	if result.Description != "original" {
		t.Errorf("expected original description, got %q", result.Description)
	}
	if result.Model != "sonnet" {
		t.Errorf("expected default model sonnet, got %q", result.Model)
	}
	if result.Analysis != "" {
		t.Errorf("expected empty analysis, got %q", result.Analysis)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
