package executor

import (
	"strings"
	"testing"

	"github.com/circle-oo/flux/internal/apiclient"
)

func TestParseDecomposition(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantLen  int
		wantTask *DecomposedTask // first task for validation
	}{
		{
			name: "valid decomposition with surrounding text",
			input: `Here's the plan for this large task:

{"decompose": true, "subtasks": [
  {"title": "Setup database", "description": "Initialize PostgreSQL schema"},
  {"title": "Create API endpoints", "description": "Implement REST handlers"}
]}

This should be broken into subtasks.`,
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Setup database",
				Description: "Initialize PostgreSQL schema",
			},
		},
		{
			name: "clean JSON without surrounding text",
			input: `{"decompose": true, "subtasks": [
				{"title": "Task 1", "description": "Implement the first task with proper validation"},
				{"title": "Task 2", "description": "Complete the second task requirements"},
				{"title": "Task 3", "description": "Finalize the third task implementation"}
			]}`,
			wantNil: false,
			wantLen: 3,
			wantTask: &DecomposedTask{
				Title:       "Task 1",
				Description: "Implement the first task with proper validation",
			},
		},
		{
			name:    "normal code output without decomposition",
			input:   `func main() { fmt.Println("Hello") }`,
			wantNil: true,
		},
		{
			name:    "decompose false",
			input:   `{"decompose": false, "subtasks": [{"title": "Test", "description": "Test"}]}`,
			wantNil: true,
		},
		{
			name:    "malformed JSON",
			input:   `{"decompose": true, "subtasks": [{"title": "Test", "description": `,
			wantNil: true,
		},
		{
			name: "max 5 subtasks enforced",
			input: `{"decompose": true, "subtasks": [
				{"title": "Task 1", "description": "Implement first component with validation"},
				{"title": "Task 2", "description": "Complete second component requirements"},
				{"title": "Task 3", "description": "Build third component integration"},
				{"title": "Task 4", "description": "Add fourth component testing"},
				{"title": "Task 5", "description": "Finalize fifth component deployment"},
				{"title": "Task 6", "description": "Extra sixth component (will be truncated)"},
				{"title": "Task 7", "description": "Extra seventh component (will be truncated)"}
			]}`,
			wantNil: false,
			wantLen: 5,
			wantTask: &DecomposedTask{
				Title:       "Task 1",
				Description: "Implement first component with validation",
			},
		},
		{
			name:    "empty subtasks array",
			input:   `{"decompose": true, "subtasks": []}`,
			wantNil: true,
		},
		{
			name: "whitespace trimming",
			input: `{"decompose": true, "subtasks": [
				{"title": "  Task with spaces  ", "description": "  Description with spaces and proper length  "},
				{"title": "Second task", "description": "Second task description with enough content"}
			]}`,
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Task with spaces",
				Description: "Description with spaces and proper length",
			},
		},
		{
			name: "JSON with extra fields",
			input: `{"decompose": true, "extraField": "ignored", "subtasks": [
				{"title": "Task", "description": "Complete task with proper description length", "anotherField": 123},
				{"title": "Second Task", "description": "Another task with sufficient detail"}
			]}`,
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Task",
				Description: "Complete task with proper description length",
			},
		},
		{
			name: "JSON with whitespace in decompose key",
			input: `{ "decompose" : true , "subtasks" : [
				{"title": "Implement backend service", "description": "Complete the task with proper description"},
				{"title": "Build frontend component", "description": "Implement frontend task with details"}
			]}`,
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Implement backend service",
				Description: "Complete the task with proper description",
			},
		},
		{
			name:    "no decompose key at all",
			input:   `{"subtasks": [{"title": "Task", "description": "Desc"}]}`,
			wantNil: true,
		},
		{
			name: "nested JSON in description",
			input: `{"decompose": true, "subtasks": [
				{"title": "Complex", "description": "Contains {\"nested\": \"json\"} data structure with validation"},
				{"title": "Simple", "description": "Handle simple case without nested structures"}
			]}`,
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Complex",
				Description: `Contains {"nested": "json"} data structure with validation`,
			},
		},
		{
			name: "pretty-printed JSON in markdown code block",
			input: "I've analyzed the task and here's the decomposition:\n\n```json\n{\n  \"decompose\": true,\n  \"subtasks\": [\n    {\n      \"title\": \"Setup backend API\",\n      \"description\": \"Create REST endpoints for user management with validation\"\n    },\n    {\n      \"title\": \"Build frontend components\",\n      \"description\": \"Implement React UI components for the dashboard\"\n    }\n  ]\n}\n```\n\nThis breaks the work into independent streams.",
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Setup backend API",
				Description: "Create REST endpoints for user management with validation",
			},
		},
		{
			name: "compact JSON in markdown code block",
			input: "Here's the plan:\n\n```json\n{\"decompose\": true, \"subtasks\": [{\"title\": \"Backend work\", \"description\": \"Implement API endpoints with proper validation\"}, {\"title\": \"Frontend work\", \"description\": \"Build React components for the dashboard UI\"}]}\n```\n\nDone.",
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Backend work",
				Description: "Implement API endpoints with proper validation",
			},
		},
		{
			name: "JSON in plain markdown code block without json tag",
			input: "Decomposition:\n\n```\n{\n  \"decompose\": true,\n  \"subtasks\": [\n    {\n      \"title\": \"Database migration\",\n      \"description\": \"Create and run database schema migrations\"\n    },\n    {\n      \"title\": \"API layer changes\",\n      \"description\": \"Update REST endpoints for new schema\"\n    }\n  ]\n}\n```",
			wantNil: false,
			wantLen: 2,
			wantTask: &DecomposedTask{
				Title:       "Database migration",
				Description: "Create and run database schema migrations",
			},
		},
		{
			name:    "code block with non-decomposition JSON",
			input:   "Here's a config example:\n\n```json\n{\"name\": \"test\", \"version\": \"1.0\"}\n```",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDecomposition(tt.input)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if !result.Decompose {
				t.Error("expected Decompose to be true")
			}

			if len(result.Subtasks) != tt.wantLen {
				t.Errorf("expected %d subtasks, got %d", tt.wantLen, len(result.Subtasks))
			}

			if tt.wantTask != nil && len(result.Subtasks) > 0 {
				got := result.Subtasks[0]
				if got.Title != tt.wantTask.Title {
					t.Errorf("expected title %q, got %q", tt.wantTask.Title, got.Title)
				}
				if got.Description != tt.wantTask.Description {
					t.Errorf("expected description %q, got %q", tt.wantTask.Description, got.Description)
				}
			}
		})
	}
}

func TestToSubtaskRequests(t *testing.T) {
	tests := []struct {
		name  string
		input *Decomposition
		want  []apiclient.SubtaskRequest
	}{
		{
			name:  "nil decomposition",
			input: nil,
			want:  nil,
		},
		{
			name: "valid decomposition",
			input: &Decomposition{
				Decompose: true,
				Subtasks: []DecomposedTask{
					{Title: "Task 1", Description: "First task"},
					{Title: "Task 2", Description: "Second task"},
				},
			},
			want: []apiclient.SubtaskRequest{
				{Title: "Task 1", Description: "First task"},
				{Title: "Task 2", Description: "Second task"},
			},
		},
		{
			name: "empty subtasks",
			input: &Decomposition{
				Decompose: true,
				Subtasks:  []DecomposedTask{},
			},
			want: []apiclient.SubtaskRequest{},
		},
		{
			name: "single subtask",
			input: &Decomposition{
				Decompose: true,
				Subtasks: []DecomposedTask{
					{Title: "Only task", Description: "Single task description"},
				},
			},
			want: []apiclient.SubtaskRequest{
				{Title: "Only task", Description: "Single task description"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToSubtaskRequests(tt.input)

			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("expected %d requests, got %d", len(tt.want), len(got))
			}

			for i := range tt.want {
				if got[i].Title != tt.want[i].Title {
					t.Errorf("request[%d].Title: expected %q, got %q", i, tt.want[i].Title, got[i].Title)
				}
				if got[i].Description != tt.want[i].Description {
					t.Errorf("request[%d].Description: expected %q, got %q", i, tt.want[i].Description, got[i].Description)
				}
			}
		})
	}
}

func TestValidateSubtasks(t *testing.T) {
	tests := []struct {
		name        string
		subtasks    []DecomposedTask
		wantErr     bool
		errContains string
	}{
		{
			name: "valid subtasks",
			subtasks: []DecomposedTask{
				{Title: "Implement backend API", Description: "Create REST endpoints for user management"},
				{Title: "Create frontend UI", Description: "Build user interface components with React"},
			},
			wantErr: false,
		},
		{
			name: "empty title",
			subtasks: []DecomposedTask{
				{Title: "", Description: "Some description"},
			},
			wantErr:     true,
			errContains: "empty title",
		},
		{
			name: "vague title",
			subtasks: []DecomposedTask{
				{Title: "Other work", Description: "Do some other work here"},
			},
			wantErr:     true,
			errContains: "vague title",
		},
		{
			name: "empty description",
			subtasks: []DecomposedTask{
				{Title: "Valid title", Description: ""},
			},
			wantErr:     true,
			errContains: "empty description",
		},
		{
			name: "too short description",
			subtasks: []DecomposedTask{
				{Title: "Valid title", Description: "Short"},
			},
			wantErr:     true,
			errContains: "too short",
		},
		{
			name: "meta-task: tests",
			subtasks: []DecomposedTask{
				{Title: "Write tests", Description: "Add unit tests for the new functionality"},
			},
			wantErr:     true,
			errContains: "meta-task",
		},
		{
			name: "meta-task: documentation",
			subtasks: []DecomposedTask{
				{Title: "Add documentation", Description: "Document the new API endpoints"},
			},
			wantErr:     true,
			errContains: "meta-task",
		},
		{
			name: "duplicate titles",
			subtasks: []DecomposedTask{
				{Title: "Implement API", Description: "Create REST endpoints for users"},
				{Title: "Implement API", Description: "Create REST endpoints for products"},
			},
			wantErr:     true,
			errContains: "duplicate title",
		},
		{
			name: "duplicate titles case insensitive",
			subtasks: []DecomposedTask{
				{Title: "Implement API", Description: "Create REST endpoints for users"},
				{Title: "implement api", Description: "Create REST endpoints for products"},
			},
			wantErr:     true,
			errContains: "duplicate title",
		},
		{
			name: "vague title with multiple words is OK",
			subtasks: []DecomposedTask{
				{Title: "Implement additional validation rules", Description: "Add validation for email and password fields"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSubtasks(tt.subtasks)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSubtasks() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ValidateSubtasks() error = %v, want to contain %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSubtasks() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestParseDecomposition_WithValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		reason  string
	}{
		{
			name: "valid decomposition passes validation",
			input: `{"decompose": true, "subtasks": [
				{"title": "Implement backend API", "description": "Create REST endpoints for user management"},
				{"title": "Create frontend UI", "description": "Build user interface components with React"}
			]}`,
			wantNil: false,
		},
		{
			name: "only one subtask rejected",
			input: `{"decompose": true, "subtasks": [
				{"title": "Do everything", "description": "Implement the entire feature in one go"}
			]}`,
			wantNil: true,
			reason:  "less than 2 subtasks",
		},
		{
			name: "vague titles rejected",
			input: `{"decompose": true, "subtasks": [
				{"title": "Other work", "description": "Do some other work here"},
				{"title": "More stuff", "description": "Handle the remaining tasks"}
			]}`,
			wantNil: true,
			reason:  "vague title validation",
		},
		{
			name: "meta-tasks rejected",
			input: `{"decompose": true, "subtasks": [
				{"title": "Implement feature", "description": "Create the main feature logic"},
				{"title": "Write tests", "description": "Add unit tests for the feature"}
			]}`,
			wantNil: true,
			reason:  "meta-task validation",
		},
		{
			name: "short descriptions rejected",
			input: `{"decompose": true, "subtasks": [
				{"title": "Backend API", "description": "API stuff"},
				{"title": "Frontend UI", "description": "UI work"}
			]}`,
			wantNil: true,
			reason:  "description too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseDecomposition(tt.input)

			if tt.wantNil && result != nil {
				t.Errorf("ParseDecomposition() expected nil due to %s, got %+v", tt.reason, result)
			}

			if !tt.wantNil && result == nil {
				t.Errorf("ParseDecomposition() expected valid result, got nil")
			}
		})
	}
}

// TestParseDecomposition_EndToEnd simulates the full pipeline:
// Claude CLI JSON stdout → ParseResponse → ParseDecomposition
func TestParseDecomposition_EndToEnd(t *testing.T) {
	tests := []struct {
		name       string
		resultText string // the "result" field value from Claude CLI JSON output
		wantNil    bool
		wantLen    int
	}{
		{
			name: "pretty-printed JSON in code block (typical Claude output)",
			resultText: "I've analyzed the task and determined it should be decomposed.\n\n```json\n{\n  \"decompose\": true,\n  \"subtasks\": [\n    {\n      \"title\": \"Implement database layer\",\n      \"description\": \"Create SQLite schema and queries for the new feature\"\n    },\n    {\n      \"title\": \"Build REST API endpoints\",\n      \"description\": \"Implement HTTP handlers with proper validation and error handling\"\n    },\n    {\n      \"title\": \"Create frontend components\",\n      \"description\": \"Build React components for the user interface with state management\"\n    }\n  ]\n}\n```\n\nThis task spans three independent subsystems.",
			wantNil: false,
			wantLen: 3,
		},
		{
			name:       "compact inline JSON (no code block)",
			resultText: "{\"decompose\": true, \"subtasks\": [{\"title\": \"Backend API\", \"description\": \"Create REST endpoints for user management with validation\"}, {\"title\": \"Frontend UI\", \"description\": \"Build React components for the dashboard interface\"}]}",
			wantNil:    false,
			wantLen:    2,
		},
		{
			name:       "normal task result (no decomposition)",
			resultText: "## Task Summary\n\n### What Was Accomplished\nFixed the login bug.\n\n### Key Changes\n- File: auth.go - Fixed token validation",
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate ParseResponse having already extracted the result text
			result := ParseDecomposition(tt.resultText)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result, got nil — decomposition was not extracted")
			}

			if !result.Decompose {
				t.Error("expected Decompose to be true")
			}

			if len(result.Subtasks) != tt.wantLen {
				t.Errorf("expected %d subtasks, got %d", tt.wantLen, len(result.Subtasks))
			}

			// Verify all subtasks have non-empty fields
			for i, st := range result.Subtasks {
				if st.Title == "" {
					t.Errorf("subtask[%d] has empty title", i)
				}
				if st.Description == "" {
					t.Errorf("subtask[%d] has empty description", i)
				}
			}
		})
	}
}

func TestExtractJSONFromCodeBlock(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int // 0 means expect empty string
	}{
		{
			name:    "json code block with pretty-printed content",
			input:   "Text before\n\n```json\n{\n  \"decompose\": true,\n  \"subtasks\": []\n}\n```\n\nText after",
			wantLen: 1, // non-empty
		},
		{
			name:    "plain code block with JSON",
			input:   "```\n{\"decompose\": true, \"subtasks\": []}\n```",
			wantLen: 1,
		},
		{
			name:    "no code block",
			input:   "Just regular text without any code blocks",
			wantLen: 0,
		},
		{
			name:    "code block without decompose",
			input:   "```json\n{\"name\": \"test\"}\n```",
			wantLen: 0,
		},
		{
			name:    "code block with non-JSON content",
			input:   "```json\nnot actually json with decompose\n```",
			wantLen: 0,
		},
		{
			name:    "multiple code blocks picks the decompose one",
			input:   "```json\n{\"config\": true}\n```\n\n```json\n{\"decompose\": true, \"subtasks\": []}\n```",
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSONFromCodeBlock(tt.input)
			if tt.wantLen == 0 && result != "" {
				t.Errorf("expected empty string, got %q", result)
			}
			if tt.wantLen > 0 && result == "" {
				t.Errorf("expected non-empty string, got empty")
			}
		})
	}
}

func TestFindMatchingBrace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		start int
		want  int // 0 means not found
	}{
		{
			name:  "simple object",
			input: `{"key": "value"}`,
			start: 0,
			want:  16,
		},
		{
			name:  "nested objects",
			input: `{"a": {"b": "c"}}`,
			start: 0,
			want:  17,
		},
		{
			name:  "with escaped quotes",
			input: `{"key": "val\"ue"}`,
			start: 0,
			want:  18,
		},
		{
			name:  "unclosed brace",
			input: `{"key": "value"`,
			start: 0,
			want:  0,
		},
		{
			name:  "braces inside strings ignored",
			input: `{"key": "{}{}"}`,
			start: 0,
			want:  15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMatchingBrace(tt.input, tt.start)
			if got != tt.want {
				t.Errorf("findMatchingBrace(%q, %d) = %d, want %d", tt.input, tt.start, got, tt.want)
			}
		})
	}
}
