package executor

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/github"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// Executor is an autonomous execution pod that picks up tasks, runs Claude Code,
// and produces PRs.
type Executor struct {
	id                 string
	config             *config.Config
	claude             *ClaudeCodeRunner
	worktree           *WorktreeManager
	manager            *ManagerClient
	github             *github.Client
	notifier           *notifier.Discord
	stopCh             chan struct{}
	executionStartTime time.Time
}

// sensitiveFile tracks a file path and its mod time for integrity verification.
type sensitiveFile struct {
	path    string
	modTime time.Time
	exists  bool
}

// NewExecutor creates a new Executor pod.
func NewExecutor(id string, cfg *config.Config, discord *notifier.Discord) *Executor {
	e := &Executor{
		id:       id,
		config:   cfg,
		claude:   NewClaudeCodeRunner(&cfg.Executor),
		worktree: NewWorktreeManager(cfg.Orchestrator.WorkspaceBase, cfg.GitHub.Token, cfg.GitHub.Username),
		manager:  NewManagerClient(fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)),
		github:   github.NewClient(cfg.GitHub.Token, cfg.GitHub.Username),
		notifier: discord,
		stopCh:   make(chan struct{}),
	}
	slog.Debug("executor created", "id", id, "manager_url", fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port))
	return e
}

// Run is the main execution loop. It polls for tasks and executes them.
func (e *Executor) Run(ctx context.Context) {
	slog.Info("executor started", "id", e.id)

	// Run smoke test on startup
	if err := e.claudeCodeSmokeTest(); err != nil {
		slog.Error("claude code smoke test failed", "error", err)
		_ = e.notifier.Send(notifier.LevelCritical,
			fmt.Sprintf("Executor %s: Claude Code smoke test failed: %v", e.id, err))
		return
	}
	slog.Info("claude code smoke test passed", "id", e.id)

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		default:
			slog.Debug("executor polling for tasks", "id", e.id)
			e.executeOnce(ctx)
			time.Sleep(5 * time.Second)
		}
	}
}

// Stop signals the executor to stop.
func (e *Executor) Stop() {
	close(e.stopCh)
}

// executeOnce runs one iteration of the task execution pipeline.
func (e *Executor) executeOnce(ctx context.Context) {
	slog.Debug("polling for next task", "executor_id", e.id)

	// 1. Request next task
	task, err := e.manager.NextTask(e.id, "executor")
	if err != nil {
		slog.Error("failed to get next task", "error", err)
		return
	}
	if task == nil {
		slog.Debug("no task available, sleeping", "executor_id", e.id, "sleep_duration", "30s")
		time.Sleep(30 * time.Second)
		return
	}

	slog.Info("picked up task", "task_id", task.ID, "title", task.Title, "type", task.Type)

	// Set executor ID on the task
	task.ExecutorID = e.id

	// 2. Get model assignment
	model, err := e.manager.GetModel(task.ID)
	if err != nil {
		slog.Error("failed to get model", "task_id", task.ID, "error", err)
		model = "sonnet" // fallback
	}
	task.Model = model
	slog.Info("model assigned", "task_id", task.ID, "model", model)

	// 3. Build system prompt
	systemPrompt := e.buildSystemPrompt(task)
	slog.Debug("system prompt built", "task_id", task.ID, "prompt_length", len(systemPrompt))

	// 4. Get project info
	project, err := e.manager.GetProject(task.ProjectID)
	if err != nil {
		slog.Error("failed to get project", "task_id", task.ID, "project_id", task.ProjectID, "error", err)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("failed to get project: %v", err), 0, 0)
		return
	}
	slog.Debug("project loaded", "task_id", task.ID, "project_id", project.ID, "project_name", project.Name)

	// 5. Create or reuse worktree
	var worktreePath string
	if task.BranchName != "" {
		// CHANGES_REQUESTED fix: reuse existing worktree
		worktreePath, err = e.worktree.FindByBranch(project.Name, task.BranchName)
		if err != nil {
			slog.Error("failed to find worktree by branch", "branch", task.BranchName, "error", err)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("worktree not found: %v", err), 0, 0)
			return
		}
		slog.Info("reusing existing worktree", "task_id", task.ID, "branch", task.BranchName, "path", worktreePath)
	} else {
		if err := e.worktree.EnsureBareRepo(project.RepoURL, project.Name); err != nil {
			slog.Error("failed to ensure bare repo", "error", err)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("bare repo failed: %v", err), 0, 0)
			return
		}
		worktreePath, task.BranchName, err = e.worktree.CreateWorktree(project.Name, task.ID)
		if err != nil {
			slog.Error("failed to create worktree", "error", err)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("worktree create failed: %v", err), 0, 0)
			return
		}
		slog.Info("created new worktree", "task_id", task.ID, "branch", task.BranchName, "path", worktreePath)
	}

	// 6. Build prompt
	prompt := e.buildPrompt(task)
	slog.Debug("execution prompt built", "task_id", task.ID, "prompt_length", len(prompt))

	// Snapshot sensitive files before execution
	preSnapshot := e.snapshotSensitiveFiles()
	slog.Debug("sensitive files snapshot taken", "task_id", task.ID, "file_count", len(preSnapshot))

	// 7. Execute Claude Code
	e.executionStartTime = time.Now()
	slog.Info("starting claude code execution", "task_id", task.ID, "model", model, "workdir", worktreePath)
	result, err := e.claude.Run(ctx, ClaudeCodeOpts{
		Prompt:       prompt,
		WorkDir:      worktreePath,
		Model:        model,
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		slog.Error("claude code execution failed", "task_id", task.ID, "error", err)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("execution error: %v", err), 0, 0)
		return
	}
	slog.Info("claude code execution completed", "task_id", task.ID, "duration", result.Duration, "exit_code", result.ExitCode, "tokens", result.TokensUsed, "cost_usd", result.CostUSD)

	// 8. Check rate limit
	if IsRateLimited(result.ExitCode, result.Stderr) {
		slog.Warn("rate limited, retrying task", "task_id", task.ID)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskRetry, "", "rate limited", result.TokensUsed, result.CostUSD)
		return
	}

	// 9. Post-execution verification
	if err := e.verifyWorktreeIntegrity(preSnapshot); err != nil {
		slog.Error("worktree integrity violation", "task_id", task.ID, "error", err)
		_ = e.notifier.Send(notifier.LevelCritical,
			fmt.Sprintf("INTEGRITY VIOLATION task %s: %v", task.ID, err))
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("integrity violation: %v", err), result.TokensUsed, result.CostUSD)
		return
	}
	slog.Debug("worktree integrity verified", "task_id", task.ID)

	// 10. Check subtask decomposition (Phase 2A stub)
	_ = e.parseDecomposition(result.Stdout)
	slog.Debug("decomposition check complete", "task_id", task.ID)

	// 11. QA: run tests if required
	if task.RequiresTest() {
		passed := e.runTests(worktreePath, task)
		task.TestPassed = &passed
		if !passed {
			slog.Warn("tests failed for task", "task_id", task.ID)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, "tests failed", result.TokensUsed, result.CostUSD)
			return
		}
		slog.Info("tests passed", "task_id", task.ID)
	}

	// 12. Commit and get diff
	diffLines, filesChanged, pushErr := e.commitAndGetDiff(worktreePath, task)
	task.DiffLines = diffLines
	task.FilesChanged = filesChanged
	if pushErr != nil {
		slog.Error("commit/push failed", "task_id", task.ID, "error", pushErr)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("git failed: %v", pushErr), result.TokensUsed, result.CostUSD)
		return
	}
	slog.Info("changes committed and pushed", "task_id", task.ID, "diff_lines", diffLines, "files_changed", filesChanged)

	// Check guardrails
	if ExceedsGuardrails(&e.config.Executor, diffLines, filesChanged) {
		slog.Warn("diff exceeds guardrails", "task_id", task.ID, "diff_lines", diffLines, "files_changed", filesChanged)
		_ = e.notifier.Send(notifier.LevelWarning,
			fmt.Sprintf("Task %s diff exceeds guardrails: %d lines, %d files", task.ID, diffLines, filesChanged))
	}

	// 13. Create PR
	owner, repo := extractOwnerRepo(project.RepoURL)
	if owner == "" || repo == "" {
		slog.Error("failed to extract owner/repo from URL", "url", project.RepoURL)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, "invalid repo URL", result.TokensUsed, result.CostUSD)
		return
	}

	prTitle := fmt.Sprintf("[flux] %s", task.Title)
	prBody := fmt.Sprintf("## Task\n\n%s\n\n## Description\n\n%s\n\n---\n*Automated by Flux executor %s*",
		task.Title, task.Description, e.id)

	prURL, prNumber, prErr := e.github.CreatePR(owner, repo, task.BranchName, "main", prTitle, prBody)
	if prErr != nil {
		slog.Error("failed to create PR", "task_id", task.ID, "error", prErr)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("PR creation failed: %v", prErr), result.TokensUsed, result.CostUSD)
		return
	}
	task.PRUrl = prURL
	task.PRStatus = "OPEN"
	slog.Info("pull request created", "task_id", task.ID, "pr_url", prURL, "pr_number", prNumber)

	// 14. Auto-merge decision
	shouldMerge, mergeReason := AutoMergeReason(task, diffLines, filesChanged)
	slog.Info("auto-merge decision", "task_id", task.ID, "should_merge", shouldMerge, "reason", mergeReason, "diff_lines", diffLines, "files_changed", filesChanged)

	// Post reason as PR comment
	commentBody := fmt.Sprintf("**Flux Auto-Merge Decision**\n\n%s\n\n| Attribute | Value |\n|-----------|-------|\n| Task | `%s` |\n| Type | %s |\n| Source | %s |\n| Priority | P:%d |\n| Diff | %d lines, %d files |",
		mergeReason, task.ID, task.Type, task.Source, task.Priority, diffLines, filesChanged)
	if commentErr := e.github.PostComment(owner, repo, prNumber, commentBody); commentErr != nil {
		slog.Error("failed to post merge-decision comment", "task_id", task.ID, "pr", prNumber, "error", commentErr)
	}

	if shouldMerge {
		if mergeErr := e.github.MergePR(owner, repo, prNumber); mergeErr != nil {
			slog.Error("auto-merge failed", "task_id", task.ID, "pr", prNumber, "error", mergeErr)
			_ = e.notifier.Send(notifier.LevelWarning,
				fmt.Sprintf("Auto-merge failed for task %s PR #%d: %v", task.ID, prNumber, mergeErr))
		} else {
			task.PRStatus = "MERGED"
			_ = e.worktree.CleanupWorktree(project.Name, worktreePath)
			slog.Info("auto-merged PR", "task_id", task.ID, "pr", prNumber)
		}
	} else {
		_ = e.notifier.Send(notifier.LevelInfo,
			fmt.Sprintf("PR ready for review: %s — %s", prURL, task.Title))
	}

	// 15. Report completion
	_ = e.manager.ReportTaskDone(task.ID, task, models.TaskCompleted, result.Stdout, "", result.TokensUsed, result.CostUSD)
	slog.Info("task completed", "task_id", task.ID, "pr_url", prURL, "pr_status", task.PRStatus)
}

// buildSystemPrompt creates a system prompt with the current goal context.
func (e *Executor) buildSystemPrompt(task *models.Task) string {
	slog.Debug("building system prompt", "task_id", task.ID, "has_goal", task.GoalID != "")
	var sb strings.Builder
	sb.WriteString("You are an autonomous coding agent working on a specific task.\n")
	sb.WriteString("Focus exclusively on the task described in the prompt.\n")
	sb.WriteString("Do not modify files outside the scope of the task.\n")
	sb.WriteString("Write clean, tested, production-quality code.\n\n")

	if task.GoalID != "" {
		sb.WriteString(fmt.Sprintf("Current Goal ID: %s\n", task.GoalID))
	}
	sb.WriteString(fmt.Sprintf("Task Type: %s\n", task.Type))
	sb.WriteString(fmt.Sprintf("Task Priority: %d\n", task.Priority))

	return sb.String()
}

// buildPrompt creates the execution prompt for Claude Code.
func (e *Executor) buildPrompt(task *models.Task) string {
	slog.Debug("building execution prompt", "task_id", task.ID, "has_custom_prompt", task.Prompt != "")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Task: %s\n\n", task.Title))
	sb.WriteString(fmt.Sprintf("## Description\n%s\n\n", task.Description))

	if task.Prompt != "" {
		sb.WriteString(fmt.Sprintf("## Additional Instructions\n%s\n\n", task.Prompt))
	}

	sb.WriteString("## Requirements\n")
	sb.WriteString("- Implement the changes described above\n")
	sb.WriteString("- Write or update tests as needed\n")
	sb.WriteString("- Follow existing code conventions\n")
	sb.WriteString("- Keep changes focused and minimal\n")

	return sb.String()
}

// snapshotSensitiveFiles records mod times of sensitive files before execution.
func (e *Executor) snapshotSensitiveFiles() []sensitiveFile {
	home, _ := os.UserHomeDir()
	paths := []string{
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".aws"),
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
	}
	slog.Debug("snapshotting sensitive files", "count", len(paths))

	snapshots := make([]sensitiveFile, 0, len(paths))
	for _, p := range paths {
		sf := sensitiveFile{path: p}
		info, err := os.Stat(p)
		if err == nil {
			sf.exists = true
			sf.modTime = info.ModTime()
		}
		snapshots = append(snapshots, sf)
	}
	return snapshots
}

// verifyWorktreeIntegrity checks that sensitive files were not modified during execution.
func (e *Executor) verifyWorktreeIntegrity(preSnapshot []sensitiveFile) error {
	slog.Debug("verifying worktree integrity", "file_count", len(preSnapshot))
	for _, pre := range preSnapshot {
		info, err := os.Stat(pre.path)
		if pre.exists {
			if err != nil {
				return fmt.Errorf("sensitive file deleted during execution: %s", pre.path)
			}
			if !info.ModTime().Equal(pre.modTime) {
				return fmt.Errorf("sensitive file modified during execution: %s", pre.path)
			}
		} else {
			// File didn't exist before; if it exists now, that's suspicious
			if err == nil {
				return fmt.Errorf("sensitive file created during execution: %s", pre.path)
			}
		}
	}
	return nil
}

// parseDecomposition is a Phase 2A stub. Full implementation in Phase 2B.
func (e *Executor) parseDecomposition(_ string) []SubtaskRequest {
	return nil
}

// runTests detects the test framework and runs tests in the worktree.
func (e *Executor) runTests(worktreePath string, task *models.Task) bool {
	// Skip tests for RESEARCH and DOCUMENT types
	if task.Type == models.TaskTypeResearch || task.Type == models.TaskTypeDocument {
		return true
	}

	// Detect test framework
	type testCmd struct {
		detectFile string
		command    string
		args       []string
	}

	testCmds := []testCmd{
		{"go.mod", "go", []string{"test", "./..."}},
		{"package.json", "npm", []string{"test", "--", "--passWithNoTests"}},
		{"requirements.txt", "pytest", []string{"-x", "--tb=short"}},
		{"pyproject.toml", "pytest", []string{"-x", "--tb=short"}},
		{"Cargo.toml", "cargo", []string{"test"}},
	}

	for _, tc := range testCmds {
		detectPath := filepath.Join(worktreePath, tc.detectFile)
		slog.Debug("checking test framework", "file", tc.detectFile, "worktree", worktreePath)
		if _, err := os.Stat(detectPath); err == nil {
			slog.Info("running tests", "framework", tc.command, "worktree", worktreePath)
			cmd := exec.Command(tc.command, tc.args...)
			cmd.Dir = worktreePath
			output, err := cmd.CombinedOutput()
			if err != nil {
				slog.Warn("tests failed", "command", tc.command, "output", string(output), "error", err)
				return false
			}
			slog.Info("tests passed", "command", tc.command)
			return true
		}
	}

	// No test framework detected, pass by default
	slog.Info("no test framework detected, skipping tests", "worktree", worktreePath)
	return true
}

// commitAndGetDiff stages, commits, pushes, and returns diff stats.
// Returns an error if git push fails (caller should not create PR).
func (e *Executor) commitAndGetDiff(worktreePath string, task *models.Task) (diffLines, filesChanged int, pushErr error) {
	// git add -A
	slog.Debug("staging changes", "worktree", worktreePath)
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = worktreePath
	if output, err := addCmd.CombinedOutput(); err != nil {
		slog.Error("git add failed", "output", string(output), "error", err)
		return 0, 0, fmt.Errorf("git add failed: %w", err)
	}

	// Check if there are changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = worktreePath
	statusOut, err := statusCmd.Output()
	if err != nil || len(strings.TrimSpace(string(statusOut))) == 0 {
		slog.Info("no changes to commit", "worktree", worktreePath)
		return 0, 0, fmt.Errorf("no changes to commit")
	}

	// git commit
	commitMsg := fmt.Sprintf("[flux] %s\n\nTask: %s\nType: %s\nPriority: %d",
		task.Title, task.ID, task.Type, task.Priority)
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitCmd.Dir = worktreePath
	if output, err := commitCmd.CombinedOutput(); err != nil {
		slog.Error("git commit failed", "output", string(output), "error", err)
		return 0, 0, fmt.Errorf("git commit failed: %w", err)
	}
	slog.Info("committed changes", "task_id", task.ID, "message_prefix", "[flux] "+task.Title)

	// git diff --stat HEAD~1
	diffCmd := exec.Command("git", "diff", "--stat", "HEAD~1")
	diffCmd.Dir = worktreePath
	diffOut, err := diffCmd.Output()
	if err != nil {
		slog.Error("git diff --stat failed", "error", err)
	} else {
		diffLines, filesChanged = parseDiffStat(string(diffOut))
	}

	// git push
	slog.Debug("pushing to remote", "branch", task.BranchName)
	pushCmd := exec.Command("git", "push", "-u", "origin", task.BranchName)
	pushCmd.Dir = worktreePath
	if output, err := pushCmd.CombinedOutput(); err != nil {
		slog.Error("git push failed", "output", string(output), "error", err)
		return diffLines, filesChanged, fmt.Errorf("git push failed: %w", err)
	}
	slog.Info("pushed to remote", "branch", task.BranchName)

	return diffLines, filesChanged, nil
}

// parseDiffStat parses the summary line of `git diff --stat` output.
// The last line looks like: " 5 files changed, 120 insertions(+), 30 deletions(-)"
func parseDiffStat(output string) (diffLines, filesChanged int) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return 0, 0
	}
	summary := lines[len(lines)-1]

	// Parse files changed
	filesRe := regexp.MustCompile(`(\d+)\s+files?\s+changed`)
	if m := filesRe.FindStringSubmatch(summary); len(m) >= 2 {
		filesChanged, _ = strconv.Atoi(m[1])
	}

	// Parse insertions and deletions
	insertRe := regexp.MustCompile(`(\d+)\s+insertions?\(\+\)`)
	deleteRe := regexp.MustCompile(`(\d+)\s+deletions?\(-\)`)

	var insertions, deletions int
	if m := insertRe.FindStringSubmatch(summary); len(m) >= 2 {
		insertions, _ = strconv.Atoi(m[1])
	}
	if m := deleteRe.FindStringSubmatch(summary); len(m) >= 2 {
		deletions, _ = strconv.Atoi(m[1])
	}
	diffLines = insertions + deletions

	return diffLines, filesChanged
}

// ShouldAutoMerge determines if a PR should be auto-merged based on task attributes and diff size.
func ShouldAutoMerge(task *models.Task, diffLines, filesChanged int) bool {
	// Large diffs always require operator review
	if diffLines > 2000 || filesChanged > 20 {
		return false
	}

	// System/self source: auto-merge
	if task.Source == models.TaskSourceSystem || task.Source == models.TaskSourceSelf {
		return true
	}

	// Maintenance type: auto-merge
	if task.Type == models.TaskTypeMaintenance {
		return true
	}

	// Bugfix with high priority (low number): auto-merge
	if task.Type == models.TaskTypeBugfix && task.Priority <= 10 {
		return true
	}

	// Small changes: auto-merge
	if filesChanged <= 3 && diffLines < 100 {
		return true
	}

	// Otherwise: operator review
	return false
}

// AutoMergeReason returns a human-readable explanation for the auto-merge decision.
// The first return value is whether the PR should be auto-merged,
// the second is the reason string.
func AutoMergeReason(task *models.Task, diffLines, filesChanged int) (bool, string) {
	// Large diffs always require operator review
	if diffLines > 2000 || filesChanged > 20 {
		var parts []string
		if diffLines > 2000 {
			parts = append(parts, fmt.Sprintf("diff too large (%d lines > 2000)", diffLines))
		}
		if filesChanged > 20 {
			parts = append(parts, fmt.Sprintf("too many files changed (%d > 20)", filesChanged))
		}
		return false, fmt.Sprintf("Requires operator review: %s.", strings.Join(parts, "; "))
	}

	// System/self source: auto-merge
	if task.Source == models.TaskSourceSystem || task.Source == models.TaskSourceSelf {
		return true, fmt.Sprintf("Auto-merged: source is %s (trusted).", strings.ToLower(task.Source))
	}

	// Maintenance type: auto-merge
	if task.Type == models.TaskTypeMaintenance {
		return true, "Auto-merged: task type is maintenance."
	}

	// Bugfix with high priority (low number): auto-merge
	if task.Type == models.TaskTypeBugfix && task.Priority <= 10 {
		return true, fmt.Sprintf("Auto-merged: high-priority bugfix (P:%d).", task.Priority)
	}

	// Small changes: auto-merge
	if filesChanged <= 3 && diffLines < 100 {
		return true, fmt.Sprintf("Auto-merged: small change (%d files, %d lines).", filesChanged, diffLines)
	}

	// Otherwise: operator review
	return false, fmt.Sprintf("Requires operator review: %d files changed, %d diff lines, source=%s, type=%s.",
		filesChanged, diffLines, task.Source, task.Type)
}

// extractOwnerRepo parses owner and repo from a GitHub URL.
// Supports both HTTPS and SSH URL formats.
func extractOwnerRepo(repoURL string) (owner, repo string) {
	// HTTPS: https://github.com/owner/repo.git or https://github.com/owner/repo
	httpsRe := regexp.MustCompile(`github\.com/([^/]+)/([^/.]+)`)
	if m := httpsRe.FindStringSubmatch(repoURL); len(m) >= 3 {
		return m[1], m[2]
	}

	// SSH: git@github.com:owner/repo.git
	sshRe := regexp.MustCompile(`github\.com:([^/]+)/([^/.]+)`)
	if m := sshRe.FindStringSubmatch(repoURL); len(m) >= 3 {
		return m[1], m[2]
	}

	return "", ""
}

// claudeCodeSmokeTest verifies Claude Code CLI is available and functional.
func (e *Executor) claudeCodeSmokeTest() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"-p", "respond with exactly: SMOKE_TEST_OK",
		"--max-turns", "1",
		"--output-format", "json",
	)

	stdout := &limitedBuffer{maxSize: 1 << 20}
	stderr := &limitedBuffer{maxSize: 1 << 20}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("smoke test command failed: %w (stderr: %s)", err, stderr.String())
	}

	output := stdout.String()

	// Check for SMOKE_TEST_OK in the raw output or parsed response
	if strings.Contains(output, "SMOKE_TEST_OK") {
		return nil
	}

	// Try to parse JSON response
	parsed, err := ParseResponse(output)
	if err == nil && strings.Contains(parsed.ResultText, "SMOKE_TEST_OK") {
		return nil
	}

	// Scan line-by-line as fallback
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "SMOKE_TEST_OK") {
			return nil
		}
	}

	return fmt.Errorf("smoke test response did not contain SMOKE_TEST_OK")
}
