package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/db"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/server"
	"github.com/circle-oo/flux/web"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
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

	// 5. Set server version
	server.SetVersion(version)

	// 6. Create and start HTTP server
	webFS, err := fs.Sub(web.DistFS, "dist")
	if err != nil {
		logger.Error("failed to get embedded web filesystem", "error", err)
		os.Exit(1)
	}
	srv := server.NewServer(cfg, database, discord, webFS)

	// 7. Send Discord notification
	discord.Send(notifier.LevelInfo, "Flux initialized. Please set a Goal.")

	// 8. Start HTTP server in background
	go func() {
		if err := srv.Start(); err != nil {
			// http.ErrServerClosed is expected on graceful shutdown
			if err.Error() != "http: Server closed" {
				logger.Error("HTTP server error", "error", err)
				os.Exit(1)
			}
		}
	}()

	logger.Info("flux ready", "port", cfg.Server.Port)

	// 9. Block on SIGTERM/SIGINT for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	logger.Info("shutting down", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.PodGracePeriod)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
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
		if err := os.MkdirAll("logs", 0755); err != nil {
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
