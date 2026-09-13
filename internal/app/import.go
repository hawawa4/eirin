package app

import (
	"log/slog"
	"path/filepath"
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
			FrameType:  store.FrameTypeImage,
			Object:     obj,
			Telescope:  dirMeta.Telescope,
			Instrument: dirMeta.Instrument,
		}
		if err := a.store.UpsertFrame(c.DestPath, frame); err != nil {
			slog.Warn("import: upsert", "path", c.DestPath, "err", err)
		}
		// Siril cannot load PNG/TIFF for plate-solving, so no analysis is spawned here.
	}
}
