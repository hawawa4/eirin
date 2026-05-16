package app

import (
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AtlasFrame holds the WCS and dimension data needed to draw an image footprint
// in the Sky Atlas. Width/Height are the physical FITS image dimensions in pixels.
type AtlasFrame struct {
	NasPath    string  `json:"nasPath"`
	Name       string  `json:"name"`
	Object     string  `json:"object"`
	FrameType  string  `json:"frameType"`
	RA         float64 `json:"ra"`
	Dec        float64 `json:"dec"`
	PixelScale float64 `json:"pixelScale"` // arcsec/pixel
	Rotation   float64 `json:"rotation"`   // degrees (N through E)
	Width      int     `json:"width"`      // image width in pixels
	Height     int     `json:"height"`     // image height in pixels
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
	entries := loadCatalog()
	out := make([]CatalogObject, len(entries))
	for i, e := range entries {
		out[i] = CatalogObject{RA: e.RA, Dec: e.Dec, Name: e.Name, Type: e.Type, Mag: e.Mag}
	}
	return out
}

// GetAtlasFrames returns stacked and processed frames that have usable WCS data.
// Light frames are never included — the Atlas shows finished results only.
// FITS headers are read sequentially (not in parallel) to avoid overwhelming the disk.
func (a *App) GetAtlasFrames(rootPath string) []AtlasFrame {
	frames, err := a.prefs.GetAllFramesUnder(rootPath)
	if err != nil {
		runtime.LogErrorf(a.ctx, "atlas: get frames: %v", err)
		return nil
	}

	var out []AtlasFrame
	for _, f := range frames {
		if f.FrameType != prefs.FrameTypeStacked && f.FrameType != prefs.FrameTypeProcessed {
			continue
		}
		if af := frameToAtlas(f); af != nil {
			out = append(out, *af)
		}
	}
	return out
}

func frameToAtlas(f prefs.Frame) *AtlasFrame {
	ra := derefFloat(f.RA)
	dec := derefFloat(f.Dec)
	scale := derefFloat(f.PixelScale)
	rot := derefFloat(f.Rotation)

	// Read the FITS header to get image dimensions (not stored in DB)
	// and to get WCS from CRVAL1/CRVAL2 when the DB has no WCS yet.
	hdr, err := readFITSHeader(f.NasPath)
	if err != nil {
		return nil
	}

	if ra == 0 {
		ra = hdr.RA
	}
	if dec == 0 {
		dec = hdr.Dec
	}
	if scale == 0 {
		scale = hdr.PixelScale
	}
	if rot == 0 {
		rot = hdr.Rotation
	}

	if ra == 0 || scale == 0 || hdr.Width == 0 || hdr.Height == 0 {
		return nil
	}

	return &AtlasFrame{
		NasPath:    f.NasPath,
		Name:       filepath.Base(f.NasPath),
		Object:     f.Object,
		FrameType:  f.FrameType,
		RA:         ra,
		Dec:        dec,
		PixelScale: scale,
		Rotation:   rot,
		Width:      hdr.Width,
		Height:     hdr.Height,
	}
}
