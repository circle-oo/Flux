package executor

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/vault"
)

// RecordTaskCompletion records a completed task to the vault.
func RecordTaskCompletion(vaultWriter *vault.Writer, task *models.Task, result *ClaudeCodeResult) error {
	slog.Debug("building task summary for vault", "task_id", task.ID)
	data := buildTaskSummaryData(task, result)
	markdown := vault.TaskSummaryTemplate(data)

	// Write to Tasks/completed/{task-id-prefix}.md
	path := fmt.Sprintf("Tasks/completed/%s.md", task.ID[:8])
	if err := vaultWriter.Write(path, markdown, vault.ModeCreate); err != nil {
		slog.Warn("failed to record task completion to vault", "task_id", task.ID, "error", err)
		return fmt.Errorf("vault write failed: %w", err)
	}

	slog.Info("recorded task completion to vault", "task_id", task.ID, "path", path)
	return nil
}

// buildTaskSummaryData maps Task and ClaudeCodeResult to TaskSummaryData.
func buildTaskSummaryData(task *models.Task, result *ClaudeCodeResult) vault.TaskSummaryData {
	duration := "unknown"
	if result != nil && result.Duration > 0 {
		duration = result.Duration.Round(time.Second).String()
	}

	prStatus := task.PRStatus
	if prStatus == "" {
		prStatus = "not created"
	}

	prURL := task.PRUrl
	if prURL == "" {
		prURL = "N/A"
	}

	resultText := task.Result
	if resultText == "" && result != nil {
		resultText = result.Stdout
	}
	if resultText == "" {
		resultText = "No result recorded"
	}

	return vault.TaskSummaryData{
		ID:           task.ID,
		Title:        task.Title,
		Type:         task.Type,
		Status:       task.Status,
		Priority:     task.Priority,
		Model:        task.Model,
		Duration:     duration,
		CreatedAt:    task.CreatedAt,
		CompletedAt:  task.CompletedAt,
		PRUrl:        prURL,
		PRStatus:     prStatus,
		DiffLines:    task.DiffLines,
		FilesChanged: task.FilesChanged,
		Result:       resultText,
	}
}
