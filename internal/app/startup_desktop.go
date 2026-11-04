//go:build !server

package app

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/hawawa4/eirin/internal/snapshot"
	"github.com/hawawa4/eirin/internal/store"
)

// snapshotSide publishes the library snapshot for the server viewer.
type snapshotSide struct {
	publisher *snapshot.Publisher
}

// startup opens the user's database and starts the background jobs.
func (a *App) startup() error {
	s, err := store.NewStore()
	if err != nil {
		slog.Error("store: failed to open", "err", err)
		return err
	}
	a.db.Store(s)
	a.startPreviews(desktopCacheDir())
	a.startAutoBackup()
	a.snapshots.publisher = snapshot.NewPublisher(a.store)
	a.snapshots.publisher.Start()
	return nil
}

// desktopCacheDir is where rendered previews are kept: the user cache
// directory (~/.cache/eirin/previews on Linux).
func desktopCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "eirin", "previews")
}

// shutdown publishes pending snapshot changes, giving a slow NAS a few seconds.
func (a *App) shutdown() {
	if a.snapshots.publisher == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	a.snapshots.publisher.Stop(ctx)
}
