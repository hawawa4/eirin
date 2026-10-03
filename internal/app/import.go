package app

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/indexer"
	"github.com/TaruDesigns/eirin/internal/store"
)

// ImportCandidate is a file in the source folder not yet in the library.
// Re-exported from the importer package for Wails binding compatibility.
type ImportCandidate = importer.Candidate

// ImportProgress is emitted as an "import:progress" event during StartImport.
// Re-exported from the importer package for Wails binding compatibility.
type ImportProgress = importer.Progress

// errImportRunning is returned by StartImport while another import is active.
var errImportRunning = errors.New("an import is already running")

// importJob tracks the single import run allowed at a time, plus the latest
// progress snapshot so a remounted UI can resume showing it.
type importJob struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	last    ImportProgress
}

// begin marks a job as running and returns its context. It fails if a job is
// already running.
func (j *importJob) begin() (context.Context, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.running {
		return nil, errImportRunning
	}
	ctx, cancel := context.WithCancel(context.Background())
	j.running = true
	j.cancel = cancel
	j.last = ImportProgress{Phase: importer.PhaseScanning, Errors: []string{}, Notes: []string{}}
	return ctx, nil
}

// update records p as the latest snapshot.
func (j *importJob) update(p ImportProgress) {
	j.mu.Lock()
	j.last = p
	j.mu.Unlock()
}

// finish records the final snapshot and releases the job slot.
func (j *importJob) finish(p ImportProgress) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.last = p
	j.running = false
	if j.cancel != nil {
		j.cancel()
		j.cancel = nil
	}
}

// GetImportStatus returns the latest import progress snapshot, or a progress
// with Phase "idle" if no import has run since startup.
func (a *App) GetImportStatus() ImportProgress {
	a.imports.mu.Lock()
	defer a.imports.mu.Unlock()
	if a.imports.last.Phase == "" {
		return ImportProgress{Phase: importer.PhaseIdle, Errors: []string{}, Notes: []string{}}
	}
	p := a.imports.last
	p.Errors = append([]string{}, p.Errors...)
	p.Notes = append([]string{}, p.Notes...)
	return p
}

// CancelImport stops the running import after the file currently being
// processed. The run then emits a final "import:progress" with phase
// "cancelled". No-op if nothing is running.
func (a *App) CancelImport() {
	a.imports.mu.Lock()
	defer a.imports.mu.Unlock()
	if a.imports.cancel != nil {
		a.imports.cancel()
	}
}

// indexImportedFile inserts a freshly-copied file into the database so it
// shows up in the library without a separate Build Index run.
// FITS files are parsed for header metadata; empty fields are filled from
// existing light frames in the same directory (e.g. Seestar files lack telescope).
// PNG/TIFF are stored as processed frames with metadata inferred from the directory.
func (a *App) indexImportedFile(c ImportCandidate) {
	a.indexImportedFileWithMeta(c, a.store.GetDirMeta(filepath.Dir(c.DestPath)))
}

func (a *App) indexImportedFileWithMeta(c ImportCandidate, dirMeta store.DirMeta) {
	dir := filepath.Dir(c.DestPath)

	switch {
	case indexer.IsFitsFile(c.DestPath):
		hdr, err := fits.ReadFITSHeader(c.DestPath)
		if err != nil {
			slog.Warn("import: index", "path", c.RelativePath, "err", err)
			return
		}
		// Fill empty header fields from existing lights in the same directory.
		obj := hdr.Object
		if obj == "" {
			obj = dirMeta.Object
		}
		if obj == "" {
			obj = filepath.Base(dir)
		}
		telescope := hdr.Telescope
		if telescope == "" {
			telescope = dirMeta.Telescope
		}
		instrument := hdr.Instrument
		if instrument == "" {
			instrument = dirMeta.Instrument
		}
		filter := hdr.Filter
		if filter == "" {
			filter = dirMeta.Filter
		}
		frame := store.Frame{
			FileSize:   c.FileSize,
			LastSeen:   time.Now().Unix(),
			FileHash:   c.FileHash,
			Object:     obj,
			Filter:     filter,
			ExpTime:    hdr.ExpTime,
			DateObs:    hdr.DateObs,
			Gain:       hdr.Gain,
			CCDTemp:    hdr.CCDTemp,
			Telescope:  telescope,
			Instrument: instrument,
			FrameType:  store.ClassifyFrameType(c.DestPath),
		}
		if hdr.RA != 0 && hdr.PixelScale > 0 {
			ra, dec, ps, rot := hdr.RA, hdr.Dec, hdr.PixelScale, hdr.Rotation
			frame.RA = &ra
			frame.Dec = &dec
			frame.PixelScale = &ps
			frame.Rotation = &rot
		}
		if err := a.store.UpsertFrame(c.DestPath, frame); err != nil {
			slog.Warn("import: upsert", "path", c.DestPath, "err", err)
		}
	case indexer.IsRasterFile(c.DestPath):
		obj := dirMeta.Object
		if obj == "" {
			obj = filepath.Base(dir)
		}
		frame := store.Frame{
			FileSize:   c.FileSize,
			LastSeen:   time.Now().Unix(),
			FileHash:   c.FileHash,
			FrameType:  store.ClassifyRasterType(c.DestPath),
			Object:     obj,
			Telescope:  dirMeta.Telescope,
			Instrument: dirMeta.Instrument,
		}
		if err := a.store.UpsertFrame(c.DestPath, frame); err != nil {
			slog.Warn("import: upsert", "path", c.DestPath, "err", err)
		}
		// Siril cannot load PNG/TIFF for plate-solving, so no analysis is spawned here.
	default:
		slog.Warn("import: skipping unrecognized file type", "path", c.DestPath)
	}
}
