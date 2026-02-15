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

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/db"
	"github.com/circle-oo/flux/internal/executor"
	"github.com/circle-oo/flux/internal/manager"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/server"
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
			Name:    p.Name,
			Type:    p.Type,
			RepoURL: p.RepoURL,
		}
	}

	// 4. Run bootstrap: schema + vault dirs + seed projects
	if err := db.Bootstrap(database, cfg.Vault.Path, seedProjects); err != nil {
		logger.Error("bootstrap failed", "error", err)
		os.Exit(1)
	}
	logger.Info("bootstrap complete")

	// 5. Initialize Manager and wire into server
	mgr := manager.NewManager(database, cfg)
	server.SetManager(mgr)
	server.SetVersion(version)

	// 6. Create and start HTTP server
	webFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		logger.Error("failed to get embedded web filesystem", "error", err)
		os.Exit(1)
	}
	srv := server.NewServer(cfg, database, discord, webFS)

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

	// 9. Start Executor pod
	ctx, ctxCancel := context.WithCancel(context.Background())
	exec := executor.NewExecutor("executor-01", cfg, discord)
	go func() {
		logger.Info("executor pod started", "id", "executor-01")
		exec.Run(ctx)
	}()

	logger.Info("flux ready", "port", cfg.Server.Port)

	// 10. Block on SIGTERM/SIGINT for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	// Stop executor
	ctxCancel()
	exec.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Shutdown.PodGracePeriod)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	logger.Info("flux stopped")
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
