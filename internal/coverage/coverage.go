// Package coverage groups light frames by framing, so the Coverage view can
// show what sky each telescope has covered regardless of the OBJECT tag the
// frames were captured under.
//
// Per scope and night, subs are gathered into pointings (subs a few percent of
// a field apart), and pointings whose fields largely overlap are chained into
// sessions: a drifting session or a Seestar mosaic-mode session (every sub at a
// different spot) becomes one. Sessions from different nights then merge when
// they're centred on the same spot with a similar spread, so a re-shot framing
// is one cluster, while a wide mosaic doesn't swallow the single-target
// sessions inside it, and mosaic panels with gaps between them stay apart.
package coverage

import (
	"math"
	"sort"
)

// PointingFraction is how close (as a fraction of the field's short side) a
// sub must be to a pointing's centre to join it.
const PointingFraction = 0.1

// LinkFraction is how close two pointings from the same night must be (as a
// fraction of the short side) to chain them into one session.
const LinkFraction = 0.5

// RevisitFraction is how close two sessions' centres must be (as a fraction of
// the short side) to count as the same framing shot on different nights.
const RevisitFraction = 0.2

// SpreadTolerance is how much two sessions' spreads (RMS distance of their
// subs from the centre) may differ, as a fraction of the short side, to merge.
const SpreadTolerance = 0.15

// fallbackShortSideDeg stands in for the field of scopes whose size is unknown.
const fallbackShortSideDeg = 0.7

// Sub is one light frame as the clusterer sees it.
type Sub struct {
	Path    string
	Object  string
	Filter  string
	DateObs string
	ExpTime float64
	// Scope is the raw TELESCOP value; subs are only clustered with their own scope.
	Scope string
	// RA/Dec (degrees) of the frame centre. For Approx subs this is the catalog
	// position of the object, not a measurement.
	RA, Dec    float64
	PixelScale float64 // arcsec/pixel; 0 when unknown (the scope's is used)
	Rotation   float64 // degrees, CROTA2
	// Approx marks a position estimated from the OBJECT name (no WCS).
	Approx bool
}

// Field is a scope's sensor geometry.
type Field struct {
	Width, Height int     // pixels
	PixelScale    float64 // arcsec/pixel (typical)
}

// ShortSideDeg is the angular size of the field's short side, or 0 when unknown.
func (f Field) ShortSideDeg() float64 {
	if f.Width <= 0 || f.Height <= 0 || f.PixelScale <= 0 {
		return 0
	}
	return float64(min(f.Width, f.Height)) * f.PixelScale / 3600
}

// ObjectCount is one OBJECT tag within a cluster.
type ObjectCount struct {
	Name  string
	Count int
}

// SkyPoint is a position in degrees.
type SkyPoint struct {
	RA, Dec float64
}

// Cluster is a group of subs sharing a framing.
type Cluster struct {
	Scope   string
	RA, Dec float64 // centroid of the sub centres
	// Hull is the convex outline (counter-clockwise on the sky) of all the
	// subs' footprints; empty when the scope's sensor size is unknown.
	Hull       []SkyPoint
	PixelScale float64 // mean of the subs that have one
	Approx     bool
	Objects    []ObjectCount // most subs first
	Filters    []string
	Frames     int
	ExpTotal   float64 // seconds
	FirstDate  string
	LastDate   string
	Nights     int
	Paths      []string // sorted
}

// group accumulates subs while they're clustered.
type group struct {
	scope  string
	approx bool
	// Sum of unit vectors; its direction is the centroid.
	x, y, z  float64
	scaleSum float64
	scaleN   int
	subs     []*Sub
}

func (g *group) centre() (ra, dec float64) {
	return fromVec(g.x, g.y, g.z)
}

func (g *group) add(s *Sub) {
	x, y, z := toVec(s.RA, s.Dec)
	g.x += x
	g.y += y
	g.z += z
	if s.PixelScale > 0 {
		g.scaleSum += s.PixelScale
		g.scaleN++
	}
	g.subs = append(g.subs, s)
}

func (g *group) absorb(o *group) {
	g.x += o.x
	g.y += o.y
	g.z += o.z
	g.scaleSum += o.scaleSum
	g.scaleN += o.scaleN
	g.subs = append(g.subs, o.subs...)
}

// Build clusters subs by scope and pointing. fields gives each scope's
// geometry (unknown scopes get no hull and a nominal clustering distance).
// Estimated positions are never mixed with measured ones. Clusters come back
// ordered by scope, then RA.
func Build(subs []Sub, fields map[string]Field) []Cluster {
	type key struct {
		scope  string
		approx bool
	}
	parts := map[key][]*Sub{}
	for i := range subs {
		s := &subs[i]
		k := key{s.Scope, s.Approx}
		parts[k] = append(parts[k], s)
	}

	var out []Cluster
	for k, part := range parts {
		f := fields[k.scope]
		side := f.ShortSideDeg()
		if side == 0 {
			side = fallbackShortSideDeg
		}
		for _, g := range clusterPart(part, side) {
			out = append(out, finish(g, f))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		if a.RA != b.RA {
			return a.RA < b.RA
		}
		return a.Paths[0] < b.Paths[0]
	})
	return out
}

// clusterPart clusters one scope's subs; side is the field's short side in degrees.
func clusterPart(subs []*Sub, side float64) []*group {
	nights := map[string][]*Sub{}
	for _, s := range subs {
		n := Night(s.DateObs)
		nights[n] = append(nights[n], s)
	}
	var sessions []*group
	for _, ns := range nights {
		sessions = append(sessions, linkPointings(groupPointings(ns, side*PointingFraction), side*LinkFraction)...)
	}
	return mergeRevisits(sessions, side*RevisitFraction, side*SpreadTolerance)
}

// mergeRevisits merges sessions, oldest first, into the nearest cluster whose
// centre is within radius and whose first session's spread is within tol of
// theirs.
func mergeRevisits(sessions []*group, radius, tol float64) []*group {
	type session struct {
		g      *group
		first  string
		spread float64
	}
	ss := make([]session, len(sessions))
	for i, g := range sessions {
		ss[i] = session{g: g, first: g.firstKey(), spread: g.spread()}
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].first < ss[j].first })

	type cluster struct {
		g      *group
		spread float64
	}
	var cs []*cluster
	for _, s := range ss {
		ra, dec := s.g.centre()
		var best *cluster
		bestD := radius
		for _, c := range cs {
			if math.Abs(c.spread-s.spread) > tol {
				continue
			}
			cra, cdec := c.g.centre()
			if d := Separation(cra, cdec, ra, dec); d <= bestD {
				best, bestD = c, d
			}
		}
		if best == nil {
			cs = append(cs, &cluster{g: s.g, spread: s.spread})
			continue
		}
		best.g.absorb(s.g)
	}
	out := make([]*group, len(cs))
	for i, c := range cs {
		out[i] = c.g
	}
	return out
}

// firstKey orders groups by their earliest sub (date, then path).
func (g *group) firstKey() string {
	first := ""
	for _, s := range g.subs {
		if k := s.DateObs + "\x00" + s.Path; first == "" || k < first {
			first = k
		}
	}
	return first
}

// spread is the RMS distance (degrees) of the subs' centres from the group's centre.
func (g *group) spread() float64 {
	ra, dec := g.centre()
	sum := 0.0
	for _, s := range g.subs {
		d := Separation(ra, dec, s.RA, s.Dec)
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(g.subs)))
}

// groupPointings assigns each sub, in capture order (so slow drift follows the
// running centre), to the nearest pointing within radius degrees.
func groupPointings(subs []*Sub, radius float64) []*group {
	sort.Slice(subs, func(i, j int) bool {
		if subs[i].DateObs != subs[j].DateObs {
			return subs[i].DateObs < subs[j].DateObs
		}
		return subs[i].Path < subs[j].Path
	})
	var gs []*group
	for _, s := range subs {
		var best *group
		bestD := radius
		for _, g := range gs {
			ra, dec := g.centre()
			if d := Separation(ra, dec, s.RA, s.Dec); d <= bestD {
				best, bestD = g, d
			}
		}
		if best == nil {
			best = &group{scope: s.Scope, approx: s.Approx}
			gs = append(gs, best)
		}
		best.add(s)
	}
	return gs
}

// linkPointings chains pointings whose centres are within radius degrees of
// each other (single linkage) and merges each chain into one group.
func linkPointings(gs []*group, radius float64) []*group {
	parent := make([]int, len(gs))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}

	type pos struct{ ra, dec float64 }
	ps := make([]pos, len(gs))
	order := make([]int, len(gs))
	for i, g := range gs {
		ps[i].ra, ps[i].dec = g.centre()
		order[i] = i
	}
	// Sweep in Dec so only pairs within radius in Dec are compared.
	sort.Slice(order, func(a, b int) bool { return ps[order[a]].dec < ps[order[b]].dec })
	for a, i := range order {
		for _, j := range order[a+1:] {
			if ps[j].dec-ps[i].dec > radius {
				break
			}
			if Separation(ps[i].ra, ps[i].dec, ps[j].ra, ps[j].dec) <= radius {
				parent[find(i)] = find(j)
			}
		}
	}

	roots := map[int]*group{}
	var out []*group
	for i, g := range gs {
		r := find(i)
		if root, ok := roots[r]; ok {
			root.absorb(g)
			continue
		}
		merged := &group{scope: g.scope, approx: g.approx}
		merged.absorb(g)
		roots[r] = merged
		out = append(out, merged)
	}
	return out
}

// ByObject groups subs that have no position at all by scope and OBJECT tag,
// ordered by scope then object. The clusters have no position or hull.
func ByObject(subs []Sub) []Cluster {
	type key struct{ scope, object string }
	groups := map[key]*group{}
	var keys []key
	for i := range subs {
		s := &subs[i]
		k := key{s.Scope, s.Object}
		g, ok := groups[k]
		if !ok {
			g = &group{scope: s.Scope, approx: true}
			groups[k] = g
			keys = append(keys, k)
		}
		g.subs = append(g.subs, s)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].scope != keys[j].scope {
			return keys[i].scope < keys[j].scope
		}
		return keys[i].object < keys[j].object
	})
	out := make([]Cluster, 0, len(keys))
	for _, k := range keys {
		c := summarize(groups[k])
		out = append(out, c)
	}
	return out
}

// finish summarizes a group and computes its position and outline.
func finish(g *group, f Field) Cluster {
	c := summarize(g)
	c.RA, c.Dec = g.centre()
	if g.scaleN > 0 {
		c.PixelScale = g.scaleSum / float64(g.scaleN)
	}
	c.Hull = outline(c.RA, c.Dec, g.subs, f)
	return c
}

// summarize fills in everything but position and outline.
func summarize(g *group) Cluster {
	c := Cluster{Scope: g.scope, Approx: g.approx, Frames: len(g.subs)}
	objects := map[string]int{}
	filters := map[string]bool{}
	nights := map[string]bool{}
	c.Paths = make([]string, 0, len(g.subs))
	for _, s := range g.subs {
		c.Paths = append(c.Paths, s.Path)
		c.ExpTotal += s.ExpTime
		objects[s.Object]++
		if s.Filter != "" {
			filters[s.Filter] = true
		}
		if s.DateObs == "" {
			continue
		}
		if c.FirstDate == "" || s.DateObs < c.FirstDate {
			c.FirstDate = s.DateObs
		}
		if s.DateObs > c.LastDate {
			c.LastDate = s.DateObs
		}
		if n := Night(s.DateObs); n != "" {
			nights[n] = true
		}
	}
	sort.Strings(c.Paths)
	for name, n := range objects {
		c.Objects = append(c.Objects, ObjectCount{Name: name, Count: n})
	}
	sort.Slice(c.Objects, func(i, j int) bool {
		a, b := c.Objects[i], c.Objects[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Name < b.Name
	})
	for name := range filters {
		c.Filters = append(c.Filters, name)
	}
	sort.Strings(c.Filters)
	c.Nights = len(nights)
	return c
}
