//go:build server

package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/hawawa4/eirin/internal/snapshot"
	"github.com/hawawa4/eirin/internal/store"
)

// EnvRoot is the environment variable holding the library root folder on the
// server (where the NAS is mounted). Required in server builds.
const EnvRoot = "EIRIN_ROOT"

// snapshotSide keeps the viewer database in sync with the published snapshot.
type snapshotSide struct {
	loader *snapshot.Loader
	cancel context.CancelFunc
	// stopWarm cancels the preview warm-up of the previous snapshot.
	warmMu   sync.Mutex
	stopWarm context.CancelFunc
	// gallery is what visitors see, built from the loaded snapshot.
	gallery gallerySide
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
	if err := a.loadGalleryConfig(local); err != nil {
		slog.Error("server: bad gallery configuration", "err", err)
		return err
	}
	l := snapshot.NewLoader(root, local, func(st *store.Store) *store.Store {
		old := a.db.Swap(st)
		a.refreshGallery(st)
		a.emitEvent("library:updated", nil) // open viewers reload
		a.warmPreviews()
		return old
	})
	st, err := l.Open()
	if err != nil {
		slog.Error("server: failed to open the viewer database", "path", local, "err", err)
		return err
	}
	a.db.Store(st)
	a.refreshGallery(st)
	a.startPreviews(filepath.Join(filepath.Dir(local), "previews"))
	a.warmPreviews()

	ctx, cancel := context.WithCancel(context.Background())
	a.snapshots.loader, a.snapshots.cancel = l, cancel
	go l.Run(ctx)
	slog.Info("server: read-only viewer", "root", root, "snapshot", snapshot.Path(root), "db", local)
	return nil
}

func (a *App) shutdown() {
	if a.snapshots.cancel != nil {
		a.snapshots.cancel()
	}
	a.snapshots.warmMu.Lock()
	if a.snapshots.stopWarm != nil {
		a.snapshots.stopWarm()
	}
	a.snapshots.warmMu.Unlock()
}

// warmPreviews renders, in the background, every level of the gallery's
// pictures that isn't cached yet, so visitors rarely wait for a render. Set
// EIRIN_PREVIEW_WARM=0 to render only on demand.
func (a *App) warmPreviews() {
	svc, st := a.images.Load(), a.snapshots.gallery.state.Load()
	if svc == nil || st == nil || os.Getenv("EIRIN_PREVIEW_WARM") == "0" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.snapshots.warmMu.Lock()
	if a.snapshots.stopWarm != nil {
		a.snapshots.stopWarm()
	}
	a.snapshots.stopWarm = cancel
	a.snapshots.warmMu.Unlock()
	go svc.Warm(ctx, st.warm)
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
