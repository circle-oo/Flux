package vault

import (
	"fmt"
	"sync"
	"time"

	"github.com/circle-oo/flux/internal/notesmd"
)

// WriteMode defines how content should be written to vault
type WriteMode int

const (
	ModeCreate  WriteMode = iota // Create new file, error if exists
	ModeAppend                   // Append to existing file or create new
	ModeReplace                  // Replace existing file or create new
)

// WriteRequest represents a queued write operation
type WriteRequest struct {
	Path    string
	Content string
	Mode    WriteMode
	Done    chan error
}

// Writer handles asynchronous writes to the vault via notesmd-cli
type Writer struct {
	client   *notesmd.Client
	requests chan WriteRequest
	wg       sync.WaitGroup
	done     chan struct{}
}

// NewWriter creates a new vault writer backed by notesmd-cli.
func NewWriter(client *notesmd.Client) *Writer {
	w := &Writer{
		client:   client,
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
		err := w.execute(req)
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

// execute performs the write operation via notesmd-cli
func (w *Writer) execute(req WriteRequest) error {
	// Strip .md extension from path — notesmd-cli adds it automatically
	path := req.Path
	if len(path) > 3 && path[len(path)-3:] == ".md" {
		path = path[:len(path)-3]
	}

	switch req.Mode {
	case ModeCreate:
		return w.client.Create(path, req.Content)
	case ModeAppend:
		return w.client.Append(path, req.Content)
	case ModeReplace:
		return w.client.Overwrite(path, req.Content)
	default:
		return fmt.Errorf("unknown write mode: %d", req.Mode)
	}
}
