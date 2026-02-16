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

// WorktreeManager manages git worktrees for task execution.
// Each task gets a dedicated bare clone + worktree for complete isolation.
type WorktreeManager struct {
	workspaceBase string // workspaces/ base directory
	githubToken   string
	githubUser    string
}

// WorktreeTask represents a task's worktree state for cleanup decisions
type WorktreeTask struct {
	ProjectName       string
	TaskID            string
	Status            string
	PRStatus          string
	CompletedAt       time.Time
	ActiveSubtaskCount int // Number of active subtasks (PENDING, READY, RUNNING)
}

// NewWorktreeManager creates a new WorktreeManager.
// Paths are resolved to absolute to avoid mismatches between git -C and executor CWD.
func NewWorktreeManager(workspaceBase, githubToken, githubUser string) *WorktreeManager {
	absBase, err := filepath.Abs(workspaceBase)
	if err != nil {
		absBase = workspaceBase
	}
	wm := &WorktreeManager{
		workspaceBase: absBase,
		githubToken:   githubToken,
		githubUser:    githubUser,
	}
	slog.Debug("worktree manager created", "workspace_base", wm.workspaceBase)
	return wm
}

// cloneDedicatedRepo creates a dedicated bare clone for a specific task.
// Returns the path to the bare repository.
func (wm *WorktreeManager) cloneDedicatedRepo(repoURL, taskBaseDir string) (string, error) {
	bareDir := filepath.Join(taskBaseDir, ".repo")
	cloneURL := wm.tokenURL(repoURL)

	// Check if bare repo already exists
	if _, err := os.Stat(bareDir); err == nil {
		// Already exists, fetch latest
		slog.Debug("updating existing dedicated repo", "path", bareDir)

		// Update remote URL in case credentials changed
		if cloneURL != repoURL {
			setURLCmd := exec.Command("git", "-C", bareDir, "remote", "set-url", "origin", cloneURL)
			_ = setURLCmd.Run()
		}

		// Fetch latest
		fetchCmd := exec.Command("git", "-C", bareDir, "fetch", "--all", "--prune")
		if output, err := fetchCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to fetch repo: %w: %s", err, output)
		}

		slog.Debug("dedicated repo updated", "path", bareDir)
		return bareDir, nil
	}

	// Create parent directory
	if err := os.MkdirAll(taskBaseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create task directory: %w", err)
	}

	// Clone as bare repository
	slog.Info("cloning dedicated bare repo", "url", repoURL, "path", bareDir)
	cmd := exec.Command("git", "clone", "--bare", cloneURL, bareDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to clone bare repo: %w: %s", err, output)
	}

	// Configure remote tracking refs so origin/<branch> works in worktrees
	wm.configureBareFetchRefspec(bareDir)

	// Fetch with the new refspec to populate refs/remotes/origin/*
	fetchCmd := exec.Command("git", "-C", bareDir, "fetch", "--all", "--prune")
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		slog.Warn("post-clone fetch failed", "error", err, "output", string(output))
	}

	slog.Info("dedicated repo ready", "path", bareDir)
	return bareDir, nil
}

// configureBareFetchRefspec sets the fetch refspec on a bare repo so that
// remote tracking refs (refs/remotes/origin/*) are created. Without this,
// bare repos only have refs/heads/* and "origin/main" doesn't resolve.
func (wm *WorktreeManager) configureBareFetchRefspec(bareDir string) {
	// Check current refspec
	getCmd := exec.Command("git", "-C", bareDir, "config", "remote.origin.fetch")
	out, err := getCmd.Output()
	if err == nil {
		current := strings.TrimSpace(string(out))
		if strings.Contains(current, "refs/remotes/origin") {
			return // Already configured correctly
		}
	}

	// Set the standard non-bare fetch refspec
	setCmd := exec.Command("git", "-C", bareDir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*")
	if err := setCmd.Run(); err != nil {
		slog.Warn("failed to configure bare repo fetch refspec", "error", err)
	} else {
		slog.Info("configured remote tracking refs for bare repo", "dir", bareDir)
	}
}

// detectDefaultBranch returns the default branch name for a bare repo.
// Checks origin/HEAD, then falls back to probing origin/main, origin/master.
func (wm *WorktreeManager) detectDefaultBranch(bareDir string) string {
	// Try symbolic-ref for origin HEAD (set by git remote set-head --auto)
	cmd := exec.Command("git", "-C", bareDir, "symbolic-ref", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		// refs/remotes/origin/main -> main
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			branch := ref[idx+1:]
			slog.Debug("detected default branch from origin/HEAD", "branch", branch)
			return branch
		}
	}

	// Try to auto-detect origin HEAD
	autoCmd := exec.Command("git", "-C", bareDir, "remote", "set-head", "origin", "--auto")
	if autoOut, autoErr := autoCmd.CombinedOutput(); autoErr == nil {
		// Re-read after auto-detect
		cmd2 := exec.Command("git", "-C", bareDir, "symbolic-ref", "refs/remotes/origin/HEAD")
		if out2, err2 := cmd2.Output(); err2 == nil {
			ref := strings.TrimSpace(string(out2))
			if idx := strings.LastIndex(ref, "/"); idx >= 0 {
				branch := ref[idx+1:]
				slog.Debug("detected default branch via set-head --auto", "branch", branch)
				return branch
			}
		}
	} else {
		slog.Debug("remote set-head --auto failed", "output", string(autoOut))
	}

	// Probe common branch names
	for _, branch := range []string{"main", "master"} {
		checkCmd := exec.Command("git", "-C", bareDir, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
		if checkCmd.Run() == nil {
			slog.Debug("detected default branch by probing", "branch", branch)
			return branch
		}
	}

	slog.Warn("could not detect default branch, falling back to main")
	return "main"
}

// detectDefaultBranchFromWorktree detects the default branch from within a worktree.
func (wm *WorktreeManager) detectDefaultBranchFromWorktree(worktreePath string) string {
	// Try symbolic-ref for origin HEAD
	cmd := exec.Command("git", "-C", worktreePath, "symbolic-ref", "refs/remotes/origin/HEAD")
	out, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		if idx := strings.LastIndex(ref, "/"); idx >= 0 {
			return ref[idx+1:]
		}
	}

	// Probe common branch names
	for _, branch := range []string{"main", "master"} {
		checkCmd := exec.Command("git", "-C", worktreePath, "rev-parse", "--verify", "refs/remotes/origin/"+branch)
		if checkCmd.Run() == nil {
			return branch
		}
	}

	return "main"
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

// CreateWorktree creates a new dedicated bare clone + worktree for a task.
// Each task gets complete isolation with its own git repository clone.
// Returns the worktree path (where Claude Code runs) and the branch name.
func (wm *WorktreeManager) CreateWorktree(projectName, taskID, repoURL string) (worktreePath string, branchName string, err error) {
	branchName = fmt.Sprintf("task/%s", taskID[:8])
	taskBaseDir := filepath.Join(wm.workspaceBase, "trees", fmt.Sprintf("%s--task-%s", projectName, taskID[:8]))
	worktreePath = filepath.Join(taskBaseDir, "worktree")

	slog.Info("creating dedicated worktree", "project", projectName, "task_id", taskID, "branch", branchName, "path", worktreePath)

	// Clean up any existing task directory (retry scenario)
	if _, err := os.Stat(taskBaseDir); err == nil {
		slog.Info("removing existing task directory for clean retry", "path", taskBaseDir)
		if err := os.RemoveAll(taskBaseDir); err != nil {
			return "", "", fmt.Errorf("failed to remove existing task directory: %w", err)
		}
	}

	// Create dedicated bare clone for this task
	bareDir, err := wm.cloneDedicatedRepo(repoURL, taskBaseDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to clone dedicated repo: %w", err)
	}

	// Detect the default branch (main, master, etc.)
	defaultBranch := wm.detectDefaultBranch(bareDir)
	startPoint := "origin/" + defaultBranch

	// Create worktree from the dedicated bare repo
	cmd := exec.Command("git", "-C", bareDir, "worktree", "add", "-b", branchName, worktreePath, startPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("failed to create worktree: %w: %s", err, output)
	}
	slog.Debug("worktree created from dedicated repo", "path", worktreePath, "start_point", startPoint)

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

	slog.Info("dedicated worktree ready", "project", projectName, "branch", branchName, "path", worktreePath)
	return worktreePath, branchName, nil
}

// UpdateWorktree fetches the latest remote refs and resets the worktree to the
// latest commit on its branch. This ensures the executor always starts a task
// with the most recent code, even if commits were pushed externally.
func (wm *WorktreeManager) UpdateWorktree(worktreePath, branchName string) error {
	// Determine the bare repo path from the worktree path
	// worktreePath is {base}/trees/{project}--task-{id}/worktree
	// bareDir is {base}/trees/{project}--task-{id}/.repo
	taskBaseDir := filepath.Dir(worktreePath)
	bareDir := filepath.Join(taskBaseDir, ".repo")

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

// No longer needed - each task has its own dedicated repo

// FindByBranch finds a worktree by its branch name.
// With the new architecture, we can derive the path directly from the task ID.
func (wm *WorktreeManager) FindByBranch(projectName, branchName string) (string, error) {
	// Branch format: task/{taskID[:8]}
	// Extract task ID from branch name
	if !strings.HasPrefix(branchName, "task/") {
		return "", fmt.Errorf("invalid branch name format: %s", branchName)
	}
	taskIDPrefix := strings.TrimPrefix(branchName, "task/")

	// Construct expected worktree path
	taskBaseDir := filepath.Join(wm.workspaceBase, "trees", fmt.Sprintf("%s--task-%s", projectName, taskIDPrefix))
	worktreePath := filepath.Join(taskBaseDir, "worktree")

	// Verify it exists
	if _, err := os.Stat(worktreePath); err != nil {
		return "", fmt.Errorf("worktree not found for branch %s: %w", branchName, err)
	}

	slog.Debug("worktree found", "branch", branchName, "path", worktreePath)
	return worktreePath, nil
}

// FindWorktreeByTaskID finds a worktree by its task ID.
// This is useful for locating parent worktrees when merging subtask branches.
func (wm *WorktreeManager) FindWorktreeByTaskID(projectName, taskID string) (string, error) {
	// Construct expected worktree path using task ID prefix
	taskIDPrefix := taskID
	if len(taskID) > 8 {
		taskIDPrefix = taskID[:8]
	}

	taskBaseDir := filepath.Join(wm.workspaceBase, "trees", fmt.Sprintf("%s--task-%s", projectName, taskIDPrefix))
	worktreePath := filepath.Join(taskBaseDir, "worktree")

	// Verify it exists
	if _, err := os.Stat(worktreePath); err != nil {
		return "", fmt.Errorf("worktree not found for task %s: %w", taskID, err)
	}

	slog.Debug("worktree found by task ID", "task_id", taskID, "path", worktreePath)
	return worktreePath, nil
}

// RebaseOnMain rebases the worktree's branch onto the default branch to resolve conflicts.
func (wm *WorktreeManager) RebaseOnMain(worktreePath string) error {
	// Detect the default branch from the worktree's remote
	defaultBranch := wm.detectDefaultBranchFromWorktree(worktreePath)
	remoteRef := "origin/" + defaultBranch

	// Fetch latest default branch
	fetchCmd := exec.Command("git", "fetch", "origin", defaultBranch)
	fetchCmd.Dir = worktreePath
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch %s: %w: %s", defaultBranch, err, output)
	}

	// Try rebase first (produces clean linear history)
	rebaseCmd := exec.Command("git", "rebase", remoteRef)
	rebaseCmd.Dir = worktreePath
	output, err := rebaseCmd.CombinedOutput()
	if err != nil {
		// Abort the failed rebase
		abortCmd := exec.Command("git", "rebase", "--abort")
		abortCmd.Dir = worktreePath
		_ = abortCmd.Run()

		slog.Warn("rebase failed, trying merge as fallback", "error", err, "output", string(output))

		// Fallback: try merge instead (handles more conflict scenarios)
		mergeCmd := exec.Command("git", "merge", remoteRef, "--no-edit")
		mergeCmd.Dir = worktreePath
		mergeOutput, mergeErr := mergeCmd.CombinedOutput()
		if mergeErr != nil {
			// Merge also failed — abort and report
			mergeAbortCmd := exec.Command("git", "merge", "--abort")
			mergeAbortCmd.Dir = worktreePath
			_ = mergeAbortCmd.Run()
			return fmt.Errorf("conflict: both rebase and merge failed.\nRebase: %s\nMerge: %s", string(output), string(mergeOutput))
		}
		slog.Info("merged remote default branch successfully (rebase had conflicts)", "ref", remoteRef)
	}

	return nil
}

// MergeSubtaskIntoParent merges commits from a subtask branch into the parent task's branch.
// This is used when subtasks need to contribute their changes to a parent task before PR creation.
func (wm *WorktreeManager) MergeSubtaskIntoParent(projectName, parentTaskID, subtaskTaskID string) error {
	// Locate parent worktree
	parentWorktree, err := wm.FindWorktreeByTaskID(projectName, parentTaskID)
	if err != nil {
		return fmt.Errorf("failed to find parent worktree: %w", err)
	}

	// Locate subtask worktree
	subtaskWorktree, err := wm.FindWorktreeByTaskID(projectName, subtaskTaskID)
	if err != nil {
		return fmt.Errorf("failed to find subtask worktree: %w", err)
	}

	// Construct subtask branch name
	subtaskBranch := fmt.Sprintf("task/%s", subtaskTaskID[:8])

	slog.Info("merging subtask into parent",
		"parent_task_id", parentTaskID,
		"subtask_task_id", subtaskTaskID,
		"subtask_branch", subtaskBranch,
		"parent_worktree", parentWorktree)

	// Get the bare repo path for the subtask (to fetch from)
	subtaskTaskBaseDir := filepath.Dir(subtaskWorktree)
	subtaskBareDir := filepath.Join(subtaskTaskBaseDir, ".repo")

	// Add subtask's bare repo as a temporary remote in parent worktree
	remoteName := fmt.Sprintf("subtask-%s", subtaskTaskID[:8])
	addRemoteCmd := exec.Command("git", "-C", parentWorktree, "remote", "add", remoteName, subtaskBareDir)
	if output, err := addRemoteCmd.CombinedOutput(); err != nil {
		// Remote might already exist, try to update URL instead
		setURLCmd := exec.Command("git", "-C", parentWorktree, "remote", "set-url", remoteName, subtaskBareDir)
		if setErr := setURLCmd.Run(); setErr != nil {
			return fmt.Errorf("failed to add remote for subtask: %w: %s", err, output)
		}
	}

	// Fetch subtask branch from the temporary remote
	fetchCmd := exec.Command("git", "-C", parentWorktree, "fetch", remoteName, subtaskBranch)
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch subtask branch: %w: %s", err, output)
	}

	// Merge the subtask commits into parent's current branch
	mergeRef := fmt.Sprintf("%s/%s", remoteName, subtaskBranch)
	mergeCmd := exec.Command("git", "-C", parentWorktree, "merge", mergeRef, "--no-edit",
		"-m", fmt.Sprintf("Merge subtask %s into parent %s", subtaskTaskID[:8], parentTaskID[:8]))
	output, err := mergeCmd.CombinedOutput()
	if err != nil {
		// Check if there are merge conflicts
		if strings.Contains(string(output), "CONFLICT") {
			// Abort the merge and return a clear error
			abortCmd := exec.Command("git", "-C", parentWorktree, "merge", "--abort")
			_ = abortCmd.Run()
			return fmt.Errorf("merge conflict detected when merging subtask %s into parent %s: %s",
				subtaskTaskID[:8], parentTaskID[:8], string(output))
		}
		return fmt.Errorf("failed to merge subtask branch: %w: %s", err, output)
	}

	// Remove temporary remote
	removeRemoteCmd := exec.Command("git", "-C", parentWorktree, "remote", "remove", remoteName)
	_ = removeRemoteCmd.Run() // Ignore errors - cleanup is best-effort

	slog.Info("successfully merged subtask into parent",
		"parent_task_id", parentTaskID,
		"subtask_task_id", subtaskTaskID)

	return nil
}

// CleanupWorktree removes the entire task directory (worktree + dedicated bare repo).
func (wm *WorktreeManager) CleanupWorktree(projectName, worktreePath string) error {
	// worktreePath is {base}/trees/{project}--task-{id}/worktree
	// We want to remove the entire task directory: {base}/trees/{project}--task-{id}/
	taskBaseDir := filepath.Dir(worktreePath)

	slog.Info("cleaning up task directory", "project", projectName, "path", taskBaseDir)

	// Remove the entire task directory (includes .repo/ and worktree/)
	if err := os.RemoveAll(taskBaseDir); err != nil {
		return fmt.Errorf("failed to remove task directory: %w", err)
	}

	slog.Info("task directory cleaned up", "project", projectName, "path", taskBaseDir)
	return nil
}

// RunCleanup performs cleanup of worktrees based on task states
func (wm *WorktreeManager) RunCleanup(tasks []WorktreeTask) error {
	slog.Info("running worktree cleanup", "task_count", len(tasks))
	now := time.Now()

	for _, task := range tasks {
		slog.Debug("evaluating task for cleanup",
			"task_id", task.TaskID,
			"status", task.Status,
			"pr_status", task.PRStatus,
			"active_subtasks", task.ActiveSubtaskCount)
		shouldCleanup := false

		// Never clean up parent tasks that still have active subtasks
		if task.ActiveSubtaskCount > 0 {
			slog.Debug("preserving parent task with active subtasks",
				"task_id", task.TaskID,
				"active_subtasks", task.ActiveSubtaskCount)
			continue
		}

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
