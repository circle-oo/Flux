package triager

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/circle-oo/flux/internal/apiclient"
	"github.com/circle-oo/flux/internal/config"
)

func TestProcessNext_TriageFailureReportsSentinelAnalysis(t *testing.T) {
	const (
		taskID   = "task-123"
		sentinel = "[triage failed - manual review recommended]"
	)

	var capturedAnalysis string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/tasks/next-pending":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"task":{"id":"task-123","title":"Needs triage","priority":50,"description":"desc"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/internal/tasks/"+taskID+"/triaged":
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode triaged request: %v", err)
			}
			analysis, _ := req["analysis"].(string)
			capturedAnalysis = analysis
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer testServer.Close()

	tr := &Triager{
		id:          "triager-test",
		config:      &config.Config{Triager: config.TriagerConfig{Model: "haiku"}},
		agentClient: nil, // Forces triageTask() error path.
		client:      apiclient.NewClient(testServer.URL),
		stopCh:      make(chan struct{}),
	}

	tr.processNext(context.Background())

	if capturedAnalysis != sentinel {
		t.Fatalf("analysis = %q, want %q", capturedAnalysis, sentinel)
	}
	if got := tr.CurrentTaskID(); strings.TrimSpace(got) != "" {
		t.Fatalf("current task should be cleared after processNext, got %q", got)
	}
}
