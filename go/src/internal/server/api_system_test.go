package server

import (
	"testing"
	"time"
)

func TestBroadcastDeployStatus(t *testing.T) {
	srv := &Server{ws: NewWebSocketHub()}

	srv.broadcastDeployStatus("failed", "build failed", "compile error")

	select {
	case event := <-srv.ws.broadcast:
		if event.Type != EventDeployStatus {
			t.Fatalf("event type = %q, want %q", event.Type, EventDeployStatus)
		}

		payload, ok := event.Data.(map[string]any)
		if !ok {
			t.Fatalf("event payload type = %T, want map[string]any", event.Data)
		}
		if payload["status"] != "failed" {
			t.Fatalf("status = %v, want failed", payload["status"])
		}
		if payload["message"] != "build failed" {
			t.Fatalf("message = %v, want build failed", payload["message"])
		}
		if payload["detail"] != "compile error" {
			t.Fatalf("detail = %v, want compile error", payload["detail"])
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for deploy status event")
	}
}

func TestBroadcastDeployStatus_NoWebSocketHub(t *testing.T) {
	srv := &Server{}
	srv.broadcastDeployStatus("failed", "restart failed", "no ws")
}
