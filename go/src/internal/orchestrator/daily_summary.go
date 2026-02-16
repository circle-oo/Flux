package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/circle-oo/flux/internal/models"
	"github.com/circle-oo/flux/internal/notifier"
)

// DailySummary implements SubComponent and sends a daily Discord summary.
type DailySummary struct {
	db              *sql.DB
	discord         *notifier.Discord
	summaryHour     int
	lastSummaryDate string
}

// NewDailySummary creates a new DailySummary.
func NewDailySummary(db *sql.DB, discord *notifier.Discord, summaryHour int) *DailySummary {
	return &DailySummary{
		db:          db,
		discord:     discord,
		summaryHour: summaryHour,
	}
}

// Name implements SubComponent.
func (d *DailySummary) Name() string {
	return "daily_summary"
}

// Tick implements SubComponent. It checks if the summary hour has been reached
// and sends a daily summary to Discord if it hasn't been sent today.
func (d *DailySummary) Tick(_ context.Context) error {
	now := time.Now()
	today := now.Format("2006-01-02")

	// Already sent today
	if d.lastSummaryDate == today {
		return nil
	}

	// Not yet the summary hour
	if now.Hour() < d.summaryHour {
		return nil
	}

	stats, err := d.queryStats(today)
	if err != nil {
		slog.Warn("daily_summary: failed to query stats", "error", err)
		// Still send what we can
		stats = &dailyStats{}
	}

	message := fmt.Sprintf(
		"Daily Summary (%s)\n"+
			"- Completed tasks: %d\n"+
			"- Failed tasks: %d\n"+
			"- Total tokens: %d\n"+
			"- Total cost: $%.2f",
		today, stats.Completed, stats.Failed, stats.TokensUsed, stats.CostUSD,
	)

	if err := d.discord.Send(notifier.LevelInfo, message); err != nil {
		slog.Warn("daily_summary: failed to send discord message", "error", err)
		return nil
	}

	d.lastSummaryDate = today
	slog.Info("daily_summary: sent", "date", today)
	return nil
}

// dailyStats holds aggregate numbers for the daily report.
type dailyStats struct {
	Completed  int
	Failed     int
	TokensUsed int64
	CostUSD    float64
}

// queryStats retrieves aggregate task statistics for the given date.
func (d *DailySummary) queryStats(dateStr string) (*dailyStats, error) {
	var stats dailyStats

	// Use the task store for status counts
	taskStore := models.NewTaskStore(d.db)

	completed, err := taskStore.CountByStatus(models.TaskCompleted)
	if err != nil {
		return nil, err
	}
	stats.Completed = completed

	failed, err := taskStore.CountByStatus(models.TaskFailed)
	if err != nil {
		return nil, err
	}
	stats.Failed = failed

	// Token and cost totals from completed tasks today
	err = d.db.QueryRow(
		`SELECT COALESCE(SUM(tokens_used), 0), COALESCE(SUM(cost_usd), 0)
		 FROM tasks WHERE DATE(completed_at) = ?`,
		dateStr,
	).Scan(&stats.TokensUsed, &stats.CostUSD)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return &stats, nil
}
