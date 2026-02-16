package main

import (
	"context"
	"flag"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/circle-oo/flux/gen/flux/v1/fluxv1connect"
	"github.com/circle-oo/flux/internal/agent"
	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/db"
	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/handler"
	"github.com/circle-oo/flux/internal/manager"
	"github.com/circle-oo/flux/internal/notesmd"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/server"
	"github.com/circle-oo/flux/internal/shutdown"
	"github.com/circle-oo/flux/internal/triager"
	"github.com/circle-oo/flux/internal/updater"
	"github.com/circle-oo/flux/internal/vault"
	"github.com/circle-oo/flux/web"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	resolved := resolveConfigPath(*configPath)
	cfg, err := config.Load(resolved)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	logger, err := setupLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info("flux starting", "version", version, "port", cfg.Server.Port)

	// 1. Open/create SQLite DB
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// 2. Initialize Discord notifier
	discord := notifier.NewDiscord(cfg.Notifications.Discord.WebhookURL)

	// 3. Convert seed projects from config
	seedProjects := make([]db.SeedProject, len(cfg.Projects))
	for i, p := range cfg.Projects {
		seedProjects[i] = db.SeedProject{
			Name:        p.Name,
			Type:        p.Type,
			RepoURL:     p.RepoURL,
			Description: p.Description,
			TechStack:   p.TechStack,
		}
	}

	// 4. Run bootstrap: schema + vault dirs + seed projects
	if err := db.Bootstrap(database, cfg.Vault.Path, seedProjects); err != nil {
		logger.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	logger.Info("bootstrap complete")

	// 4b. Recover from crash: move any RUNNING tasks to RETRY
	if err := shutdown.RecoverFromCrash(database, discord); err != nil {
		logger.Error("crash recovery failed", "error", err)
	}

	// 4c. Initialize Vault Writer (via notesmd-cli)
	notesmdClient := notesmd.NewClient(cfg.Vault.Name)
	vaultWriter := vault.NewWriter(notesmdClient)
	defer vaultWriter.Close()

	// 5. Initialize Manager
	mgr := manager.NewManager(database, cfg)

	// 6. Create and start HTTP server
	webFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		logger.Error("failed to get embedded web filesystem", "error", err)
		os.Exit(1)
	}
	srv := server.NewServer(server.ServerDeps{
		Config:  cfg,
		DB:      database,
		Manager: mgr,
		Discord: discord,
		WebFS:   webFS,
		Version: version,
	})

	// 6a. Connect-RPC: Python Agent Manager client + FluxService handler
	agentClient, agentErr := agent.NewClient("localhost:50051", logger)
	if agentErr != nil {
		logger.Warn("agent manager not available, Connect-RPC and gRPC execution disabled", "error", agentErr)
	} else {
		fluxHandler := handler.NewFluxServiceHandler(agentClient, logger)
		path, connectHandler := fluxv1connect.NewFluxServiceHandler(fluxHandler)
		srv.Mux().Handle(path, connectHandler)
		logger.Info("connect-rpc enabled", "path", path)

		// Health check: verify Python Agent Manager connectivity in background
		go waitForAgentManager(agentClient, logger)
	}
	// Note: agentClient may be nil if connection failed. Executor handles this
	// gracefully by returning an error from runExecution when agentClient is nil.

	// 6a-ii. Wrap server handler with h2c (HTTP/2 cleartext) and CORS for Connect-RPC
	srv.WrapHandler(func(h http.Handler) http.Handler {
		return h2c.NewHandler(server.WithCORS(h), &http2.Server{})
	})

	// 6b. Wrap logger with broadcast handler so logs stream to the Web UI
	logBroadcast := server.NewLogBroadcastHandler(slog.Default().Handler(), srv.Hub())
	broadcastLogger := slog.New(logBroadcast)
	slog.SetDefault(broadcastLogger)
	logger = broadcastLogger
	srv.SetLogHandler(logBroadcast)

	// 7. Send Discord notification
	discord.Send(notifier.LevelInfo, "Flux initialized. Please set a Goal.")

	// 8. Start HTTP server in background
	go func() {
		if err := srv.Start(); err != nil {
			// http.ErrServerClosed is expected on graceful shutdown
			if !errors.Is(err, http.ErrServerClosed) {
				logger.Error("HTTP server error", "error", err)
				os.Exit(1)
			}
		}
	}()

	// 9. Start Executor pods
	ctx, ctxCancel := context.WithCancel(context.Background())
	executorCount := cfg.Orchestrator.MaxTotalPods
	executors := make([]*executor.Executor, executorCount)
	for i := 0; i < executorCount; i++ {
		execID := fmt.Sprintf("executor-%02d", i+1)
		executors[i] = executor.NewExecutor(execID, cfg, discord, vaultWriter, agentClient)
		go func(e *executor.Executor, id string) {
			logger.Info("executor pod started", "id", id)
			e.Run(ctx)
		}(executors[i], execID)
	}

	// 9b. Start Triager component (if enabled)
	var triage *triager.Triager
	if cfg.Triager.Enabled {
		triage = triager.New("triager-01", cfg, discord, agentClient)
		go func() {
			logger.Info("triager pod started", "id", "triager-01", "component", "main")
			triage.Run(ctx)
		}()
	} else {
		logger.Info("triager disabled", "component", "main")
	}

	// 10. Initialize updater (always create instance for manual checks)
	var autoUpdater *updater.Updater
	repoDir, err := os.Getwd()
	if err != nil {
		logger.Error("failed to get working directory for updater", "error", err)
	} else {
		autoUpdater = updater.New(cfg.AutoUpdate, repoDir)
		autoUpdater.OnStatusChange = func(status updater.Status) {
			srv.Hub().Broadcast(server.Event{Type: "DEPLOY_STATUS", Data: status})
		}

		// Only start automatic polling if auto-updater is enabled
		if cfg.AutoUpdate.Enabled {
			go autoUpdater.Start(ctx)
		}
	}

	// Wire updater into server for deploy API (manual checks work even if auto-update disabled)
	if autoUpdater != nil {
		srv.SetUpdater(autoUpdater)
	}

	logger.Info("flux ready", "port", cfg.Server.Port)

	// 11. Block on SIGTERM/SIGINT for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	// Stop auto-updater
	if autoUpdater != nil {
		autoUpdater.Stop()
	}

	// Build pod list for graceful shutdown
	pods := make([]shutdown.Pod, 0, executorCount+1)
	for _, exec := range executors {
		pods = append(pods, exec)
	}
	if triage != nil {
		pods = append(pods, triage)
	}

	// Initiate graceful shutdown with context and timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Shutdown.PodGracePeriod)
	defer shutdownCancel()

	// Cancel executor/triager context to signal them to stop accepting new work
	ctxCancel()

	// Use GracefulShutdown to coordinate pod termination
	if err := shutdown.GracefulShutdown(shutdownCtx, &cfg.Shutdown, pods, database, discord); err != nil {
		logger.Error("graceful shutdown error", "error", err)
	}

	// Clean up incomplete worktrees to prevent disk space leaks
	if err := shutdown.CleanupIncompleteWorktrees(cfg.Orchestrator.WorkspaceBase, database); err != nil {
		logger.Error("worktree cleanup error", "error", err)
	}

	// Shutdown HTTP server
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// Close agent client connection
	if agentClient != nil {
		if err := agentClient.Close(); err != nil {
			logger.Error("agent client close error", "error", err)
		}
	}

	logger.Info("flux stopped")
}

// waitForAgentManager polls the Python Agent Manager until it responds.
// Runs in background so the Go server can start accepting HTTP requests immediately.
func waitForAgentManager(ac *agent.Client, logger *slog.Logger) {
	for attempt := 1; attempt <= 30; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := ac.PodStatus(ctx)
		cancel()
		if err == nil {
			logger.Info("python agent manager is healthy", "attempts", attempt)
			return
		}
		logger.Warn("waiting for python agent manager",
			"attempt", attempt, "error", err)
		time.Sleep(2 * time.Second)
	}
	logger.Error("python agent manager not reachable after 30 attempts, continuing anyway")
}

func setupLogger(lc config.LoggingConfig) (*slog.Logger, error) {
	var level slog.Level
	switch lc.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	if lc.File != "" {
		if err := os.MkdirAll(filepath.Dir(lc.File), 0755); err != nil {
			return nil, fmt.Errorf("create logs directory: %w", err)
		}
		f, err := os.OpenFile(lc.File, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		handler := slog.NewJSONHandler(f, opts)
		logger := slog.New(handler)
		slog.SetDefault(logger)
		return logger, nil
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}

// resolveConfigPath finds the config file in order:
// 1. Explicit --config flag value
// 2. config.yaml next to the binary
// 3. config.yaml in the current working directory
// 4. Falls back to "config.yaml" (will error on load if missing)
func resolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}

	// Next to the binary
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "..", "config.yaml")
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	// Current working directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	return "config.yaml"
}
