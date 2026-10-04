//go:build !server

package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/TaruDesigns/eirin/internal/snapshot"
	"github.com/TaruDesigns/eirin/internal/store"
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
	a.startAutoBackup()
	a.snapshots.publisher = snapshot.NewPublisher(a.store)
	a.snapshots.publisher.Start()
	return nil
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
