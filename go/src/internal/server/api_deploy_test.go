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

func TestHandleDeploy_NoUpdater(t *testing.T) {
	srv, _ := setupTestServer(t)

	// POST /api/system/deploy without an updater configured
	// Should fall back to legacy restart behavior
	rr := doAuthRequest(t, srv, "POST", "/api/system/deploy", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)

	// Should have status field
	status, ok := resp["status"].(string)
	if !ok {
		t.Error("expected 'status' field in response")
	}

	// When no updater configured, should say "restarting"
	if status != "restarting" && status != "deploying" {
		t.Errorf("expected status 'restarting' or 'deploying', got %q", status)
	}

	// Should have message field
	if _, ok := resp["message"]; !ok {
		t.Error("expected 'message' field in response")
	}

	t.Logf("deploy response: %+v", resp)
}

func TestHandleDeploy_WithUpdater(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a mock updater
	mockUpdater := updater.New(config.AutoUpdateConfig{
		Enabled:       true,
		Branch:        "main",
		CheckInterval: 5 * time.Minute,
	}, ".")
	srv.SetUpdater(mockUpdater)

	// POST /api/system/deploy
	rr := doAuthRequest(t, srv, "POST", "/api/system/deploy", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)

	// Should have status field
	status, ok := resp["status"].(string)
	if !ok {
		t.Error("expected 'status' field in response")
	}

	// When updater is configured, should say "deploying"
	if status != "deploying" {
		t.Errorf("expected status 'deploying', got %q", status)
	}

	// Should have message field
	if _, ok := resp["message"]; !ok {
		t.Error("expected 'message' field in response")
	}

	t.Logf("deploy response: %+v", resp)
}
