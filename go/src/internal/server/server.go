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
	"time"

	"github.com/circle-oo/flux/internal/ccusage"
	"github.com/circle-oo/flux/internal/cleanup"
	"github.com/circle-oo/flux/internal/config"
	github_pkg "github.com/circle-oo/flux/internal/github"
	"github.com/circle-oo/flux/internal/insights"
	"github.com/circle-oo/flux/internal/manager"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/orchestrator"
	"github.com/circle-oo/flux/internal/updater"
	"github.com/circle-oo/flux/internal/vault"
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
	updater    *updater.Updater
	webFS      fs.FS
	mgr        *manager.Manager
	ghClient   *github_pkg.Client
	podRegistry *PodRegistry

	vault       vault.VaultReader
	vaultWriter vault.VaultWriter

	goals            *models.GoalStore
	tasks            *models.TaskStore
	taskAttempts     *models.TaskAttemptStore
	taskUsageEvents  *models.TaskUsageEventStore
	projects         *models.ProjectStore
	alerts           *models.AlertStore
	usage            *models.UsageStore
	insights         *insights.Collector

	billingCache *ccusage.BillingCache

	orch         *orchestrator.Orchestrator
	scaleManager *orchestrator.ScaleManager
	cleaner      *cleanup.Cleaner
}

// ServerDeps bundles all dependencies required to create a Server.
type ServerDeps struct {
	Config   *config.Config
	DB       *sql.DB
	Manager  *manager.Manager
	Discord  *notifier.Discord
	WebFS    fs.FS
	Version  string
	Vault    vault.VaultWriter
}

// NewServer creates a new Server with the provided dependencies.
func NewServer(deps ServerDeps) *Server {
	if deps.Version != "" {
		version = deps.Version
	}
	s := &Server{
		config:      deps.Config,
		db:          deps.DB,
		mgr:         deps.Manager,
		mux:         http.NewServeMux(),
		notifier:    deps.Discord,
		webFS:       deps.WebFS,
		podRegistry: NewPodRegistry(),
		goals:           models.NewGoalStore(deps.DB),
		tasks:           models.NewTaskStore(deps.DB),
		taskAttempts:    models.NewTaskAttemptStore(deps.DB),
		taskUsageEvents: models.NewTaskUsageEventStore(deps.DB),
		projects:        models.NewProjectStore(deps.DB),
		alerts:          models.NewAlertStore(deps.DB),
		usage:           models.NewUsageStore(deps.DB),
	}

	s.auth = NewAuthManager(deps.Config.Server.Auth)
	s.ws = NewWebSocketHub()
	s.initInsights()

	// Start background billing cache if ccusage is configured
	if cmd := deps.Config.CCUsage.Command; cmd != "" {
		s.billingCache = ccusage.NewBillingCache(cmd, deps.DB, 2*time.Minute)
		s.billingCache.Start()
	}

	// Initialize vault if provided
	if deps.Vault != nil {
		s.vault = deps.Vault
		s.vaultWriter = deps.Vault
	}

	// Initialize GitHub client if configured
	if deps.Config.GitHub.Token != "" {
		s.ghClient = github_pkg.NewClient(deps.Config.GitHub.Token, deps.Config.GitHub.Username)
	}

	s.setupRoutes()

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", deps.Config.Server.Host, deps.Config.Server.Port),
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
	if s.billingCache != nil {
		s.billingCache.Stop()
	}
	s.ws.Stop()
	return s.server.Shutdown(ctx)
}

// setupRoutes registers all HTTP handlers.
//
// Flux exposes four API surfaces that intentionally coexist:
//
//   - REST API (/api/*) — Primary for the full orchestration UI: goals, tasks,
//     projects, PRs, deploy, pods, insights. Backed by SQLite via models.*Store.
//
//   - Connect-RPC (/flux.v1.FluxService/*) — Typed API for agent task creation
//     and streaming (SSE). Mounted in main.go via fluxv1connect handler. Backed
//     by the in-memory store (internal/store). Will unify with SQLite in Phase 3.
//
//   - Internal API (/internal/*) — Localhost-only endpoints for pod-to-manager
//     coordination: task claiming, status reporting, pod registration.
//
//   - WebSocket (/ws/events) — Deprecated; replaced by Connect-RPC SSE for new
//     consumers. Still load-bearing for the Dashboard (connection badge) and
//     TaskDetail (refresh triggers). Remove once the UI fully migrates to SSE.
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

	// Task stats (requires auth)
	s.mux.Handle("GET /api/tasks/stats", s.authMiddleware(http.HandlerFunc(s.handleTaskStats)))

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
	s.mux.Handle("GET /api/insights", s.authMiddleware(http.HandlerFunc(s.handleInsights)))

	// Config endpoint (requires auth)
	s.mux.Handle("GET /api/config", s.authMiddleware(http.HandlerFunc(s.handleConfig)))

	// Billing info endpoint (requires auth)
	s.mux.Handle("GET /api/billing", s.authMiddleware(http.HandlerFunc(s.handleBillingInfo)))

	// Orchestrator & system status (requires auth)
	s.mux.Handle("GET /api/orchestrator/status", s.authMiddleware(http.HandlerFunc(s.handleOrchestratorStatus)))
	s.mux.Handle("GET /api/system/disk", s.authMiddleware(http.HandlerFunc(s.handleDiskUsage)))
	s.mux.Handle("GET /api/system/health", s.authMiddleware(http.HandlerFunc(s.handleSystemHealth)))

	// Deploy endpoints (requires auth)
	s.mux.Handle("GET /api/system/deploy/status", s.authMiddleware(http.HandlerFunc(s.handleDeployStatus)))
	s.mux.Handle("POST /api/system/deploy", s.authMiddleware(http.HandlerFunc(s.handleDeploy)))
	s.mux.Handle("POST /api/system/deploy/check-remote", s.authMiddleware(http.HandlerFunc(s.handleCheckRemote)))

	// Archive endpoint (requires auth)
	s.mux.Handle("POST /api/tasks/{id}/archive", s.authMiddleware(http.HandlerFunc(s.handleArchiveTask)))

	// Task attempts API (requires auth)
	s.mux.Handle("GET /api/tasks/{id}/attempts", s.authMiddleware(http.HandlerFunc(s.handleListAttempts)))

	// Task usage API (requires auth)
	s.mux.Handle("GET /api/tasks/{id}/usage", s.authMiddleware(http.HandlerFunc(s.handleListUsageEvents)))

	// Subtasks API (requires auth)
	s.mux.Handle("GET /api/tasks/{id}/subtasks", s.authMiddleware(http.HandlerFunc(s.handleListSubtasks)))
	s.mux.Handle("GET /api/tasks/{id}/subtasks/dependencies", s.authMiddleware(http.HandlerFunc(s.handleGetSubtaskDependencies)))

	// Internal API (localhost only, no auth)
	s.mux.Handle("POST /internal/tasks/next", s.localhostOnly(http.HandlerFunc(s.handleInternalNextTask)))
	s.mux.Handle("POST /internal/tasks/next-pending", s.localhostOnly(http.HandlerFunc(s.handleInternalNextPending)))
	s.mux.Handle("POST /internal/tasks/{id}/started", s.localhostOnly(http.HandlerFunc(s.handleInternalTaskStarted)))
	s.mux.Handle("POST /internal/tasks/{id}/done", s.localhostOnly(http.HandlerFunc(s.handleInternalTaskDone)))
	s.mux.Handle("POST /internal/tasks/{id}/triaged", s.localhostOnly(http.HandlerFunc(s.handleInternalTriaged)))
	s.mux.Handle("POST /internal/tasks", s.localhostOnly(http.HandlerFunc(s.handleInternalCreateTask)))
	s.mux.Handle("POST /internal/subtasks", s.localhostOnly(http.HandlerFunc(s.handleInternalCreateSubtasks)))
	s.mux.Handle("GET /internal/model/{task_id}", s.localhostOnly(http.HandlerFunc(s.handleInternalGetModel)))
	s.mux.Handle("GET /internal/tasks/{id}/status", s.localhostOnly(http.HandlerFunc(s.handleInternalTaskStatus)))
	s.mux.Handle("POST /internal/tasks/{id}/usage", s.localhostOnly(http.HandlerFunc(s.handleInternalTaskUsage)))
	s.mux.Handle("GET /internal/projects/{id}", s.localhostOnly(http.HandlerFunc(s.handleInternalGetProject)))

	// Knowledge API (requires auth)
	s.registerKnowledgeRoutes()

	// Insights API (requires auth)
	s.registerInsightsRoutes()

	// PR Review API (requires auth)
	s.RegisterPRRoutes()

	// Logs API (requires auth)
	s.mux.Handle("GET /api/logs/recent", s.authMiddleware(http.HandlerFunc(s.handleRecentLogs)))

	// Pods API (requires auth)
	s.mux.Handle("GET /api/pods", s.authMiddleware(http.HandlerFunc(s.handleListPods)))

	// Pod registration (internal, localhost only)
	s.mux.Handle("POST /internal/pods/register", s.localhostOnly(http.HandlerFunc(s.handlePodRegister)))
	s.mux.Handle("POST /internal/pods/status", s.localhostOnly(http.HandlerFunc(s.handlePodStatus)))

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
		if _, err := fs.Stat(s.webFS, path[1:]); err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

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
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"version":      version,
		"auth_enabled": s.config.Server.Auth.Enabled,
	})
}

// version is set via ServerDeps.Version during initialization.
var version = "dev"

// Common error messages returned by API handlers.
const (
	errInternalServer   = "internal server error"
	errInvalidBody      = "invalid request body"
	errTaskNotFound     = "task not found"
	errGoalNotFound     = "goal not found"
	errProjectNotFound  = "project not found"
)

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// serverError logs the error and writes a 500 JSON response.
func serverError(w http.ResponseWriter, msg string, args ...any) {
	slog.Error(msg, args...)
	writeError(w, http.StatusInternalServerError, errInternalServer)
}

// Mux returns the HTTP mux for mounting additional handlers (e.g. Connect-RPC).
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// Hub returns the WebSocket hub for external wiring (e.g. log broadcasting).
func (s *Server) Hub() *WebSocketHub {
	return s.ws
}

// PodRegistry returns the pod registry for external access (e.g. executor registration).
func (s *Server) PodRegistry() *PodRegistry {
	return s.podRegistry
}

// WrapHandler replaces the server's HTTP handler (e.g. to add h2c or CORS).
func (s *Server) WrapHandler(wrap func(http.Handler) http.Handler) {
	s.server.Handler = wrap(s.server.Handler)
}

// SetLogHandler sets the log broadcast handler so the server can serve recent logs.
func (s *Server) SetLogHandler(h *LogBroadcastHandler) {
	s.logHandler = h
}

// SetOrchestrator sets the orchestrator for status reporting.
func (s *Server) SetOrchestrator(o *orchestrator.Orchestrator) {
	s.orch = o
}

// SetScaleManager sets the scale manager for status reporting.
func (s *Server) SetScaleManager(sm *orchestrator.ScaleManager) {
	s.scaleManager = sm
}

// SetCleaner sets the cleaner for disk status reporting.
func (s *Server) SetCleaner(c *cleanup.Cleaner) {
	s.cleaner = c
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
