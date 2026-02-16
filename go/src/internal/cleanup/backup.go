package cleanup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunBackup performs a SQLite backup using VACUUM INTO and optionally backs up the vault.
func (c *Cleaner) RunBackup() error {
	backupDir := c.dbConfig.BackupDir
	if backupDir == "" {
		slog.Debug("backup: no backup directory configured, skipping")
		return nil
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	// Database backup via VACUUM INTO
	date := time.Now().Format("2006-01-02")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("flux-backup-%s.db", date))

	// Remove existing backup for today if it exists (idempotent)
	os.Remove(backupPath)

	slog.Info("running database backup", "dest", backupPath)
	_, err := c.db.Exec(fmt.Sprintf(`VACUUM INTO '%s'`, backupPath))
	if err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}
	slog.Info("database backup completed", "path", backupPath)

	// Vault backup (tar.gz)
	if c.vaultPath != "" {
		if _, err := os.Stat(c.vaultPath); err == nil {
			vaultBackupPath := filepath.Join(backupDir, fmt.Sprintf("vault-backup-%s.tar.gz", date))
			if err := createTarGz(c.vaultPath, vaultBackupPath); err != nil {
				slog.Error("vault backup failed", "error", err)
			} else {
				slog.Info("vault backup completed", "path", vaultBackupPath)
			}
		}
	}

	// Cleanup old backups
	c.cleanOldBackups()

	return nil
}

// cleanOldBackups removes backups older than the retention period.
func (c *Cleaner) cleanOldBackups() {
	retentionDays := c.dbConfig.BackupRetentionDays
	if retentionDays <= 0 {
		retentionDays = 7
	}

	backupDir := c.dbConfig.BackupDir
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		slog.Error("failed to read backup dir for cleanup", "error", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	var removed int

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name()
		isBackup := (strings.HasPrefix(name, "flux-backup-") && strings.HasSuffix(name, ".db")) ||
			(strings.HasPrefix(name, "vault-backup-") && strings.HasSuffix(name, ".tar.gz"))

		if !isBackup {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(backupDir, name)
			if err := os.Remove(path); err != nil {
				slog.Error("failed to remove old backup", "path", path, "error", err)
			} else {
				removed++
			}
		}
	}

	if removed > 0 {
		slog.Info("removed old backups", "count", removed, "retention_days", retentionDays)
	}
}

// createTarGz creates a tar.gz archive of the source directory.
func createTarGz(sourceDir, destPath string) error {
	outFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create tar.gz file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("file info header: %w", err)
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("relative path: %w", err)
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("write header: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})
}
