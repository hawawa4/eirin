package app

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/TaruDesigns/eirin/internal/catalog"
	"github.com/TaruDesigns/eirin/internal/coverage"
	"github.com/TaruDesigns/eirin/internal/fits"
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
	Hull       []SkyPoint `json:"hull"`
	PixelScale float64    `json:"pixelScale"` // arcsec/pixel
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
}

// fieldCache remembers each scope's sensor size, read once from a FITS header.
type fieldCache struct {
	mu    sync.Mutex
	sizes map[string][2]int
}

func (c *fieldCache) get(scope string) ([2]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sz, ok := c.sizes[scope]
	return sz, ok
}

func (c *fieldCache) put(scope string, sz [2]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sizes == nil {
		c.sizes = map[string][2]int{}
	}
	c.sizes[scope] = sz
}

// maxSizeProbes caps the headers read per scope when earlier ones fail.
const maxSizeProbes = 3

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

	scales := medianScales(frames)
	estimate := a.objectEstimator(rootPath, frames)

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
		Scopes:   coverageScopes(scopeFrames),
	}
	return out, nil
}

// objectEstimator returns a function that estimates where an OBJECT tag
// points: the mean centre of the measured lights with that tag, else of the
// stacked/processed frames with it, else its catalog position. It reports
// whether it found one.
func (a *App) objectEstimator(rootPath string, lights []store.Frame) func(object string, ra, dec *float64) bool {
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
	if stacks, err := a.store().GetAtlasIndexFrames(rootPath); err == nil {
		addAll(stacks)
	} else {
		slog.Warn("coverage: get stacked frames", "err", err)
	}

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

// scopeSize returns the scope's sensor size, reading the first readable FITS
// header among paths on a cache miss. Zero when none could be read.
func (a *App) scopeSize(scope string, paths []string) [2]int {
	if sz, ok := a.fields.get(scope); ok {
		return sz
	}
	for _, p := range paths {
		hdr, err := fits.ReadFITSHeader(p)
		if err != nil || hdr.Width <= 0 || hdr.Height <= 0 {
			slog.Warn("coverage: read sensor size", "path", p, "err", err)
			continue
		}
		sz := [2]int{hdr.Width, hdr.Height}
		a.fields.put(scope, sz)
		return sz
	}
	return [2]int{}
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
		hull := make([]SkyPoint, len(c.Hull))
		for i, p := range c.Hull {
			hull[i] = SkyPoint(p)
		}
		out = append(out, CoverageCluster{
			ID:         c.Paths[0],
			Scope:      c.Scope,
			RA:         c.RA,
			Dec:        c.Dec,
			Hull:       hull,
			PixelScale: scale,
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
