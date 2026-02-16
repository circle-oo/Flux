package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/circle-oo/flux/internal/config"
)

// mockVaultReader implements vault.VaultReader for testing.
type mockVaultReader struct {
	notes   []string
	content map[string]string
	mode    string
	healthy bool
}

func (m *mockVaultReader) Read(path string) (string, error) {
	if c, ok := m.content[path]; ok {
		return c, nil
	}
	return "", http.ErrNoLocation
}

func (m *mockVaultReader) List(folder string) ([]string, error) {
	return m.notes, nil
}

func (m *mockVaultReader) Search(query string) (string, error) {
	return "notes/test.md", nil
}

func (m *mockVaultReader) Frontmatter(path string) (string, error) {
	return "title: Test", nil
}

func (m *mockVaultReader) IsHealthy() bool {
	return m.healthy
}

func (m *mockVaultReader) Mode() string {
	return m.mode
}

func newTestServerWithVault() *Server {
	mock := &mockVaultReader{
		notes: []string{"notes/test.md", "projects/flux/index.md"},
		content: map[string]string{
			"notes/test":          "# Test Note\nHello world",
			"projects/flux/index": "# Flux Project",
		},
		mode:    "full",
		healthy: true,
	}

	s := &Server{
		mux:   http.NewServeMux(),
		vault: mock,
	}
	return s
}

func TestHandleKnowledgeListNotes(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/notes", nil)
	w := httptest.NewRecorder()

	s.handleKnowledgeListNotes(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	notes, ok := resp["notes"].([]interface{})
	if !ok {
		t.Fatal("expected notes array in response")
	}
	if len(notes) != 2 {
		t.Errorf("expected 2 notes, got %d", len(notes))
	}
}

func TestHandleKnowledgeReadNote(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/notes/notes/test", nil)
	req.SetPathValue("path", "notes/test")
	w := httptest.NewRecorder()

	s.handleKnowledgeReadNote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["content"] != "# Test Note\nHello world" {
		t.Errorf("unexpected content: %v", resp["content"])
	}
}

func TestHandleKnowledgeReadNote_NotFound(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/notes/nonexistent", nil)
	req.SetPathValue("path", "nonexistent")
	w := httptest.NewRecorder()

	s.handleKnowledgeReadNote(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleKnowledgeHealth(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/health", nil)
	w := httptest.NewRecorder()

	s.handleKnowledgeHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["mode"] != "full" {
		t.Errorf("expected mode 'full', got %v", resp["mode"])
	}
	if resp["healthy"] != true {
		t.Errorf("expected healthy true, got %v", resp["healthy"])
	}
}

func TestHandleKnowledgeStats(t *testing.T) {
	s := newTestServerWithVault()
	s.config = &config.Config{}
	req := httptest.NewRequest("GET", "/api/knowledge/stats", nil)
	w := httptest.NewRecorder()

	s.handleKnowledgeStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["note_count"].(float64) != 2 {
		t.Errorf("expected 2 notes, got %v", resp["note_count"])
	}
	if resp["healthy"] != true {
		t.Errorf("expected healthy true")
	}
}

func TestHandleKnowledgeSearch(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/search?q=test", nil)
	w := httptest.NewRecorder()

	s.handleKnowledgeSearch(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	results, ok := resp["results"].([]interface{})
	if !ok {
		t.Fatal("expected results array")
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestHandleKnowledgeSearch_MissingQuery(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/search", nil)
	w := httptest.NewRecorder()

	s.handleKnowledgeSearch(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleKnowledgeFolders(t *testing.T) {
	s := newTestServerWithVault()
	req := httptest.NewRequest("GET", "/api/knowledge/folders", nil)
	w := httptest.NewRecorder()

	s.handleKnowledgeFolders(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	folders, ok := resp["folders"].([]interface{})
	if !ok {
		t.Fatal("expected folders array")
	}
	if len(folders) != 2 {
		t.Errorf("expected 2 folders, got %d", len(folders))
	}
}
