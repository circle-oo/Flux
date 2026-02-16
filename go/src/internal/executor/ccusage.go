package executor

import (
	"encoding/json"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/circle-oo/flux/internal/models"
)

// EncodeCCProjectName encodes an absolute path into a ccusage project name.
// Replaces '/' and '.' with '-'.
// Example: /home/user/workspaces/trees/flux--task-abc123 -> -home-user-workspaces-trees-flux--task-abc123
func EncodeCCProjectName(absolutePath string) string {
	encoded := strings.ReplaceAll(absolutePath, "/", "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	return encoded
}

// ccusageDailyResponse matches the top-level JSON from `ccusage daily --json`.
type ccusageDailyResponse struct {
	Daily []ccusageDayEntry `json:"daily"`
}

// ccusageDayEntry matches a single day entry in ccusage's camelCase JSON output.
type ccusageDayEntry struct {
	TotalTokens int     `json:"totalTokens"`
	TotalCost   float64 `json:"totalCost"`
}

// CollectTaskUsage queries ccusage for the task's token and cost data.
// Updates task.TokensUsed and task.CostUSD.
// Graceful degradation: logs errors but does not fail the task.
func CollectTaskUsage(ccusageCmd, worktreePath string, task *models.Task) error {
	projectName := EncodeCCProjectName(worktreePath)

	// Split command in case ccusageCmd contains args (e.g. "npx ccusage@latest")
	parts := strings.Fields(ccusageCmd)
	if len(parts) == 0 {
		return nil
	}
	args := append(parts[1:], "daily", "--project", projectName, "--json")
	cmd := exec.Command(parts[0], args...)
	output, err := cmd.Output()
	if err != nil {
		slog.Warn("ccusage command failed, skipping usage collection", "error", err, "project", projectName)
		return nil // Graceful degradation
	}

	var response ccusageDailyResponse
	if err := json.Unmarshal(output, &response); err != nil {
		slog.Warn("failed to parse ccusage JSON response", "error", err, "output", string(output))
		return nil // Graceful degradation
	}

	// Sum across all daily entries (task may span multiple days)
	var totalTokens int
	var totalCost float64
	for _, day := range response.Daily {
		totalTokens += day.TotalTokens
		totalCost += day.TotalCost
	}

	task.TokensUsed = totalTokens
	task.CostUSD = totalCost

	if totalTokens > 0 {
		slog.Info("collected task usage from ccusage",
			"task_id", task.ID,
			"tokens", task.TokensUsed,
			"cost_usd", task.CostUSD)
	}

	return nil
}
