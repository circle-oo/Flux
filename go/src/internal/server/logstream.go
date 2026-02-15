package server

import (
	"context"
	"log/slog"
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
