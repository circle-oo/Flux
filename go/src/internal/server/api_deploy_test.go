package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/updater"
)

func TestHandleDeployStatus_NoUpdater(t *testing.T) {
	srv, _ := setupTestServer(t)

	// GET /api/system/deploy/status without an updater configured
	rr := doAuthRequest(t, srv, "GET", "/api/system/deploy/status", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)

	// Should have version field
	if _, ok := resp["version"]; !ok {
		t.Error("expected 'version' field in response")
	}

	// Should have updater field
	updaterData, ok := resp["updater"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'updater' object in response")
	}

	// When no updater is configured, should still have local_commit
	// (if we're in a git repository)
	if enabled, ok := updaterData["enabled"].(bool); ok && enabled {
		t.Error("expected updater to be disabled when no updater is configured")
	}

	if state, ok := updaterData["state"].(string); ok && state != "disabled" {
		t.Errorf("expected state 'disabled', got %q", state)
	}

	// local_commit may or may not exist depending on whether we're in a git repo
	// Just verify the response structure is valid
	t.Logf("updater response: %+v", updaterData)
}

func TestHandleDeployStatus_WithUpdater(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a mock updater
	mockUpdater := updater.New(config.AutoUpdateConfig{
		Enabled:       true,
		Branch:        "main",
		CheckInterval: 5 * time.Minute,
	}, ".")
	srv.SetUpdater(mockUpdater)

	// GET /api/system/deploy/status
	rr := doAuthRequest(t, srv, "GET", "/api/system/deploy/status", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)

	updaterData, ok := resp["updater"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'updater' object in response")
	}

	// When updater is configured, should have enabled=true
	if enabled, ok := updaterData["enabled"].(bool); !ok || !enabled {
		t.Error("expected updater to be enabled")
	}
}
