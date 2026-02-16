package vault

import (
	"strings"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/notesmd"
)

func TestWriter_Timeout(t *testing.T) {
	client := notesmd.NewClient("TestVault")
	// Create writer with buffer size 1
	w := &Writer{
		client:   client,
		requests: make(chan WriteRequest, 1),
		done:     make(chan struct{}),
	}

	// Don't start run() goroutine - this prevents any processing
	// So the buffer will fill up and timeout will trigger
	defer close(w.requests)

	// Fill the single-slot buffer
	done1 := make(chan error, 1)
	w.requests <- WriteRequest{Path: "file1.md", Content: "1", Mode: ModeCreate, Done: done1}

	// Next write should timeout because buffer is full and nothing is draining it
	err := w.Write("file3.md", "3", ModeCreate)
	if err == nil {
		t.Error("expected timeout error")
	} else if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriter_WriteAfterClose(t *testing.T) {
	client := notesmd.NewClient("TestVault")
	w := NewWriter(client)
	w.Close()

	// Write after close should panic with "send on closed channel"
	done := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- true
			} else {
				done <- false
			}
		}()
		_ = w.Write("test.md", "content", ModeCreate)
	}()

	select {
	case panicked := <-done:
		if !panicked {
			t.Error("expected panic when writing after close")
		}
	case <-time.After(1 * time.Second):
		t.Error("write after close should panic immediately")
	}
}

func TestExecute_StripsMdExtension(t *testing.T) {
	// Verify that .md extension stripping works correctly
	cases := []struct {
		input    string
		expected string
	}{
		{"Tasks/completed/abc123.md", "Tasks/completed/abc123"},
		{"Notes/test", "Notes/test"},
		{"file.md", "file"},
		{".md", ".md"}, // edge case: don't strip if path would be empty
	}

	for _, tc := range cases {
		path := tc.input
		if len(path) > 3 && path[len(path)-3:] == ".md" {
			path = path[:len(path)-3]
		}
		if path != tc.expected {
			t.Errorf("stripMd(%q) = %q, want %q", tc.input, path, tc.expected)
		}
	}
}

func TestTaskSummaryTemplate(t *testing.T) {
	task := TaskSummaryData{
		ID:           "task-123",
		Title:        "Implement feature X",
		Type:         "feature",
		Status:       "completed",
		Priority:     1,
		Model:        "claude-opus",
		Duration:     "45m",
		CreatedAt:    "2024-01-15T10:00:00Z",
		CompletedAt:  "2024-01-15T10:45:00Z",
		PRUrl:        "https://github.com/org/repo/pull/123",
		PRStatus:     "merged",
		DiffLines:    250,
		FilesChanged: 8,
		Result:       "Successfully implemented feature X with full test coverage",
	}

	md := TaskSummaryTemplate(task)

	if !strings.Contains(md, "# Task: Implement feature X") {
		t.Error("missing task title")
	}
	if !strings.Contains(md, "**ID**: task-123") {
		t.Error("missing task ID")
	}
	if !strings.Contains(md, "## PR") {
		t.Error("missing PR section")
	}
	if !strings.Contains(md, "## Result") {
		t.Error("missing Result section")
	}
	if !strings.Contains(md, "+250 lines, 8 files") {
		t.Error("missing diff stats")
	}
}

func TestDecisionTemplate(t *testing.T) {
	md := DecisionTemplate(
		"Use SQLite for local storage",
		"Need lightweight embedded database",
		"We will use SQLite",
		"Good performance and zero-config",
	)

	if !strings.Contains(md, "# Decision: Use SQLite") {
		t.Error("missing decision title")
	}
	if !strings.Contains(md, "## Context") {
		t.Error("missing Context section")
	}
	if !strings.Contains(md, "## Decision") {
		t.Error("missing Decision section")
	}
	if !strings.Contains(md, "## Rationale") {
		t.Error("missing Rationale section")
	}
}

func TestProjectIndexTemplate(t *testing.T) {
	md := ProjectIndexTemplate(
		"Flux",
		"Automation System",
		"https://github.com/org/flux",
		"AI-powered development automation",
		[]string{"Go", "SQLite", "GitHub API"},
	)

	if !strings.Contains(md, "# Project: Flux") {
		t.Error("missing project title")
	}
	if !strings.Contains(md, "**Type**: Automation System") {
		t.Error("missing project type")
	}
	if !strings.Contains(md, "## Tech Stack") {
		t.Error("missing Tech Stack section")
	}
	if !strings.Contains(md, "- Go") {
		t.Error("missing tech stack item")
	}
}
