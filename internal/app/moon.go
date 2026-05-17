package app

import (
	"time"

	"github.com/TaruDesigns/eirin/internal/catalog"
)

// MoonIllumination returns the fraction of the moon's disk that is illuminated
// (0.0 = new moon, 1.0 = full moon) for a given UTC time.
func MoonIllumination(t time.Time) float64 {
	return catalog.MoonIllumination(t)
}
