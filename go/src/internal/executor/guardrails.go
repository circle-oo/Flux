package executor

import (
	"log/slog"

	"github.com/circle-oo/flux/internal/config"
)

// ExceedsGuardrails checks whether the diff exceeds configured limits.
func ExceedsGuardrails(cfg *config.ExecutorConfig, diffLines, filesChanged int) bool {
	slog.Debug("checking guardrails", "diff_lines", diffLines, "files_changed", filesChanged, "max_diff", cfg.MaxDiffLines, "max_files", cfg.MaxFilesChanged)
	if cfg.MaxDiffLines > 0 && diffLines > cfg.MaxDiffLines {
		return true
	}
	if cfg.MaxFilesChanged > 0 && filesChanged > cfg.MaxFilesChanged {
		return true
	}
	return false
}
