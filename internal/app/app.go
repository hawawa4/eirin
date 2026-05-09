package app

import (
	"context"

	"github.com/TaruDesigns/eirin/internal/browser"
	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx   context.Context
	prefs *prefs.Store
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
}

func (a *App) Shutdown(_ context.Context) {
	if a.prefs != nil {
		a.prefs.Close()
	}
}

// ── Preferences ───────────────────────────────────────────────────────────────

// LoadPrefs returns all persisted preferences, with defaults for missing keys.
func (a *App) LoadPrefs() prefs.Prefs {
	if a.prefs == nil {
		return prefs.DefaultPrefs()
	}
	return a.prefs.Load()
}

// SetPref persists a single preference by key. Both key and value are strings;
// booleans are "true"/"false" and integers are their decimal string form.
func (a *App) SetPref(key, value string) {
	if a.prefs == nil {
		return
	}
	if err := a.prefs.Set(key, value); err != nil {
		runtime.LogErrorf(a.ctx, "prefs: set %q=%q: %v", key, value, err)
	}
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
// stretchLevel: 0=linear, 1=gentle, 2=normal, 3=strong
func (a *App) GeneratePreview(path string, stretchLevel int) (string, error) {
	return fits.GeneratePreview(path, 1024, stretchLevel)
}
