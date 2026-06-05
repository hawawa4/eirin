package app

import (
	"context"
	"net/http"

	"github.com/TaruDesigns/eirin/internal/store"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	store   *store.Store
	indexer appIndexer
	server  *http.Server
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	s, err := store.NewStore()
	if err != nil {
		runtime.LogErrorf(ctx, "store: failed to open: %v", err)
		return
	}
	a.store = s
	a.startServer()
	a.startAutoBackup()
}

func (a *App) Shutdown(_ context.Context) {
	a.CancelIndex()
	a.stopServer()
	if a.store != nil {
		_ = a.store.Close()
	}
}
