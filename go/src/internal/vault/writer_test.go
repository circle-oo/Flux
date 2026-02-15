package vault

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWriter_Create(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)
	defer w.Close()

	// Create new file should succeed
	err := w.Write("test.md", "# Test\n", ModeCreate)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(filepath.Join(tmpDir, "test.md"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "# Test\n" {
		t.Errorf("unexpected content: %s", content)
	}

	// Create existing file should fail
	err = w.Write("test.md", "# Test 2\n", ModeCreate)
	if err == nil {
		t.Error("expected error when creating existing file")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWriter_Append(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)
	defer w.Close()

	// Append to new file
	err := w.Write("append.md", "Line 1\n", ModeAppend)
	if err != nil {
		t.Fatalf("failed to append to new file: %v", err)
	}

	// Append to existing file
	err = w.Write("append.md", "Line 2\n", ModeAppend)
	if err != nil {
		t.Fatalf("failed to append to existing file: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(filepath.Join(tmpDir, "append.md"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	expected := "Line 1\nLine 2\n"
	if string(content) != expected {
		t.Errorf("unexpected content: got %q, want %q", content, expected)
	}
}

func TestWriter_Replace(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)
	defer w.Close()

	// Write initial content
	err := w.Write("replace.md", "Original\n", ModeReplace)
	if err != nil {
		t.Fatalf("failed to write initial content: %v", err)
	}

	// Replace content
	err = w.Write("replace.md", "Replaced\n", ModeReplace)
	if err != nil {
		t.Fatalf("failed to replace content: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(filepath.Join(tmpDir, "replace.md"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(content) != "Replaced\n" {
		t.Errorf("unexpected content: %s", content)
	}
}

func TestWriter_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)
	defer w.Close()

	// Write to nested path
	err := w.Write("projects/flux/notes.md", "# Notes\n", ModeCreate)
	if err != nil {
		t.Fatalf("failed to write to nested path: %v", err)
	}

	// Verify directory was created
	fullPath := filepath.Join(tmpDir, "projects/flux/notes.md")
	if _, err := os.Stat(fullPath); err != nil {
		t.Errorf("file not created at expected path: %v", err)
	}
}

func TestWriter_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	// Create writer with buffer size 1
	w := &Writer{
		basePath: tmpDir,
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

func TestWriter_CloseWaitsForWrites(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)

	// Queue multiple writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := w.Write(filepath.Join("file", string(rune(i+'0'))+".md"), "content\n", ModeCreate)
			if err != nil {
				t.Errorf("write %d failed: %v", i, err)
			}
		}(i)
	}

	// Wait for all writes to be queued
	wg.Wait()

	// Close should wait for all writes to complete
	w.Close()

	// Verify all files were written
	entries, err := os.ReadDir(filepath.Join(tmpDir, "file"))
	if err != nil {
		t.Fatalf("failed to read directory: %v", err)
	}
	if len(entries) != 10 {
		t.Errorf("expected 10 files, got %d", len(entries))
	}
}

func TestWriter_ConcurrentWritesSequential(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)
	defer w.Close()

	// Track write order
	var mu sync.Mutex
	var order []int

	// Launch concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			err := w.Write("concurrent.md", string(rune(n+'0')), ModeAppend)
			if err != nil {
				t.Errorf("write %d failed: %v", n, err)
			}
			mu.Lock()
			order = append(order, n)
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify file has all writes (no corruption)
	content, err := os.ReadFile(filepath.Join(tmpDir, "concurrent.md"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(content) != 20 {
		t.Errorf("expected 20 bytes, got %d", len(content))
	}

	// Verify all writes were processed
	if len(order) != 20 {
		t.Errorf("expected 20 writes, got %d", len(order))
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

	// Verify key sections exist
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

func TestWriteAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	w := NewWriter(tmpDir)
	w.Close()

	// Write after close should panic with "send on closed channel"
	done := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic is expected - writing to closed channel
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
