package app

import (
	"path/filepath"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// LibraryFrame is the data exported to the frontend for the library view.
type LibraryFrame struct {
	NasPath    string  `json:"nasPath"`
	FileName   string  `json:"fileName"`
	FrameType  string  `json:"frameType"`
	Object     string  `json:"object"`
	Filter     string  `json:"filter"`
	ExpTime    float64 `json:"expTime"`
	DateObs    string  `json:"dateObs"`
	Gain       float64 `json:"gain"`
	CCDTemp    float64 `json:"ccdTemp"`
	Telescope  string  `json:"telescope"`
	Instrument string  `json:"instrument"`
	FileSize   int64   `json:"fileSize"`
	IsRejected bool    `json:"isRejected"`

	// Plate solve results (zero when not solved)
	RA         float64 `json:"ra"`
	Dec        float64 `json:"dec"`
	PixelScale float64 `json:"pixelScale"` // arcsec/pixel
	Rotation   float64 `json:"rotation"`   // degrees
	WCSSolved  bool    `json:"wcsSolved"`

	// Quality metrics from Siril analysis (zero-value when not yet analyzed)
	FWHM            float64 `json:"fwhm"`
	FWHMUnit        string  `json:"fwhmUnit"` // "px" or "arcsec"
	Background      float64 `json:"background"`
	Noise           float64 `json:"noise"`
	SNR             float64 `json:"snr"` // Background/Noise ratio
	StarCount       int64   `json:"starCount"`
	QualityAnalyzed bool    `json:"qualityAnalyzed"`
}

// GetLibraryFrames returns all indexed frames under rootPath, converted to the
// LibraryFrame shape for the frontend library view.
func (a *App) GetLibraryFrames(rootPath string) []LibraryFrame {
	frames, err := a.prefs.GetAllFramesUnder(rootPath)
	if err != nil {
		runtime.LogErrorf(a.ctx, "library: get frames: %v", err)
		return nil
	}
	result := make([]LibraryFrame, 0, len(frames))
	for _, f := range frames {
		result = append(result, LibraryFrame{
			NasPath:         f.NasPath,
			FileName:        filepath.Base(f.NasPath),
			FrameType:       f.FrameType,
			Object:          f.Object,
			Filter:          f.Filter,
			ExpTime:         f.ExpTime,
			DateObs:         f.DateObs,
			Gain:            f.Gain,
			CCDTemp:         f.CCDTemp,
			Telescope:       f.Telescope,
			Instrument:      f.Instrument,
			FileSize:        f.FileSize,
			IsRejected:      f.Rejected,
			RA:              derefFloat(f.RA),
			Dec:             derefFloat(f.Dec),
			PixelScale:      derefFloat(f.PixelScale),
			Rotation:        derefFloat(f.Rotation),
			WCSSolved:       f.WCSSolved,
			FWHM:            derefFloat(f.FWHM),
			FWHMUnit:        f.FWHMUnit,
			Background:      derefFloat(f.Background),
			Noise:           derefFloat(f.Noise),
			SNR:             derefFloat(f.SNR),
			StarCount:       derefInt64(f.StarCount),
			QualityAnalyzed: f.QualityAnalyzed,
		})
	}
	return result
}

func derefFloat(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func derefInt64(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// SetFrameType lets the user manually override the classified type for a frame
// (e.g. to promote a stacked image to "processed").
func (a *App) SetFrameType(nasPath string, frameType string) error {
	validTypes := map[string]bool{
		prefs.FrameTypeLight:     true,
		prefs.FrameTypeDark:      true,
		prefs.FrameTypeFlat:      true,
		prefs.FrameTypeBias:      true,
		prefs.FrameTypeStacked:   true,
		prefs.FrameTypeProcessed: true,
	}
	if !validTypes[frameType] {
		return nil
	}
	return a.prefs.SetFrameType(nasPath, frameType)
}
