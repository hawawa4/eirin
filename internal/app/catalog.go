package app

import "github.com/TaruDesigns/eirin/internal/catalog"

// GetAnnotations returns catalog objects visible within the frame defined by the
// plate-solve result (raCenter/decCenter in degrees, pixelScale in arcsec/pixel,
// rotation in degrees CCW from North, image dimensions in pixels).
func (a *App) GetAnnotations(raCenter, decCenter, pixelScale, rotation float64, width, height int) []catalog.Annotation {
	return catalog.GetAnnotations(raCenter, decCenter, pixelScale, rotation, width, height)
}
