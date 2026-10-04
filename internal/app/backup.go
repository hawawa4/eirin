package app

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"
)

// BackupDatabase writes a consistent copy of the SQLite database (including
// changes still in the WAL) to the same directory with a timestamp suffix,
// e.g. prefs_2026-06-05T153000.db. Returns the backup path.
func (a *App) BackupDatabase() (string, error) {
	if a.store == nil {
		return "", fmt.Errorf("store not initialised")
	}
	src := a.store.DBPath()
	ext := filepath.Ext(src)
	base := src[:len(src)-len(ext)]
	stamp := time.Now().Format("2006-01-02T150405")
	dst := fmt.Sprintf("%s_%s%s", base, stamp, ext)

	if err := a.store.VacuumInto(dst); err != nil {
		return "", fmt.Errorf("backup: %w", err)
	}
	return dst, nil
}

// startAutoBackup runs a background goroutine that backs up the database every
// 30 minutes for as long as the app is running. Errors are logged but do not
// stop the loop.
func (a *App) startAutoBackup() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := a.BackupDatabase(); err != nil {
				slog.Warn("auto-backup", "err", err)
			} else {
				slog.Info("auto-backup: database backed up")
			}
		}
	}()
}
