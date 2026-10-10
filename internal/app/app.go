package app

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	wails *application.App
	// db holds the current store. The server viewer swaps it whenever it loads
	// a new library snapshot, so always read it through store().
	db      atomic.Pointer[store.Store]
	indexer appIndexer
	server  *http.Server
	imports importJob
	// previews caches Blink previews across calls (see fits.PreviewCache).
	previews fits.PreviewCache
	// fields caches each scope's sensor size for the Coverage view.
	fields fieldCache
	// snapshots is the build-specific half of the library snapshot: the
	// publisher on the desktop, the loader on the server.
	snapshots snapshotSide
}

func NewApp() *App {
	return &App{}
}

// store returns the current store, or nil before startup.
func (a *App) store() *store.Store {
	return a.db.Load()
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.wails = application.Get()
	if err := a.startup(); err != nil {
		return err
	}
	a.startServer()
	return nil
}

func (a *App) ServiceShutdown() error {
	a.CancelIndex()
	a.stopServer()
	a.shutdown()
	if st := a.store(); st != nil {
		_ = st.Close()
	}
	return nil
}

// emitEvent emits a Wails custom event. It is a no-op when the Wails app is
// not wired up (unit tests construct App without it).
func (a *App) emitEvent(name string, data any) {
	if a.wails == nil {
		return
	}
	a.wails.Event.EmitEvent(&application.CustomEvent{Name: name, Data: data})
}
