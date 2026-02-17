package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	fluxv1 "github.com/circle-oo/flux/gen/flux/v1"
	"github.com/circle-oo/flux/internal/agent"
	"github.com/circle-oo/flux/internal/apiclient"
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
)

// ExecutionResult holds the outcome of a task execution via the Python Agent Manager.
type ExecutionResult struct {
	Output    string        // accumulated agent output
	Duration  time.Duration // total execution time
	SessionID string        // Claude Code session ID for ccusage tracking
}

// Executor is an autonomous execution pod that picks up tasks, delegates execution
// to the Python Agent Manager via gRPC, and produces PRs.
type Executor struct {
	id                 string
	config             *config.Config
	agentClient        *agent.Client
	worktree           *WorktreeManager
	manager            *apiclient.Client
	github             *github.Client
	notifier           *notifier.Discord
	vaultWriter        *vault.Writer
	stopCh             chan struct{}
	stopOnce           sync.Once
	executionStartTime time.Time

	mu            sync.Mutex // guards currentTaskID and running
	currentTaskID string
	running       bool
}

// sensitiveFile tracks a file path and its mod time for integrity verification.
type sensitiveFile struct {
	path    string
	modTime time.Time
	exists  bool
}

// NewExecutor creates a new Executor pod.
func NewExecutor(id string, cfg *config.Config, discord *notifier.Discord, vw *vault.Writer, ac *agent.Client) *Executor {
	return &Executor{
		id:                 id,
		config:             cfg,
		agentClient:        ac,
		worktree:           NewWorktreeManager(cfg.Orchestrator.WorkspaceBase, cfg.GitHub.Token, cfg.GitHub.Username),
		manager:            apiclient.NewClient(fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)),
		github:             github.NewClient(cfg.GitHub.Token, cfg.GitHub.Username),
		notifier:           discord,
		vaultWriter:        vw,
		stopCh:             make(chan struct{}),
		executionStartTime: time.Now(),
		currentTaskID:      "",
		running:            false,
	}
}

// Run is the main execution loop. It polls for tasks and executes them.
func (e *Executor) Run(ctx context.Context) {
	slog.Info("executor started", "id", e.id)
	e.mu.Lock()
	e.running = true
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running = false
		e.mu.Unlock()
	}()

	// Register with server
	if err := e.registerPod(); err != nil {
		slog.Warn("failed to register pod", "id", e.id, "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("executor stopping due to context cancellation", "id", e.id)
			return
		case <-e.stopCh:
			slog.Info("executor stopping due to stop signal", "id", e.id)
			return
		default:
			e.executeOnce(ctx)

			// Interruptible wait — respond to stop signals immediately
			select {
			case <-ctx.Done():
				slog.Info("executor stopping due to context cancellation", "id", e.id)
				return
			case <-e.stopCh:
				slog.Info("executor stopping due to stop signal", "id", e.id)
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

// Stop signals the executor to stop. Safe to call multiple times.
func (e *Executor) Stop() {
	e.stopOnce.Do(func() { close(e.stopCh) })
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
		// Interruptible wait — respond to stop signals immediately
		select {
		case <-ctx.Done():
		case <-e.stopCh:
		case <-time.After(30 * time.Second):
		}
		return
	}

	slog.Info("picked up task", "task_id", task.ID, "title", task.Title)
	task.ExecutorID = e.id
	e.mu.Lock()
	e.currentTaskID = task.ID
	e.mu.Unlock()

	// 2. Prepare execution (model + prompt)
	project, err := e.manager.GetProject(task.ProjectID)
	if err != nil {
		slog.Error("failed to get project", "task_id", task.ID, "project_id", task.ProjectID, "error", err)
		e.reportFailure(task.ID, task, "", fmt.Sprintf("failed to get project: %v", err))
		return
	}

	model, systemPrompt, err := e.prepareExecution(task, project)
	if err != nil {
		slog.Error("failed to prepare execution", "task_id", task.ID, "error", err)
		e.reportFailure(task.ID, task, "", fmt.Sprintf("preparation failed: %v", err))
		return
	}

	// 3. Setup worktree
	worktreePath, err := e.setupWorktree(task, project)
	if err != nil {
		slog.Error("failed to setup worktree", "task_id", task.ID, "error", err)
		e.reportFailure(task.ID, task, "", fmt.Sprintf("worktree setup failed: %v", err))
		return
	}

	// Report execution start
	if err := e.manager.ReportTaskStarted(task.ID, e.id, model, task.BranchName); err != nil {
		slog.Warn("failed to report task started", "task_id", task.ID, "error", err)
	}

	// Update pod status to busy; defer reset to idle so every exit path is covered.
	if err := e.updatePodStatus("busy", task.ID, task.Title); err != nil {
		slog.Warn("failed to update pod status", "id", e.id, "error", err)
	}
	defer func() {
		e.mu.Lock()
		e.currentTaskID = ""
		e.mu.Unlock()
		if err := e.updatePodStatus("idle", "", ""); err != nil {
			slog.Warn("failed to update pod status to idle", "id", e.id, "error", err)
		}
	}()

	// 4. Run execution
	result, err := e.runExecution(ctx, task, project, worktreePath, model, systemPrompt)
	if err != nil {
		slog.Error("execution failed", "task_id", task.ID, "error", err)
		e.reportFailure(task.ID, task, "", fmt.Sprintf("execution error: %v", err))
		return
	}
	if result == nil {
		// Early exit (rate limit, decomposition, etc.)
		return
	}

	// 5. Process results (build, test, commit, PR)
	processErr := e.processResults(task, result, worktreePath, project)
	if processErr != nil {
		slog.Error("failed to process results", "task_id", task.ID, "error", processErr)
	}

	// 6. Post-task hooks: ccusage + vault always run regardless of processResults outcome.
	// Streaming events already provide usage data; ccusage reconciles asynchronously.
	if e.config.CCUsage.Command != "" {
		ccCmd := e.config.CCUsage.Command
		taskID := task.ID
		sessionID := result.SessionID
		model := task.Model
		go func() {
			// Wait for JSONL logs to flush before querying ccusage.
			time.Sleep(5 * time.Second)
			t := &models.Task{ID: taskID}
			_ = CollectTaskUsage(ccCmd, worktreePath, sessionID, t)
			if t.TokensUsed > 0 || t.CostUSD > 0 {
				meta := map[string]string{"model": model}
				if sessionID != "" {
					meta["session_id"] = sessionID
				}
				_ = e.manager.ReportTaskUsage(taskID, t.TokensUsed, t.CostUSD, "ccusage", meta)
			}
		}()
	}

	if e.vaultWriter != nil {
		_ = RecordTaskCompletion(e.vaultWriter, task, result)
	}

	if processErr != nil {
		return
	}
}

// reportFailure reports a task as failed to the manager. Consolidates the
// repeated pattern of ReportTaskDone + warn-on-error logging.
func (e *Executor) reportFailure(taskID string, task *models.Task, output, reason string) {
	if err := e.manager.ReportTaskDone(taskID, task, models.TaskFailed, output, reason, task.TokensUsed, task.CostUSD); err != nil {
		slog.Warn("failed to report task done", "task_id", taskID, "error", err)
	}
}

// prepareExecution assigns model and builds system prompt.
func (e *Executor) prepareExecution(task *models.Task, project *models.Project) (model, systemPrompt string, err error) {
	model, err = e.manager.GetModel(task.ID)
	if err != nil {
		slog.Error("failed to get model", "task_id", task.ID, "error", err)
		model = models.DefaultModel // fallback
	}
	task.Model = model

	techStack := strings.Join(project.TechStack, ", ")
	systemPrompt = e.buildSystemPrompt(task, project.Name, project.Description, techStack, "", "")

	return model, systemPrompt, nil
}

// setupWorktree creates or reuses a worktree for the task.
// Each task gets a dedicated clone + worktree for complete isolation.
func (e *Executor) setupWorktree(task *models.Task, project *models.Project) (string, error) {
	var worktreePath string
	var err error

	if task.BranchName != "" {
		// Reuse existing worktree
		worktreePath, err = e.worktree.FindByBranch(project.Name, task.BranchName)
		if err != nil {
			return "", fmt.Errorf("worktree not found: %w", err)
		}
		if err := e.worktree.UpdateWorktree(worktreePath, task.BranchName); err != nil {
			return "", fmt.Errorf("worktree update failed: %w", err)
		}
		slog.Info("reusing existing worktree", "task_id", task.ID, "branch", task.BranchName, "path", worktreePath)
	} else {
		// Create new dedicated worktree with its own repo clone
		worktreePath, task.BranchName, err = e.worktree.CreateWorktree(project.Name, task.ID, project.RepoURL)
		if err != nil {
			return "", fmt.Errorf("worktree create failed: %w", err)
		}
		slog.Info("created dedicated worktree", "task_id", task.ID, "branch", task.BranchName, "path", worktreePath)
	}

	// Generate protobuf code if the worktree has a buf config (gitignored gen/ files).
	e.generateProto(worktreePath)

	return worktreePath, nil
}

// generateProto runs `buf generate proto` if buf.gen.yaml exists in the worktree.
// Generated proto files are gitignored, so fresh worktrees need them regenerated.
func (e *Executor) generateProto(worktreePath string) {
	bufConfig := filepath.Join(worktreePath, "buf.gen.yaml")
	if _, err := os.Stat(bufConfig); err != nil {
		return // no buf config, nothing to generate
	}
	protoDir := filepath.Join(worktreePath, "proto")
	if _, err := os.Stat(protoDir); err != nil {
		return // no proto directory
	}

	slog.Info("generating protobuf code in worktree", "path", worktreePath)
	cmd := exec.Command("buf", "generate", "proto")
	cmd.Dir = worktreePath
	if output, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("buf generate failed in worktree", "error", err, "output", string(output))
	}
}

// mapAgentType maps a task's tags to a Python agent type.
func mapAgentType(task *models.Task) string {
	for _, tag := range task.Tags {
		switch strings.ToLower(tag) {
		case "research":
			return "rnd"
		case "deploy":
			return "devops"
		}
	}
	return "dev"
}

// runExecution delegates execution to the Python Agent Manager via gRPC.
// Returns nil result if execution completed with early exit (caller should return).
func (e *Executor) runExecution(ctx context.Context, task *models.Task, project *models.Project, worktreePath, model, systemPrompt string) (*ExecutionResult, error) {
	if e.agentClient == nil {
		return nil, fmt.Errorf("agent client not available (Python Agent Manager not connected)")
	}

	techStack := strings.Join(project.TechStack, ", ")
	prompt := BuildAutopilotPrompt(task, project.Name, project.Description, techStack)

	// Snapshot sensitive files before execution
	preSnapshot := e.snapshotSensitiveFiles()

	// Build gRPC request for Python Agent Manager
	agentType := mapAgentType(task)
	req := &fluxv1.ExecuteTaskRequest{
		TaskId:           task.ID,
		AgentType:        agentType,
		Prompt:           prompt,
		WorkingDirectory: worktreePath,
		SystemPrompt:     systemPrompt,
		Metadata: map[string]string{
			"model":    model,
			"executor": e.id,
		},
	}

	// Execute via gRPC with timeout from config
	execCtx, cancel := context.WithTimeout(ctx, e.config.Executor.MaxExecutionTime)
	defer cancel()

	e.executionStartTime = time.Now()

	agentResult, err := e.agentClient.ExecuteTask(execCtx, req, func(event *fluxv1.TaskEvent) {
		slog.Debug("agent event", "task_id", task.ID, "type", event.GetType().String())

		// Extract and report incremental usage from event metadata
		if meta := event.GetMetadata(); meta != nil {
			if costStr, ok := meta["cost_usd"]; ok && costStr != "" {
				cost, _ := strconv.ParseFloat(costStr, 64)
				tokens := 0
				if tokensStr, ok := meta["total_tokens"]; ok {
					tokens, _ = strconv.Atoi(tokensStr)
				}
				if cost > 0 || tokens > 0 {
					go func() {
						if err := e.manager.ReportTaskUsage(task.ID, tokens, cost, "executor", map[string]string{
							"session_id": meta["session_id"],
							"num_turns":  meta["num_turns"],
							"model":      model,
						}); err != nil {
							slog.Debug("failed to report usage event", "task_id", task.ID, "error", err)
						}
					}()
				}
			}
		}
	})

	duration := time.Since(e.executionStartTime)

	// Use the final result from TASK_COMPLETE; fall back to intermediate output
	outputText := ""
	if agentResult != nil {
		if agentResult.Result != "" {
			outputText = agentResult.Result
		} else {
			outputText = agentResult.Output
		}
	}

	sessionID := ""
	if agentResult != nil {
		sessionID = agentResult.SessionID
	}

	result := &ExecutionResult{
		Output:    outputText,
		Duration:  duration,
		SessionID: sessionID,
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			slog.Warn("task execution timed out", "task_id", task.ID, "duration", duration)
			e.reportFailure(task.ID, task, result.Output, "execution timed out")
			return nil, nil
		}
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	// Check if the agent reported an error
	if agentResult != nil && agentResult.IsError {
		slog.Warn("agent reported task error", "task_id", task.ID, "error", agentResult.ErrorMessage)
		e.reportFailure(task.ID, task, result.Output, agentResult.ErrorMessage)
		return nil, nil
	}

	// Post-execution verification
	if err := e.verifyWorktreeIntegrity(preSnapshot); err != nil {
		_ = e.notifier.Send(notifier.LevelCritical,
			fmt.Sprintf("INTEGRITY VIOLATION task %s: %v", task.ID, err))
		e.reportFailure(task.ID, task, "", fmt.Sprintf("integrity violation: %v", err))
		return nil, fmt.Errorf("integrity violation: %w", err)
	}

	// Check subtask decomposition
	if decomp := ParseDecomposition(result.Output); decomp != nil {
		subtasks := ToSubtaskRequests(decomp)
		slog.Info("task decomposed into subtasks", "task_id", task.ID, "subtask_count", len(subtasks))
		if err := e.manager.CreateSubtasks(task.ID, subtasks, nil); err != nil {
			slog.Error("failed to create subtasks", "task_id", task.ID, "error", err)
			e.reportFailure(task.ID, task, result.Output, fmt.Sprintf("subtask creation failed: %v", err))
			return nil, fmt.Errorf("subtask creation failed: %w", err)
		}
		if err := e.manager.ReportTaskDone(task.ID, task, models.TaskDecomposed, result.Output, "", task.TokensUsed, task.CostUSD); err != nil {
			slog.Warn("failed to report task done", "task_id", task.ID, "error", err)
		}
		return nil, nil
	}

	return result, nil
}

// processResults handles build, test, commit, PR creation, and usage collection.
func (e *Executor) processResults(task *models.Task, result *ExecutionResult, worktreePath string, project *models.Project) error {
	// Build verification
	if task.RequiresTest() {
		buildOK, buildOutput := e.runBuild(worktreePath)
		if !buildOK {
			slog.Warn("build failed for task", "task_id", task.ID)
			e.reportFailure(task.ID, task, result.Output, fmt.Sprintf("build failed: %s", buildOutput))
			return fmt.Errorf("build failed")
		}
	}

	// QA: run tests
	if task.RequiresTest() {
		passed := e.runTests(worktreePath)
		task.TestPassed = &passed
		if !passed {
			slog.Warn("tests failed for task", "task_id", task.ID)
			e.reportFailure(task.ID, task, result.Output, "tests failed")
			return fmt.Errorf("tests failed")
		}
	}

	// Commit and get diff
	diffLines, filesChanged, commitErr := e.commitAndGetDiff(worktreePath, task)
	task.DiffLines = diffLines
	task.FilesChanged = filesChanged

	if commitErr != nil {
		if errors.Is(commitErr, ErrNoChanges) {
			slog.Info("no changes produced, completing without PR", "task_id", task.ID)
			if err := e.manager.ReportTaskDone(task.ID, task, models.TaskCompleted, result.Output, "", task.TokensUsed, task.CostUSD); err != nil {
				slog.Warn("failed to report task done", "task_id", task.ID, "error", err)
			}
			return nil
		}
		e.reportFailure(task.ID, task, result.Output, fmt.Sprintf("git failed: %v", commitErr))
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
		e.reportFailure(task.ID, task, result.Output, fmt.Sprintf("rebase conflict: %v", err))
		return fmt.Errorf("rebase failed: %w", err)
	}

	// Force push after rebase
	forcePushCmd := exec.Command("git", "push", "-f", "origin", task.BranchName)
	forcePushCmd.Dir = worktreePath
	if output, err := forcePushCmd.CombinedOutput(); err != nil {
		e.reportFailure(task.ID, task, result.Output, "force push failed after rebase")
		return fmt.Errorf("force push failed: %s", string(output))
	}

	// Extract task result for PR description
	taskSummary := ExtractTaskSummary(result.Output)
	if taskSummary != "" {
		task.Result = taskSummary
	} else if result.Output != "" {
		task.Result = result.Output
	}

	// Create PR
	owner, repo := github.ExtractOwnerRepo(project.RepoURL)
	if owner == "" || repo == "" {
		e.reportFailure(task.ID, task, result.Output, "invalid repo URL")
		return fmt.Errorf("invalid repo URL: %s", project.RepoURL)
	}

	defaultBranch := e.worktree.detectDefaultBranchFromWorktree(worktreePath)
	prBuilder := github.NewPRDescriptionBuilder(task, worktreePath, e.id, defaultBranch)
	prTitle, prBody := prBuilder.Build()
	prURL, prNumber, prErr := e.github.CreatePR(owner, repo, task.BranchName, defaultBranch, prTitle, prBody)
	if prErr != nil {
		e.reportFailure(task.ID, task, result.Output, fmt.Sprintf("PR creation failed: %v", prErr))
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

	// Report completion
	if err := e.manager.ReportTaskDone(task.ID, task, models.TaskCompleted, result.Output, "", task.TokensUsed, task.CostUSD); err != nil {
		slog.Warn("failed to report task done", "task_id", task.ID, "error", err)
	}
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
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home directory for integrity checks", "error", err)
		return nil
	}
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
			slog.Info("running project command", "type", commandType, "command", cmd.command, "worktree", worktreePath)
			execCmd := exec.Command(cmd.command, cmd.args...)
			execCmd.Dir = worktreePath
			output, err := execCmd.CombinedOutput()
			if err != nil {
				slog.Warn("project command failed", "type", commandType, "command", cmd.command, "output", string(output), "error", err)
				return false, string(output)
			}
			slog.Info("project command passed", "type", commandType, "command", cmd.command)
			return true, ""
		}
	}

	// No command detected
	slog.Info("no project command detected, skipping", "type", commandType, "worktree", worktreePath)
	return true, ""
}

// runTests detects the test framework and runs tests in the worktree.
func (e *Executor) runTests(worktreePath string) bool {
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
func (e *Executor) runBuild(worktreePath string) (bool, string) {
	buildCmds := []projectCommand{
		{"go.mod", "go", []string{"build", "./..."}},
		{"package.json", "npm", []string{"run", "build", "--if-present"}},
		{"Cargo.toml", "cargo", []string{"build"}},
		{"Makefile", "make", []string{"build"}},
	}

	return runProjectCommand(worktreePath, buildCmds, "build")
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
	commitMsg := fmt.Sprintf("[flux] %s\n\nTask: %s\nPriority: %d",
		task.Title, task.ID, task.Priority)
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

	// Tasks with maintenance or bugfix tags: auto-merge
	if hasTag(task, "maintenance") {
		return true, fmt.Sprintf("✅ **Auto-merged**: Maintenance task (%d lines, %d files)", diffLines, filesChanged)
	}

	// Bugfix with high priority (low number): auto-merge
	if hasTag(task, "bugfix") && task.Priority <= 15 {
		return true, fmt.Sprintf("✅ **Auto-merged**: High-priority bugfix (P:%d, %d lines, %d files)", task.Priority, diffLines, filesChanged)
	}

	// Small changes: auto-merge
	if filesChanged <= 3 && diffLines < 100 {
		return true, fmt.Sprintf("✅ **Auto-merged**: Small change (%d lines, %d files)", diffLines, filesChanged)
	}

	// Otherwise: operator review
	return false, fmt.Sprintf("⏸️ **Auto-merge skipped**: Requires operator review (Priority: %d, %d lines, %d files)", task.Priority, diffLines, filesChanged)
}

// hasTag checks if a task has a specific tag.
func hasTag(task *models.Task, tag string) bool {
	for _, t := range task.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// registerPod registers this executor with the server pod registry with exponential backoff retry.
func (e *Executor) registerPod() error {
	payload := map[string]interface{}{
		"id":         e.id,
		"started_at": e.executionStartTime,
		"pod_type":   "executor",
	}

	// Default retry parameters if not configured
	maxRetries := e.config.Executor.RegistrationMaxRetries
	if maxRetries <= 0 {
		maxRetries = 10 // default: 10 attempts
	}

	initialDelay := e.config.Executor.RegistrationInitialDelay
	if initialDelay <= 0 {
		initialDelay = 100 * time.Millisecond // default: 100ms
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := e.manager.PostInternal("/internal/pods/register", payload, nil)
		if err == nil {
			if attempt > 1 {
				slog.Info("pod registration succeeded after retries",
					"id", e.id, "attempt", attempt, "total_attempts", maxRetries)
			}
			return nil
		}

		lastErr = err

		// Don't retry on last attempt
		if attempt == maxRetries {
			break
		}

		// Calculate exponential backoff with jitter
		delay := time.Duration(1<<uint(attempt-1)) * initialDelay
		// Add jitter (0-25% of delay) to avoid thundering herd
		jitter := time.Duration(float64(delay) * 0.25 * rand.Float64())
		totalDelay := delay + jitter

		// Cap maximum delay at 10 seconds
		if totalDelay > 10*time.Second {
			totalDelay = 10 * time.Second
		}

		slog.Warn("pod registration failed, retrying",
			"id", e.id,
			"attempt", attempt,
			"max_attempts", maxRetries,
			"error", err,
			"retry_after", totalDelay)

		time.Sleep(totalDelay)
	}

	// Registration failed after all retries - log but don't fail startup
	slog.Error("pod registration failed after all retries, continuing anyway",
		"id", e.id,
		"attempts", maxRetries,
		"last_error", lastErr)

	return lastErr
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

// IsRunning returns whether the executor is currently running.
// Implements the shutdown.Pod interface.
func (e *Executor) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// CurrentTaskID returns the ID of the task currently being executed.
// Returns empty string if no task is active.
// Implements the shutdown.Pod interface.
func (e *Executor) CurrentTaskID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.currentTaskID
}
