package app

import "github.com/TaruDesigns/eirin/internal/fits"

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
