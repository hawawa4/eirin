package app

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"

	"github.com/TaruDesigns/eirin/internal/catalog"
	"github.com/TaruDesigns/eirin/internal/coverage"
	"github.com/TaruDesigns/eirin/internal/store"
)

// CoverageCluster is a group of light frames sharing a framing (see
// internal/coverage), as drawn by the Coverage view.
type CoverageCluster struct {
	// ID is stable across reloads while the cluster's first path survives.
	ID    string `json:"id"`
	Scope string `json:"scope"`
	// Centroid of the sub centres; zero for unplaced groups.
	RA  float64 `json:"ra"`
	Dec float64 `json:"dec"`
	// Hull outlines all the subs' footprints; empty when the scope's sensor
	// size couldn't be read (the view then draws a marker).
	Hull []SkyPoint `json:"hull"`
	// Rotation is the outline's rotation (degrees, mod 180); RotationSpread
	// how far the subs' rotations range around it (field rotation).
	Rotation       float64 `json:"rotation"`
	RotationSpread float64 `json:"rotationSpread"`
	PixelScale     float64 `json:"pixelScale"` // arcsec/pixel
	// Approx marks a position estimated from the OBJECT name (no WCS).
	Approx    bool             `json:"approx"`
	Objects   []CoverageObject `json:"objects"`
	Filters   []string         `json:"filters"`
	Frames    int              `json:"frames"`
	ExpTotal  float64          `json:"expTotal"` // seconds
	FirstDate string           `json:"firstDate"`
	LastDate  string           `json:"lastDate"`
	Nights    int              `json:"nights"`
	Paths     []string         `json:"paths"`
}

// SkyPoint is a sky position in degrees.
type SkyPoint struct {
	RA  float64 `json:"ra"`
	Dec float64 `json:"dec"`
}

// CoverageObject is one OBJECT tag within a cluster.
type CoverageObject struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// CoverageScope is one telescope with lights in the library.
type CoverageScope struct {
	Scope  string `json:"scope"`
	Label  string `json:"label"`
	Frames int    `json:"frames"`
}

// LightCoverage is everything the Coverage view shows.
type LightCoverage struct {
	Clusters []CoverageCluster `json:"clusters"`
	// Unplaced holds lights with neither WCS nor a catalog match for their
	// OBJECT, grouped by scope and object.
	Unplaced []CoverageCluster `json:"unplaced"`
	// Scopes, most frames first.
	Scopes []CoverageScope `json:"scopes"`
	// Processed are the footprints of processed images with WCS: data that
	// has already been worked up, shown for reference.
	Processed []ProcessedFootprint `json:"processed"`
}

// ProcessedFootprint is where a processed image sits on the sky.
type ProcessedFootprint struct {
	NasPath string     `json:"nasPath"`
	Name    string     `json:"name"`
	Object  string     `json:"object"`
	Hull    []SkyPoint `json:"hull"`
}

// GetLightCoverage groups the non-rejected light frames under rootPath by
// telescope and framing. Lights without WCS are placed (Approx) where other
// frames of their OBJECT were measured or at its catalog position, or, failing
// both, listed as unplaced.
func (a *App) GetLightCoverage(rootPath string) (LightCoverage, error) {
	frames, err := a.store().GetCoverageLights(rootPath)
	if err != nil {
		slog.Error("coverage: get lights", "err", err)
		return LightCoverage{}, fmt.Errorf("reading library: %w", err)
	}

	finals, err := a.store().GetAtlasIndexFrames(rootPath)
	if err != nil {
		slog.Warn("coverage: get stacked frames", "err", err)
	}
	scales := medianScales(frames)
	estimate := objectEstimator(frames, finals)

	var placed, unplaced []coverage.Sub
	samples := map[string][]string{}
	scopeFrames := map[string]int{}
	for _, f := range frames {
		s := coverage.Sub{
			Path: f.NasPath, Object: f.Object, Filter: f.Filter, DateObs: f.DateObs,
			ExpTime: f.ExpTime, Scope: f.Telescope,
			PixelScale: derefFloat(f.PixelScale), Rotation: derefFloat(f.Rotation),
		}
		scopeFrames[s.Scope]++
		if len(samples[s.Scope]) < maxSizeProbes {
			samples[s.Scope] = append(samples[s.Scope], s.Path)
		}
		switch {
		case hasPosition(f):
			s.RA, s.Dec = *f.RA, *f.Dec
		case estimate(f.Object, &s.RA, &s.Dec):
			s.PixelScale, s.Rotation, s.Approx = 0, 0, true
		default:
			unplaced = append(unplaced, s)
			continue
		}
		placed = append(placed, s)
	}

	fields := map[string]coverage.Field{}
	for scope, paths := range samples {
		sz := a.scopeSize(scope, paths)
		fields[scope] = coverage.Field{Width: sz[0], Height: sz[1], PixelScale: scales[scope]}
	}

	out := LightCoverage{
		Clusters: toCoverageClusters(coverage.Build(placed, fields), fields),
		Unplaced: toCoverageClusters(coverage.ByObject(unplaced), nil),
		Scopes:    coverageScopes(scopeFrames),
		Processed: a.processedFootprints(finals),
	}
	return out, nil
}

// objectEstimator returns a function that estimates where an OBJECT tag
// points: the mean centre of the measured lights with that tag, else of the
// stacked/processed frames with it, else its catalog position. It reports
// whether it found one.
func objectEstimator(lights, stacks []store.Frame) func(object string, ra, dec *float64) bool {
	measured := map[string]*coverage.Centroid{}
	addAll := func(frames []store.Frame) {
		seen := map[string]bool{}
		for o := range measured {
			seen[o] = true // lights win over stacks
		}
		for _, f := range frames {
			if f.Object == "" || seen[f.Object] || !hasPosition(f) {
				continue
			}
			c := measured[f.Object]
			if c == nil {
				c = &coverage.Centroid{}
				measured[f.Object] = c
			}
			c.Add(*f.RA, *f.Dec)
		}
	}
	addAll(lights)
	addAll(stacks)

	type pos struct {
		ra, dec float64
		ok      bool
	}
	cache := map[string]pos{}
	return func(object string, ra, dec *float64) bool {
		p, seen := cache[object]
		if !seen {
			if c := measured[object]; c != nil {
				p.ra, p.dec, p.ok = c.Position()
			} else {
				p.ra, p.dec, p.ok = catalog.LookupByName(object)
			}
			cache[object] = p
		}
		*ra, *dec = p.ra, p.dec
		return p.ok
	}
}

// processedFootprints outlines the processed images among frames that have
// WCS. A PNG exported next to its FITS lands on the same spot, so footprints
// with the same object and centre are shown once.
func (a *App) processedFootprints(frames []store.Frame) []ProcessedFootprint {
	out := []ProcessedFootprint{}
	seen := map[string]bool{}
	for _, f := range frames {
		scale := derefFloat(f.PixelScale)
		if f.FrameType != store.FrameTypeProcessed || !hasPosition(f) || scale <= 0 {
			continue
		}
		key := fmt.Sprintf("%s|%.3f|%.3f", f.Object, *f.RA, *f.Dec)
		if seen[key] {
			continue
		}
		sz := a.frameSize(f.NasPath)
		hull := coverage.Footprint(*f.RA, *f.Dec, scale, derefFloat(f.Rotation), sz[0], sz[1])
		if hull == nil {
			continue
		}
		seen[key] = true
		out = append(out, ProcessedFootprint{
			NasPath: f.NasPath,
			Name:    filepath.Base(f.NasPath),
			Object:  f.Object,
			Hull:    toSkyPoints(hull),
		})
	}
	return out
}

func toSkyPoints(ps []coverage.SkyPoint) []SkyPoint {
	out := make([]SkyPoint, len(ps))
	for i, p := range ps {
		out[i] = SkyPoint(p)
	}
	return out
}

// hasPosition reports whether a frame has measured (header or plate-solve) coordinates.
func hasPosition(f store.Frame) bool {
	return f.RA != nil && f.Dec != nil && (*f.RA != 0 || *f.Dec != 0)
}

// medianScales is the median pixel scale of each scope's lights that have one.
func medianScales(frames []store.Frame) map[string]float64 {
	byScope := map[string][]float64{}
	for _, f := range frames {
		if s := derefFloat(f.PixelScale); s > 0 {
			byScope[f.Telescope] = append(byScope[f.Telescope], s)
		}
	}
	out := make(map[string]float64, len(byScope))
	for scope, v := range byScope {
		sort.Float64s(v)
		out[scope] = v[len(v)/2]
	}
	return out
}

func toCoverageClusters(cs []coverage.Cluster, fields map[string]coverage.Field) []CoverageCluster {
	out := make([]CoverageCluster, 0, len(cs))
	for _, c := range cs {
		f := fields[c.Scope]
		scale := c.PixelScale
		if scale == 0 {
			scale = f.PixelScale
		}
		objects := make([]CoverageObject, len(c.Objects))
		for i, o := range c.Objects {
			objects[i] = CoverageObject(o)
		}
		out = append(out, CoverageCluster{
			ID:         c.Paths[0],
			Scope:      c.Scope,
			RA:         c.RA,
			Dec:        c.Dec,
			Hull:           toSkyPoints(c.Hull),
			Rotation:       c.Rotation,
			RotationSpread: c.RotationSpread,
			PixelScale:     scale,
			Approx:     c.Approx,
			Objects:    objects,
			Filters:    c.Filters,
			Frames:     c.Frames,
			ExpTotal:   c.ExpTotal,
			FirstDate:  c.FirstDate,
			LastDate:   c.LastDate,
			Nights:     c.Nights,
			Paths:      c.Paths,
		})
	}
	return out
}

// coverageScopes lists scopes by frame count, with display labels that drop
// the Seestar serial unless two scopes would then share a label.
func coverageScopes(frames map[string]int) []CoverageScope {
	labelUses := map[string]int{}
	for scope := range frames {
		labelUses[coverage.ScopeLabel(scope)]++
	}
	out := make([]CoverageScope, 0, len(frames))
	for scope, n := range frames {
		label := coverage.ScopeLabel(scope)
		if labelUses[label] > 1 && scope != "" {
			label = scope
		}
		out = append(out, CoverageScope{Scope: scope, Label: label, Frames: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Frames != out[j].Frames {
			return out[i].Frames > out[j].Frames
		}
		return out[i].Scope < out[j].Scope
	})
	return out
}
