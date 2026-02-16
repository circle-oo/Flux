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
			name: "three sections with ### headers",
			input: `### ANALYSIS
This task fixes a bug in the login flow.
- Affects auth module
- Risk: session handling

### PRIORITY
15

### DESCRIPTION
Fix the login bug.
- Acceptance: users can log in
- Technical: check JWT validation`,
			expected: map[string]string{
				"analysis":    "This task fixes a bug in the login flow.\n- Affects auth module\n- Risk: session handling",
				"priority":    "15",
				"description": "Fix the login bug.\n- Acceptance: users can log in\n- Technical: check JWT validation",
			},
		},
		{
			name: "sections with ## headers",
			input: `## ANALYSIS
Simple task analysis.

## PRIORITY
50

## DESCRIPTION
A simple description.`,
			expected: map[string]string{
				"analysis":    "Simple task analysis.",
				"priority":    "50",
				"description": "A simple description.",
			},
		},
		{
			name:     "empty input",
			input:    "",
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

func TestParseTriageResponse_JSON(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		task         *models.Task
		wantPriority int
		wantAnalysis bool
		wantDesc     bool
	}{
		{
			name:         "valid JSON response",
			input:        `{"analysis": "This is a critical security fix.", "priority": 5, "description": "Fix the auth bypass.\n- Validate tokens\n- Add rate limiting"}`,
			task:         &models.Task{Priority: 50, Description: "fix auth bypass"},
			wantPriority: 5,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "JSON with normal priority",
			input:        `{"analysis": "Standard feature addition.", "priority": 35, "description": "Add a new button to the UI."}`,
			task:         &models.Task{Priority: 50, Description: "add button"},
			wantPriority: 35,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "JSON with zero priority keeps task default",
			input:        `{"analysis": "Some analysis.", "priority": 0, "description": "New desc."}`,
			task:         &models.Task{Priority: 40, Description: "original"},
			wantPriority: 40,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "JSON with out-of-range priority keeps task default",
			input:        `{"analysis": "Analysis.", "priority": 150, "description": "Desc."}`,
			task:         &models.Task{Priority: 30, Description: "old"},
			wantPriority: 30,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "JSON with empty analysis",
			input:        `{"analysis": "", "priority": 42, "description": "New desc."}`,
			task:         &models.Task{Priority: 50, Description: "original"},
			wantPriority: 42,
			wantAnalysis: false,
			wantDesc:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTriageResponse(tt.input, tt.task)

			if result.Priority != tt.wantPriority {
				t.Errorf("priority: want %d, got %d", tt.wantPriority, result.Priority)
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

func TestParseTriageResponse_JSONFallbackToMarkdown(t *testing.T) {
	task := &models.Task{Priority: 50, Description: "original desc"}

	// Invalid JSON should fall back to markdown parsing
	input := `### ANALYSIS
Standard feature addition.

### PRIORITY
35

### DESCRIPTION
Add a new button to the UI.`

	result := parseTriageResponse(input, task)

	if result.Priority != 35 {
		t.Errorf("priority: want 35, got %d", result.Priority)
	}
	if result.Analysis == "" {
		t.Error("expected non-empty analysis from markdown fallback")
	}
	if result.Description == task.Description {
		t.Error("expected description to be rewritten from markdown fallback")
	}
}

func TestParseTriageResponse(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		task         *models.Task
		wantPriority int
		wantAnalysis bool
		wantDesc     bool
	}{
		{
			name: "full markdown response",
			input: `### ANALYSIS
This is a critical security fix that touches authentication.

### PRIORITY
5

### DESCRIPTION
Fix the authentication bypass vulnerability.
- Validate all JWT tokens
- Add rate limiting`,
			task:         &models.Task{Priority: 50, Description: "fix authentication bypass vulnerability in production"},
			wantPriority: 5,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "markdown with sonnet model (model section ignored)",
			input: `### ANALYSIS
Standard feature addition.

### PRIORITY
35

### DESCRIPTION
Add a new button to the UI.`,
			task:         &models.Task{Priority: 50, Description: "add button"},
			wantPriority: 35,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "invalid priority keeps original",
			input: `### ANALYSIS
Analysis text.

### PRIORITY
invalid

### DESCRIPTION
New description.`,
			task:         &models.Task{Priority: 25, Description: "old desc"},
			wantPriority: 25,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name: "priority out of range keeps original",
			input: `### ANALYSIS
Analysis text.

### PRIORITY
150

### DESCRIPTION
Desc.`,
			task:         &models.Task{Priority: 30, Description: "old"},
			wantPriority: 30,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "empty response keeps defaults",
			input:        "",
			task:         &models.Task{Priority: 50, Description: "original desc"},
			wantPriority: 50,
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

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON",
			input: `{"analysis": "test", "priority": 5}`,
			want:  `{"analysis": "test", "priority": 5}`,
		},
		{
			name:  "markdown code fence",
			input: "```json\n{\"analysis\": \"test\", \"priority\": 5}\n```",
			want:  `{"analysis": "test", "priority": 5}`,
		},
		{
			name:  "markdown code fence without language",
			input: "```\n{\"analysis\": \"test\"}\n```",
			want:  `{"analysis": "test"}`,
		},
		{
			name:  "JSON embedded in narrative",
			input: "Here is the analysis:\n{\"analysis\": \"critical fix\", \"priority\": 10, \"description\": \"fix it\"}\nDone.",
			want:  `{"analysis": "critical fix", "priority": 10, "description": "fix it"}`,
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "no JSON at all",
			input: "This is just plain text with no JSON.",
			want:  "This is just plain text with no JSON.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON():\n  got:  %q\n  want: %q", got, tt.want)
			}
		})
	}
}

func TestParseTriageResponse_WrappedJSON(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		task         *models.Task
		wantPriority int
		wantAnalysis bool
		wantDesc     bool
	}{
		{
			name:         "JSON in markdown code fence",
			input:        "```json\n{\"analysis\": \"Security fix needed.\", \"priority\": 25, \"description\": \"Fix the auth bypass.\"}\n```",
			task:         &models.Task{Priority: 40, Description: "fix auth"},
			wantPriority: 25,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "JSON embedded in narrative text",
			input:        "Here is my analysis of the task:\n{\"analysis\": \"This is a standard feature.\", \"priority\": 40, \"description\": \"Add the button.\"}\nLet me know if you need more details.",
			task:         &models.Task{Priority: 40, Description: "add button"},
			wantPriority: 40,
			wantAnalysis: true,
			wantDesc:     true,
		},
		{
			name:         "pure narrative falls back to markdown parsing",
			input:        "Perfect. All autopilot modes have been successfully cancelled. The system is now idle.",
			task:         &models.Task{Priority: 40, Description: "original desc"},
			wantPriority: 40,
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
		"20",
		"Users cannot log in after password reset",
		"auth, urgent",
		"most tasks should fall in the 31-55 range",
		"When in doubt, assign priority 40",
		`"analysis"`,
		`"priority"`,
		`"description"`,
		"JSON object",
	}

	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected content: %q", check)
		}
	}

	// Verify MODEL section is NOT present
	forbidden := []string{
		"### MODEL",
		"When in doubt, use sonnet",
	}
	for _, check := range forbidden {
		if contains(prompt, check) {
			t.Errorf("prompt should NOT contain: %q", check)
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

func TestParseTriageResponse_SanityGuard(t *testing.T) {
	tests := []struct {
		name         string
		task         *models.Task
		input        string
		wantPriority int
	}{
		{
			name:         "short garbage input with high priority gets overridden (JSON)",
			task:         &models.Task{Title: "asdf", Description: "", Priority: 40},
			input:        `{"analysis": "test", "priority": 1, "description": "new desc"}`,
			wantPriority: 40, // overridden: inputLen < 10 and priority < 20
		},
		{
			name:         "short input with normal priority stays (JSON)",
			task:         &models.Task{Title: "test", Description: "", Priority: 40},
			input:        `{"analysis": "test", "priority": 35, "description": "new desc"}`,
			wantPriority: 35,
		},
		{
			name:         "long input with high priority stays (JSON)",
			task:         &models.Task{Title: "Fix critical auth bypass vulnerability", Description: "Production is down", Priority: 50},
			input:        `{"analysis": "test", "priority": 3, "description": "new desc"}`,
			wantPriority: 3, // not overridden: inputLen >= 10
		},
		{
			name:         "short input with priority in guard range gets overridden (JSON)",
			task:         &models.Task{Title: "fix", Description: "", Priority: 40},
			input:        `{"analysis": "test", "priority": 10, "description": "new desc"}`,
			wantPriority: 40, // guard fires for < 20 when inputLen < 10
		},
		{
			name:         "short garbage input with high priority gets overridden (markdown fallback)",
			task:         &models.Task{Title: "asdf", Description: "", Priority: 40},
			input:        "### ANALYSIS\ntest\n\n### PRIORITY\n1\n\n### DESCRIPTION\nnew desc",
			wantPriority: 40, // overridden: inputLen < 10 and priority < 20
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTriageResponse(tt.input, tt.task)
			if result.Priority != tt.wantPriority {
				t.Errorf("priority: want %d, got %d", tt.wantPriority, result.Priority)
			}
		})
	}
}

func TestBuildTriagePrompt_ProjectName(t *testing.T) {
	task := &models.Task{
		Title:       "Add feature",
		Type:        models.TaskTypeCoding,
		Priority:    50,
		Description: "Add a new feature",
		ProjectID:   "my-project",
	}

	prompt := buildTriagePrompt(task)
	if !contains(prompt, "my-project") {
		t.Error("prompt should contain project name from ProjectID")
	}
}

func TestBuildTriagePromptWithContext_ProjectInfo(t *testing.T) {
	task := &models.Task{
		Title:       "Add authentication",
		Type:        models.TaskTypeCoding,
		Priority:    50,
		Description: "Add JWT authentication to the API",
		ProjectID:   "api-service",
	}

	project := &models.Project{
		ID:          "api-service",
		Name:        "API Service",
		Description: "Core REST API for the platform",
		Type:        models.ProjectTypeService,
		TechStack:   []string{"Go", "PostgreSQL", "Redis"},
	}

	prompt := buildTriagePromptWithContext(task, project, nil)

	checks := []string{
		"API Service",
		"Core REST API for the platform",
		"SERVICE",
		"Go, PostgreSQL, Redis",
		"Project Context",
	}

	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected project context: %q", check)
		}
	}
}

func TestBuildTriagePromptWithContext_GoalInfo(t *testing.T) {
	task := &models.Task{
		Title:       "Optimize query performance",
		Type:        models.TaskTypeCoding,
		Priority:    50,
		Description: "Reduce API latency",
		ProjectID:   "api-service",
		GoalID:      "perf-goal",
	}

	project := &models.Project{
		ID:     "api-service",
		Name:   "API Service",
		GoalID: "perf-goal",
	}

	goal := &models.Goal{
		ID:          "perf-goal",
		Title:       "Improve System Performance",
		Description: "Reduce p99 latency below 100ms",
		Priorities:  []string{"database optimization", "caching strategy"},
	}

	prompt := buildTriagePromptWithContext(task, project, goal)

	checks := []string{
		"Improve System Performance",
		"Reduce p99 latency below 100ms",
		"database optimization, caching strategy",
		"Goal Context",
	}

	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected goal context: %q", check)
		}
	}
}

func TestBuildTriagePromptWithContext_NoContext(t *testing.T) {
	task := &models.Task{
		Title:       "Fix bug",
		Type:        models.TaskTypeBugfix,
		Priority:    50,
		Description: "Fix the thing",
	}

	prompt := buildTriagePromptWithContext(task, nil, nil)

	// Should still work without context
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}

	// Should not contain context sections
	forbidden := []string{"Project Context", "Goal Context"}
	for _, check := range forbidden {
		if contains(prompt, check) {
			t.Errorf("prompt should NOT contain context section when no context provided: %q", check)
		}
	}
}

func TestBuildTriagePrompt_NoProjectName(t *testing.T) {
	task := &models.Task{
		Title:       "Add feature",
		Type:        models.TaskTypeCoding,
		Priority:    50,
		Description: "Add a new feature",
	}

	prompt := buildTriagePrompt(task)
	// Just verify it doesn't crash and produces output
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
}

func TestBuildTriagePromptWithContext_FullContext(t *testing.T) {
	task := &models.Task{
		Title:       "Implement rate limiting",
		Type:        models.TaskTypeCoding,
		Priority:    50,
		Description: "Add rate limiting to prevent abuse",
		ProjectID:   "api-service",
		GoalID:      "security-goal",
		Tags:        []string{"security", "performance"},
	}

	project := &models.Project{
		ID:          "api-service",
		Name:        "API Service",
		Description: "Production REST API serving 10M requests/day",
		Type:        models.ProjectTypeService,
		TechStack:   []string{"Go", "Redis", "PostgreSQL"},
		GoalID:      "security-goal",
	}

	goal := &models.Goal{
		ID:          "security-goal",
		Title:       "Enhance Platform Security",
		Description: "Implement security best practices across all services",
		Priorities:  []string{"authentication", "rate limiting", "input validation"},
	}

	prompt := buildTriagePromptWithContext(task, project, goal)

	// Verify all context is included
	checks := []string{
		// Task info
		"Implement rate limiting",
		"Add rate limiting to prevent abuse",
		"security, performance",
		// Project context
		"Project Context",
		"API Service",
		"Production REST API serving 10M requests/day",
		"SERVICE",
		"Go, Redis, PostgreSQL",
		// Goal context
		"Goal Context",
		"Enhance Platform Security",
		"Implement security best practices across all services",
		"authentication, rate limiting, input validation",
		// Analysis instructions should reference context
		"How this aligns with the project's stated goals",
		"How this relates to the current goal priorities",
	}

	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected context: %q", check)
		}
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
