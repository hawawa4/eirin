package app

import (
	"github.com/hawawa4/eirin/internal/fits"
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
