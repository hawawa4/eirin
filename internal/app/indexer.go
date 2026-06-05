package app

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"log/slog"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/indexer"
	"github.com/TaruDesigns/eirin/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type indexProgressEvent = indexer.ProgressEvent

type appIndexer struct {
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
	ctx, cancel := context.WithCancel(context.Background())
	a.indexer.cancel = cancel
	a.indexer.mu.Unlock()

	go func() {
		defer func() {
			a.indexer.mu.Lock()
			a.indexer.cancel = nil
			a.indexer.mu.Unlock()
		}()

		a.emitIndexProgress(indexProgressEvent{Phase: "scanning", Current: "Scanning directories…"})

		// ── Phase 1: collect all indexable paths under rootPath ─────────────
		// fitsPaths need header parsing; rasterPaths are indexed as-is.
		var fitsPaths, rasterPaths []string
		_ = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil || ctx.Err() != nil {
				return nil
			}
			if d.IsDir() {
				return nil
			}
			switch {
			case indexer.IsFitsFile(d.Name()):
				fitsPaths = append(fitsPaths, path)
			case indexer.IsRasterFile(d.Name()):
				rasterPaths = append(rasterPaths, path)
			}
			return nil
		})

		if ctx.Err() != nil {
			a.emitIndexProgress(indexProgressEvent{Phase: "cancelled", Total: len(fitsPaths) + len(rasterPaths)})
			return
		}

		total := len(fitsPaths) + len(rasterPaths)
		a.emitIndexProgress(indexProgressEvent{Phase: "indexing", Total: total, Current: "Checking cache…"})

		if total == 0 {
			a.emitIndexProgress(indexProgressEvent{Phase: "done"})
			return
		}

		// ── Phase 2: find which paths are NOT yet indexed ────────────────────
		allPaths := append(fitsPaths, rasterPaths...)
		indexed0, err := a.store.GetFrames(allPaths)
		if err != nil {
			slog.Error("index: get frames", "err", err)
			indexed0 = map[string]store.Frame{}
		}

		toIndexFits := make([]string, 0, len(fitsPaths))
		for _, p := range fitsPaths {
			if f, ok := indexed0[p]; !ok || f.CachedAt == 0 {
				toIndexFits = append(toIndexFits, p)
			}
		}
		toIndexRaster := make([]string, 0, len(rasterPaths))
		for _, p := range rasterPaths {
			if f, ok := indexed0[p]; !ok || f.CachedAt == 0 {
				toIndexRaster = append(toIndexRaster, p)
			}
		}
		// Paths already indexed but missing a hash (existing DB rows before this feature).
		toHashOnly := make([]string, 0)
		for _, p := range allPaths {
			if f, ok := indexed0[p]; ok && f.CachedAt > 0 && f.FileHash == "" {
				toHashOnly = append(toHashOnly, p)
			}
		}

		alreadyCached := total - len(toIndexFits) - len(toIndexRaster)
		a.emitIndexProgress(indexProgressEvent{
			Phase:   "indexing",
			Total:   total,
			Done:    alreadyCached,
			Indexed: 0,
			Current: "Reading headers…",
		})

		// ── Phase 3: read headers and batch-write to frames ─────────────────
		const batchSize = 100
		batch := make(map[string]store.Frame, batchSize)
		done := alreadyCached
		newlyIndexed := 0
		errs := 0
		lastEmit := time.Now()

		flushBatch := func() {
			if len(batch) == 0 {
				return
			}
			if err := a.store.BatchUpsertFrames(batch); err != nil {
				slog.Error("index: batch upsert", "err", err)
			}
			batch = make(map[string]store.Frame, batchSize)
		}

		for _, p := range toIndexFits {
			if ctx.Err() != nil {
				break
			}

			hdr, err := fits.ReadFITSHeader(p)
			if err != nil {
				errs++
			} else {
				f := store.Frame{
					Object:     hdr.Object,
					Filter:     hdr.Filter,
					ExpTime:    hdr.ExpTime,
					DateObs:    hdr.DateObs,
					Gain:       hdr.Gain,
					CCDTemp:    hdr.CCDTemp,
					Telescope:  hdr.Telescope,
					Instrument: hdr.Instrument,
					FrameType:  store.ClassifyFrameType(p),
				}
				if hdr.RA != 0 && hdr.PixelScale > 0 {
					ra, dec, ps, rot := hdr.RA, hdr.Dec, hdr.PixelScale, hdr.Rotation
					f.RA = &ra
					f.Dec = &dec
					f.PixelScale = &ps
					f.Rotation = &rot
				}
				if hash, hashErr := importer.HashFilePrefix(p); hashErr == nil {
					f.FileHash = hash
				}
				batch[p] = f
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

		for _, p := range toIndexRaster {
			if ctx.Err() != nil {
				break
			}
			info, statErr := os.Stat(p)
			var fileSize int64
			if statErr == nil {
				fileSize = info.Size()
			}
			dm := a.store.GetDirMeta(filepath.Dir(p))
			obj := dm.Object
			if obj == "" {
				obj = filepath.Base(filepath.Dir(p))
			}
			f := store.Frame{
				FileSize:   fileSize,
				LastSeen:   time.Now().Unix(),
				FrameType:  store.FrameTypeProcessed,
				Object:     obj,
				Telescope:  dm.Telescope,
				Instrument: dm.Instrument,
			}
			if hash, hashErr := importer.HashFilePrefix(p); hashErr == nil {
				f.FileHash = hash
			}
			batch[p] = f
			newlyIndexed++
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

		// ── Phase 4: back-fill hashes for already-indexed rows that lack one ──
		for _, p := range toHashOnly {
			if ctx.Err() != nil {
				break
			}
			hash, err := importer.HashFilePrefix(p)
			if err != nil {
				slog.Warn("index: hash", "path", p, "err", err)
				continue
			}
			if err := a.store.SetFrameHash(p, hash); err != nil {
				slog.Warn("index: set hash", "path", p, "err", err)
			}
		}

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
	a.wails.Event.EmitEvent(&application.CustomEvent{Name: "index:progress", Data: evt})
}
