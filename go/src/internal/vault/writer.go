package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WriteMode defines how content should be written to vault
type WriteMode int

const (
	ModeCreate WriteMode = iota // Create new file, error if exists
	ModeAppend                  // Append to existing file or create new
	ModeReplace                 // Replace existing file or create new
)

// WriteRequest represents a queued write operation
type WriteRequest struct {
	Path    string
	Content string
	Mode    WriteMode
	Done    chan error
}

// Writer handles asynchronous writes to the vault
type Writer struct {
	basePath string
	requests chan WriteRequest
	wg       sync.WaitGroup
	done     chan struct{}
}

// NewWriter creates a new vault writer with background processing
func NewWriter(basePath string) *Writer {
	w := &Writer{
		basePath: basePath,
		requests: make(chan WriteRequest, 100),
		done:     make(chan struct{}),
	}
	go w.run()
	return w
}

// run processes write requests sequentially in background
func (w *Writer) run() {
	for req := range w.requests {
		w.wg.Add(1)
		err := w.atomicWrite(req)
		req.Done <- err
		close(req.Done)
		w.wg.Done()
	}
}

// Write queues a write operation with timeout
func (w *Writer) Write(path, content string, mode WriteMode) error {
	done := make(chan error, 1)
	req := WriteRequest{
		Path:    path,
		Content: content,
		Mode:    mode,
		Done:    done,
	}

	select {
	case w.requests <- req:
		// Successfully queued
	case <-time.After(5 * time.Second):
		return fmt.Errorf("vault writer queue full, write timed out")
	}

	return <-done
}

// Close shuts down the writer and waits for pending writes
func (w *Writer) Close() {
	close(w.requests)
	w.wg.Wait()
	close(w.done)
}

// atomicWrite performs the actual write operation
func (w *Writer) atomicWrite(req WriteRequest) error {
	fullPath := filepath.Join(w.basePath, req.Path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	switch req.Mode {
	case ModeCreate:
		// Check if file exists
		if _, err := os.Stat(fullPath); err == nil {
			return fmt.Errorf("file already exists: %s", fullPath)
		}
		return os.WriteFile(fullPath, []byte(req.Content), 0644)

	case ModeAppend:
		f, err := os.OpenFile(fullPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open file for append: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString(req.Content); err != nil {
			return fmt.Errorf("failed to append content: %w", err)
		}
		return nil

	case ModeReplace:
		return os.WriteFile(fullPath, []byte(req.Content), 0644)

	default:
		return fmt.Errorf("unknown write mode: %d", req.Mode)
	}
}
