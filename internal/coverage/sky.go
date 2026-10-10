package coverage

import (
	"math"
	"sort"
)

const deg = math.Pi / 180

// Separation is the angular distance in degrees between two sky positions.
func Separation(ra1, dec1, ra2, dec2 float64) float64 {
	sd := math.Sin((dec2 - dec1) * deg / 2)
	sr := math.Sin((ra2 - ra1) * deg / 2)
	h := sd*sd + math.Cos(dec1*deg)*math.Cos(dec2*deg)*sr*sr
	return 2 * math.Asin(math.Min(1, math.Sqrt(h))) / deg
}

func toVec(ra, dec float64) (x, y, z float64) {
	cd := math.Cos(dec * deg)
	return cd * math.Cos(ra*deg), cd * math.Sin(ra*deg), math.Sin(dec * deg)
}

func fromVec(x, y, z float64) (ra, dec float64) {
	ra = math.Atan2(y, x) / deg
	if ra < 0 {
		ra += 360
	}
	return ra, math.Atan2(z, math.Hypot(x, y)) / deg
}

// Centroid averages sky positions (as unit vectors, so RA 359° and 1° average to 0°).
type Centroid struct {
	x, y, z float64
	n       int
}

// Add includes a position in degrees.
func (c *Centroid) Add(ra, dec float64) {
	x, y, z := toVec(ra, dec)
	c.x += x
	c.y += y
	c.z += z
	c.n++
}

// Position is the mean position; ok is false when nothing was added.
func (c Centroid) Position() (ra, dec float64, ok bool) {
	if c.n == 0 {
		return 0, 0, false
	}
	ra, dec = fromVec(c.x, c.y, c.z)
	return ra, dec, true
}

// toTangent projects (ra, dec) onto the plane tangent at (ra0, dec0)
// (gnomonic; xi East, eta North, in radians). ok is false behind the plane.
func toTangent(ra0, dec0, ra, dec float64) (xi, eta float64, ok bool) {
	d0, d := dec0*deg, dec*deg
	dra := (ra - ra0) * deg
	cosc := math.Sin(d0)*math.Sin(d) + math.Cos(d0)*math.Cos(d)*math.Cos(dra)
	if cosc <= 0 {
		return 0, 0, false
	}
	xi = math.Cos(d) * math.Sin(dra) / cosc
	eta = (math.Cos(d0)*math.Sin(d) - math.Sin(d0)*math.Cos(d)*math.Cos(dra)) / cosc
	return xi, eta, true
}

// fromTangent is the inverse of toTangent.
func fromTangent(ra0, dec0, xi, eta float64) (ra, dec float64) {
	d0 := dec0 * deg
	rho := math.Hypot(xi, eta)
	if rho == 0 {
		return ra0, dec0
	}
	c := math.Atan(rho)
	sc, cc := math.Sin(c), math.Cos(c)
	dec = math.Asin(cc*math.Sin(d0)+eta*sc*math.Cos(d0)/rho) / deg
	ra = ra0 + math.Atan2(xi*sc, rho*math.Cos(d0)*cc-eta*math.Sin(d0)*sc)/deg
	ra = math.Mod(ra+360, 360)
	return ra, dec
}

// Footprint is the outline (4 corners) of a single image: its centre, scale
// (arcsec/pixel), rotation (CROTA2) and size in pixels. Nil when the size or
// scale is unknown.
func Footprint(ra, dec, scale, rotation float64, width, height int) []SkyPoint {
	s := &Sub{RA: ra, Dec: dec, PixelScale: scale}
	return outline(ra, dec, rotation, []*Sub{s}, Field{Width: width, Height: height})
}

// outline is the convex hull of the subs' footprints, centred on (ra0, dec0).
// Footprints use each sub's centre and scale with the scope's sensor size and
// the given rotation, mapped the same way the Sky Atlas maps image pixels
// (u right, v down; CROTA2 rotation with CDELT1 < 0). Nil when the size is
// unknown.
func outline(ra0, dec0, rotation float64, subs []*Sub, f Field) []SkyPoint {
	if f.Width <= 0 || f.Height <= 0 {
		return nil
	}
	hw, hh := float64(f.Width)/2, float64(f.Height)/2
	corners := [4][2]float64{{-hw, -hh}, {hw, -hh}, {hw, hh}, {-hw, hh}}
	r := rotation * deg
	cr, sr := math.Cos(r), math.Sin(r)
	pts := make([][2]float64, 0, 4*len(subs))
	for _, s := range subs {
		scale := s.PixelScale
		if scale <= 0 {
			scale = f.PixelScale
		}
		if scale <= 0 {
			continue
		}
		xi0, eta0, ok := toTangent(ra0, dec0, s.RA, s.Dec)
		if !ok {
			continue
		}
		k := scale / 3600 * deg
		for _, uv := range corners {
			u, v := uv[0], uv[1]
			pts = append(pts, [2]float64{
				xi0 + k*(-cr*u+sr*v),
				eta0 + k*(-sr*u-cr*v),
			})
		}
	}
	hull := convexHull(pts)
	if len(hull) < 3 {
		return nil
	}
	out := make([]SkyPoint, len(hull))
	for i, p := range hull {
		ra, dec := fromTangent(ra0, dec0, p[0], p[1])
		out[i] = SkyPoint{RA: ra, Dec: dec}
	}
	return out
}

// convexHull returns the hull of pts counter-clockwise (Andrew's monotone chain).
func convexHull(pts [][2]float64) [][2]float64 {
	if len(pts) < 3 {
		return pts
	}
	sort.Slice(pts, func(i, j int) bool {
		if pts[i][0] != pts[j][0] {
			return pts[i][0] < pts[j][0]
		}
		return pts[i][1] < pts[j][1]
	})
	cross := func(o, a, b [2]float64) float64 {
		return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
	}
	hull := make([][2]float64, 0, 2*len(pts))
	for _, p := range pts {
		for len(hull) >= 2 && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	lower := len(hull) + 1
	for i := len(pts) - 2; i >= 0; i-- {
		p := pts[i]
		for len(hull) >= lower && cross(hull[len(hull)-2], hull[len(hull)-1], p) <= 0 {
			hull = hull[:len(hull)-1]
		}
		hull = append(hull, p)
	}
	return hull[:len(hull)-1]
}
