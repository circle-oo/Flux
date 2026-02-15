package updater

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/circle-oo/flux/internal/config"
)

// Status represents the current state of the auto-updater.
type Status struct {
	Enabled       bool      `json:"enabled"`
	Running       bool      `json:"running"`
	State         string    `json:"state"` // "idle", "checking", "updating", "restarting", "error"
	Branch        string    `json:"branch"`
	CheckInterval string    `json:"check_interval"`
	LastCheckAt   *string   `json:"last_check_at"`
	LastUpdateAt  *string   `json:"last_update_at"`
	LastError     string    `json:"last_error,omitempty"`
	LocalCommit   string    `json:"local_commit,omitempty"`
	RemoteCommit  string    `json:"remote_commit,omitempty"`
	UpdateCount   int       `json:"update_count"`
	NextCheckAt   *string   `json:"next_check_at"`
}

// Updater polls the git remote for new commits and triggers a rebuild + restart.
// It relies on launchd (KeepAlive: true) to restart the process after exit.
type Updater struct {
	cfg     config.AutoUpdateConfig
	repoDir string // root of the git repository
	cancel  context.CancelFunc
	done    chan struct{}

	mu           sync.RWMutex
	running      bool
	state        string
	lastCheckAt  *time.Time
	lastUpdateAt *time.Time
	lastError    string
	localCommit  string
	remoteCommit string
	updateCount  int
	nextCheckAt  *time.Time

	// OnStatusChange is called when the updater state changes.
	// Set this before calling Start. The callback receives a copy of the status.
	OnStatusChange func(Status)
}

// New creates an Updater. repoDir should be the flux repository root.
func New(cfg config.AutoUpdateConfig, repoDir string) *Updater {
	return &Updater{
		cfg:     cfg,
		repoDir: repoDir,
		done:    make(chan struct{}),
		state:   "idle",
	}
}

// Status returns a snapshot of the updater's current state.
func (u *Updater) Status() Status {
	u.mu.RLock()
	defer u.mu.RUnlock()

	branch := u.cfg.Branch
	if branch == "" {
		branch = "main"
	}

	interval := u.cfg.CheckInterval
	if interval < 1*time.Minute {
		interval = 5 * time.Minute
	}

	s := Status{
		Enabled:       u.cfg.Enabled,
		Running:       u.running,
		State:         u.state,
		Branch:        branch,
		CheckInterval: interval.String(),
		LastError:     u.lastError,
		LocalCommit:   u.localCommit,
		RemoteCommit:  u.remoteCommit,
		UpdateCount:   u.updateCount,
	}

	if u.lastCheckAt != nil {
		t := u.lastCheckAt.UTC().Format(time.RFC3339)
		s.LastCheckAt = &t
	}
	if u.lastUpdateAt != nil {
		t := u.lastUpdateAt.UTC().Format(time.RFC3339)
		s.LastUpdateAt = &t
	}
	if u.nextCheckAt != nil {
		t := u.nextCheckAt.UTC().Format(time.RFC3339)
		s.NextCheckAt = &t
	}

	return s
}

func (u *Updater) setState(state string) {
	u.mu.Lock()
	u.state = state
	u.mu.Unlock()
	u.notifyChange()
}

func (u *Updater) notifyChange() {
	if u.OnStatusChange != nil {
		u.OnStatusChange(u.Status())
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

	u.mu.Lock()
	u.running = true
	u.mu.Unlock()

	slog.Info("auto-updater started", "interval", interval, "branch", branch)
	u.notifyChange()

	// Get initial local commit
	if head, err := u.gitOutput(ctx, "rev-parse", "HEAD"); err == nil {
		u.mu.Lock()
		u.localCommit = head
		u.mu.Unlock()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		// Calculate next check time
		u.mu.Lock()
		next := time.Now().Add(interval)
		u.nextCheckAt = &next
		u.mu.Unlock()

		select {
		case <-ctx.Done():
			u.mu.Lock()
			u.running = false
			u.state = "idle"
			u.mu.Unlock()
			slog.Info("auto-updater stopped")
			u.notifyChange()
			return
		case <-ticker.C:
			if err := u.checkAndUpdate(ctx, branch); err != nil {
				slog.Error("auto-updater check failed", "error", err)
				u.mu.Lock()
				u.lastError = err.Error()
				u.state = "error"
				u.mu.Unlock()
				u.notifyChange()
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

// TriggerUpdate manually triggers an update check and deploy.
// Returns nil if no update was needed, or an error if something failed.
// If an update is applied, the process will receive SIGTERM.
func (u *Updater) TriggerUpdate(ctx context.Context) error {
	branch := u.cfg.Branch
	if branch == "" {
		branch = "main"
	}

	slog.Info("manual deploy triggered", "branch", branch)
	return u.doUpdate(ctx, branch, true)
}

// FetchRemoteCommit fetches the latest remote commit without deploying.
// This updates the RemoteCommit field in the status so the user can see
// if an update is available before deciding to deploy.
func (u *Updater) FetchRemoteCommit(ctx context.Context) error {
	branch := u.cfg.Branch
	if branch == "" {
		branch = "main"
	}

	slog.Info("fetching remote commit", "branch", branch)
	u.setState("checking")

	// Fetch latest from remote
	if err := u.gitCommand(ctx, "fetch", "origin", branch); err != nil {
		u.setState("error")
		return fmt.Errorf("git fetch: %w", err)
	}

	// Get local HEAD
	localHead, err := u.gitOutput(ctx, "rev-parse", "HEAD")
	if err != nil {
		u.setState("error")
		return fmt.Errorf("get local HEAD: %w", err)
	}

	// Get remote HEAD
	remoteRef := "origin/" + branch
	remoteHead, err := u.gitOutput(ctx, "rev-parse", remoteRef)
	if err != nil {
		u.setState("error")
		return fmt.Errorf("get remote HEAD: %w", err)
	}

	u.mu.Lock()
	u.localCommit = localHead
	u.remoteCommit = remoteHead
	now := time.Now()
	u.lastCheckAt = &now
	u.lastError = ""
	u.mu.Unlock()

	u.setState("idle")
	u.notifyChange()

	slog.Info("remote commit fetched",
		"local", localHead[:8],
		"remote", remoteHead[:8],
		"update_available", localHead != remoteHead,
	)

	return nil
}

// checkAndUpdate fetches the remote, compares HEAD with remote branch,
// and if there are new commits: pulls, rebuilds, and signals the process to restart.
func (u *Updater) checkAndUpdate(ctx context.Context, branch string) error {
	u.setState("checking")

	now := time.Now()
	u.mu.Lock()
	u.lastCheckAt = &now
	u.lastError = ""
	u.mu.Unlock()
	u.notifyChange()

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

	u.mu.Lock()
	u.localCommit = localHead
	u.remoteCommit = remoteHead
	u.mu.Unlock()

	if localHead == remoteHead {
		slog.Debug("auto-updater: up to date", "head", localHead[:8])
		u.setState("idle")
		return nil
	}

	slog.Info("auto-updater: update available",
		"local", localHead[:8],
		"remote", remoteHead[:8],
		"branch", branch,
	)

	return u.doUpdate(ctx, branch, false)
}

// doUpdate performs the actual pull + build + restart sequence.
func (u *Updater) doUpdate(ctx context.Context, branch string, force bool) error {
	u.setState("updating")

	// If force (manual deploy), always fetch first
	if force {
		if err := u.gitCommand(ctx, "fetch", "origin", branch); err != nil {
			return fmt.Errorf("git fetch: %w", err)
		}
	}

	// Pull changes
	if err := u.gitCommand(ctx, "pull", "origin", branch); err != nil {
		return fmt.Errorf("git pull: %w", err)
	}

	slog.Info("updater: pulled latest changes")

	// Get new local HEAD after pull
	if head, err := u.gitOutput(ctx, "rev-parse", "HEAD"); err == nil {
		u.mu.Lock()
		u.localCommit = head
		u.mu.Unlock()
	}

	// Rebuild
	if err := u.rebuild(ctx); err != nil {
		return fmt.Errorf("rebuild: %w", err)
	}

	now := time.Now()
	u.mu.Lock()
	u.lastUpdateAt = &now
	u.updateCount++
	u.mu.Unlock()

	slog.Info("updater: rebuild complete, restarting...")
	u.setState("restarting")

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
