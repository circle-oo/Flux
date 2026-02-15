package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/apiclient"
	"github.com/circle-oo/flux/internal/claudecli"
	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/executor/prompts"
	"github.com/circle-oo/flux/internal/github"
	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
	"github.com/circle-oo/flux/internal/vault"
)

// Package-level compiled regexes for performance
var (
	filesChangedRe = regexp.MustCompile(`(\d+)\s+files?\s+changed`)
	insertionsRe   = regexp.MustCompile(`(\d+)\s+insertions?\(\+\)`)
	deletionsRe    = regexp.MustCompile(`(\d+)\s+deletions?\(-\)`)
	httpsRepoRe    = regexp.MustCompile(`github\.com/([^/]+)/([^/.]+)`)
	sshRepoRe      = regexp.MustCompile(`github\.com:([^/]+)/([^/.]+)`)
)

// Executor is an autonomous execution pod that picks up tasks, runs Claude Code,
// and produces PRs.
type Executor struct {
	id                 string
	config             *config.Config
	claude             *claudecli.Runner
	worktree           *WorktreeManager
	manager            *apiclient.Client
	github             *github.Client
	notifier           *notifier.Discord
	vaultWriter        *vault.Writer
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
func NewExecutor(id string, cfg *config.Config, discord *notifier.Discord, vw *vault.Writer) *Executor {
	return &Executor{
		id:          id,
		config:      cfg,
		claude:      claudecli.NewRunner(&cfg.Executor),
		worktree:    NewWorktreeManager(cfg.Orchestrator.WorkspaceBase, cfg.GitHub.Token, cfg.GitHub.Username),
		manager:     apiclient.NewClient(fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)),
		github:      github.NewClient(cfg.GitHub.Token, cfg.GitHub.Username),
		notifier:    discord,
		vaultWriter: vw,
		stopCh:      make(chan struct{}),
	}
}

// Run is the main execution loop. It polls for tasks and executes them.
func (e *Executor) Run(ctx context.Context) {
	slog.Info("executor started", "id", e.id)

	// Register with server
	if err := e.registerPod(); err != nil {
		slog.Warn("failed to register pod", "id", e.id, "error", err)
	}

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
	// 1. Request next task
	task, err := e.manager.NextTask(e.id, "executor")
	if err != nil {
		slog.Error("failed to get next task", "error", err)
		return
	}
	if task == nil {
		time.Sleep(30 * time.Second)
		return
	}

	slog.Info("picked up task", "task_id", task.ID, "title", task.Title, "type", task.Type)
	task.ExecutorID = e.id

	// 2. Prepare execution (model + prompt)
	project, err := e.manager.GetProject(task.ProjectID)
	if err != nil {
		slog.Error("failed to get project", "task_id", task.ID, "project_id", task.ProjectID, "error", err)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("failed to get project: %v", err), 0, 0)
		return
	}

	model, systemPrompt, err := e.prepareExecution(task, project)
	if err != nil {
		slog.Error("failed to prepare execution", "task_id", task.ID, "error", err)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("preparation failed: %v", err), 0, 0)
		return
	}

	// 3. Setup worktree
	worktreePath, err := e.setupWorktree(task, project)
	if err != nil {
		slog.Error("failed to setup worktree", "task_id", task.ID, "error", err)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("worktree setup failed: %v", err), 0, 0)
		return
	}

	// Report execution start
	if err := e.manager.ReportTaskStarted(task.ID, e.id, model, task.BranchName); err != nil {
		slog.Warn("failed to report task started", "task_id", task.ID, "error", err)
	}

	// Update pod status to busy
	if err := e.updatePodStatus("busy", task.ID, task.Title); err != nil {
		slog.Warn("failed to update pod status", "id", e.id, "error", err)
	}

	// 4. Run execution
	result, err := e.runExecution(ctx, task, project, worktreePath, model, systemPrompt)
	if err != nil {
		slog.Error("execution failed", "task_id", task.ID, "error", err)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("execution error: %v", err), 0, 0)
		return
	}
	if result == nil {
		// Early exit (rate limit, decomposition, etc.)
		return
	}

	// 5. Process results (build, test, commit, PR, usage)
	if err := e.processResults(task, result, worktreePath, project); err != nil {
		slog.Error("failed to process results", "task_id", task.ID, "error", err)
		return
	}

	// 6. Update pod status to idle after task completion
	if err := e.updatePodStatus("idle", "", ""); err != nil {
		slog.Warn("failed to update pod status to idle", "id", e.id, "error", err)
	}
}

// prepareExecution assigns model and builds system prompt.
func (e *Executor) prepareExecution(task *models.Task, project *models.Project) (model, systemPrompt string, err error) {
	model, err = e.manager.GetModel(task.ID)
	if err != nil {
		slog.Error("failed to get model", "task_id", task.ID, "error", err)
		model = "sonnet" // fallback
	}
	task.Model = model

	techStack := strings.Join(project.TechStack, ", ")
	systemPrompt = e.buildSystemPrompt(task, project.Name, project.Description, techStack, "", "")

	return model, systemPrompt, nil
}

// setupWorktree creates or reuses a worktree for the task.
func (e *Executor) setupWorktree(task *models.Task, project *models.Project) (string, error) {
	if err := e.worktree.EnsureBareRepo(project.RepoURL, project.Name); err != nil {
		return "", fmt.Errorf("bare repo failed: %w", err)
	}

	var worktreePath string
	var err error

	if task.BranchName != "" {
		// Reuse existing worktree
		worktreePath, err = e.worktree.FindByBranch(project.Name, task.BranchName)
		if err != nil {
			return "", fmt.Errorf("worktree not found: %w", err)
		}
		if err := e.worktree.UpdateWorktree(project.Name, worktreePath, task.BranchName); err != nil {
			return "", fmt.Errorf("worktree update failed: %w", err)
		}
		slog.Info("reusing existing worktree", "task_id", task.ID, "branch", task.BranchName, "path", worktreePath)
	} else {
		// Create new worktree
		worktreePath, task.BranchName, err = e.worktree.CreateWorktree(project.Name, task.ID)
		if err != nil {
			return "", fmt.Errorf("worktree create failed: %w", err)
		}
	}

	return worktreePath, nil
}

// runExecution executes Claude Code and handles early exits (rate limit, decomposition).
// Returns nil result if execution completed with early exit (caller should return).
func (e *Executor) runExecution(ctx context.Context, task *models.Task, project *models.Project, worktreePath, model, systemPrompt string) (*claudecli.Result, error) {
	techStack := strings.Join(project.TechStack, ", ")
	prompt := BuildAutopilotPrompt(task, project.Name, project.Description, techStack)

	// Snapshot sensitive files before execution
	preSnapshot := e.snapshotSensitiveFiles()

	// Execute Claude Code
	e.executionStartTime = time.Now()
	result, err := e.claude.Run(ctx, claudecli.Opts{
		Prompt:       prompt,
		WorkDir:      worktreePath,
		Model:        model,
		SystemPrompt: systemPrompt,
	})
	if err != nil {
		return nil, fmt.Errorf("claude code failed: %w", err)
	}

	// Check rate limit
	if claudecli.IsRateLimited(result.ExitCode, result.Stderr) {
		slog.Warn("rate limited, retrying task", "task_id", task.ID)
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskRetry, "", "rate limited", result.TokensUsed, result.CostUSD)
		return nil, nil
	}

	// Post-execution verification
	if err := e.verifyWorktreeIntegrity(preSnapshot); err != nil {
		_ = e.notifier.Send(notifier.LevelCritical,
			fmt.Sprintf("INTEGRITY VIOLATION task %s: %v", task.ID, err))
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, "", fmt.Sprintf("integrity violation: %v", err), result.TokensUsed, result.CostUSD)
		return nil, fmt.Errorf("integrity violation: %w", err)
	}

	// Check subtask decomposition
	parsed, parseErr := claudecli.ParseResponse(result.Stdout)
	if parseErr == nil {
		if decomp := ParseDecomposition(parsed.ResultText); decomp != nil {
			subtasks := ToSubtaskRequests(decomp)
			slog.Info("task decomposed into subtasks", "task_id", task.ID, "subtask_count", len(subtasks))
			if err := e.manager.CreateSubtasks(task.ID, subtasks); err != nil {
				slog.Error("failed to create subtasks", "task_id", task.ID, "error", err)
				_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("subtask creation failed: %v", err), result.TokensUsed, result.CostUSD)
				return nil, fmt.Errorf("subtask creation failed: %w", err)
			}
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskDecomposed, result.Stdout, "", result.TokensUsed, result.CostUSD)
			return nil, nil
		}
	}

	return result, nil
}

// processResults handles build, test, commit, PR creation, and usage collection.
func (e *Executor) processResults(task *models.Task, result *claudecli.Result, worktreePath string, project *models.Project) error {
	// Build verification
	if task.RequiresTest() {
		buildOK, buildOutput := e.runBuild(worktreePath, task)
		if !buildOK {
			slog.Warn("build failed for task", "task_id", task.ID)
			e.registerBuildFailureTask(task, buildOutput)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("build failed: %s", buildOutput), result.TokensUsed, result.CostUSD)
			return fmt.Errorf("build failed")
		}
	}

	// QA: run tests
	if task.RequiresTest() {
		passed := e.runTests(worktreePath, task)
		task.TestPassed = &passed
		if !passed {
			slog.Warn("tests failed for task", "task_id", task.ID)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, "tests failed", result.TokensUsed, result.CostUSD)
			return fmt.Errorf("tests failed")
		}
	}

	// Commit and get diff
	diffLines, filesChanged, commitErr := e.commitAndGetDiff(worktreePath, task)
	task.DiffLines = diffLines
	task.FilesChanged = filesChanged

	if commitErr != nil {
		if errors.Is(commitErr, ErrNoChanges) {
			slog.Info("no changes produced by Claude Code, completing without PR", "task_id", task.ID)
			_ = e.manager.ReportTaskDone(task.ID, task, models.TaskCompleted, result.Stdout, "", result.TokensUsed, result.CostUSD)
			return nil
		}
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("git failed: %v", commitErr), result.TokensUsed, result.CostUSD)
		return fmt.Errorf("commit failed: %w", commitErr)
	}

	// Check guardrails
	if ExceedsGuardrails(&e.config.Executor, diffLines, filesChanged) {
		slog.Warn("diff exceeds guardrails", "task_id", task.ID, "diff_lines", diffLines, "files_changed", filesChanged)
		_ = e.notifier.Send(notifier.LevelWarning,
			fmt.Sprintf("Task %s diff exceeds guardrails: %d lines, %d files", task.ID, diffLines, filesChanged))
	}

	// Rebase onto main
	if err := e.worktree.RebaseOnMain(worktreePath); err != nil {
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("rebase conflict: %v", err), result.TokensUsed, result.CostUSD)
		return fmt.Errorf("rebase failed: %w", err)
	}

	// Force push after rebase
	forcePushCmd := exec.Command("git", "push", "-f", "origin", task.BranchName)
	forcePushCmd.Dir = worktreePath
	if output, err := forcePushCmd.CombinedOutput(); err != nil {
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, "force push failed after rebase", result.TokensUsed, result.CostUSD)
		return fmt.Errorf("force push failed: %s", string(output))
	}

	// Create PR
	// TODO: move extractOwnerRepo to github.ExtractOwnerRepo
	owner, repo := extractOwnerRepo(project.RepoURL)
	if owner == "" || repo == "" {
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, "invalid repo URL", result.TokensUsed, result.CostUSD)
		return fmt.Errorf("invalid repo URL: %s", project.RepoURL)
	}

	prBuilder := github.NewPRDescriptionBuilder(task, worktreePath, e.id)
	prTitle, prBody := prBuilder.Build()
	defaultBranch := e.worktree.detectDefaultBranchFromWorktree(worktreePath)
	prURL, prNumber, prErr := e.github.CreatePR(owner, repo, task.BranchName, defaultBranch, prTitle, prBody)
	if prErr != nil {
		_ = e.manager.ReportTaskDone(task.ID, task, models.TaskFailed, result.Stdout, fmt.Sprintf("PR creation failed: %v", prErr), result.TokensUsed, result.CostUSD)
		return fmt.Errorf("PR creation failed: %w", prErr)
	}
	task.PRUrl = prURL
	task.PRStatus = "OPEN"

	// Auto-merge decision
	shouldMerge, reason := ShouldAutoMerge(task, diffLines, filesChanged)
	if commentErr := e.github.CreateComment(owner, repo, prNumber, reason); commentErr != nil {
		slog.Warn("failed to post auto-merge comment", "task_id", task.ID, "pr", prNumber, "error", commentErr)
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

	// Collect usage
	if e.config.CCUsage.Command != "" {
		_ = CollectTaskUsage(e.config.CCUsage.Command, worktreePath, task)
	}

	// Record to vault
	if e.vaultWriter != nil {
		_ = RecordTaskCompletion(e.vaultWriter, task, result)
	}

	// Report completion
	_ = e.manager.ReportTaskDone(task.ID, task, models.TaskCompleted, result.Stdout, "", result.TokensUsed, result.CostUSD)
	slog.Info("task completed", "task_id", task.ID, "pr_url", prURL, "pr_status", task.PRStatus)

	return nil
}

// buildSystemPrompt creates a system prompt with the current goal context.
func (e *Executor) buildSystemPrompt(task *models.Task, projectName, projectDesc, projectTech, goalTitle, goalDesc string) string {
	result, err := prompts.Render("system.txt", prompts.SystemPromptData{
		ProjectName:        projectName,
		ProjectDescription: projectDesc,
		ProjectTechStack:   projectTech,
		GoalID:             task.GoalID,
		GoalTitle:          goalTitle,
		GoalDescription:    goalDesc,
		TaskType:           task.Type,
		Priority:           task.Priority,
	})
	if err != nil {
		slog.Warn("failed to render system prompt template", "error", err)
		return "You are an autonomous coding agent. Focus on the task described in the prompt."
	}
	return result
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

// projectCommand represents a command to run when a detection file is found.
type projectCommand struct {
	detectFile string
	command    string
	args       []string
}

// runProjectCommand runs the first matching command from a list of project commands.
// Returns (found, passed, output).
func runProjectCommand(worktreePath string, commands []projectCommand, commandType string) (bool, string) {
	for _, cmd := range commands {
		detectPath := filepath.Join(worktreePath, cmd.detectFile)
		if _, err := os.Stat(detectPath); err == nil {
			slog.Info(fmt.Sprintf("running %s", commandType), "command", cmd.command, "worktree", worktreePath)
			execCmd := exec.Command(cmd.command, cmd.args...)
			execCmd.Dir = worktreePath
			output, err := execCmd.CombinedOutput()
			if err != nil {
				slog.Warn(fmt.Sprintf("%s failed", commandType), "command", cmd.command, "output", string(output), "error", err)
				return false, string(output)
			}
			slog.Info(fmt.Sprintf("%s passed", commandType), "command", cmd.command)
			return true, ""
		}
	}

	// No command detected
	slog.Info(fmt.Sprintf("no %s system detected, skipping", commandType), "worktree", worktreePath)
	return true, ""
}

// runTests detects the test framework and runs tests in the worktree.
func (e *Executor) runTests(worktreePath string, task *models.Task) bool {
	// Skip tests for RESEARCH and DOCUMENT types
	if task.Type == models.TaskTypeResearch || task.Type == models.TaskTypeDocument {
		return true
	}

	testCmds := []projectCommand{
		{"go.mod", "go", []string{"test", "./..."}},
		{"package.json", "npm", []string{"test", "--", "--passWithNoTests"}},
		{"requirements.txt", "pytest", []string{"-x", "--tb=short"}},
		{"pyproject.toml", "pytest", []string{"-x", "--tb=short"}},
		{"Cargo.toml", "cargo", []string{"test"}},
	}

	passed, _ := runProjectCommand(worktreePath, testCmds, "tests")
	return passed
}

// runBuild detects the build system and runs a build in the worktree.
// Returns (passed, output) where output contains error details on failure.
func (e *Executor) runBuild(worktreePath string, task *models.Task) (bool, string) {
	buildCmds := []projectCommand{
		{"go.mod", "go", []string{"build", "./..."}},
		{"package.json", "npm", []string{"run", "build", "--if-present"}},
		{"Cargo.toml", "cargo", []string{"build"}},
		{"Makefile", "make", []string{"build"}},
	}

	return runProjectCommand(worktreePath, buildCmds, "build")
}

// buildFailureTask constructs a BUGFIX task for a build failure.
func buildFailureTask(failedTask *models.Task, buildOutput string) *models.Task {
	// Truncate build output to keep the description manageable
	const maxOutputLen = 2000
	truncatedOutput := buildOutput
	if len(truncatedOutput) > maxOutputLen {
		truncatedOutput = truncatedOutput[len(truncatedOutput)-maxOutputLen:]
	}

	return &models.Task{
		Title:       fmt.Sprintf("Fix build failure from: %s", failedTask.Title),
		Description: fmt.Sprintf("The task %q (ID: %s) produced code that fails to build.\n\nBuild output:\n```\n%s\n```\n\nPlease fix the build errors in branch `%s`.", failedTask.Title, failedTask.ID, truncatedOutput, failedTask.BranchName),
		Type:        models.TaskTypeBugfix,
		Priority:    min(failedTask.Priority, 10), // High priority — build is broken
		Source:      models.TaskSourceSystem,
		ProjectID:   failedTask.ProjectID,
		ParentID:    failedTask.ID,
		Depth:       failedTask.Depth + 1,
		GoalID:      failedTask.GoalID,
		BranchName:  failedTask.BranchName, // Reuse the same branch
		Tags:        []string{"build-failure", "auto-registered"},
	}
}

// registerBuildFailureTask creates a follow-up BUGFIX task to fix the build failure.
func (e *Executor) registerBuildFailureTask(failedTask *models.Task, buildOutput string) {
	bugfixTask := buildFailureTask(failedTask, buildOutput)

	if err := e.manager.CreateTask(bugfixTask); err != nil {
		slog.Error("failed to register build failure task", "parent_task_id", failedTask.ID, "error", err)
		_ = e.notifier.Send(notifier.LevelWarning,
			fmt.Sprintf("Failed to auto-register build fix task for %s: %v", failedTask.ID, err))
		return
	}

	slog.Info("registered build failure task", "parent_task_id", failedTask.ID, "bugfix_task_id", bugfixTask.ID)
	_ = e.notifier.Send(notifier.LevelWarning,
		fmt.Sprintf("Build failed for task %s — auto-registered bugfix task: %s", failedTask.ID, bugfixTask.Title))
}

// ErrNoChanges indicates Claude Code made no code changes.
var ErrNoChanges = fmt.Errorf("no changes to commit")

// commitAndGetDiff stages, commits, and returns diff stats.
// Returns ErrNoChanges if there's nothing to commit.
// Does NOT push — pushing happens after rebase in the caller.
func (e *Executor) commitAndGetDiff(worktreePath string, task *models.Task) (diffLines, filesChanged int, err error) {
	// git add -A
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = worktreePath
	if output, addErr := addCmd.CombinedOutput(); addErr != nil {
		return 0, 0, fmt.Errorf("git add failed: %s", string(output))
	}

	// Check if there are changes to commit
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = worktreePath
	statusOut, statusErr := statusCmd.Output()
	if statusErr != nil || len(strings.TrimSpace(string(statusOut))) == 0 {
		return 0, 0, ErrNoChanges
	}

	// git commit
	commitMsg := fmt.Sprintf("[flux] %s\n\nTask: %s\nType: %s\nPriority: %d",
		task.Title, task.ID, task.Type, task.Priority)
	commitCmd := exec.Command("git", "commit", "-m", commitMsg)
	commitCmd.Dir = worktreePath
	if output, commitErr := commitCmd.CombinedOutput(); commitErr != nil {
		return 0, 0, fmt.Errorf("git commit failed: %s", string(output))
	}

	// git diff --stat HEAD~1
	diffCmd := exec.Command("git", "diff", "--stat", "HEAD~1")
	diffCmd.Dir = worktreePath
	diffOut, diffErr := diffCmd.Output()
	if diffErr != nil {
		slog.Error("git diff --stat failed", "error", diffErr)
	} else {
		diffLines, filesChanged = parseDiffStat(string(diffOut))
	}

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
	if m := filesChangedRe.FindStringSubmatch(summary); len(m) >= 2 {
		filesChanged, _ = strconv.Atoi(m[1])
	}

	// Parse insertions and deletions
	var insertions, deletions int
	if m := insertionsRe.FindStringSubmatch(summary); len(m) >= 2 {
		insertions, _ = strconv.Atoi(m[1])
	}
	if m := deletionsRe.FindStringSubmatch(summary); len(m) >= 2 {
		deletions, _ = strconv.Atoi(m[1])
	}
	diffLines = insertions + deletions

	return diffLines, filesChanged
}

// ShouldAutoMerge determines if a PR should be auto-merged based on task attributes and diff size.
// Returns (shouldMerge bool, reason string).
func ShouldAutoMerge(task *models.Task, diffLines, filesChanged int) (bool, string) {
	// Large diffs always require operator review
	if diffLines > 2000 || filesChanged > 20 {
		return false, fmt.Sprintf("⏸️ **Auto-merge skipped**: Large diff (%d lines, %d files) requires operator review", diffLines, filesChanged)
	}

	// System/self source: auto-merge
	if task.Source == models.TaskSourceSystem || task.Source == models.TaskSourceSelf {
		return true, fmt.Sprintf("✅ **Auto-merged**: System/self-generated task (%d lines, %d files)", diffLines, filesChanged)
	}

	// Maintenance type: auto-merge
	if task.Type == models.TaskTypeMaintenance {
		return true, fmt.Sprintf("✅ **Auto-merged**: Maintenance task (%d lines, %d files)", diffLines, filesChanged)
	}

	// Bugfix with high priority (low number): auto-merge
	if task.Type == models.TaskTypeBugfix && task.Priority <= 10 {
		return true, fmt.Sprintf("✅ **Auto-merged**: High-priority bugfix (P:%d, %d lines, %d files)", task.Priority, diffLines, filesChanged)
	}

	// Small changes: auto-merge
	if filesChanged <= 3 && diffLines < 100 {
		return true, fmt.Sprintf("✅ **Auto-merged**: Small change (%d lines, %d files)", diffLines, filesChanged)
	}

	// Otherwise: operator review
	return false, fmt.Sprintf("⏸️ **Auto-merge skipped**: Requires operator review (Type: %s, Priority: %d, %d lines, %d files)", task.Type, task.Priority, diffLines, filesChanged)
}

// extractOwnerRepo parses owner and repo from a GitHub URL.
// Supports both HTTPS and SSH URL formats.
// TODO: move to github.ExtractOwnerRepo
func extractOwnerRepo(repoURL string) (owner, repo string) {
	// HTTPS: https://github.com/owner/repo.git or https://github.com/owner/repo
	if m := httpsRepoRe.FindStringSubmatch(repoURL); len(m) >= 3 {
		return m[1], m[2]
	}

	// SSH: git@github.com:owner/repo.git
	if m := sshRepoRe.FindStringSubmatch(repoURL); len(m) >= 3 {
		return m[1], m[2]
	}

	return "", ""
}

// claudeCodeSmokeTest verifies Claude Code CLI is available and functional.
func (e *Executor) claudeCodeSmokeTest() error {
	return SmokeTest(e.claude, "claude-sonnet-4-20250514")
}

// SmokeTest verifies the Claude CLI is available and responsive.
// This is exported so it can be reused by other components (e.g., triager).
func SmokeTest(runner *claudecli.Runner, model string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "claude",
		"-p", "respond with exactly: SMOKE_TEST_OK",
		"--max-turns", "1",
		"--output-format", "json",
	)

	stdout := &claudecli.LimitedBuffer{MaxSize: 1 << 20}
	stderr := &claudecli.LimitedBuffer{MaxSize: 1 << 20}
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
	parsed, err := claudecli.ParseResponse(output)
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

// registerPod registers this executor with the server pod registry.
func (e *Executor) registerPod() error {
	payload := map[string]interface{}{
		"id":         e.id,
		"started_at": e.executionStartTime,
	}

	return e.manager.PostInternal("/internal/pods/register", payload, nil)
}

// updatePodStatus updates the pod's current status in the registry.
func (e *Executor) updatePodStatus(status, taskID, taskTitle string) error {
	payload := map[string]interface{}{
		"id":         e.id,
		"status":     status,
		"task_id":    taskID,
		"task_title": taskTitle,
	}

	return e.manager.PostInternal("/internal/pods/status", payload, nil)
}
