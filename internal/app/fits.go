package app

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/indexer"
)

func (a *App) ReadFITSHeader(path string) (*fits.FITSHeader, error) {
	return fits.ReadFITSHeader(path)
}

// GeneratePreview returns a PNG preview as a base64 data URL, scaled to 1024 px.
// stretchLevel: 0=linear, 1=gentle, 2=normal, 3=strong
func (a *App) GeneratePreview(path string, stretchLevel int) (string, error) {
	return fits.GeneratePreview(path, 1024, stretchLevel)
}

// GeneratePreviewRawSized returns raw float32 RGBA pixel data (base64-encoded) plus
// per-channel statistics for CPU/GPU-based MTF rendering on the frontend.
// maxSize=0 means native resolution (no downscaling).
func (a *App) GeneratePreviewRawSized(path string, maxSize int) (fits.RawPreviewData, error) {
	if maxSize <= 0 {
		maxSize = 1<<31 - 1
	}
	return fits.GeneratePreviewRaw(path, maxSize)
}

// LoadRasterImage reads a PNG or TIFF file from disk and returns it as a
// base64-encoded data URL (e.g. "data:image/png;base64,..."). The file must
// be under the configured NAS root. This avoids any dependency on the local
// HTTP server's URL being available in the frontend.
func (a *App) LoadRasterImage(path string) (string, error) {
	if !indexer.IsRasterFile(path) {
		return "", fmt.Errorf("not a supported raster file: %s", filepath.Base(path))
	}

	// Security: only serve files under the configured NAS root.
	rootFolder := a.store.Load().RootFolder
	absPath := filepath.Clean(path)
	absRoot := filepath.Clean(rootFolder)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		return "", fmt.Errorf("path outside NAS root")
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	mime := http.DetectContentType(data)
	// DetectContentType may return "image/tiff" or "application/octet-stream" for TIFFs;
	// fix up the latter by extension.
	ext := strings.ToLower(filepath.Ext(absPath))
	if mime == "application/octet-stream" {
		switch ext {
		case ".tif", ".tiff":
			mime = "image/tiff"
		case ".png":
			mime = "image/png"
		}
	}

	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
