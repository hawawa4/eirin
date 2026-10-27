// Package telescopes identifies a Seestar telescope model from a frame's
// field of view, for stacked/processed images that never carried a TELESCOP
// header (e.g. Siril outputs) or had it inherited incorrectly from a project.
package telescopes

import "math"

// Model is a known Seestar telescope with the optical properties used to
// recognize it from a plate-solved frame.
type Model struct {
	Name             string
	PixelScaleArcsec float64 // arcsec/pixel, at native (unbinned, uncropped) resolution
	Width, Height    int     // native sensor resolution, pixels
}

// pixelScaleTolerance is how far a frame's measured pixel scale may differ
// from a model's nominal value (as a fraction of the nominal value) and
// still count as a match. Plate-solve noise and firmware/lens sample
// variation are both a few percent; this is intentionally generous since a
// wrong match is only ever a label, never destructive.
const pixelScaleTolerance = 0.08

// Models are the Seestar telescopes eirin can recognize by field of view.
// Pixel scale is derived from each model's published focal length and pixel
// size (arcsec/px = pixel_size_um / focal_len_mm * 206.265):
//
//	S50:     50mm aperture, 260mm FL, Sony IMX462, 2.9um px -> ~2.30"/px
//	S50 Pro: 50mm aperture, 260mm FL, 2.9um px, 4K sensor   -> ~2.30"/px
//	S30:     30mm aperture, 150mm FL, Sony IMX662, 2.9um px -> ~3.99"/px
//	S30 Pro: 30mm aperture, 160mm FL, Sony IMX585, 2.9um px -> ~3.74"/px
var Models = []Model{
	{Name: "S50", PixelScaleArcsec: 2.30, Width: 1920, Height: 1080},
	{Name: "S50 Pro", PixelScaleArcsec: 2.30, Width: 3840, Height: 2160},
	{Name: "S30", PixelScaleArcsec: 3.99, Width: 1920, Height: 1080},
	{Name: "S30 Pro", PixelScaleArcsec: 3.74, Width: 3840, Height: 2160},
}

// GuessFromFOV identifies the most likely telescope model for a frame given
// its pixel scale (arcsec/pixel, from a plate solve) and image dimensions in
// pixels. Pixel scale is the primary signal, since it stays valid under
// mosaics and most crops; dimensions only break ties between models that
// share a near-identical pixel scale (none currently do, but stay robust to
// future additions). Width/height may be 0 if unknown — the match still
// proceeds on pixel scale alone, just without tiebreaking.
//
// ok is false when pixelScale is non-positive or no model is within
// tolerance; confidence is 1 for an exact pixel-scale match, decreasing
// linearly to 0 at the tolerance boundary.
func GuessFromFOV(widthPx, heightPx int, pixelScale float64) (model string, confidence float64, ok bool) {
	if pixelScale <= 0 {
		return "", 0, false
	}

	bestIdx := -1
	bestDelta := math.Inf(1)
	var tied []int
	for i, m := range Models {
		delta := math.Abs(pixelScale-m.PixelScaleArcsec) / m.PixelScaleArcsec
		if delta > pixelScaleTolerance {
			continue
		}
		switch {
		case delta < bestDelta-1e-9:
			bestDelta = delta
			bestIdx = i
			tied = []int{i}
		case delta <= bestDelta+1e-9:
			tied = append(tied, i)
		}
	}
	if bestIdx < 0 {
		return "", 0, false
	}

	if len(tied) > 1 && widthPx > 0 && heightPx > 0 {
		if sizeMatch := matchBySize(tied, widthPx, heightPx); sizeMatch >= 0 {
			bestIdx = sizeMatch
		}
	}

	confidence = 1 - bestDelta/pixelScaleTolerance
	return Models[bestIdx].Name, confidence, true
}

// matchBySize returns the index (into Models) of the candidate whose native
// resolution matches widthPx x heightPx (in either orientation), or -1 if
// none does.
func matchBySize(candidates []int, widthPx, heightPx int) int {
	for _, i := range candidates {
		m := Models[i]
		if (m.Width == widthPx && m.Height == heightPx) || (m.Width == heightPx && m.Height == widthPx) {
			return i
		}
	}
	return -1
}
