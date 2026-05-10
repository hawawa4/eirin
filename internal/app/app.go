package app

import (
	"context"
	"net/http"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	prefs   *prefs.Store
	indexer indexer
	server  *http.Server
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	store, err := prefs.NewStore()
	if err != nil {
		runtime.LogErrorf(ctx, "prefs: failed to open store: %v", err)
		return
	}
	a.prefs = store
	a.startServer()
}

func (a *App) Shutdown(_ context.Context) {
	a.CancelIndex()
	a.stopServer()
	if a.prefs != nil {
		_ = a.prefs.Close()
	}
}
