package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// Server is the main HTTP server for Flux.
type Server struct {
	config     *config.Config
	db         *sql.DB
	mux        *http.ServeMux
	server     *http.Server
	auth       *AuthManager
	ws         *WebSocketHub
	logHandler *LogBroadcastHandler
	notifier   *notifier.Discord
	webFS      fs.FS

	goals    *models.GoalStore
	tasks    *models.TaskStore
	projects *models.ProjectStore
	alerts   *models.AlertStore
	usage    *models.UsageStore
}

// NewServer creates a new Server.
func NewServer(cfg *config.Config, db *sql.DB, discord *notifier.Discord, webFS fs.FS) *Server {
	s := &Server{
		config:   cfg,
		db:       db,
		mux:      http.NewServeMux(),
		notifier: discord,
		webFS:    webFS,
		goals:    models.NewGoalStore(db),
		tasks:    models.NewTaskStore(db),
		projects: models.NewProjectStore(db),
		alerts:   models.NewAlertStore(db),
		usage:    models.NewUsageStore(db),
	}

	s.auth = NewAuthManager(cfg.Server.Auth)
	s.ws = NewWebSocketHub()

	s.setupRoutes()

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: s.mux,
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	slog.Info("starting HTTP server", "addr", s.server.Addr)
	go s.ws.Run()
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.ws.Stop()
	return s.server.Shutdown(ctx)
}

func (s *Server) setupRoutes() {
	// Health endpoint (no auth)
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Auth endpoints (no auth required)
	s.mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/auth/logout", s.handleLogout)

	// Goals API (requires auth)
	s.mux.Handle("POST /api/goals", s.authMiddleware(http.HandlerFunc(s.handleCreateGoal)))
	s.mux.Handle("GET /api/goals", s.authMiddleware(http.HandlerFunc(s.handleListGoals)))
	s.mux.Handle("GET /api/goals/current", s.authMiddleware(http.HandlerFunc(s.handleGetCurrentGoal)))
	s.mux.Handle("PATCH /api/goals/{id}", s.authMiddleware(http.HandlerFunc(s.handleUpdateGoal)))
	s.mux.Handle("POST /api/goals/{id}/activate", s.authMiddleware(http.HandlerFunc(s.handleActivateGoal)))

	// Tasks API (requires auth)
	s.mux.Handle("POST /api/tasks", s.authMiddleware(http.HandlerFunc(s.handleCreateTask)))
	s.mux.Handle("GET /api/tasks", s.authMiddleware(http.HandlerFunc(s.handleListTasks)))
	s.mux.Handle("GET /api/tasks/{id}", s.authMiddleware(http.HandlerFunc(s.handleGetTask)))
	s.mux.Handle("PATCH /api/tasks/{id}", s.authMiddleware(http.HandlerFunc(s.handleUpdateTask)))
	s.mux.Handle("DELETE /api/tasks/{id}", s.authMiddleware(http.HandlerFunc(s.handleDeleteTask)))
	s.mux.Handle("POST /api/tasks/{id}/cancel", s.authMiddleware(http.HandlerFunc(s.handleCancelTask)))
	s.mux.Handle("POST /api/tasks/{id}/retry", s.authMiddleware(http.HandlerFunc(s.handleRetryTask)))

	// Projects API (requires auth)
	s.mux.Handle("POST /api/projects", s.authMiddleware(http.HandlerFunc(s.handleCreateProject)))
	s.mux.Handle("GET /api/projects", s.authMiddleware(http.HandlerFunc(s.handleListProjects)))
	s.mux.Handle("GET /api/projects/{id}", s.authMiddleware(http.HandlerFunc(s.handleGetProject)))
	s.mux.Handle("PATCH /api/projects/{id}", s.authMiddleware(http.HandlerFunc(s.handleUpdateProject)))
	s.mux.Handle("POST /api/projects/{id}/approve", s.authMiddleware(http.HandlerFunc(s.handleApproveProject)))
	s.mux.Handle("POST /api/projects/{id}/reject", s.authMiddleware(http.HandlerFunc(s.handleRejectProject)))

	// Services & Alerts stubs (requires auth)
	s.mux.Handle("GET /api/services", s.authMiddleware(http.HandlerFunc(s.handleListServices)))
	s.mux.Handle("GET /api/alerts", s.authMiddleware(http.HandlerFunc(s.handleListAlerts)))

	// System endpoints (requires auth)
	s.mux.Handle("POST /api/system/restart", s.authMiddleware(http.HandlerFunc(s.handleRestart)))

	// Internal API (localhost only, no auth)
	s.mux.Handle("POST /internal/tasks/next", s.localhostOnly(http.HandlerFunc(s.handleInternalNextTask)))
	s.mux.Handle("POST /internal/tasks/{id}/done", s.localhostOnly(http.HandlerFunc(s.handleInternalTaskDone)))
	s.mux.Handle("POST /internal/subtasks", s.localhostOnly(http.HandlerFunc(s.handleInternalCreateSubtasks)))
	s.mux.Handle("GET /internal/model/{task_id}", s.localhostOnly(http.HandlerFunc(s.handleInternalGetModel)))
	s.mux.Handle("GET /internal/projects/{id}", s.localhostOnly(http.HandlerFunc(s.handleInternalGetProject)))

	// PR Review API (requires auth)
	s.RegisterPRRoutes()

	// Logs API (requires auth)
	s.mux.Handle("GET /api/logs/recent", s.authMiddleware(http.HandlerFunc(s.handleRecentLogs)))

	// WebSocket
	s.mux.Handle("GET /ws/events", s.authMiddleware(http.HandlerFunc(s.handleWebSocket)))

	// Static files (embedded React frontend)
	// SPA fallback: serve index.html for non-API, non-file routes
	s.mux.Handle("/", s.spaHandler())
}

// spaHandler serves the embedded React frontend with SPA fallback.
func (s *Server) spaHandler() http.Handler {
	fileServer := http.FileServer(http.FS(s.webFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			path = "index.html"
		}

		// Check if file exists in embedded FS
		f, err := s.webFS.Open(path[1:]) // strip leading /
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})
}

// localhostOnly middleware rejects requests not from localhost.
func (s *Server) localhostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if host != "127.0.0.1" && host != "::1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// handleHealth returns system health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": version,
	})
}

// version is set at build time via -ldflags.
var version = "dev"

// SetVersion sets the server version string (called from main).
func SetVersion(v string) {
	version = v
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Hub returns the WebSocket hub for external wiring (e.g. log broadcasting).
func (s *Server) Hub() *WebSocketHub {
	return s.ws
}

// SetLogHandler sets the log broadcast handler so the server can serve recent logs.
func (s *Server) SetLogHandler(h *LogBroadcastHandler) {
	s.logHandler = h
}

// handleRecentLogs returns buffered log entries.
func (s *Server) handleRecentLogs(w http.ResponseWriter, r *http.Request) {
	if s.logHandler == nil {
		writeJSON(w, http.StatusOK, map[string]any{"logs": []LogEntry{}})
		return
	}
	logs := s.logHandler.GetRecentLogs()
	writeJSON(w, http.StatusOK, map[string]any{"logs": logs})
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// readJSON decodes a JSON request body into v.
// Limits request body to 1MB to prevent abuse.
func readJSON(w http.ResponseWriter, r *http.Request, v interface{}) error {
	if r.Body == nil {
		return fmt.Errorf("empty request body")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	return json.NewDecoder(r.Body).Decode(v)
}
