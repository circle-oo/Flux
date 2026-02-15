package executor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// WorktreeManager manages git worktrees for task execution
type WorktreeManager struct {
	reposDir    string // workspaces/repos/
	treesDir    string // workspaces/trees/
	githubToken string
	githubUser  string
}

// WorktreeTask represents a task's worktree state for cleanup decisions
type WorktreeTask struct {
	ProjectName string
	TaskID      string
	Status      string
	PRStatus    string
	CompletedAt time.Time
}

// NewWorktreeManager creates a new WorktreeManager.
// Paths are resolved to absolute to avoid mismatches between git -C and executor CWD.
func NewWorktreeManager(workspaceBase, githubToken, githubUser string) *WorktreeManager {
	absBase, err := filepath.Abs(workspaceBase)
	if err != nil {
		absBase = workspaceBase
	}
	wm := &WorktreeManager{
		reposDir:    filepath.Join(absBase, "repos"),
		treesDir:    filepath.Join(absBase, "trees"),
		githubToken: githubToken,
		githubUser:  githubUser,
	}
	slog.Debug("worktree manager created", "repos_dir", wm.reposDir, "trees_dir", wm.treesDir)
	return wm
}

// EnsureBareRepo ensures a bare repository exists and is up to date.
// Uses token-based HTTPS URL when GitHub credentials are available.
func (wm *WorktreeManager) EnsureBareRepo(repoURL, projectName string) error {
	slog.Debug("ensuring bare repo", "repo_url", repoURL, "project", projectName)
	bareDir := filepath.Join(wm.reposDir, projectName+".git")
	cloneURL := wm.tokenURL(repoURL)

	// Check if bare repo exists
	if _, err := os.Stat(bareDir); err == nil {
		// Update remote URL in case credentials changed
		if cloneURL != repoURL {
			setURLCmd := exec.Command("git", "-C", bareDir, "remote", "set-url", "origin", cloneURL)
			_ = setURLCmd.Run()
		}
		// Repository exists, fetch updates
		slog.Debug("fetching updates for bare repo", "project", projectName)
		cmd := exec.Command("git", "-C", bareDir, "fetch", "--all")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to fetch repo: %w: %s", err, output)
		}
		slog.Info("bare repo ready", "project", projectName)
		return nil
	}

	// Repository doesn't exist, clone it
	if err := os.MkdirAll(wm.reposDir, 0755); err != nil {
		return fmt.Errorf("failed to create repos directory: %w", err)
	}

	slog.Info("cloning bare repo", "project", projectName)
	cmd := exec.Command("git", "clone", "--bare", cloneURL, bareDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone bare repo: %w: %s", err, output)
	}

	slog.Info("bare repo ready", "project", projectName)
	return nil
}

// tokenURL converts a repo URL to token-based HTTPS if credentials are available.
func (wm *WorktreeManager) tokenURL(repoURL string) string {
	if wm.githubToken == "" {
		return repoURL
	}
	owner, repo := extractOwnerRepo(repoURL)
	if owner == "" || repo == "" {
		return repoURL
	}
	user := wm.githubUser
	if user == "" {
		user = "flux-bot"
	}
	return fmt.Sprintf("https://%s:%s@github.com/%s/%s.git", user, wm.githubToken, owner, repo)
}

// CreateWorktree creates a new worktree for a task
func (wm *WorktreeManager) CreateWorktree(projectName, taskID string) (worktreePath string, branchName string, err error) {
	slog.Info("creating worktree", "project", projectName, "task_id", taskID, "branch", fmt.Sprintf("task/%s", taskID[:8]))
	bareDir := filepath.Join(wm.reposDir, projectName+".git")
	branchName = fmt.Sprintf("task/%s", taskID[:8])
	worktreePath = filepath.Join(wm.treesDir, fmt.Sprintf("%s--task-%s", projectName, taskID[:8]))

	// Ensure trees directory exists
	if err := os.MkdirAll(wm.treesDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create trees directory: %w", err)
	}

	// Clean up ANY existing worktree using this branch (may be at old wrong paths)
	wm.cleanupBranchWorktree(bareDir, branchName)

	// Clean up stale worktree directory if it still exists
	if _, statErr := os.Stat(worktreePath); statErr == nil {
		slog.Info("removing stale worktree directory", "path", worktreePath)
		if err := os.RemoveAll(worktreePath); err != nil {
			return "", "", fmt.Errorf("failed to remove stale worktree directory: %w", err)
		}
		pruneCmd := exec.Command("git", "-C", bareDir, "worktree", "prune")
		_ = pruneCmd.Run()
	}

	// Check if branch already exists (from a previous failed attempt / retry)
	checkCmd := exec.Command("git", "-C", bareDir, "rev-parse", "--verify", "refs/heads/"+branchName)
	var cmd *exec.Cmd
	if checkCmd.Run() == nil {
		// Branch exists — create worktree using existing branch
		slog.Info("reusing existing branch for retry", "branch", branchName)
		cmd = exec.Command("git", "-C", bareDir, "worktree", "add", worktreePath, branchName)
	} else {
		// Branch doesn't exist — create new branch from main
		cmd = exec.Command("git", "-C", bareDir, "worktree", "add", "-b", branchName, worktreePath, "main")
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("failed to create worktree: %w: %s", err, output)
	}
	slog.Debug("worktree directory created", "path", worktreePath)

	// Configure git user and token-based push URL in the worktree
	if err := wm.configureWorktreeGit(worktreePath); err != nil {
		slog.Warn("failed to configure worktree git settings", "error", err)
	}

	// Setup Claude configuration
	if err := setupClaudeSettings(worktreePath); err != nil {
		return "", "", fmt.Errorf("failed to setup Claude settings: %w", err)
	}

	if err := setupClaudeMD(worktreePath, projectName); err != nil {
		return "", "", fmt.Errorf("failed to setup CLAUDE.md: %w", err)
	}
	slog.Debug("claude settings configured", "path", worktreePath)

	slog.Info("worktree ready", "project", projectName, "branch", branchName, "path", worktreePath)
	return worktreePath, branchName, nil
}

// UpdateWorktree fetches the latest remote refs and resets the worktree to the
// latest commit on its branch. This ensures the executor always starts a task
// with the most recent code, even if commits were pushed externally.
func (wm *WorktreeManager) UpdateWorktree(projectName, worktreePath, branchName string) error {
	bareDir := filepath.Join(wm.reposDir, projectName+".git")

	// Fetch latest from remote into the bare repo
	fetchCmd := exec.Command("git", "-C", bareDir, "fetch", "--all")
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch updates: %w: %s", err, output)
	}

	// Reset the worktree to the latest remote branch commit
	// Use origin/<branch> to get the remote tracking ref
	remoteRef := fmt.Sprintf("origin/%s", branchName)
	resetCmd := exec.Command("git", "-C", worktreePath, "reset", "--hard", remoteRef)
	if output, err := resetCmd.CombinedOutput(); err != nil {
		// If the remote ref doesn't exist (new branch not yet pushed), that's OK —
		// fall back to just making sure we're on the right local branch
		slog.Warn("reset to remote ref failed, staying on local branch",
			"branch", branchName, "error", err, "output", string(output))
	}

	return nil
}

// configureWorktreeGit sets up git user identity and token-based push URL in a worktree.
func (wm *WorktreeManager) configureWorktreeGit(worktreePath string) error {
	// Set user identity for commits
	user := wm.githubUser
	if user == "" {
		user = "flux-bot"
	}
	gitCfg := [][]string{
		{"config", "user.name", user},
		{"config", "user.email", user + "@users.noreply.github.com"},
	}

	// Set token-based HTTPS push URL if we have credentials
	if wm.githubToken != "" {
		// Read current remote URL to extract owner/repo
		getURL := exec.Command("git", "-C", worktreePath, "remote", "get-url", "origin")
		urlOut, err := getURL.Output()
		if err == nil {
			remoteURL := strings.TrimSpace(string(urlOut))
			owner, repo := extractOwnerRepo(remoteURL)
			if owner != "" && repo != "" {
				tokenURL := fmt.Sprintf("https://%s:%s@github.com/%s/%s.git", user, wm.githubToken, owner, repo)
				gitCfg = append(gitCfg, []string{"remote", "set-url", "origin", tokenURL})
				slog.Debug("configured token-based push URL", "owner", owner, "repo", repo)
			}
		}
	}

	for _, args := range gitCfg {
		cmd := exec.Command("git", append([]string{"-C", worktreePath}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git %s failed: %w: %s", args[0], err, output)
		}
	}

	slog.Info("worktree git configured", "path", worktreePath, "user", user)
	return nil
}

// cleanupBranchWorktree finds and removes any existing worktree checked out on the given branch.
// This handles stale worktrees at old paths (e.g. from before the absolute path fix).
func (wm *WorktreeManager) cleanupBranchWorktree(bareDir, branchName string) {
	// List all worktrees and find the one using this branch
	listCmd := exec.Command("git", "-C", bareDir, "worktree", "list", "--porcelain")
	output, err := listCmd.CombinedOutput()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	var currentPath string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			currentPath = ""
			continue
		}
		if strings.HasPrefix(line, "worktree ") {
			currentPath = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if branch == branchName && currentPath != "" && currentPath != bareDir {
				slog.Info("cleaning up existing worktree for branch", "branch", branchName, "path", currentPath)
				// Try git worktree remove
				rmCmd := exec.Command("git", "-C", bareDir, "worktree", "remove", "--force", currentPath)
				if rmOut, rmErr := rmCmd.CombinedOutput(); rmErr != nil {
					slog.Warn("git worktree remove failed, removing directory manually", "error", rmErr, "output", string(rmOut))
					_ = os.RemoveAll(currentPath)
				}
			}
		}
	}

	// Prune any dangling references
	pruneCmd := exec.Command("git", "-C", bareDir, "worktree", "prune")
	_ = pruneCmd.Run()
}

// FindByBranch finds a worktree by its branch name
func (wm *WorktreeManager) FindByBranch(projectName, branchName string) (string, error) {
	slog.Debug("searching for worktree by branch", "project", projectName, "branch", branchName)
	bareDir := filepath.Join(wm.reposDir, projectName+".git")

	cmd := exec.Command("git", "-C", bareDir, "worktree", "list", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to list worktrees: %w: %s", err, output)
	}

	// Parse porcelain output
	// Format:
	// worktree /path/to/worktree
	// HEAD abcd1234...
	// branch refs/heads/branch-name
	// [blank line between worktrees]

	lines := strings.Split(string(output), "\n")
	var currentWorktree string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			currentWorktree = ""
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			currentWorktree = strings.TrimPrefix(line, "worktree ")
		} else if strings.HasPrefix(line, "branch ") {
			branch := strings.TrimPrefix(line, "branch refs/heads/")
			if branch == branchName && currentWorktree != "" {
				slog.Debug("worktree found", "branch", branchName, "path", currentWorktree)
				return currentWorktree, nil
			}
		}
	}

	return "", fmt.Errorf("worktree not found for branch: %s", branchName)
}

// CleanupWorktree removes a worktree
func (wm *WorktreeManager) CleanupWorktree(projectName, worktreePath string) error {
	slog.Info("cleaning up worktree", "project", projectName, "path", worktreePath)
	bareDir := filepath.Join(wm.reposDir, projectName+".git")

	cmd := exec.Command("git", "-C", bareDir, "worktree", "remove", "--force", worktreePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove worktree: %w: %s", err, output)
	}

	slog.Info("worktree cleaned up", "project", projectName)
	return nil
}

// RunCleanup performs cleanup of worktrees based on task states
func (wm *WorktreeManager) RunCleanup(tasks []WorktreeTask) error {
	slog.Info("running worktree cleanup", "task_count", len(tasks))
	now := time.Now()

	for _, task := range tasks {
		slog.Debug("evaluating task for cleanup", "task_id", task.TaskID, "status", task.Status, "pr_status", task.PRStatus)
		shouldCleanup := false

		switch task.Status {
		case "COMPLETED":
			// Clean up if PR is merged
			if task.PRStatus == "MERGED" {
				shouldCleanup = true
			}
			// Preserve if PR is still open for review

		case "FAILED":
			// Clean up after 24 hours
			if !task.CompletedAt.IsZero() && now.Sub(task.CompletedAt) > 24*time.Hour {
				shouldCleanup = true
			}

		case "CHANGES_REQUESTED":
			// Preserve for fix task - don't clean up
			shouldCleanup = false
		}

		if shouldCleanup {
			branchName := fmt.Sprintf("task/%s", task.TaskID[:8])
			worktreePath, err := wm.FindByBranch(task.ProjectName, branchName)
			if err != nil {
				// Worktree might already be cleaned up, continue
				continue
			}

			if err := wm.CleanupWorktree(task.ProjectName, worktreePath); err != nil {
				return fmt.Errorf("failed to cleanup worktree for task %s: %w", task.TaskID, err)
			}
		}
	}

	slog.Info("worktree cleanup complete")
	return nil
}

// setupClaudeSettings creates .claude/settings.json in the worktree
func setupClaudeSettings(worktreePath string) error {
	claudeDir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claude directory: %w", err)
	}

	settings := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []string{
				"Bash(*)",
				"Read(*)",
				"Write(*)",
				"Edit(*)",
				"Grep(*)",
				"Glob(*)",
				"TodoRead(*)",
				"TodoWrite(*)",
			},
		},
	}

	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, settingsJSON, 0644); err != nil {
		return fmt.Errorf("failed to write settings.json: %w", err)
	}

	return nil
}

// setupClaudeMD creates CLAUDE.md in the worktree root
func setupClaudeMD(worktreePath, projectName string) error {
	content := fmt.Sprintf(`# %s

You are working on a task for the %s project.
Follow existing code conventions. Write tests for new code.
Keep changes focused on the task description.
`, projectName, projectName)

	claudeMDPath := filepath.Join(worktreePath, "CLAUDE.md")
	if err := os.WriteFile(claudeMDPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write CLAUDE.md: %w", err)
	}

	return nil
}
