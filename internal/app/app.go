package app

import (
	"context"
	"strings"

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
		_ = a.prefs.Close()
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

// ListDirectoryEnriched returns directory entries enriched with cached FITS
// header metadata. Uncached FITS files are read and added to the cache on the
// spot; subsequent calls for the same directory return instantly from cache.
func (a *App) ListDirectoryEnriched(path string) ([]browser.EnrichedFileEntry, error) {
	entries, err := browser.ListDirectory(path)
	if err != nil {
		return nil, err
	}

	if a.prefs == nil {
		result := make([]browser.EnrichedFileEntry, len(entries))
		for i, e := range entries {
			result[i] = browser.EnrichedFileEntry{FileEntry: e}
		}
		return result, nil
	}

	// Collect paths of FITS files in this directory.
	fitsPaths := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir && isFitsFile(e.Name) {
			fitsPaths = append(fitsPaths, e.Path)
		}
	}

	// Fetch whatever is already cached.
	cached, err := a.prefs.GetFITSCache(fitsPaths)
	if err != nil {
		runtime.LogErrorf(a.ctx, "fits cache: get: %v", err)
		cached = map[string]prefs.CachedFITSHeader{}
	}

	// Read and cache any FITS files not yet in the cache.
	for _, p := range fitsPaths {
		if _, ok := cached[p]; ok {
			continue
		}
		hdr, err := fits.ReadHeader(p)
		if err != nil {
			runtime.LogErrorf(a.ctx, "fits: read header %q: %v", p, err)
			continue
		}
		ch := prefs.CachedFITSHeader{
			Object:     hdr.Object,
			Filter:     hdr.Filter,
			ExpTime:    hdr.ExpTime,
			DateObs:    hdr.DateObs,
			Gain:       hdr.Gain,
			CCDTemp:    hdr.CCDTemp,
			Telescope:  hdr.Telescope,
			Instrument: hdr.Instrument,
		}
		if err := a.prefs.UpsertFITSCache(p, ch); err != nil {
			runtime.LogErrorf(a.ctx, "fits cache: upsert %q: %v", p, err)
		}
		cached[p] = ch
	}

	// Assemble enriched entries.
	result := make([]browser.EnrichedFileEntry, len(entries))
	for i, e := range entries {
		ee := browser.EnrichedFileEntry{FileEntry: e}
		if ch, ok := cached[e.Path]; ok {
			ee.Object     = ch.Object
			ee.Filter     = ch.Filter
			ee.ExpTime    = ch.ExpTime
			ee.DateObs    = ch.DateObs
			ee.Gain       = ch.Gain
			ee.CCDTemp    = ch.CCDTemp
			ee.Telescope  = ch.Telescope
			ee.Instrument = ch.Instrument
			ee.HasMeta    = true
		}
		result[i] = ee
	}
	return result, nil
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

func isFitsFile(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".fits") || strings.HasSuffix(l, ".fit")
}
