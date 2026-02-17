package executor

import (
	"log/slog"
	"strings"

	"github.com/circle-oo/flux/internal/ccusage"
	"github.com/circle-oo/flux/internal/models"
)

// EncodeCCProjectName encodes an absolute path into a ccusage project name.
// Replaces '/' and '.' with '-'.
func EncodeCCProjectName(absolutePath string) string {
	encoded := strings.ReplaceAll(absolutePath, "/", "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	return encoded
}

// CollectTaskUsage queries ccusage for the task's token and cost data.
// Prefers session-based lookup (exact match) when sessionID is available,
// falls back to project-based lookup (worktree directory).
// ccusage calculates cost from model pricing regardless of billing plan.
func CollectTaskUsage(ccusageCmd, worktreePath, sessionID string, task *models.Task) error {
	// Prefer session-based lookup: exact per-session usage
	if sessionID != "" {
		totals := ccusage.QuerySession(ccusageCmd, sessionID)
		if totals != nil && totals.TotalTokens > 0 {
			task.TokensUsed = totals.TotalTokens
			task.CostUSD = totals.TotalCost
			slog.Info("collected task usage from ccusage (session)",
				"task_id", task.ID,
				"session_id", sessionID,
				"tokens", task.TokensUsed,
				"cost_usd", task.CostUSD)
			return nil
		}
	}

	// Fallback: project-based lookup
	projectName := EncodeCCProjectName(worktreePath)
	totals := ccusage.QueryProjectUsage(ccusageCmd, projectName)
	if totals == nil {
		return nil
	}

	task.TokensUsed = totals.TotalTokens
	task.CostUSD = totals.TotalCost

	if totals.TotalTokens > 0 {
		slog.Info("collected task usage from ccusage (project)",
			"task_id", task.ID,
			"project", projectName,
			"tokens", task.TokensUsed,
			"cost_usd", task.CostUSD)
	}

	return nil
}
