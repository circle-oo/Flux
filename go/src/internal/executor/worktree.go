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
	reposDir string // workspaces/repos/
	treesDir string // workspaces/trees/
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
func NewWorktreeManager(workspaceBase string) *WorktreeManager {
	absBase, err := filepath.Abs(workspaceBase)
	if err != nil {
		absBase = workspaceBase
	}
	wm := &WorktreeManager{
		reposDir: filepath.Join(absBase, "repos"),
		treesDir: filepath.Join(absBase, "trees"),
	}
	slog.Debug("worktree manager created", "repos_dir", wm.reposDir, "trees_dir", wm.treesDir)
	return wm
}

// EnsureBareRepo ensures a bare repository exists and is up to date
func (wm *WorktreeManager) EnsureBareRepo(repoURL, projectName string) error {
	slog.Debug("ensuring bare repo", "repo_url", repoURL, "project", projectName)
	bareDir := filepath.Join(wm.reposDir, projectName+".git")

	// Check if bare repo exists
	if _, err := os.Stat(bareDir); err == nil {
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

	slog.Info("cloning bare repo", "repo_url", repoURL, "project", projectName)
	cmd := exec.Command("git", "clone", "--bare", repoURL, bareDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clone bare repo: %w: %s", err, output)
	}

	slog.Info("bare repo ready", "project", projectName)
	return nil
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

	// Clean up stale worktree directory if it exists from a previous attempt
	if _, statErr := os.Stat(worktreePath); statErr == nil {
		slog.Info("removing stale worktree from previous attempt", "path", worktreePath)
		// Try git worktree remove first
		rmCmd := exec.Command("git", "-C", bareDir, "worktree", "remove", "--force", worktreePath)
		if rmOut, rmErr := rmCmd.CombinedOutput(); rmErr != nil {
			slog.Warn("git worktree remove failed, falling back to manual cleanup", "error", rmErr, "output", string(rmOut))
			// Manual fallback: remove directory and prune
			if err := os.RemoveAll(worktreePath); err != nil {
				return "", "", fmt.Errorf("failed to remove stale worktree directory: %w", err)
			}
		}
		// Always prune dangling worktree references
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
