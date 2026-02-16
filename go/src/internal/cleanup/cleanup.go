package cleanup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/circle-oo/flux/internal/config"
	"github.com/circle-oo/flux/internal/notifier"
)

// Disk space thresholds in bytes.
const (
	ThresholdWarning  = 10 * 1024 * 1024 * 1024 // 10 GB
	ThresholdBlock    = 5 * 1024 * 1024 * 1024   // 5 GB
	ThresholdCritical = 2 * 1024 * 1024 * 1024   // 2 GB
	ThresholdForce    = 1 * 1024 * 1024 * 1024   // 1 GB
)

// DiskStatus represents the current disk space condition.
type DiskStatus struct {
	AvailableBytes uint64 `json:"available_bytes"`
	TotalBytes     uint64 `json:"total_bytes"`
	UsedBytes      uint64 `json:"used_bytes"`
	Level          string `json:"level"` // "ok", "warning", "block", "critical", "force"
}

// Cleaner implements the SubComponent interface for periodic data cleanup and backups.
type Cleaner struct {
	db              *sql.DB
	config          *config.CleanupConfig
	dbConfig        *config.DatabaseConfig
	vaultPath       string
	discord         *notifier.Discord
	lastCleanupDate string // YYYY-MM-DD to track once-per-day
}

// NewCleaner creates a new Cleaner.
func NewCleaner(
	db *sql.DB,
	cfg *config.CleanupConfig,
	dbCfg *config.DatabaseConfig,
	vaultPath string,
	discord *notifier.Discord,
) *Cleaner {
	return &Cleaner{
		db:        db,
		config:    cfg,
		dbConfig:  dbCfg,
		vaultPath: vaultPath,
		discord:   discord,
	}
}

func (c *Cleaner) Name() string { return "cleanup" }

// Tick runs cleanup once per day and checks disk space on every tick.
func (c *Cleaner) Tick(ctx context.Context) error {
	// Always check disk space
	diskStatus := c.CheckDiskSpace()
	if diskStatus.Level != "ok" {
		c.handleDiskAlert(diskStatus)
	}

	// Run daily cleanup only once per calendar day
	today := time.Now().Format("2006-01-02")
	if c.lastCleanupDate == today {
		return nil
	}

	slog.Info("running daily cleanup")

	if err := c.cleanServiceMetrics(ctx); err != nil {
		slog.Error("cleanup: service_metrics", "error", err)
	}
	if err := c.cleanInsightsSnapshots(ctx); err != nil {
		slog.Error("cleanup: insights_snapshots", "error", err)
	}
	if err := c.cleanUsageSnapshots(ctx); err != nil {
		slog.Error("cleanup: usage_snapshots", "error", err)
	}
	if err := c.cleanJSONLFiles(); err != nil {
		slog.Error("cleanup: jsonl files", "error", err)
	}

	// Run backup
	if err := c.RunBackup(); err != nil {
		slog.Error("cleanup: backup failed", "error", err)
		if c.discord != nil {
			c.discord.Send(notifier.LevelWarning, fmt.Sprintf("Daily backup failed: %v", err))
		}
	}

	c.lastCleanupDate = today
	slog.Info("daily cleanup completed")

	if c.discord != nil {
		c.discord.Send(notifier.LevelInfo, "Daily cleanup and backup completed")
	}

	return nil
}

func (c *Cleaner) cleanServiceMetrics(ctx context.Context) error {
	days := c.config.ServiceMetricsRawDays
	if days <= 0 {
		days = 7
	}

	result, err := c.db.ExecContext(ctx,
		`DELETE FROM service_metrics WHERE recorded_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return fmt.Errorf("delete service_metrics: %w", err)
	}

	if n, _ := result.RowsAffected(); n > 0 {
		slog.Info("cleaned service_metrics", "rows", n, "older_than_days", days)
	}
	return nil
}

func (c *Cleaner) cleanInsightsSnapshots(ctx context.Context) error {
	days := c.config.UsageSnapshotsDays
	if days <= 0 {
		days = 90
	}

	result, err := c.db.ExecContext(ctx,
		`DELETE FROM insights_snapshots WHERE recorded_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return fmt.Errorf("delete insights_snapshots: %w", err)
	}

	if n, _ := result.RowsAffected(); n > 0 {
		slog.Info("cleaned insights_snapshots", "rows", n, "older_than_days", days)
	}
	return nil
}

func (c *Cleaner) cleanUsageSnapshots(ctx context.Context) error {
	days := c.config.UsageSnapshotsDays
	if days <= 0 {
		days = 90
	}

	result, err := c.db.ExecContext(ctx,
		`DELETE FROM usage_snapshots WHERE recorded_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", days),
	)
	if err != nil {
		return fmt.Errorf("delete usage_snapshots: %w", err)
	}

	if n, _ := result.RowsAffected(); n > 0 {
		slog.Info("cleaned usage_snapshots", "rows", n, "older_than_days", days)
	}
	return nil
}

func (c *Cleaner) cleanJSONLFiles() error {
	days := c.config.JSONLRetentionDays
	if days <= 0 {
		days = 7
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	// Walk the workspace looking for .jsonl files
	workspaceBase := filepath.Dir(c.dbConfig.Path)
	if workspaceBase == "" || workspaceBase == "." {
		return nil
	}

	var cleaned int
	err := filepath.Walk(workspaceBase, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				slog.Warn("failed to remove old jsonl file", "path", path, "error", err)
				return nil
			}
			cleaned++
		}
		return nil
	})

	if cleaned > 0 {
		slog.Info("cleaned old jsonl files", "count", cleaned, "older_than_days", days)
	}
	return err
}

// CheckDiskSpace returns the current disk status.
func (c *Cleaner) CheckDiskSpace() DiskStatus {
	var stat syscall.Statfs_t
	path := c.dbConfig.Path
	if path == "" {
		path = "/"
	}

	if err := syscall.Statfs(filepath.Dir(path), &stat); err != nil {
		slog.Error("failed to check disk space", "error", err)
		return DiskStatus{Level: "ok"}
	}

	available := stat.Bavail * uint64(stat.Bsize)
	total := stat.Blocks * uint64(stat.Bsize)
	used := total - available

	level := "ok"
	switch {
	case available < ThresholdForce:
		level = "force"
	case available < ThresholdCritical:
		level = "critical"
	case available < ThresholdBlock:
		level = "block"
	case available < ThresholdWarning:
		level = "warning"
	}

	return DiskStatus{
		AvailableBytes: available,
		TotalBytes:     total,
		UsedBytes:      used,
		Level:          level,
	}
}

func (c *Cleaner) handleDiskAlert(status DiskStatus) {
	availGB := float64(status.AvailableBytes) / (1024 * 1024 * 1024)

	switch status.Level {
	case "warning":
		slog.Warn("disk space warning", "available_gb", fmt.Sprintf("%.1f", availGB))
		if c.discord != nil {
			c.discord.Send(notifier.LevelWarning, fmt.Sprintf("Low disk space: %.1f GB available", availGB))
		}
	case "block":
		slog.Warn("disk space low - blocking new tasks", "available_gb", fmt.Sprintf("%.1f", availGB))
		if c.discord != nil {
			c.discord.Send(notifier.LevelWarning, fmt.Sprintf("Disk space critical: %.1f GB - blocking new tasks", availGB))
		}
	case "critical":
		slog.Error("disk space critical - emergency cleanup", "available_gb", fmt.Sprintf("%.1f", availGB))
		if c.discord != nil {
			c.discord.Send(notifier.LevelCritical, fmt.Sprintf("CRITICAL disk space: %.1f GB - running emergency cleanup", availGB))
		}
		c.emergencyCleanup()
	case "force":
		slog.Error("disk space exhausted - refusing new work", "available_gb", fmt.Sprintf("%.1f", availGB))
		if c.discord != nil {
			c.discord.Send(notifier.LevelCritical, fmt.Sprintf("EMERGENCY: disk space exhausted (%.1f GB) - all new work blocked", availGB))
		}
		c.emergencyCleanup()
	}
}

// emergencyCleanup deletes old backups to reclaim disk space.
func (c *Cleaner) emergencyCleanup() {
	backupDir := c.dbConfig.BackupDir
	if backupDir == "" {
		return
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		slog.Error("emergency cleanup: cannot read backup dir", "error", err)
		return
	}

	// Delete all but the most recent backup
	var backups []os.DirEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "flux-backup-") && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e)
		}
	}

	if len(backups) <= 1 {
		return
	}

	for _, b := range backups[:len(backups)-1] {
		path := filepath.Join(backupDir, b.Name())
		if err := os.Remove(path); err != nil {
			slog.Error("emergency cleanup: failed to remove backup", "path", path, "error", err)
		} else {
			slog.Info("emergency cleanup: removed old backup", "path", path)
		}
	}
}
