package updater

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/circle-oo/flux/internal/config"
)

// Updater polls the git remote for new commits and triggers a rebuild + restart.
// It relies on launchd (KeepAlive: true) to restart the process after exit.
type Updater struct {
	cfg     config.AutoUpdateConfig
	repoDir string // root of the git repository
	cancel  context.CancelFunc
	done    chan struct{}
}

// New creates an Updater. repoDir should be the flux repository root.
func New(cfg config.AutoUpdateConfig, repoDir string) *Updater {
	return &Updater{
		cfg:     cfg,
		repoDir: repoDir,
		done:    make(chan struct{}),
	}
}

// Start begins the polling loop. It blocks until ctx is cancelled.
func (u *Updater) Start(ctx context.Context) {
	ctx, u.cancel = context.WithCancel(ctx)
	defer close(u.done)

	interval := u.cfg.CheckInterval
	if interval < 1*time.Minute {
		interval = 5 * time.Minute
	}

	branch := u.cfg.Branch
	if branch == "" {
		branch = "main"
	}

	slog.Info("auto-updater started", "interval", interval, "branch", branch)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("auto-updater stopped")
			return
		case <-ticker.C:
			if err := u.checkAndUpdate(ctx, branch); err != nil {
				slog.Error("auto-updater check failed", "error", err)
			}
		}
	}
}

// Stop cancels the polling loop and waits for it to finish.
func (u *Updater) Stop() {
	if u.cancel != nil {
		u.cancel()
	}
	<-u.done
}

// checkAndUpdate fetches the remote, compares HEAD with remote branch,
// and if there are new commits: pulls, rebuilds, and signals the process to restart.
func (u *Updater) checkAndUpdate(ctx context.Context, branch string) error {
	// Fetch latest from remote
	if err := u.gitCommand(ctx, "fetch", "origin", branch); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	// Get local and remote HEADs
	localHead, err := u.gitOutput(ctx, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("get local HEAD: %w", err)
	}

	remoteRef := "origin/" + branch
	remoteHead, err := u.gitOutput(ctx, "rev-parse", remoteRef)
	if err != nil {
		return fmt.Errorf("get remote HEAD: %w", err)
	}

	if localHead == remoteHead {
		slog.Debug("auto-updater: up to date", "head", localHead[:8])
		return nil
	}

	slog.Info("auto-updater: update available",
		"local", localHead[:8],
		"remote", remoteHead[:8],
		"branch", branch,
	)

	// Pull changes
	if err := u.gitCommand(ctx, "pull", "origin", branch); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	slog.Info("auto-updater: pulled latest changes")

	// Rebuild
	if err := u.rebuild(ctx); err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}

	slog.Info("auto-updater: rebuild complete, restarting...")

	// Signal self to shut down. launchd KeepAlive will restart the new binary.
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return fmt.Errorf("find self process: %w", err)
	}
	return p.Signal(syscall.SIGTERM)
}

// rebuild runs "make build" in the repo directory.
func (u *Updater) rebuild(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "make", "build")
	cmd.Dir = u.repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitCommand runs a git command in the repo directory.
func (u *Updater) gitCommand(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = u.repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// gitOutput runs a git command and returns trimmed stdout.
func (u *Updater) gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = u.repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
