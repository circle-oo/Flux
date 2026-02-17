package server

import (
	"context"
	"testing"
)

func TestWebSocketHub_AddClientLimit(t *testing.T) {
	hub := NewWebSocketHub()

	cancels := make([]context.CancelFunc, 0, maxClients+1)
	t.Cleanup(func() {
		for _, cancel := range cancels {
			cancel()
		}
	})

	for i := 0; i < maxClients; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancels = append(cancels, cancel)
		client := &wsClient{ctx: ctx, cancel: cancel}
		if !hub.addClient(client) {
			t.Fatalf("expected client %d to be accepted", i)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancels = append(cancels, cancel)
	overflow := &wsClient{ctx: ctx, cancel: cancel}
	if hub.addClient(overflow) {
		t.Fatalf("expected client %d to be rejected at maxClients=%d", maxClients+1, maxClients)
	}
}
