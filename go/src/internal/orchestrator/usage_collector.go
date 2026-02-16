package orchestrator

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/circle-oo/flux/internal/models"
)

// UsageCollector implements SubComponent and collects hourly ccusage snapshots.
type UsageCollector struct {
	db             *sql.DB
	ccusageCmd     string
	lastCollection time.Time
}

// NewUsageCollector creates a new UsageCollector.
func NewUsageCollector(db *sql.DB, ccusageCmd string) *UsageCollector {
	return &UsageCollector{
		db:         db,
		ccusageCmd: ccusageCmd,
	}
}

// Name implements SubComponent.
func (u *UsageCollector) Name() string {
	return "usage_collector"
}

// Tick implements SubComponent. It checks if an hour has passed since last
// collection and, if so, runs the ccusage CLI and stores the result.
func (u *UsageCollector) Tick(_ context.Context) error {
	if u.ccusageCmd == "" {
		return nil
	}

	if !u.lastCollection.IsZero() && time.Since(u.lastCollection) < time.Hour {
		return nil
	}

	output, err := u.runCCUsage()
	if err != nil {
		slog.Warn("usage_collector: failed to run ccusage", "error", err)
		return nil // fire-and-forget
	}

	if output == "" {
		return nil
	}

	store := models.NewUsageStore(u.db)
	snapshot := &models.UsageSnapshot{
		Type: models.UsageTypeHourly,
		Data: output,
	}

	if err := store.CreateSnapshot(snapshot); err != nil {
		slog.Warn("usage_collector: failed to store snapshot", "error", err)
		return nil // fire-and-forget
	}

	u.lastCollection = time.Now()
	slog.Info("usage_collector: stored hourly snapshot", "bytes", len(output))
	return nil
}

func (u *UsageCollector) runCCUsage() (string, error) {
	parts := strings.Fields(u.ccusageCmd)
	if len(parts) == 0 {
		return "", nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout.String()), nil
}
