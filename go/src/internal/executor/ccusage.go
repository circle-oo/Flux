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

// CCUsageResponse represents the JSON response from ccusage daily --json.
type CCUsageResponse struct {
	TotalTokens int     `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost"`
}

// CollectTaskUsage queries ccusage for the task's token and cost data.
// Updates task.TokensUsed and task.CostUSD.
// Graceful degradation: logs errors but does not fail the task.
func CollectTaskUsage(ccusageCmd, worktreePath string, task *models.Task) error {
	projectName := EncodeCCProjectName(worktreePath)

	// Run: ccusage daily --project {projectName} --json
	cmd := exec.Command(ccusageCmd, "daily", "--project", projectName, "--json")
	output, err := cmd.Output()
	if err != nil {
		slog.Warn("ccusage command failed, skipping usage collection", "error", err, "project", projectName)
		return nil // Graceful degradation
	}

	var response CCUsageResponse
	if err := json.Unmarshal(output, &response); err != nil {
		slog.Warn("failed to parse ccusage JSON response", "error", err, "output", string(output))
		return nil // Graceful degradation
	}

	task.TokensUsed = response.TotalTokens
	task.CostUSD = response.TotalCost

	slog.Info("collected task usage from ccusage",
		"task_id", task.ID,
		"tokens", task.TokensUsed,
		"cost_usd", task.CostUSD)

	return nil
}
