package executor

import (
	"fmt"
	"os/exec"

	"github.com/circle-oo/flux/internal/config"
)

// CheckDiffLimits runs git diff --stat HEAD~1 in the worktree and parses the output.
func CheckDiffLimits(worktreePath string) (diffLines, filesChanged int, err error) {
	cmd := exec.Command("git", "diff", "--stat", "HEAD~1")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, fmt.Errorf("git diff --stat failed: %w", err)
	}

	diffLines, filesChanged = parseDiffStat(string(output))
	return diffLines, filesChanged, nil
}

// ExceedsGuardrails checks whether the diff exceeds configured limits.
func ExceedsGuardrails(cfg *config.ExecutorConfig, diffLines, filesChanged int) bool {
	if cfg.MaxDiffLines > 0 && diffLines > cfg.MaxDiffLines {
		return true
	}
	if cfg.MaxFilesChanged > 0 && filesChanged > cfg.MaxFilesChanged {
		return true
	}
	return false
}
