package catalog

import (
	_ "embed"
	"math"
	"strconv"
	"strings"
	"sync"
)

// Annotation is a labeled point projected onto the image plane.
type Annotation struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Label string  `json:"label"`
	Type  string  `json:"type"` // "star" or "dso"
	Mag   float64 `json:"mag"`
}

//go:embed data/catalog.tsv
var catalogTSV string

type catalogEntry struct {
	RA, Dec float64
	Name    string
	Mag     float64
	Type    string
}

// Entry is a catalog object exposed to callers outside this package.
type Entry struct {
	RA, Dec float64
	Name    string
	Mag     float64
	Type    string
}

// AllEntries returns all catalog objects.
func AllEntries() []Entry {
	raw := loadCatalog()
	out := make([]Entry, len(raw))
	for i, e := range raw {
		out[i] = Entry{RA: e.RA, Dec: e.Dec, Name: e.Name, Mag: e.Mag, Type: e.Type}
	}
	return out
}

var (
	catalogOnce    sync.Once
	catalogEntries []catalogEntry
)

func loadCatalog() []catalogEntry {
	catalogOnce.Do(func() {
		lines := strings.Split(catalogTSV, "\n")
		entries := make([]catalogEntry, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) < 5 {
				continue
			}
			ra, err1 := strconv.ParseFloat(strings.TrimSpace(fields[0]), 64)
			dec, err2 := strconv.ParseFloat(strings.TrimSpace(fields[1]), 64)
			if err1 != nil || err2 != nil {
				continue
			}
			mag, _ := strconv.ParseFloat(strings.TrimSpace(fields[3]), 64)
			entries = append(entries, catalogEntry{
				RA:   ra,
				Dec:  dec,
				Name: strings.TrimSpace(fields[2]),
				Mag:  mag,
				Type: strings.TrimSpace(fields[4]),
			})
		}
		catalogEntries = entries
	})
	return catalogEntries
}

// GetAnnotations returns catalog objects visible within the frame defined by the
// plate-solve result (raCenter/decCenter in degrees, pixelScale in arcsec/pixel,
// rotation in degrees CCW from North, image dimensions in pixels).
func GetAnnotations(raCenter, decCenter, pixelScale, rotation float64, width, height int) []Annotation {
	if pixelScale <= 0 || width <= 0 || height <= 0 {
		return nil
	}

	entries := loadCatalog()

	ra0 := deg2rad(raCenter)
	dec0 := deg2rad(decCenter)
	rotRad := deg2rad(rotation)
	scaleDeg := deg2rad(pixelScale / 3600.0) // arcsec/px → deg/px → rad/px

	cx := float64(width) / 2
	cy := float64(height) / 2

	// Deduplicate by label to avoid showing the same object twice from the catalog
	seen := make(map[string]bool)
	var out []Annotation

	for _, s := range entries {
		if seen[s.Name] {
			continue
		}
		ra := deg2rad(s.RA)
		dec := deg2rad(s.Dec)

		// Gnomonic (tangential) projection
		dRA := ra - ra0
		denom := math.Sin(dec0)*math.Sin(dec) + math.Cos(dec0)*math.Cos(dec)*math.Cos(dRA)
		if denom <= 0 {
			continue
		}
		xi := math.Cos(dec) * math.Sin(dRA) / denom
		eta := (math.Cos(dec0)*math.Sin(dec) - math.Sin(dec0)*math.Cos(dec)*math.Cos(dRA)) / denom

		xPx := xi / scaleDeg
		yPx := -eta / scaleDeg // flip: eta north = y up

		// Apply field rotation
		cosR := math.Cos(rotRad)
		sinR := math.Sin(rotRad)
		xRot := xPx*cosR - yPx*sinR
		yRot := xPx*sinR + yPx*cosR

		px := cx + xRot
		py := cy + yRot

		margin := 20.0
		if px < -margin || px > float64(width)+margin || py < -margin || py > float64(height)+margin {
			continue
		}

		seen[s.Name] = true
		out = append(out, Annotation{
			X:     math.Round(px*10) / 10,
			Y:     math.Round(py*10) / 10,
			Label: s.Name,
			Type:  s.Type,
			Mag:   s.Mag,
		})
	}
	return out
}

