package server

import (
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
)

func TestAuthManager_SessionExpiry(t *testing.T) {
	am := NewAuthManager(config.AuthConfig{
		Password:      "testpass",
		SessionExpiry: "1m",
	})

	token := am.CreateSession()

	am.mu.Lock()
	am.sessions[token] = time.Now().Add(-2 * time.Minute)
	am.mu.Unlock()

	if am.ValidateSession(token) {
		t.Fatalf("expected expired session to be invalid")
	}

	am.mu.RLock()
	_, exists := am.sessions[token]
	am.mu.RUnlock()
	if exists {
		t.Fatalf("expected expired session to be removed")
	}
}

func TestAuthManager_CleanupStaleData(t *testing.T) {
	am := NewAuthManager(config.AuthConfig{
		Password:      "testpass",
		SessionExpiry: "1h",
	})

	now := time.Now()
	am.sessions["fresh"] = now.Add(-10 * time.Minute)
	am.sessions["stale"] = now.Add(-2 * time.Hour)
	am.loginAttempts["old-ip"] = []time.Time{now.Add(-2 * time.Hour)}
	am.loginAttempts["mixed-ip"] = []time.Time{now.Add(-2 * time.Hour), now.Add(-10 * time.Minute)}

	am.cleanupStaleData(now)

	am.mu.RLock()
	if _, ok := am.sessions["stale"]; ok {
		t.Fatalf("expected stale session to be cleaned up")
	}
	if _, ok := am.sessions["fresh"]; !ok {
		t.Fatalf("expected fresh session to be preserved")
	}
	am.mu.RUnlock()

	am.loginAttemptsMu.Lock()
	if _, ok := am.loginAttempts["old-ip"]; ok {
		t.Fatalf("expected stale login-attempt entry to be cleaned up")
	}
	if len(am.loginAttempts["mixed-ip"]) != 1 {
		t.Fatalf("expected mixed-ip to retain only recent attempt, got %d", len(am.loginAttempts["mixed-ip"]))
	}
	am.loginAttemptsMu.Unlock()
}
