package app

import (
	"fmt"
	"image"
	"os"
	"path/filepath"

	"log/slog"

	"github.com/TaruDesigns/eirin/internal/catalog"
	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/indexer"
	_ "golang.org/x/image/tiff" // register TIFF decoder
)

// AtlasIndexEntry holds the DB-resident data for a single stacked/processed
// frame. Width and Height are NOT included — they require a FITS read and are
// fetched lazily via GetAtlasFrameSize when the frame is visible in the viewport.
type AtlasIndexEntry struct {
	NasPath    string  `json:"nasPath"`
	Name       string  `json:"name"`
	Object     string  `json:"object"`
	FrameType  string  `json:"frameType"`
	RA         float64 `json:"ra"`
	Dec        float64 `json:"dec"`
	PixelScale float64 `json:"pixelScale"` // arcsec/pixel
	Rotation   float64 `json:"rotation"`   // degrees (N through E)
}

// AtlasFrameSize holds the pixel dimensions read from a FITS header.
// Fetched lazily by the frontend when a frame enters the viewport at sufficient zoom.
type AtlasFrameSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// CatalogObject is a catalog entry exposed to the frontend for the Sky Atlas overlay.
type CatalogObject struct {
	RA   float64 `json:"ra"`
	Dec  float64 `json:"dec"`
	Name string  `json:"name"`
	Type string  `json:"type"`
	Mag  float64 `json:"mag"`
}

// GetCatalog returns all catalog objects (stars and DSOs) for the Sky Atlas star layer.
func (a *App) GetCatalog() []CatalogObject {
	entries := catalog.AllEntries()
	out := make([]CatalogObject, len(entries))
	for i, e := range entries {
		out[i] = CatalogObject{RA: e.RA, Dec: e.Dec, Name: e.Name, Type: e.Type, Mag: e.Mag}
	}
	return out
}

// GetAtlasIndex returns all stacked and processed frames under rootPath that
// have WCS coordinates stored in the DB. No FITS files are read — this is a
// fast DB-only query that returns immediately even for large libraries.
// Light frames are never included in the Atlas.
func (a *App) GetAtlasIndex(rootPath string) []AtlasIndexEntry {
	frames, err := a.store.GetAtlasIndexFrames(rootPath)
	if err != nil {
		slog.Error("atlas: get index", "err", err)
		return nil
	}

	var out []AtlasIndexEntry
	for _, f := range frames {
		ra := derefFloat(f.RA)
		dec := derefFloat(f.Dec)
		scale := derefFloat(f.PixelScale)
		rot := derefFloat(f.Rotation)
		if ra == 0 || scale == 0 {
			continue // no WCS in DB yet; user needs to re-index
		}
		out = append(out, AtlasIndexEntry{
			NasPath:    f.NasPath,
			Name:       filepath.Base(f.NasPath),
			Object:     f.Object,
			FrameType:  f.FrameType,
			RA:         ra,
			Dec:        dec,
			PixelScale: scale,
			Rotation:   rot,
		})
	}
	return out
}

// GetAtlasFrameSize reads pixel dimensions for any supported file type.
// For FITS files the header is parsed; for PNG/TIFF the image config is decoded
// (much cheaper — no full pixel decode). Called lazily by the frontend.
func (a *App) GetAtlasFrameSize(nasPath string) (AtlasFrameSize, error) {
	if indexer.IsRasterFile(nasPath) {
		f, err := os.Open(nasPath)
		if err != nil {
			return AtlasFrameSize{}, err
		}
		defer f.Close()
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			return AtlasFrameSize{}, fmt.Errorf("decode config: %w", err)
		}
		return AtlasFrameSize{Width: cfg.Width, Height: cfg.Height}, nil
	}
	hdr, err := fits.ReadFITSHeader(nasPath)
	if err != nil {
		return AtlasFrameSize{}, err
	}
	return AtlasFrameSize{Width: hdr.Width, Height: hdr.Height}, nil
}

