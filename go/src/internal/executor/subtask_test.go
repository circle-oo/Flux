package executor

import (
	"testing"
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
				{"title": "Task 1", "description": "First task"},
				{"title": "Task 2", "description": "Second task"},
				{"title": "Task 3", "description": "Third task"}
			]}`,
			wantNil: false,
			wantLen: 3,
			wantTask: &DecomposedTask{
				Title:       "Task 1",
				Description: "First task",
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
				{"title": "Task 1", "description": "First"},
				{"title": "Task 2", "description": "Second"},
				{"title": "Task 3", "description": "Third"},
				{"title": "Task 4", "description": "Fourth"},
				{"title": "Task 5", "description": "Fifth"},
				{"title": "Task 6", "description": "Sixth"},
				{"title": "Task 7", "description": "Seventh"}
			]}`,
			wantNil: false,
			wantLen: 5,
			wantTask: &DecomposedTask{
				Title:       "Task 1",
				Description: "First",
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
				{"title": "  Task with spaces  ", "description": "  Description with spaces  "}
			]}`,
			wantNil: false,
			wantLen: 1,
			wantTask: &DecomposedTask{
				Title:       "Task with spaces",
				Description: "Description with spaces",
			},
		},
		{
			name: "JSON with extra fields",
			input: `{"decompose": true, "extraField": "ignored", "subtasks": [
				{"title": "Task", "description": "Desc", "anotherField": 123}
			]}`,
			wantNil: false,
			wantLen: 1,
			wantTask: &DecomposedTask{
				Title:       "Task",
				Description: "Desc",
			},
		},
		{
			name: "JSON with whitespace in decompose key",
			input: `{ "decompose" : true , "subtasks" : [
				{"title": "Task", "description": "Desc"}
			]}`,
			wantNil: false,
			wantLen: 1,
			wantTask: &DecomposedTask{
				Title:       "Task",
				Description: "Desc",
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
				{"title": "Complex", "description": "Contains {\"nested\": \"json\"}"}
			]}`,
			wantNil: false,
			wantLen: 1,
			wantTask: &DecomposedTask{
				Title:       "Complex",
				Description: `Contains {"nested": "json"}`,
			},
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
		want  []SubtaskRequest
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
			want: []SubtaskRequest{
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
			want: []SubtaskRequest{},
		},
		{
			name: "single subtask",
			input: &Decomposition{
				Decompose: true,
				Subtasks: []DecomposedTask{
					{Title: "Only task", Description: "Single task description"},
				},
			},
			want: []SubtaskRequest{
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
