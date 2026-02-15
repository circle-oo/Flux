package server

import (
	"context"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"
)

const logRingBufferSize = 500

// LogEntry is a structured log entry for broadcasting.
type LogEntry struct {
	Time  string            `json:"time"`
	Level string            `json:"level"`
	Msg   string            `json:"msg"`
	Attrs map[string]any    `json:"attrs"`
}

// LogBroadcastHandler wraps an slog.Handler and broadcasts log entries to WebSocket clients.
type LogBroadcastHandler struct {
	inner  slog.Handler
	hub    *WebSocketHub
	buf    []LogEntry
	bufIdx int
	bufFull bool
	mu     sync.Mutex
	attrs  []slog.Attr
	group  string
}

// NewLogBroadcastHandler creates a handler that tees to inner and broadcasts via hub.
func NewLogBroadcastHandler(inner slog.Handler, hub *WebSocketHub) *LogBroadcastHandler {
	return &LogBroadcastHandler{
		inner: inner,
		hub:   hub,
		buf:   make([]LogEntry, logRingBufferSize),
	}
}

func (h *LogBroadcastHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *LogBroadcastHandler) Handle(ctx context.Context, r slog.Record) error {
	// Infer component from call site and inject into the record for inner handler.
	component := inferComponent(r.PC)
	r.AddAttrs(slog.String("component", component))

	// Always forward to the inner handler first.
	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}

	// Build the log entry for broadcasting.
	attrs := make(map[string]any)

	// Collect pre-configured attrs from WithAttrs.
	for _, a := range h.attrs {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs[key] = a.Value.Any()
	}

	// Collect attrs from the record itself.
	r.Attrs(func(a slog.Attr) bool {
		key := a.Key
		if h.group != "" {
			key = h.group + "." + key
		}
		attrs[key] = a.Value.Any()
		return true
	})

	entry := LogEntry{
		Time:  r.Time.Format(time.RFC3339Nano),
		Level: r.Level.String(),
		Msg:   r.Message,
		Attrs: attrs,
	}

	// Store in ring buffer.
	h.mu.Lock()
	h.buf[h.bufIdx] = entry
	h.bufIdx++
	if h.bufIdx >= logRingBufferSize {
		h.bufIdx = 0
		h.bufFull = true
	}
	h.mu.Unlock()

	// Broadcast to WebSocket clients.
	h.hub.Broadcast(Event{
		Type: EventLogEntry,
		Data: entry,
	})

	return nil
}

func (h *LogBroadcastHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &LogBroadcastHandler{
		inner: h.inner.WithAttrs(attrs),
		hub:   h.hub,
		buf:   h.buf,
		mu:    h.mu,
		attrs: newAttrs,
		group: h.group,
	}
}

func (h *LogBroadcastHandler) WithGroup(name string) slog.Handler {
	g := h.group
	if g != "" {
		g += "." + name
	} else {
		g = name
	}
	return &LogBroadcastHandler{
		inner: h.inner.WithGroup(name),
		hub:   h.hub,
		buf:   h.buf,
		mu:    h.mu,
		attrs: h.attrs,
		group: g,
	}
}

// inferComponent extracts a component name from the program counter of the log call site.
// e.g. "github.com/circle-oo/flux/internal/executor.(*Executor).Run" → "executor"
func inferComponent(pc uintptr) string {
	if pc == 0 {
		return "unknown"
	}
	fs := runtime.CallersFrames([]uintptr{pc})
	f, _ := fs.Next()
	if f.Function == "" {
		return "unknown"
	}

	// Map known package paths to short component names.
	switch {
	case strings.Contains(f.Function, "/executor"):
		return "executor"
	case strings.Contains(f.Function, "/manager"):
		return "manager"
	case strings.Contains(f.Function, "/orchestrator"):
		return "orchestrator"
	case strings.Contains(f.Function, "/server"):
		return "server"
	case strings.Contains(f.Function, "/shutdown"):
		return "shutdown"
	case strings.Contains(f.Function, "/github"):
		return "github"
	case strings.Contains(f.Function, "/notifier"):
		return "notifier"
	case strings.Contains(f.Function, "cmd/flux"):
		return "main"
	default:
		// Extract last package segment as fallback.
		parts := strings.Split(f.Function, "/")
		last := parts[len(parts)-1]
		if idx := strings.Index(last, "."); idx > 0 {
			return last[:idx]
		}
		return last
	}
}

// GetRecentLogs returns the buffered log entries in chronological order.
func (h *LogBroadcastHandler) GetRecentLogs() []LogEntry {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.bufFull {
		result := make([]LogEntry, h.bufIdx)
		copy(result, h.buf[:h.bufIdx])
		return result
	}

	result := make([]LogEntry, logRingBufferSize)
	// Oldest entries start at bufIdx (it wrapped around).
	copy(result, h.buf[h.bufIdx:])
	copy(result[logRingBufferSize-h.bufIdx:], h.buf[:h.bufIdx])
	return result
}
