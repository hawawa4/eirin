//go:build server

package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/snapshot"
	"github.com/TaruDesigns/eirin/internal/store"
)

// EnvRoot is the environment variable holding the library root folder on the
// server (where the NAS is mounted). Required in server builds.
const EnvRoot = "EIRIN_ROOT"

// snapshotSide keeps the viewer database in sync with the published snapshot.
type snapshotSide struct {
	loader *snapshot.Loader
	cancel context.CancelFunc
}

// startup opens the local viewer database and starts watching for snapshots.
func (a *App) startup() error {
	root := os.Getenv(EnvRoot)
	if root == "" {
		err := fmt.Errorf("%s must be set to the library root folder", EnvRoot)
		slog.Error("server: missing configuration", "err", err)
		return err
	}
	local := viewerDBPath()
	l := snapshot.NewLoader(root, local, func(st *store.Store) *store.Store {
		old := a.db.Swap(st)
		a.emitEvent("library:updated", nil) // open viewers reload
		return old
	})
	st, err := l.Open()
	if err != nil {
		slog.Error("server: failed to open the viewer database", "path", local, "err", err)
		return err
	}
	a.db.Store(st)

	ctx, cancel := context.WithCancel(context.Background())
	a.snapshots = snapshotSide{loader: l, cancel: cancel}
	go l.Run(ctx)
	slog.Info("server: read-only viewer", "root", root, "snapshot", snapshot.Path(root), "db", local)
	return nil
}

func (a *App) shutdown() {
	if a.snapshots.cancel != nil {
		a.snapshots.cancel()
	}
}

// viewerDBPath is where the server keeps its local copy of the snapshot:
// EIRIN_DB_PATH if set, otherwise the user cache directory.
func viewerDBPath() string {
	if p := os.Getenv(store.EnvDBPath); p != "" {
		return p
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "eirin", "library.db")
}
