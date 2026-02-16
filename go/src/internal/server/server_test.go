package server

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/manager"
	"github.com/circle-oo/flux/internal/testutil"
)

// setupTestServer creates a test server backed by an in-memory SQLite DB.
func setupTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()

	database := testutil.NewTestDB(t)

	cfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8080,
			Auth: config.AuthConfig{
				Enabled:  true,
				Password: "testpass",
			},
		},
		Subtask: config.SubtaskConfig{
			MaxDepth:   1,
			MaxPerTask: 5,
		},
	}

	webFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
	}

	var subFS fs.FS = webFS
	srv := NewServer(cfg, database, nil, nil, subFS)

	return srv, database
}

// doRequest is a helper to perform an HTTP request against the server mux.
func doRequest(t *testing.T, srv *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:12345" // Set localhost for internal endpoints
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	return rr
}

// doAuthRequest is a helper that adds a valid session cookie.
func doAuthRequest(t *testing.T, srv *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reqBody = bytes.NewBuffer(data)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}

	token := srv.auth.CreateSession()

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "flux_session", Value: token})
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)
	return rr
}

// parseResponse unmarshals the JSON response body into v.
func parseResponse(t *testing.T, rr *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, rr.Body.String())
	}
}

// --- Health endpoint ---

func TestHealth(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doRequest(t, srv, "GET", "/health", nil)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]any
	parseResponse(t, rr, &resp)

	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}
	if resp["version"] == "" {
		t.Error("expected version to be set")
	}
	// Check that auth_enabled field is present
	if _, ok := resp["auth_enabled"]; !ok {
		t.Error("expected auth_enabled to be present")
	}
}

// --- Auth tests ---

func TestLogin_Correct(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doRequest(t, srv, "POST", "/api/auth/login", map[string]string{
		"password": "testpass",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Should have set-cookie
	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "flux_session" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected flux_session cookie to be set")
	}
}

func TestLogin_Wrong(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doRequest(t, srv, "POST", "/api/auth/login", map[string]string{
		"password": "wrongpass",
	})

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestLogout(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Login first
	token := srv.auth.CreateSession()

	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "flux_session", Value: token})
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Session should be invalid now
	if srv.auth.ValidateSession(token) {
		t.Error("expected session to be invalidated after logout")
	}
}

func TestAuth_SessionValidation(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Request without auth should be rejected
	rr := doRequest(t, srv, "GET", "/api/goals", nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rr.Code)
	}

	// Request with valid session should succeed
	rr = doAuthRequest(t, srv, "GET", "/api/goals", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAuth_RateLimit(t *testing.T) {
	srv, _ := setupTestServer(t)

	// 5 failed attempts should be allowed
	for i := 0; i < 5; i++ {
		rr := doRequest(t, srv, "POST", "/api/auth/login", map[string]string{
			"password": "wrongpass",
		})
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rr.Code)
		}
	}

	// 6th attempt should be rate-limited
	rr := doRequest(t, srv, "POST", "/api/auth/login", map[string]string{
		"password": "wrongpass",
	})
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on 6th attempt, got %d", rr.Code)
	}
}

// --- Goals API tests ---

func TestGoals_Create(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{
		"title":       "Build MVP",
		"description": "Build the minimum viable product",
		"priorities":  []string{"speed"},
		"metrics":     []string{"coverage"},
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)

	if resp["id"] == nil || resp["id"] == "" {
		t.Error("expected id to be set")
	}
	if resp["title"] != "Build MVP" {
		t.Errorf("expected title 'Build MVP', got %v", resp["title"])
	}
	if resp["status"] != "PROPOSED" {
		t.Errorf("expected status PROPOSED, got %v", resp["status"])
	}
}

func TestGoals_Create_MissingTitle(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{
		"description": "No title",
	})

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestGoals_List(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create two goals
	doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{
		"title": "Goal 1",
	})
	doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{
		"title": "Goal 2",
	})

	rr := doAuthRequest(t, srv, "GET", "/api/goals", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)

	goals, ok := resp["goals"].([]interface{})
	if !ok {
		t.Fatal("expected goals array in response")
	}
	if len(goals) != 2 {
		t.Errorf("expected 2 goals, got %d", len(goals))
	}
}

func TestGoals_GetCurrent(t *testing.T) {
	srv, _ := setupTestServer(t)

	// No active goal initially
	rr := doAuthRequest(t, srv, "GET", "/api/goals/current", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["goal"] != nil {
		t.Error("expected null goal when none active")
	}
}

func TestGoals_UpdateAndActivate(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a goal
	rr := doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{
		"title": "Original",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	// Update it
	rr = doAuthRequest(t, srv, "PATCH", "/api/goals/"+id, map[string]interface{}{
		"title": "Updated",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var updated map[string]interface{}
	parseResponse(t, rr, &updated)
	if updated["title"] != "Updated" {
		t.Errorf("expected Updated, got %v", updated["title"])
	}

	// Activate it
	rr = doAuthRequest(t, srv, "POST", "/api/goals/"+id+"/activate", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var activated map[string]interface{}
	parseResponse(t, rr, &activated)
	if activated["status"] != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %v", activated["status"])
	}

	// GetCurrent should return it
	rr = doAuthRequest(t, srv, "GET", "/api/goals/current", nil)
	var currentResp map[string]interface{}
	parseResponse(t, rr, &currentResp)
	goal := currentResp["goal"].(map[string]interface{})
	if goal["id"] != id {
		t.Errorf("expected current goal id=%s, got %v", id, goal["id"])
	}
}

func TestGoals_Activate_Supersedes(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create and activate goal1
	rr1 := doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{"title": "Goal 1"})
	var g1 map[string]interface{}
	parseResponse(t, rr1, &g1)
	id1 := g1["id"].(string)
	doAuthRequest(t, srv, "POST", "/api/goals/"+id1+"/activate", nil)

	// Create and activate goal2 — should supersede goal1
	rr2 := doAuthRequest(t, srv, "POST", "/api/goals", map[string]interface{}{"title": "Goal 2"})
	var g2 map[string]interface{}
	parseResponse(t, rr2, &g2)
	id2 := g2["id"].(string)
	doAuthRequest(t, srv, "POST", "/api/goals/"+id2+"/activate", nil)

	// goal1 should now be SUPERSEDED
	rr := doAuthRequest(t, srv, "GET", "/api/goals", nil)
	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	goals := resp["goals"].([]interface{})

	for _, g := range goals {
		gm := g.(map[string]interface{})
		if gm["id"] == id1 && gm["status"] != "SUPERSEDED" {
			t.Errorf("expected goal1 SUPERSEDED, got %v", gm["status"])
		}
		if gm["id"] == id2 && gm["status"] != "ACTIVE" {
			t.Errorf("expected goal2 ACTIVE, got %v", gm["status"])
		}
	}
}

// --- Tasks API tests ---

func TestTasks_Create(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Implement feature",
		"type":  "CODING",
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("expected id to be set")
	}
	if resp["title"] != "Implement feature" {
		t.Errorf("expected title, got %v", resp["title"])
	}
}

func TestTasks_Create_MissingTitle(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"type": "CODING",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestTasks_Create_MissingType(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "No type",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestTasks_ListWithFilters(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create tasks
	doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Task A", "type": "CODING", "source": "OPERATOR",
	})
	doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Task B", "type": "RESEARCH", "source": "SYSTEM",
	})

	// List all
	rr := doAuthRequest(t, srv, "GET", "/api/tasks", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}

	// List with status filter — both tasks should be READY
	// (OPERATOR promoted immediately when triage is not configured, SYSTEM goes directly to READY)
	rr = doAuthRequest(t, srv, "GET", "/api/tasks?status=READY", nil)
	var filteredResp map[string]interface{}
	parseResponse(t, rr, &filteredResp)
	readyTasks := filteredResp["tasks"].([]interface{})
	if len(readyTasks) != 2 {
		t.Errorf("expected 2 READY tasks, got %d", len(readyTasks))
	}

	// List with pagination
	rr = doAuthRequest(t, srv, "GET", "/api/tasks?page=1&limit=1", nil)
	var pagedResp map[string]interface{}
	parseResponse(t, rr, &pagedResp)
	pagedTasks := pagedResp["tasks"].([]interface{})
	if len(pagedTasks) != 1 {
		t.Errorf("expected 1 task with limit=1, got %d", len(pagedTasks))
	}
}

func TestTasks_GetByID(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Get me", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["id"] != id {
		t.Errorf("expected id=%s, got %v", id, resp["id"])
	}
}

func TestTasks_GetByID_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "GET", "/api/tasks/nonexistent", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestTasks_Update(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Original", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "PATCH", "/api/tasks/"+id, map[string]interface{}{
		"title":  "Updated",
		"status": "RUNNING",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var updated map[string]interface{}
	parseResponse(t, rr, &updated)
	if updated["title"] != "Updated" {
		t.Errorf("expected Updated, got %v", updated["title"])
	}
	if updated["status"] != "RUNNING" {
		t.Errorf("expected RUNNING, got %v", updated["status"])
	}
}

func TestTasks_Delete(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Delete me", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "DELETE", "/api/tasks/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify deleted
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rr.Code)
	}
}

func TestTasks_Cancel(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Cancel me", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "POST", "/api/tasks/"+id+"/cancel", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var cancelled map[string]interface{}
	parseResponse(t, rr, &cancelled)
	if cancelled["status"] != "CANCELLED" {
		t.Errorf("expected CANCELLED, got %v", cancelled["status"])
	}
}

// --- Projects API tests ---

func TestProjects_Create(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name":      "flux",
		"type":      "REPO",
		"repo_url":  "https://github.com/circle-oo/flux",
		"tech_stack": []string{"go", "react"},
	})

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["name"] != "flux" {
		t.Errorf("expected name flux, got %v", resp["name"])
	}
	if resp["status"] != "PROPOSED" {
		t.Errorf("expected PROPOSED, got %v", resp["status"])
	}
}

func TestProjects_Create_MissingName(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"type": "REPO",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestProjects_Create_MissingType(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "no-type",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestProjects_List(t *testing.T) {
	srv, _ := setupTestServer(t)

	doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "proj-a", "type": "REPO",
	})
	doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "proj-b", "type": "SERVICE",
	})

	rr := doAuthRequest(t, srv, "GET", "/api/projects", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	projects := resp["projects"].([]interface{})
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestProjects_GetByID(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "get-me", "type": "REPO",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "GET", "/api/projects/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["id"] != id {
		t.Errorf("expected id=%s, got %v", id, resp["id"])
	}
}

func TestProjects_GetByID_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "GET", "/api/projects/nonexistent", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestProjects_Approve(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "approve-me", "type": "REPO",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "POST", "/api/projects/"+id+"/approve", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var approved map[string]interface{}
	parseResponse(t, rr, &approved)
	if approved["status"] != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %v", approved["status"])
	}
}

func TestProjects_Reject(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "reject-me", "type": "REPO",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "POST", "/api/projects/"+id+"/reject", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var rejected map[string]interface{}
	parseResponse(t, rr, &rejected)
	if rejected["status"] != "REJECTED" {
		t.Errorf("expected REJECTED, got %v", rejected["status"])
	}
}

func TestProjects_Update(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "POST", "/api/projects", map[string]interface{}{
		"name": "update-me", "type": "REPO",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "PATCH", "/api/projects/"+id, map[string]interface{}{
		"description": "updated description",
		"tech_stack":  []string{"python", "django"},
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var updated map[string]interface{}
	parseResponse(t, rr, &updated)
	if updated["description"] != "updated description" {
		t.Errorf("expected 'updated description', got %v", updated["description"])
	}
}

func TestProjects_Update_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "PATCH", "/api/projects/nonexistent", map[string]interface{}{
		"description": "nope",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// --- Internal API tests ---

func TestInternal_NextTask(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Internal endpoints use localhost — simulate with RemoteAddr
	req := httptest.NewRequest("POST", "/internal/tasks/next", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["task"] != nil {
		t.Error("expected null task from stub")
	}
}

func TestInternal_TaskDone(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a task first
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Complete me", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	// Mark done via internal API
	body, _ := json.Marshal(map[string]interface{}{
		"status":      "COMPLETED",
		"result":      "all good",
		"tokens_used": 1000,
		"cost_usd":    0.05,
	})
	req := httptest.NewRequest("POST", "/internal/tasks/"+id+"/done", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the task was updated
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	var task map[string]interface{}
	parseResponse(t, rr, &task)
	if task["status"] != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %v", task["status"])
	}
	if task["result"] != "all good" {
		t.Errorf("expected 'all good', got %v", task["result"])
	}
}

func TestInternal_TaskDone_ExecutionDetails(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a task first
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Exec details", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	// Mark done with execution details via internal API
	body, _ := json.Marshal(map[string]interface{}{
		"status":        "COMPLETED",
		"result":        "implemented feature",
		"tokens_used":   5000,
		"cost_usd":      0.25,
		"executor_id":   "executor-1",
		"model":         "opus",
		"branch_name":   "flux/task-abc123",
		"diff_lines":    150,
		"files_changed": 7,
		"test_passed":   true,
		"pr_url":        "https://github.com/org/repo/pull/42",
		"pr_status":     "MERGED",
	})
	req := httptest.NewRequest("POST", "/internal/tasks/"+id+"/done", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify execution details were persisted
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	var task map[string]interface{}
	parseResponse(t, rr, &task)

	if task["status"] != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %v", task["status"])
	}
	if task["result"] != "implemented feature" {
		t.Errorf("expected 'implemented feature', got %v", task["result"])
	}
	if task["executor_id"] != "executor-1" {
		t.Errorf("expected executor_id 'executor-1', got %v", task["executor_id"])
	}
	if task["model"] != "opus" {
		t.Errorf("expected model 'opus', got %v", task["model"])
	}
	if task["branch_name"] != "flux/task-abc123" {
		t.Errorf("expected branch_name 'flux/task-abc123', got %v", task["branch_name"])
	}
	if int(task["diff_lines"].(float64)) != 150 {
		t.Errorf("expected diff_lines 150, got %v", task["diff_lines"])
	}
	if int(task["files_changed"].(float64)) != 7 {
		t.Errorf("expected files_changed 7, got %v", task["files_changed"])
	}
	if task["test_passed"] != true {
		t.Errorf("expected test_passed true, got %v", task["test_passed"])
	}
	if int(task["tokens_used"].(float64)) != 5000 {
		t.Errorf("expected tokens_used 5000, got %v", task["tokens_used"])
	}
	if task["cost_usd"].(float64) != 0.25 {
		t.Errorf("expected cost_usd 0.25, got %v", task["cost_usd"])
	}
	if task["pr_url"] != "https://github.com/org/repo/pull/42" {
		t.Errorf("expected pr_url, got %v", task["pr_url"])
	}
	if task["pr_status"] != "MERGED" {
		t.Errorf("expected pr_status MERGED, got %v", task["pr_status"])
	}
}

func TestInternal_TaskDone_CancelledTask(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a task and transition it to RUNNING
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Running task", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	// Transition to RUNNING (simulating executor picking it up)
	body, _ := json.Marshal(map[string]interface{}{
		"status": "RUNNING",
	})
	req := httptest.NewRequest("POST", "/internal/tasks/"+id+"/done", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for RUNNING transition, got %d: %s", rr.Code, rr.Body.String())
	}

	// Cancel the task while it's running
	rr = doAuthRequest(t, srv, "POST", "/api/tasks/"+id+"/cancel", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for cancel, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify task is CANCELLED
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	var cancelled map[string]interface{}
	parseResponse(t, rr, &cancelled)
	if cancelled["status"] != "CANCELLED" {
		t.Fatalf("expected CANCELLED after cancel, got %v", cancelled["status"])
	}

	// Executor tries to report completion (simulating it finishing work)
	body, _ = json.Marshal(map[string]interface{}{
		"status":      "COMPLETED",
		"result":      "work completed",
		"tokens_used": 2000,
		"cost_usd":    0.10,
		"executor_id": "executor-1",
	})
	req = httptest.NewRequest("POST", "/internal/tasks/"+id+"/done", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	// Should return 200 (allowing executor to move on)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for cancelled task completion, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify task remains CANCELLED (status not changed to COMPLETED)
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	var final map[string]interface{}
	parseResponse(t, rr, &final)
	if final["status"] != "CANCELLED" {
		t.Errorf("expected status to remain CANCELLED, got %v", final["status"])
	}

	// Verify cost/tokens were still recorded for accounting
	if int(final["tokens_used"].(float64)) != 2000 {
		t.Errorf("expected tokens_used 2000, got %v", final["tokens_used"])
	}
	if final["cost_usd"].(float64) != 0.10 {
		t.Errorf("expected cost_usd 0.10, got %v", final["cost_usd"])
	}
}

func TestInternal_CreateSubtasks(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create parent task
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Parent task", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	parentID := created["id"].(string)

	// Create subtasks
	body, _ := json.Marshal(map[string]interface{}{
		"parent_id": parentID,
		"subtasks": []map[string]string{
			{"title": "Sub 1", "description": "First subtask"},
			{"title": "Sub 2", "description": "Second subtask"},
		},
	})
	req := httptest.NewRequest("POST", "/internal/subtasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(tasks))
	}
}

func TestInternal_CreateSubtasks_DepthExceeded(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create parent task
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Parent", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	parentID := created["id"].(string)

	// Create first level subtask
	body, _ := json.Marshal(map[string]interface{}{
		"parent_id": parentID,
		"subtasks":  []map[string]string{{"title": "Sub 1"}},
	})
	req := httptest.NewRequest("POST", "/internal/subtasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	var subResp map[string]interface{}
	parseResponse(t, rr, &subResp)
	subtasks := subResp["tasks"].([]interface{})
	subID := subtasks[0].(map[string]interface{})["id"].(string)

	// Try to create subtask of subtask (depth 2 > maxDepth 1)
	body, _ = json.Marshal(map[string]interface{}{
		"parent_id": subID,
		"subtasks":  []map[string]string{{"title": "Too deep"}},
	})
	req = httptest.NewRequest("POST", "/internal/subtasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for depth exceeded, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInternal_CreateSubtasks_CountExceeded(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create parent
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Parent", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	parentID := created["id"].(string)

	// Try to create 6 subtasks (maxPerTask is 5)
	subtasks := make([]map[string]string, 6)
	for i := range subtasks {
		subtasks[i] = map[string]string{"title": "Sub"}
	}
	body, _ := json.Marshal(map[string]interface{}{
		"parent_id": parentID,
		"subtasks":  subtasks,
	})
	req := httptest.NewRequest("POST", "/internal/subtasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for count exceeded, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInternal_CreateTask(t *testing.T) {
	srv, _ := setupTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"title":       "Fix build failure from: Original Task",
		"description": "Build output:\n```\nmain.go:5:2: undefined: foo\n```",
		"type":        "BUGFIX",
		"priority":    10,
		"source":      "SYSTEM",
		"project_id":  "proj-123",
		"branch_name": "task/abc12345",
		"tags":        []string{"build-failure", "auto-registered"},
	})
	req := httptest.NewRequest("POST", "/internal/tasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("expected id to be set")
	}
	if resp["title"] != "Fix build failure from: Original Task" {
		t.Errorf("expected title, got %v", resp["title"])
	}
	if resp["type"] != "BUGFIX" {
		t.Errorf("expected BUGFIX, got %v", resp["type"])
	}
	if resp["source"] != "SYSTEM" {
		t.Errorf("expected SYSTEM, got %v", resp["source"])
	}
	if resp["branch_name"] != "task/abc12345" {
		t.Errorf("expected branch_name task/abc12345, got %v", resp["branch_name"])
	}
}

func TestAPI_ListSubtasks(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create parent task
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Parent task", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	parentID := created["id"].(string)

	// Create subtasks via internal API
	body, _ := json.Marshal(map[string]interface{}{
		"parent_id": parentID,
		"subtasks": []map[string]string{
			{"title": "Sub 1", "description": "First subtask"},
			{"title": "Sub 2", "description": "Second subtask"},
		},
	})
	req := httptest.NewRequest("POST", "/internal/subtasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	// List subtasks via public API
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+parentID+"/subtasks", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 2 {
		t.Errorf("expected 2 subtasks, got %d", len(tasks))
	}
}

func TestAPI_ListSubtasks_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create task with no subtasks
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "No children", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id+"/subtasks", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	tasks := resp["tasks"].([]interface{})
	if len(tasks) != 0 {
		t.Errorf("expected 0 subtasks, got %d", len(tasks))
	}
}

func TestInternal_NextPending(t *testing.T) {
	srv, database := setupTestServer(t)

	// Wire up manager so PopNextPending works
	cfg := &config.Config{}
	m := manager.NewManager(database, cfg)
	srv.mgr = m

	// Create a PENDING operator task directly in DB
	_, err := database.Exec(
		`INSERT INTO tasks (id, title, type, status, priority, source, depends_on, tags)
		 VALUES (?, ?, ?, ?, ?, ?, '[]', '[]')`,
		"test-pending-001", "Pending task", "CODING", "PENDING", 50, "OPERATOR",
	)
	if err != nil {
		t.Fatalf("insert pending task: %v", err)
	}

	// Request next pending
	body, _ := json.Marshal(map[string]string{"triager_id": "triager-01"})
	req := httptest.NewRequest("POST", "/internal/tasks/next-pending", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	task := resp["task"]
	if task == nil {
		t.Fatal("expected a pending task, got nil")
	}
}

func TestInternal_TaskStarted(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a task
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Track me", "type": "CODING",
	})
	var created map[string]interface{}
	parseResponse(t, rr, &created)
	id := created["id"].(string)

	// Report execution start
	body, _ := json.Marshal(map[string]interface{}{
		"executor_id": "executor-01",
		"model":       "opus",
		"branch_name": "flux/task-abc",
	})
	req := httptest.NewRequest("POST", "/internal/tasks/"+id+"/started", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify fields were persisted
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+id, nil)
	var task map[string]interface{}
	parseResponse(t, rr, &task)

	if task["executor_id"] != "executor-01" {
		t.Errorf("expected executor_id executor-01, got %v", task["executor_id"])
	}
	if task["model"] != "opus" {
		t.Errorf("expected model opus, got %v", task["model"])
	}
	if task["branch_name"] != "flux/task-abc" {
		t.Errorf("expected branch flux/task-abc, got %v", task["branch_name"])
	}
}

func TestInternal_Triaged(t *testing.T) {
	srv, database := setupTestServer(t)

	// Create a PENDING task
	_, err := database.Exec(
		`INSERT INTO tasks (id, title, type, status, priority, source, depends_on, tags)
		 VALUES (?, ?, ?, ?, ?, ?, '[]', '[]')`,
		"test-triage-001", "Triage me", "CODING", "PENDING", 50, "OPERATOR",
	)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// Report triage completion
	body, _ := json.Marshal(map[string]interface{}{
		"analysis":    "This task is well-defined",
		"description": "Updated description with clear requirements",
		"priority":    30,
	})
	req := httptest.NewRequest("POST", "/internal/tasks/test-triage-001/triaged", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify task was promoted to READY with triage results
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/test-triage-001", nil)
	var task map[string]interface{}
	parseResponse(t, rr, &task)

	if task["status"] != "READY" {
		t.Errorf("expected READY, got %v", task["status"])
	}
	if task["triage_analysis"] != "This task is well-defined" {
		t.Errorf("expected triage analysis, got %v", task["triage_analysis"])
	}
	if task["description"] != "Updated description with clear requirements" {
		t.Errorf("expected updated description, got %v", task["description"])
	}
	if int(task["priority"].(float64)) != 30 {
		t.Errorf("expected priority 30, got %v", task["priority"])
	}
}

// TestTriageAnalysis_FullLifecycle tests that triage_analysis is correctly
// persisted and preserved through the full task lifecycle:
// PENDING -> triaged -> READY -> popped by executor -> RUNNING -> done -> COMPLETED
func TestTriageAnalysis_FullLifecycle(t *testing.T) {
	srv, database := setupTestServer(t)

	// Wire up manager — set directly on server struct
	cfg := &config.Config{}
	m := manager.NewManager(database, cfg)
	srv.mgr = m

	// Step 1: Create a PENDING operator task directly in DB
	_, err := database.Exec(
		`INSERT INTO tasks (id, title, type, status, priority, source, depends_on, tags)
		 VALUES (?, ?, ?, ?, ?, ?, '[]', '[]')`,
		"triage-lifecycle-001", "Full lifecycle test", "CODING", "PENDING", 50, "OPERATOR",
	)
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}

	// Step 2: Triager reports triage completion with analysis
	body, _ := json.Marshal(map[string]interface{}{
		"analysis":    "This is a detailed triage analysis for the task.",
		"description": "Rewritten description with clear requirements.",
		"priority":    25,
		"model":       "opus",
	})
	req := httptest.NewRequest("POST", "/internal/tasks/triage-lifecycle-001/triaged", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("triaged: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify triage_analysis after triage
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/triage-lifecycle-001", nil)
	var task map[string]interface{}
	parseResponse(t, rr, &task)

	if task["status"] != "READY" {
		t.Errorf("after triage: expected READY, got %v", task["status"])
	}
	if task["triage_analysis"] != "This is a detailed triage analysis for the task." {
		t.Errorf("after triage: expected triage analysis, got %v", task["triage_analysis"])
	}

	// Step 3: Executor pops the task via /internal/tasks/next
	body, _ = json.Marshal(map[string]string{"pod_id": "executor-01", "pod_type": "EXECUTOR"})
	req = httptest.NewRequest("POST", "/internal/tasks/next", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("next task: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var nextResp map[string]interface{}
	parseResponse(t, rr, &nextResp)
	poppedTask := nextResp["task"]
	if poppedTask == nil {
		t.Fatal("next task: expected a task, got nil")
	}
	poppedMap := poppedTask.(map[string]interface{})
	if poppedMap["triage_analysis"] != "This is a detailed triage analysis for the task." {
		t.Errorf("popped task: expected triage analysis, got %v", poppedMap["triage_analysis"])
	}

	// Step 4: Executor reports task started
	body, _ = json.Marshal(map[string]interface{}{
		"executor_id": "executor-01",
		"model":       "opus",
		"branch_name": "flux/task-triage-lifecycle-001",
	})
	req = httptest.NewRequest("POST", "/internal/tasks/triage-lifecycle-001/started", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("started: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify triage_analysis still present after started
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/triage-lifecycle-001", nil)
	parseResponse(t, rr, &task)
	if task["triage_analysis"] != "This is a detailed triage analysis for the task." {
		t.Errorf("after started: expected triage analysis, got %v", task["triage_analysis"])
	}

	// Step 5: Executor reports task done
	body, _ = json.Marshal(map[string]interface{}{
		"status":       "COMPLETED",
		"result":       "task completed successfully",
		"tokens_used":  1000,
		"cost_usd":     0.05,
		"executor_id":  "executor-01",
		"model":        "opus",
		"branch_name":  "flux/task-triage-lifecycle-001",
		"diff_lines":   50,
		"files_changed": 3,
		"pr_url":       "https://github.com/test/repo/pull/1",
		"pr_status":    "OPEN",
	})
	req = httptest.NewRequest("POST", "/internal/tasks/triage-lifecycle-001/done", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("done: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Step 6: Verify triage_analysis is STILL present after task completion
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/triage-lifecycle-001", nil)
	parseResponse(t, rr, &task)

	if task["status"] != "COMPLETED" {
		t.Errorf("after done: expected COMPLETED, got %v", task["status"])
	}
	if task["triage_analysis"] != "This is a detailed triage analysis for the task." {
		t.Errorf("after done: triage_analysis lost! expected analysis text, got %v", task["triage_analysis"])
	}

	// Also verify via direct DB query
	var dbAnalysis string
	err = database.QueryRow("SELECT triage_analysis FROM tasks WHERE id = ?", "triage-lifecycle-001").Scan(&dbAnalysis)
	if err != nil {
		t.Fatalf("direct DB query: %v", err)
	}
	if dbAnalysis != "This is a detailed triage analysis for the task." {
		t.Errorf("direct DB: triage_analysis lost! expected analysis text, got %q", dbAnalysis)
	}
}

func TestTasks_CancelCascade(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create parent task
	rr := doAuthRequest(t, srv, "POST", "/api/tasks", map[string]interface{}{
		"title": "Parent", "type": "CODING",
	})
	var parent map[string]interface{}
	parseResponse(t, rr, &parent)
	parentID := parent["id"].(string)

	// Create subtasks
	body, _ := json.Marshal(map[string]interface{}{
		"parent_id": parentID,
		"subtasks": []map[string]string{
			{"title": "Sub 1"},
			{"title": "Sub 2"},
		},
	})
	req := httptest.NewRequest("POST", "/internal/subtasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	// Cancel parent
	rr = doAuthRequest(t, srv, "POST", "/api/tasks/"+parentID+"/cancel", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify subtasks are cancelled
	rr = doAuthRequest(t, srv, "GET", "/api/tasks/"+parentID+"/subtasks", nil)
	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	tasks := resp["tasks"].([]interface{})
	for _, st := range tasks {
		sub := st.(map[string]interface{})
		if sub["status"] != "CANCELLED" {
			t.Errorf("expected subtask CANCELLED, got %v", sub["status"])
		}
	}
}

func TestInternal_CreateTask_MissingTitle(t *testing.T) {
	srv, _ := setupTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"type": "BUGFIX",
	})
	req := httptest.NewRequest("POST", "/internal/tasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInternal_CreateTask_MissingType(t *testing.T) {
	srv, _ := setupTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"title": "No type",
	})
	req := httptest.NewRequest("POST", "/internal/tasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestInternal_CreateTask_DefaultValues(t *testing.T) {
	srv, _ := setupTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"title": "Minimal task",
		"type":  "BUGFIX",
	})
	req := httptest.NewRequest("POST", "/internal/tasks", bytes.NewBuffer(body))
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	// Priority defaults to 40
	if int(resp["priority"].(float64)) != 40 {
		t.Errorf("expected default priority 40, got %v", resp["priority"])
	}
	// Source defaults to SYSTEM
	if resp["source"] != "SYSTEM" {
		t.Errorf("expected default source SYSTEM, got %v", resp["source"])
	}
}

func TestInternal_GetModel(t *testing.T) {
	srv, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/internal/model/some-task-id", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	parseResponse(t, rr, &resp)
	if resp["model"] != "sonnet" {
		t.Errorf("expected model=sonnet, got %s", resp["model"])
	}
}

func TestInternal_Forbidden(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Request from non-localhost should be forbidden
	req := httptest.NewRequest("POST", "/internal/tasks/next", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

// --- Services & Alerts stubs ---

func TestServices_List(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "GET", "/api/services", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	services := resp["services"].([]interface{})
	if len(services) != 0 {
		t.Errorf("expected empty services, got %d", len(services))
	}
}

func TestAlerts_List(t *testing.T) {
	srv, _ := setupTestServer(t)

	rr := doAuthRequest(t, srv, "GET", "/api/alerts", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	alerts := resp["alerts"].([]interface{})
	if len(alerts) != 0 {
		t.Errorf("expected empty alerts, got %d", len(alerts))
	}
}

// --- Error masking tests ---

func TestErrorMasking_500(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Try to activate a non-existent goal — should return 500 with generic message
	rr := doAuthRequest(t, srv, "POST", "/api/goals/nonexistent/activate", nil)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}

	var resp map[string]string
	parseResponse(t, rr, &resp)
	if resp["error"] != "internal server error" {
		t.Errorf("expected generic error, got %q", resp["error"])
	}
}

// --- Request body size limit test ---

func TestRequestBodySizeLimit(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Create a body larger than 1MB
	bigBody := make([]byte, 2<<20) // 2MB
	for i := range bigBody {
		bigBody[i] = 'a'
	}

	token := srv.auth.CreateSession()
	req := httptest.NewRequest("POST", "/api/goals", bytes.NewBuffer(bigBody))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "flux_session", Value: token})
	rr := httptest.NewRecorder()
	srv.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized body, got %d", rr.Code)
	}
}

func TestPodsAPI(t *testing.T) {
	srv, _ := setupTestServer(t)

	// Test GET /api/pods endpoint (initially empty)
	rr := doAuthRequest(t, srv, "GET", "/api/pods", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	parseResponse(t, rr, &resp)
	pods, ok := resp["pods"].([]interface{})
	if !ok {
		t.Fatal("expected 'pods' array in response")
	}
	if len(pods) != 0 {
		t.Errorf("expected 0 pods initially, got %d", len(pods))
	}

	// Test pod registration (internal endpoint, no auth required)
	regReq := map[string]interface{}{
		"id":         "executor-01",
		"started_at": "2024-01-01T00:00:00Z",
	}
	rr = doRequest(t, srv, "POST", "/internal/pods/register", regReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for registration, got %d", rr.Code)
	}

	// Test pod status update (internal endpoint)
	statusReq := map[string]interface{}{
		"id":         "executor-01",
		"status":     "busy",
		"task_id":    "task-123",
		"task_title": "Test Task",
	}
	rr = doRequest(t, srv, "POST", "/internal/pods/status", statusReq)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 for status update, got %d", rr.Code)
	}

	// Test GET /api/pods again (should have one pod)
	rr = doAuthRequest(t, srv, "GET", "/api/pods", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	parseResponse(t, rr, &resp)
	pods, ok = resp["pods"].([]interface{})
	if !ok {
		t.Fatal("expected 'pods' array in response")
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}

	pod := pods[0].(map[string]interface{})
	if pod["id"] != "executor-01" {
		t.Errorf("expected pod ID 'executor-01', got %v", pod["id"])
	}
	if pod["status"] != "busy" {
		t.Errorf("expected pod status 'busy', got %v", pod["status"])
	}
	if pod["current_task"] != "task-123" {
		t.Errorf("expected current_task 'task-123', got %v", pod["current_task"])
	}
}
