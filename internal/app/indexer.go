package app

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type indexProgressEvent struct {
	Phase   string `json:"phase"`   // "scanning" | "indexing" | "done" | "cancelled"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Indexed int    `json:"indexed"` // files newly read and added to cache
	Errors  int    `json:"errors"`
	Current string `json:"current"` // short display name of file being processed
}

type indexer struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

// BuildIndex traverses every subdirectory under rootPath, reads FITS headers
// for files not yet in the cache, and stores them. Progress is reported via
// "index:progress" Wails events. Calling BuildIndex again only processes
// files added since the last run.
func (a *App) BuildIndex(rootPath string) {
	a.indexer.mu.Lock()
	if a.indexer.cancel != nil {
		a.indexer.cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.indexer.cancel = cancel
	a.indexer.mu.Unlock()

	go func() {
		defer func() {
			a.indexer.mu.Lock()
			a.indexer.cancel = nil
			a.indexer.mu.Unlock()
		}()

		a.emitIndexProgress(indexProgressEvent{Phase: "scanning", Current: "Scanning directories…"})

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
			a.emitIndexProgress(indexProgressEvent{Phase: "cancelled", Total: len(fitsPaths)})
			return
		}

		total := len(fitsPaths)
		a.emitIndexProgress(indexProgressEvent{Phase: "indexing", Total: total, Current: "Checking cache…"})

		if total == 0 {
			a.emitIndexProgress(indexProgressEvent{Phase: "done"})
			return
		}

		// ── Phase 2: find which paths are NOT yet indexed ────────────────────
		// A frame with CachedAt==0 exists only due to a prior reject action;
		// its FITS header still needs to be read.
		indexed0, err := a.prefs.GetFrames(fitsPaths)
		if err != nil {
			runtime.LogErrorf(a.ctx, "index: get frames: %v", err)
			indexed0 = map[string]prefs.Frame{}
		}

		toIndex := make([]string, 0, len(fitsPaths))
		for _, p := range fitsPaths {
			if f, ok := indexed0[p]; !ok || f.CachedAt == 0 {
				toIndex = append(toIndex, p)
			}
		}

		alreadyCached := total - len(toIndex)
		a.emitIndexProgress(indexProgressEvent{
			Phase:   "indexing",
			Total:   total,
			Done:    alreadyCached,
			Indexed: 0,
			Current: "Reading headers…",
		})

		// ── Phase 3: read headers and batch-write to frames ─────────────────
		const batchSize = 100
		batch := make(map[string]prefs.Frame, batchSize)
		done := alreadyCached
		newlyIndexed := 0
		errs := 0
		lastEmit := time.Now()

		flushBatch := func() {
			if len(batch) == 0 {
				return
			}
			if err := a.prefs.BatchUpsertFrames(batch); err != nil {
				runtime.LogErrorf(a.ctx, "index: batch upsert: %v", err)
			}
			batch = make(map[string]prefs.Frame, batchSize)
		}

		for _, p := range toIndex {
			if ctx.Err() != nil {
				break
			}

			hdr, err := readFITSHeader(p)
			if err != nil {
				errs++
			} else {
				batch[p] = prefs.Frame{
					Object:     hdr.Object,
					Filter:     hdr.Filter,
					ExpTime:    hdr.ExpTime,
					DateObs:    hdr.DateObs,
					Gain:       hdr.Gain,
					CCDTemp:    hdr.CCDTemp,
					Telescope:  hdr.Telescope,
					Instrument: hdr.Instrument,
					FrameType:  prefs.ClassifyFrameType(p),
				}
				newlyIndexed++
			}
			done++

			if len(batch) >= batchSize {
				flushBatch()
			}

			if time.Since(lastEmit) >= 80*time.Millisecond {
				a.emitIndexProgress(indexProgressEvent{
					Phase:   "indexing",
					Total:   total,
					Done:    done,
					Indexed: newlyIndexed,
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
		a.emitIndexProgress(indexProgressEvent{
			Phase:   phase,
			Total:   total,
			Done:    done,
			Indexed: newlyIndexed,
			Errors:  errs,
		})
	}()
}

// CancelIndex stops any running BuildIndex operation.
func (a *App) CancelIndex() {
	a.indexer.mu.Lock()
	defer a.indexer.mu.Unlock()
	if a.indexer.cancel != nil {
		a.indexer.cancel()
		a.indexer.cancel = nil
	}
}

func (a *App) emitIndexProgress(evt indexProgressEvent) {
	runtime.EventsEmit(a.ctx, "index:progress", evt)
}

func isFitsFile(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".fits") || strings.HasSuffix(l, ".fit")
}
