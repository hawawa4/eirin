package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/indexer"
)

func (a *App) ReadFITSHeader(path string) (*fits.FITSHeader, error) {
	if err := a.checkServable(path); err != nil {
		return nil, err
	}
	return fits.ReadFITSHeader(path)
}

// GeneratePreview returns a JPEG preview as a base64 data URL, scaled to
// 1024 px and cached in memory (Blink loops over the same frames).
// stretchLevel: 0=linear, 1=gentle, 2=normal, 3=strong
func (a *App) GeneratePreview(path string, stretchLevel int) (string, error) {
	if err := a.checkServable(path); err != nil {
		return "", err
	}
	return a.previews.Preview(path, 1024, stretchLevel)
}

// GeneratePreviewRawSized returns raw float32 RGBA pixel data (base64-encoded) plus
// per-channel statistics for CPU/GPU-based MTF rendering on the frontend.
// maxSize=0 means native resolution (no downscaling).
func (a *App) GeneratePreviewRawSized(path string, maxSize int) (fits.RawPreviewData, error) {
	if err := a.checkServable(path); err != nil {
		return fits.RawPreviewData{}, err
	}
	if maxSize <= 0 {
		maxSize = 1<<31 - 1
	}
	return fits.GeneratePreviewRaw(path, maxSize)
}

// viewerPreviewSize is the longest side of the server viewer's previews:
// the same size the desktop renders FITS previews at.
const viewerPreviewSize = 2048

// LoadRasterImage returns a PNG, JPEG or TIFF file as a data URL a browser
// can display (TIFF is converted to PNG). The file must be under the
// configured NAS root.
func (a *App) LoadRasterImage(path string) (string, error) {
	if err := a.checkServable(path); err != nil {
		return "", err
	}
	if !indexer.IsRasterFile(path) {
		return "", fmt.Errorf("not a supported raster file: %s", filepath.Base(path))
	}

	// Security: only serve files under the configured NAS root.
	rootFolder := a.store().Load().RootFolder
	absPath := filepath.Clean(path)
	absRoot := filepath.Clean(rootFolder)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside NAS root")
	}

	return fits.RasterDataURL(absPath)
}

// GetViewerPreview renders a preview on the server for the read-only viewer,
// which reaches the backend over the network: a JPEG of at most
// viewerPreviewSize pixels (~1 MB) instead of the raw float pixels or the
// original file (tens of MB). FITS files are stretched at stretchLevel
// (0=linear … 3=strong); raster images are shown as they are.
func (a *App) GetViewerPreview(path string, stretchLevel int) (fits.RenderedPreview, error) {
	if err := a.checkServable(path); err != nil {
		return fits.RenderedPreview{}, err
	}
	if indexer.IsRasterFile(path) {
		return a.previews.Raster(path, viewerPreviewSize)
	}
	return a.previews.Rendered(path, viewerPreviewSize, stretchLevel)
}
