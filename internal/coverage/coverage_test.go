package coverage

import (
	"fmt"
	"math"
	"testing"
)

// s50 is roughly a Seestar S50 field: 1080×1920 px at 2.37″/px (≈0.71°×1.26°).
var s50 = Field{Width: 1080, Height: 1920, PixelScale: 2.37}

func sub(path, scope, object string, ra, dec float64) Sub {
	return Sub{
		Path: path, Scope: scope, Object: object, RA: ra, Dec: dec,
		PixelScale: 2.37, ExpTime: 10, DateObs: "2026-01-05T21:00:00",
	}
}

func TestBuild_MergesDifferentTagsWithSameFraming(t *testing.T) {
	subs := []Sub{
		sub("/a/1.fit", "S50", "M 42", 83.82, -5.39),
		sub("/a/2.fit", "S50", "M 42", 83.83, -5.40),
		sub("/b/1.fit", "S50", "Running Man", 83.84, -5.38),
	}
	got := Build(subs, map[string]Field{"S50": s50})
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(got))
	}
	c := got[0]
	if c.Frames != 3 || c.ExpTotal != 30 {
		t.Errorf("frames/exp = %d/%v, want 3/30", c.Frames, c.ExpTotal)
	}
	if len(c.Objects) != 2 || c.Objects[0].Name != "M 42" || c.Objects[0].Count != 2 {
		t.Errorf("objects = %+v, want M 42 (2) first", c.Objects)
	}
}

func TestBuild_SplitsMosaicPanels(t *testing.T) {
	// Two panels 0.6° apart: same object tag, different framings.
	subs := []Sub{
		sub("/m/1.fit", "S50", "Mintaka", 82.5, -1.4),
		sub("/m/2.fit", "S50", "Mintaka", 83.1, -1.4),
		sub("/m/3.fit", "S50", "Mintaka", 82.51, -1.41),
	}
	got := Build(subs, map[string]Field{"S50": s50})
	if len(got) != 2 {
		t.Fatalf("want 2 clusters, got %d", len(got))
	}
	if got[0].Frames != 2 || got[1].Frames != 1 {
		t.Errorf("frames = %d,%d; want 2,1 (sorted by RA)", got[0].Frames, got[1].Frames)
	}
}

func TestBuild_KeepsScopesAndEstimatesApart(t *testing.T) {
	approx := sub("/c/1.fit", "S50", "M 42", 83.82, -5.39)
	approx.Approx = true
	subs := []Sub{
		sub("/a/1.fit", "S50", "M 42", 83.82, -5.39),
		sub("/b/1.fit", "S30", "M 42", 83.82, -5.39),
		approx,
	}
	got := Build(subs, map[string]Field{"S50": s50})
	if len(got) != 3 {
		t.Fatalf("want 3 clusters, got %d", len(got))
	}
	nApprox := 0
	for _, c := range got {
		if c.Approx {
			nApprox++
		}
	}
	if nApprox != 1 {
		t.Errorf("want 1 approximate cluster, got %d", nApprox)
	}
}

func TestBuild_ChainsMosaicModeSession(t *testing.T) {
	// Seestar mosaic mode: every sub lands somewhere else on a grid ~0.15°
	// apart. Overlapping pointings chain into one cluster.
	var subs []Sub
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			subs = append(subs, sub(fmt.Sprintf("/m/%d_%d.fit", i, j), "S50", "M 42",
				83.3+0.15*float64(i), -6.5+0.15*float64(j)))
		}
	}
	got := Build(subs, map[string]Field{"S50": s50})
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(got))
	}
	// The outline spans the whole grid plus half a field each way.
	ext := 0.0
	for _, p := range got[0].Hull {
		ext = math.Max(ext, Separation(got[0].RA, got[0].Dec, p.RA, p.Dec))
	}
	if ext < 0.8 || ext > 1.5 {
		t.Errorf("hull extends %.2f° from the centre, want ≈1°", ext)
	}
}

func TestBuild_HullMatchesSingleFootprint(t *testing.T) {
	s := sub("/a/1.fit", "S50", "M 1", 83.6, 22)
	got := Build([]Sub{s}, map[string]Field{"S50": s50})
	if len(got[0].Hull) != 4 {
		t.Fatalf("hull = %v, want the 4 corners", got[0].Hull)
	}
	// Half-diagonal of a 0.711°×1.264° field.
	want := math.Hypot(0.711, 1.264) / 2
	for _, p := range got[0].Hull {
		if d := Separation(83.6, 22, p.RA, p.Dec); math.Abs(d-want) > 0.005 {
			t.Errorf("corner %v is %.3f° from the centre, want %.3f°", p, d, want)
		}
	}
}

func TestBuild_NoHullWithoutSensorSize(t *testing.T) {
	got := Build([]Sub{sub("/a/1.fit", "S50", "M 1", 83.6, 22)}, nil)
	if got[0].Hull != nil {
		t.Errorf("hull = %v, want none", got[0].Hull)
	}
}

func TestByObject(t *testing.T) {
	got := ByObject([]Sub{
		{Path: "/b", Scope: "S50", Object: "NGC 1"},
		{Path: "/a", Scope: "S50", Object: "NGC 1"},
		{Path: "/c", Scope: "S30", Object: "NGC 1"},
	})
	if len(got) != 2 || got[0].Scope != "S30" || got[1].Frames != 2 || got[1].Paths[0] != "/a" {
		t.Errorf("got %+v", got)
	}
}

func TestBuild_CentroidAcrossRAZero(t *testing.T) {
	got := Build([]Sub{
		sub("/a/1.fit", "S50", "X", 359.95, 10),
		sub("/a/2.fit", "S50", "X", 0.05, 10),
	}, map[string]Field{"S50": s50})
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(got))
	}
	if d := Separation(got[0].RA, got[0].Dec, 0, 10); d > 0.001 {
		t.Errorf("centroid %v,%v is %v° from 0,10", got[0].RA, got[0].Dec, d)
	}
}

func TestBuild_CountsNights(t *testing.T) {
	a := sub("/a/1.fit", "S50", "M 1", 83.6, 22)
	b := sub("/a/2.fit", "S50", "M 1", 83.6, 22)
	c := sub("/a/3.fit", "S50", "M 1", 83.6, 22)
	a.DateObs = "2026-01-05T22:00:00"
	b.DateObs = "2026-01-06T02:00:00" // same night, after UTC midnight
	c.DateObs = "2026-01-07T21:00:00"
	got := Build([]Sub{a, b, c}, nil)
	if got[0].Nights != 2 {
		t.Errorf("nights = %d, want 2", got[0].Nights)
	}
	if got[0].FirstDate != a.DateObs || got[0].LastDate != c.DateObs {
		t.Errorf("dates = %s..%s", got[0].FirstDate, got[0].LastDate)
	}
}

func TestNight(t *testing.T) {
	cases := map[string]string{
		"2026-03-12T20:47:13.908856": "2026-03-12",
		"2026-03-13T03:10:00":        "2026-03-12",
		"2026-03-13":                 "2026-03-13",
		"":                           "",
	}
	for in, want := range cases {
		if got := Night(in); got != want {
			t.Errorf("Night(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScopeLabel(t *testing.T) {
	cases := map[string]string{
		"S50 Pro_8dc7900b": "S50 Pro",
		"S30 Pro_ebe39b47": "S30 Pro",
		"Seestar_S50":      "Seestar_S50",
		"":                 "Unknown scope",
		"RedCat 51":        "RedCat 51",
	}
	for in, want := range cases {
		if got := ScopeLabel(in); got != want {
			t.Errorf("ScopeLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFieldShortSideDeg(t *testing.T) {
	if got := s50.ShortSideDeg(); math.Abs(got-0.711) > 0.001 {
		t.Errorf("short side = %v, want ≈0.711", got)
	}
	if (Field{}).ShortSideDeg() != 0 {
		t.Error("unknown field should give 0")
	}
}

func TestBuild_RevisitsMergeButMosaicsDontSwallowTargets(t *testing.T) {
	var subs []Sub
	// Night 1: a mosaic-mode session over ~0.6°×0.6°.
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			s := sub(fmt.Sprintf("/mosaic/%d_%d.fit", i, j), "S50", "Orion", 83.3+0.15*float64(i), -6.5+0.15*float64(j))
			s.DateObs = "2026-01-05T21:00:00"
			subs = append(subs, s)
		}
	}
	// Nights 2 and 3: a single target inside the mosaic area, re-pointed a little.
	for i, d := range []string{"2026-01-08T21:00:00", "2026-01-09T21:00:00"} {
		s := sub(fmt.Sprintf("/m42/%d.fit", i), "S50", "M 42", 83.6+0.02*float64(i), -6.2)
		s.DateObs = d
		subs = append(subs, s)
	}
	got := Build(subs, map[string]Field{"S50": s50})
	if len(got) != 2 {
		t.Fatalf("want the mosaic and the target as 2 clusters, got %d", len(got))
	}
	for _, c := range got {
		if c.Objects[0].Name == "M 42" && (c.Frames != 2 || c.Nights != 2) {
			t.Errorf("M 42 cluster = %d frames over %d nights, want 2 over 2", c.Frames, c.Nights)
		}
	}
}

func TestBuild_FieldRotationKeepsARectangle(t *testing.T) {
	// Alt-az field rotation: same pointing, rotation drifting 135°→182°.
	var subs []Sub
	for i, rot := range []float64{135, 145, 156, 179, 182} {
		s := sub(fmt.Sprintf("/n/%d.fit", i), "S50", "NGC 1055", 40.6, 0.25)
		s.Rotation = rot
		subs = append(subs, s)
	}
	got := Build(subs, map[string]Field{"S50": s50})
	if len(got) != 1 {
		t.Fatalf("want 1 cluster, got %d", len(got))
	}
	c := got[0]
	if len(c.Hull) != 4 {
		t.Errorf("hull has %d corners, want a rectangle at the mean rotation", len(c.Hull))
	}
	if math.Abs(c.RotationSpread-47) > 0.5 {
		t.Errorf("rotation spread = %.1f°, want 47°", c.RotationSpread)
	}
	if c.Rotation < 150 || c.Rotation > 165 {
		t.Errorf("mean rotation = %.1f°, want ≈159°", c.Rotation)
	}
}

func TestRotationStats_WrapsAt180(t *testing.T) {
	// A meridian flip (0.5° vs 179.5°) is the same rectangle: no spread.
	mean, spread := rotationStats([]*Sub{{Rotation: 0.5}, {Rotation: 179.5}})
	if math.Min(mean, 180-mean) > 0.01 || spread > 1.01 {
		t.Errorf("mean, spread = %.2f, %.2f; want ≈0, 1", mean, spread)
	}
}

func TestFootprint(t *testing.T) {
	got := Footprint(83.6, 22, 2.37, 30, 1080, 1920)
	if len(got) != 4 {
		t.Fatalf("got %v, want 4 corners", got)
	}
	if Footprint(83.6, 22, 2.37, 30, 0, 0) != nil {
		t.Error("unknown size should give no footprint")
	}
}
