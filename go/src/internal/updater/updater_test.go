package updater

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/circle-oo/flux/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.AutoUpdateConfig{
		Enabled:       true,
		CheckInterval: 10 * time.Minute,
		Branch:        "main",
	}
	u := New(cfg, "/tmp/repo")

	if u.repoDir != "/tmp/repo" {
		t.Errorf("expected repoDir /tmp/repo, got %s", u.repoDir)
	}
	if u.cfg.Branch != "main" {
		t.Errorf("expected branch main, got %s", u.cfg.Branch)
	}
	if u.cfg.CheckInterval != 10*time.Minute {
		t.Errorf("expected interval 10m, got %v", u.cfg.CheckInterval)
	}
}

func TestNew_Defaults(t *testing.T) {
	cfg := config.AutoUpdateConfig{
		Enabled: true,
	}
	u := New(cfg, "/tmp/repo")

	// Branch defaults handled in Start, but config should be empty
	if u.cfg.Branch != "" {
		t.Errorf("expected empty branch (default applied at runtime), got %s", u.cfg.Branch)
	}
}

// initTestRepo creates a bare remote + a cloned working repo for testing.
// Returns (workingDir, cleanup).
func initTestRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	base := t.TempDir()
	bare := filepath.Join(base, "remote.git")
	work := filepath.Join(base, "work")

	// Create bare remote with explicit main branch
	run(t, base, "git", "init", "--bare", "--initial-branch=main", bare)

	// Clone it
	run(t, base, "git", "clone", bare, work)

	// Configure git user in working repo
	run(t, work, "git", "config", "user.email", "test@test.com")
	run(t, work, "git", "config", "user.name", "Test")

	// Create initial commit on main
	initial := filepath.Join(work, "README.md")
	if err := os.WriteFile(initial, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run(t, work, "git", "add", ".")
	run(t, work, "git", "commit", "-m", "initial")
	run(t, work, "git", "branch", "-M", "main")
	run(t, work, "git", "push", "-u", "origin", "main")

	return work
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %s %v failed: %v\n%s", name, args, err, out)
	}
}

func TestGitOutput(t *testing.T) {
	work := initTestRepo(t)

	cfg := config.AutoUpdateConfig{Enabled: true, Branch: "main"}
	u := New(cfg, work)

	ctx := context.Background()

	// Should be able to get HEAD
	head, err := u.gitOutput(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("gitOutput: %v", err)
	}
	if len(head) != 40 {
		t.Errorf("expected 40-char SHA, got %q (len %d)", head, len(head))
	}
}

func TestCheckAndUpdate_NoChanges(t *testing.T) {
	work := initTestRepo(t)

	cfg := config.AutoUpdateConfig{Enabled: true, Branch: "main"}
	u := New(cfg, work)

	ctx := context.Background()

	// No new commits on remote — should return nil (no update needed)
	err := u.checkAndUpdate(ctx, "main")
	if err != nil {
		t.Fatalf("checkAndUpdate with no changes: %v", err)
	}
}

func TestCheckAndUpdate_DetectsNewCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	base := t.TempDir()
	bare := filepath.Join(base, "remote.git")
	work1 := filepath.Join(base, "work1") // simulates "deployed" clone
	work2 := filepath.Join(base, "work2") // simulates a developer pushing

	// Create bare remote with explicit main branch
	run(t, base, "git", "init", "--bare", "--initial-branch=main", bare)

	// Clone twice
	run(t, base, "git", "clone", bare, work1)
	run(t, base, "git", "clone", bare, work2)

	// Configure both
	for _, w := range []string{work1, work2} {
		run(t, w, "git", "config", "user.email", "test@test.com")
		run(t, w, "git", "config", "user.name", "Test")
	}

	// Create initial commit from work1
	if err := os.WriteFile(filepath.Join(work1, "README.md"), []byte("# v1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, work1, "git", "add", ".")
	run(t, work1, "git", "commit", "-m", "v1")
	run(t, work1, "git", "branch", "-M", "main")
	run(t, work1, "git", "push", "-u", "origin", "main")

	// Sync work2
	run(t, work2, "git", "pull", "origin", "main")

	// Push a new commit from work2 (simulating a developer push)
	if err := os.WriteFile(filepath.Join(work2, "update.txt"), []byte("new stuff\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run(t, work2, "git", "add", ".")
	run(t, work2, "git", "commit", "-m", "v2")
	run(t, work2, "git", "push", "origin", "main")

	// Now work1 (the "deployed" clone) should detect the update
	cfg := config.AutoUpdateConfig{Enabled: true, Branch: "main"}
	u := New(cfg, work1)

	ctx := context.Background()

	// Fetch to see new commits
	if err := u.gitCommand(ctx, "fetch", "origin", "main"); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	localHead, err := u.gitOutput(ctx, "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("local HEAD: %v", err)
	}

	remoteHead, err := u.gitOutput(ctx, "rev-parse", "origin/main")
	if err != nil {
		t.Fatalf("remote HEAD: %v", err)
	}

	if localHead == remoteHead {
		t.Fatal("expected local and remote to differ, but they are the same")
	}

	// Note: We don't call checkAndUpdate because it would try to run `make build`
	// and send SIGTERM. Instead, we verified the detection logic above.
	// The full integration is tested manually.
}

func TestStartStop(t *testing.T) {
	work := initTestRepo(t)

	cfg := config.AutoUpdateConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Hour, // long interval so it doesn't fire
		Branch:        "main",
	}
	u := New(cfg, work)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		u.Start(ctx)
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop should return promptly
	u.Stop()

	select {
	case <-done:
		// Success — Start returned after Stop
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after Stop within 5 seconds")
	}
}
