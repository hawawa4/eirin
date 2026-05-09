package app

import (
	"context"

	"github.com/TaruDesigns/eirin/internal/browser"
	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// ── File browser ──────────────────────────────────────────────────────────────

func (a *App) SelectRootFolder() string {
	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Astrophotography Root Folder",
	})
	if err != nil {
		return ""
	}
	return path
}

func (a *App) ListDirectory(path string) ([]browser.FileEntry, error) {
	return browser.ListDirectory(path)
}

// ── FITS ──────────────────────────────────────────────────────────────────────

func (a *App) ReadFITSHeader(path string) (*fits.FITSHeader, error) {
	return fits.ReadHeader(path)
}

// GeneratePreview returns a PNG preview as a base64 data URL, scaled to 1024 px.
// stretchLevel: 0=none, 1=gentle, 2=normal, 3=strong
func (a *App) GeneratePreview(path string, stretchLevel int) (string, error) {
	return fits.GeneratePreview(path, 1024, stretchLevel)
}
