package app

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TaruDesigns/eirin/internal/browser"
	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// indexProgressEvent is the payload emitted on the "index:progress" Wails event.
type indexProgressEvent struct {
	Phase   string `json:"phase"`   // "scanning" | "indexing" | "done" | "cancelled"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Indexed int    `json:"indexed"` // files newly read and added to cache
	Errors  int    `json:"errors"`
	Current string `json:"current"` // short display name of file being processed
}

type App struct {
	ctx         context.Context
	prefs       *prefs.Store
	indexMu     sync.Mutex
	indexCancel context.CancelFunc
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
	a.CancelIndex()
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
// header metadata. It is cache-only and returns immediately — files not yet in
// the cache will have HasMeta=false. Run BuildIndex to populate the cache.
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

	// Collect FITS file paths in this directory.
	fitsPaths := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir && isFitsFile(e.Name) {
			fitsPaths = append(fitsPaths, e.Path)
		}
	}

	// Fetch whatever is already cached (no header reading here).
	cached, err := a.prefs.GetFITSCache(fitsPaths)
	if err != nil {
		runtime.LogErrorf(a.ctx, "fits cache: get: %v", err)
		cached = map[string]prefs.CachedFITSHeader{}
	}

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

// ── Index builder ─────────────────────────────────────────────────────────────

// BuildIndex traverses every subdirectory under rootPath, reads FITS headers
// for files not yet in the cache, and stores them. Progress is reported via
// "index:progress" Wails events. Calling BuildIndex again only processes
// files added since the last run.
func (a *App) BuildIndex(rootPath string) {
	a.indexMu.Lock()
	if a.indexCancel != nil {
		a.indexCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.indexCancel = cancel
	a.indexMu.Unlock()

	go func() {
		defer func() {
			a.indexMu.Lock()
			a.indexCancel = nil
			a.indexMu.Unlock()
		}()

		a.emit(indexProgressEvent{Phase: "scanning", Current: "Scanning directories…"})

		// ── Phase 1: collect all FITS paths under rootPath ──────────────────
		var fitsPaths []string
		_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || ctx.Err() != nil {
				return nil
			}
			if !d.IsDir() && isFitsFile(d.Name()) {
				fitsPaths = append(fitsPaths, path)
			}
			return nil
		})

		if ctx.Err() != nil {
			a.emit(indexProgressEvent{Phase: "cancelled", Total: len(fitsPaths)})
			return
		}

		total := len(fitsPaths)
		a.emit(indexProgressEvent{Phase: "indexing", Total: total, Current: "Checking cache…"})

		if total == 0 {
			a.emit(indexProgressEvent{Phase: "done"})
			return
		}

		// ── Phase 2: find which paths are NOT yet cached ────────────────────
		cached, err := a.prefs.GetFITSCache(fitsPaths)
		if err != nil {
			runtime.LogErrorf(a.ctx, "index: get cache: %v", err)
			cached = map[string]prefs.CachedFITSHeader{}
		}

		toIndex := make([]string, 0, len(fitsPaths)-len(cached))
		for _, p := range fitsPaths {
			if _, ok := cached[p]; !ok {
				toIndex = append(toIndex, p)
			}
		}

		// Emit once so the UI shows "X already indexed, Y to process".
		alreadyCached := total - len(toIndex)
		a.emit(indexProgressEvent{
			Phase:   "indexing",
			Total:   total,
			Done:    alreadyCached,
			Indexed: 0,
			Current: "Reading headers…",
		})

		// ── Phase 3: read headers and batch-write to cache ──────────────────
		const batchSize = 100
		batch := make(map[string]prefs.CachedFITSHeader, batchSize)
		done := alreadyCached
		indexed := 0
		errs := 0
		lastEmit := time.Now()

		flushBatch := func() {
			if len(batch) == 0 {
				return
			}
			if err := a.prefs.BatchUpsertFITSCache(batch); err != nil {
				runtime.LogErrorf(a.ctx, "index: batch upsert: %v", err)
			}
			batch = make(map[string]prefs.CachedFITSHeader, batchSize)
		}

		for _, p := range toIndex {
			if ctx.Err() != nil {
				break
			}

			hdr, err := fits.ReadHeader(p)
			if err != nil {
				errs++
			} else {
				batch[p] = prefs.CachedFITSHeader{
					Object:     hdr.Object,
					Filter:     hdr.Filter,
					ExpTime:    hdr.ExpTime,
					DateObs:    hdr.DateObs,
					Gain:       hdr.Gain,
					CCDTemp:    hdr.CCDTemp,
					Telescope:  hdr.Telescope,
					Instrument: hdr.Instrument,
				}
				indexed++
			}
			done++

			if len(batch) >= batchSize {
				flushBatch()
			}

			if time.Since(lastEmit) >= 80*time.Millisecond {
				a.emit(indexProgressEvent{
					Phase:   "indexing",
					Total:   total,
					Done:    done,
					Indexed: indexed,
					Errors:  errs,
					Current: filepath.Base(p),
				})
				lastEmit = time.Now()
			}
		}

		flushBatch()

		phase := "done"
		if ctx.Err() != nil {
			phase = "cancelled"
		}
		a.emit(indexProgressEvent{
			Phase:   phase,
			Total:   total,
			Done:    done,
			Indexed: indexed,
			Errors:  errs,
		})
	}()
}

// CancelIndex stops any running BuildIndex operation.
func (a *App) CancelIndex() {
	a.indexMu.Lock()
	defer a.indexMu.Unlock()
	if a.indexCancel != nil {
		a.indexCancel()
		a.indexCancel = nil
	}
}

func (a *App) emit(evt indexProgressEvent) {
	runtime.EventsEmit(a.ctx, "index:progress", evt)
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
