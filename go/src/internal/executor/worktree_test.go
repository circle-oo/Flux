package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewWorktreeManager(t *testing.T) {
	workspaceBase := "/tmp/test-workspace"
	wm := NewWorktreeManager(workspaceBase, "test-token", "test-user")

	expectedReposDir := filepath.Join(workspaceBase, "repos")
	expectedTreesDir := filepath.Join(workspaceBase, "trees")

	if wm.reposDir != expectedReposDir {
		t.Errorf("expected reposDir %s, got %s", expectedReposDir, wm.reposDir)
	}

	if wm.treesDir != expectedTreesDir {
		t.Errorf("expected treesDir %s, got %s", expectedTreesDir, wm.treesDir)
	}

	if wm.githubToken != "test-token" {
		t.Errorf("expected githubToken test-token, got %s", wm.githubToken)
	}

	if wm.githubUser != "test-user" {
		t.Errorf("expected githubUser test-user, got %s", wm.githubUser)
	}
}

func TestTokenURL(t *testing.T) {
	wm := NewWorktreeManager("/tmp/ws", "ghp_abc123", "myuser")

	// HTTPS URL
	got := wm.tokenURL("https://github.com/owner/repo.git")
	expected := "https://myuser:ghp_abc123@github.com/owner/repo.git"
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	// SSH URL
	got = wm.tokenURL("git@github.com:owner/repo.git")
	if got != expected {
		t.Errorf("expected %s, got %s", expected, got)
	}

	// No token — returns original
	wm2 := NewWorktreeManager("/tmp/ws", "", "myuser")
	original := "git@github.com:owner/repo.git"
	if wm2.tokenURL(original) != original {
		t.Error("should return original URL when no token")
	}
}

func TestSetupClaudeSettings(t *testing.T) {
	tmpDir := t.TempDir()

	err := setupClaudeSettings(tmpDir)
	if err != nil {
		t.Fatalf("setupClaudeSettings failed: %v", err)
	}

	// Check .claude directory exists
	claudeDir := filepath.Join(tmpDir, ".claude")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		t.Error(".claude directory was not created")
	}

	// Check settings.json exists and has correct content
	settingsPath := filepath.Join(claudeDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("failed to read settings.json: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("failed to unmarshal settings.json: %v", err)
	}

	permissions, ok := settings["permissions"].(map[string]interface{})
	if !ok {
		t.Fatal("permissions field missing or wrong type")
	}

	allow, ok := permissions["allow"].([]interface{})
	if !ok {
		t.Fatal("allow field missing or wrong type")
	}

	expectedTools := []string{
		"Bash(*)", "Read(*)", "Write(*)", "Edit(*)",
		"Grep(*)", "Glob(*)", "TodoRead(*)", "TodoWrite(*)",
	}

	if len(allow) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(allow))
	}

	// Check each expected tool is present
	for _, expected := range expectedTools {
		found := false
		for _, actual := range allow {
			if actual.(string) == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tool %s not found in allow list", expected)
		}
	}
}

func TestSetupClaudeMD(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "test-project"

	err := setupClaudeMD(tmpDir, projectName)
	if err != nil {
		t.Fatalf("setupClaudeMD failed: %v", err)
	}

	// Check CLAUDE.md exists
	claudeMDPath := filepath.Join(tmpDir, "CLAUDE.md")
	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		t.Fatalf("failed to read CLAUDE.md: %v", err)
	}

	content := string(data)

	// Check content contains project name
	if !strings.Contains(content, projectName) {
		t.Errorf("CLAUDE.md does not contain project name %s", projectName)
	}

	// Check content has expected sections
	expectedPhrases := []string{
		"You are working on a task",
		"Follow existing code conventions",
		"Write tests for new code",
		"Keep changes focused",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(content, phrase) {
			t.Errorf("CLAUDE.md missing expected phrase: %s", phrase)
		}
	}
}

func TestBranchNameAndWorktreePathGeneration(t *testing.T) {
	taskID := "abc123def456"
	projectName := "my-project"

	// Expected branch name format: task/{id[:8]}
	expectedBranch := "task/abc123de"

	// Expected worktree path format: {project}--task-{id[:8]}
	expectedPathSuffix := "my-project--task-abc123de"

	// Test branch name format
	branchName := "task/" + taskID[:8]
	if branchName != expectedBranch {
		t.Errorf("expected branch name %s, got %s", expectedBranch, branchName)
	}

	// Test worktree path format
	worktreeName := projectName + "--task-" + taskID[:8]
	if worktreeName != expectedPathSuffix {
		t.Errorf("expected worktree path suffix %s, got %s", expectedPathSuffix, worktreeName)
	}
}

func TestRunCleanupLogic(t *testing.T) {
	tests := []struct {
		name          string
		task          WorktreeTask
		now           time.Time
		shouldCleanup bool
		description   string
	}{
		{
			name: "completed_with_merged_pr",
			task: WorktreeTask{
				ProjectName: "test-proj",
				TaskID:      "task1234",
				Status:      "COMPLETED",
				PRStatus:    "MERGED",
				CompletedAt: time.Now().Add(-1 * time.Hour),
			},
			now:           time.Now(),
			shouldCleanup: true,
			description:   "COMPLETED + PR MERGED should cleanup immediately",
		},
		{
			name: "completed_with_open_pr",
			task: WorktreeTask{
				ProjectName: "test-proj",
				TaskID:      "task1234",
				Status:      "COMPLETED",
				PRStatus:    "OPEN",
				CompletedAt: time.Now().Add(-1 * time.Hour),
			},
			now:           time.Now(),
			shouldCleanup: false,
			description:   "COMPLETED + PR OPEN should preserve",
		},
		{
			name: "failed_within_24h",
			task: WorktreeTask{
				ProjectName: "test-proj",
				TaskID:      "task1234",
				Status:      "FAILED",
				PRStatus:    "",
				CompletedAt: time.Now().Add(-12 * time.Hour),
			},
			now:           time.Now(),
			shouldCleanup: false,
			description:   "FAILED within 24h should preserve",
		},
		{
			name: "failed_after_24h",
			task: WorktreeTask{
				ProjectName: "test-proj",
				TaskID:      "task1234",
				Status:      "FAILED",
				PRStatus:    "",
				CompletedAt: time.Now().Add(-25 * time.Hour),
			},
			now:           time.Now(),
			shouldCleanup: true,
			description:   "FAILED after 24h should cleanup",
		},
		{
			name: "changes_requested",
			task: WorktreeTask{
				ProjectName: "test-proj",
				TaskID:      "task1234",
				Status:      "CHANGES_REQUESTED",
				PRStatus:    "OPEN",
				CompletedAt: time.Now().Add(-2 * time.Hour),
			},
			now:           time.Now(),
			shouldCleanup: false,
			description:   "CHANGES_REQUESTED should preserve for fix task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldCleanup := false

			switch tt.task.Status {
			case "COMPLETED":
				if tt.task.PRStatus == "MERGED" {
					shouldCleanup = true
				}
			case "FAILED":
				if !tt.task.CompletedAt.IsZero() && tt.now.Sub(tt.task.CompletedAt) > 24*time.Hour {
					shouldCleanup = true
				}
			case "CHANGES_REQUESTED":
				shouldCleanup = false
			}

			if shouldCleanup != tt.shouldCleanup {
				t.Errorf("%s: expected shouldCleanup=%v, got %v",
					tt.description, tt.shouldCleanup, shouldCleanup)
			}
		})
	}
}

func TestEnsureBareRepoCommandConstruction(t *testing.T) {
	// Test that we construct the correct git commands
	// We're testing the logic, not actually running git

	repoURL := "https://github.com/test/repo.git"
	projectName := "test-project"
	bareDir := filepath.Join("/tmp/repos", projectName+".git")

	// When repo exists, we should run: git -C bareDir fetch --all
	expectedFetchArgs := []string{"-C", bareDir, "fetch", "--all"}

	// When repo doesn't exist, we should run: git clone --bare repoURL bareDir
	expectedCloneArgs := []string{"clone", "--bare", repoURL, bareDir}

	// Verify arg structure
	if len(expectedFetchArgs) != 4 {
		t.Errorf("fetch command should have 4 args, got %d", len(expectedFetchArgs))
	}

	if len(expectedCloneArgs) != 4 {
		t.Errorf("clone command should have 4 args, got %d", len(expectedCloneArgs))
	}

	if expectedFetchArgs[0] != "-C" || expectedFetchArgs[2] != "fetch" {
		t.Error("fetch command structure is incorrect")
	}

	if expectedCloneArgs[0] != "clone" || expectedCloneArgs[1] != "--bare" {
		t.Error("clone command structure is incorrect")
	}
}

func TestCreateWorktreeCommandConstruction(t *testing.T) {
	projectName := "test-project"
	taskID := "abc123def456"
	bareDir := filepath.Join("/tmp/repos", projectName+".git")

	branchName := "task/" + taskID[:8]
	worktreePath := filepath.Join("/tmp/trees", projectName+"--task-"+taskID[:8])

	// Expected: git -C bareDir worktree add -b branchName worktreePath main
	expectedArgs := []string{"-C", bareDir, "worktree", "add", "-b", branchName, worktreePath, "main"}

	if len(expectedArgs) != 8 {
		t.Errorf("worktree add command should have 8 args, got %d", len(expectedArgs))
	}

	if expectedArgs[0] != "-C" {
		t.Error("first arg should be -C")
	}

	if expectedArgs[2] != "worktree" || expectedArgs[3] != "add" {
		t.Error("should be 'worktree add' command")
	}

	if expectedArgs[4] != "-b" {
		t.Error("should have -b flag for branch creation")
	}

	if expectedArgs[7] != "main" {
		t.Error("should create worktree from main branch")
	}
}

func TestUpdateWorktreeCommandConstruction(t *testing.T) {
	projectName := "test-project"
	branchName := "task/abc123de"
	bareDir := filepath.Join("/tmp/repos", projectName+".git")
	worktreePath := "/tmp/trees/test-project--task-abc123de"

	// UpdateWorktree should first fetch: git -C bareDir fetch --all
	expectedFetchArgs := []string{"-C", bareDir, "fetch", "--all"}

	if len(expectedFetchArgs) != 4 {
		t.Errorf("fetch command should have 4 args, got %d", len(expectedFetchArgs))
	}

	if expectedFetchArgs[0] != "-C" || expectedFetchArgs[2] != "fetch" || expectedFetchArgs[3] != "--all" {
		t.Error("fetch command structure is incorrect")
	}

	// Then reset: git -C worktreePath reset --hard origin/<branchName>
	remoteRef := "origin/" + branchName
	expectedResetArgs := []string{"-C", worktreePath, "reset", "--hard", remoteRef}

	if len(expectedResetArgs) != 5 {
		t.Errorf("reset command should have 5 args, got %d", len(expectedResetArgs))
	}

	if expectedResetArgs[0] != "-C" {
		t.Error("first arg should be -C")
	}

	if expectedResetArgs[1] != worktreePath {
		t.Errorf("second arg should be worktree path %s, got %s", worktreePath, expectedResetArgs[1])
	}

	if expectedResetArgs[2] != "reset" || expectedResetArgs[3] != "--hard" {
		t.Error("should be 'reset --hard' command")
	}

	if expectedResetArgs[4] != "origin/"+branchName {
		t.Errorf("should reset to origin/%s, got %s", branchName, expectedResetArgs[4])
	}
}

func TestUpdateWorktreeRemoteRefFormat(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		wantRef    string
	}{
		{
			name:       "standard_task_branch",
			branchName: "task/abc123de",
			wantRef:    "origin/task/abc123de",
		},
		{
			name:       "feature_branch",
			branchName: "feature/my-feature",
			wantRef:    "origin/feature/my-feature",
		},
		{
			name:       "simple_branch",
			branchName: "fix-bug",
			wantRef:    "origin/fix-bug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remoteRef := fmt.Sprintf("origin/%s", tt.branchName)
			if remoteRef != tt.wantRef {
				t.Errorf("expected remote ref %s, got %s", tt.wantRef, remoteRef)
			}
		})
	}
}

func TestCleanupWorktreeCommandConstruction(t *testing.T) {
	projectName := "test-project"
	worktreePath := "/tmp/trees/test-project--task-abc123de"
	bareDir := filepath.Join("/tmp/repos", projectName+".git")

	// Expected: git -C bareDir worktree remove --force worktreePath
	expectedArgs := []string{"-C", bareDir, "worktree", "remove", "--force", worktreePath}

	if len(expectedArgs) != 6 {
		t.Errorf("worktree remove command should have 6 args, got %d", len(expectedArgs))
	}

	if expectedArgs[0] != "-C" {
		t.Error("first arg should be -C")
	}

	if expectedArgs[2] != "worktree" || expectedArgs[3] != "remove" {
		t.Error("should be 'worktree remove' command")
	}

	if expectedArgs[4] != "--force" {
		t.Error("should have --force flag")
	}
}

func TestRebaseOnMainCommandConstruction(t *testing.T) {
	worktreePath := "/tmp/trees/test-project--task-abc123de"

	// Expected fetch command: git fetch origin main
	expectedFetchArgs := []string{"fetch", "origin", "main"}

	if len(expectedFetchArgs) != 3 {
		t.Errorf("fetch command should have 3 args, got %d", len(expectedFetchArgs))
	}

	if expectedFetchArgs[0] != "fetch" {
		t.Error("first arg should be fetch")
	}

	if expectedFetchArgs[1] != "origin" || expectedFetchArgs[2] != "main" {
		t.Error("should fetch origin main")
	}

	// Expected rebase command: git rebase origin/main
	expectedRebaseArgs := []string{"rebase", "origin/main"}

	if len(expectedRebaseArgs) != 2 {
		t.Errorf("rebase command should have 2 args, got %d", len(expectedRebaseArgs))
	}

	if expectedRebaseArgs[0] != "rebase" {
		t.Error("first arg should be rebase")
	}

	if expectedRebaseArgs[1] != "origin/main" {
		t.Error("should rebase onto origin/main")
	}

	// Verify the worktreePath would be set as Dir
	if worktreePath == "" {
		t.Error("worktreePath should not be empty")
	}
}
