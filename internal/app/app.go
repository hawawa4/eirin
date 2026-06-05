package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/TaruDesigns/eirin/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type App struct {
	wails   *application.App
	store   *store.Store
	indexer appIndexer
	server  *http.Server
}

func NewApp() *App {
	return &App{}
}

func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.wails = application.Get()
	s, err := store.NewStore()
	if err != nil {
		slog.Error("store: failed to open", "err", err)
		return err
	}
	a.store = s
	a.startServer()
	a.startAutoBackup()
	return nil
}

func (a *App) ServiceShutdown() error {
	a.CancelIndex()
	a.stopServer()
	if a.store != nil {
		_ = a.store.Close()
	}
	return nil
}
