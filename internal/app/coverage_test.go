package app

import (
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func ptr(v float64) *float64 { return &v }

func TestGetLightCoverage(t *testing.T) {
	a := newTestApp(t)
	st := a.store()
	solved := func(ra, dec float64) store.Frame {
		return store.Frame{
			FrameType: store.FrameTypeLight, Telescope: "S50_e4256f5d", Object: "M 42",
			ExpTime: 10, DateObs: "2026-01-05T21:00:00",
			RA: ptr(ra), Dec: ptr(dec), PixelScale: ptr(2.37), Rotation: ptr(140),
		}
	}
	must(t, st.UpsertFrame("/nas/M 42_sub/Light_1.fit", solved(83.82, -5.39)))
	// Same framing, different tag: one cluster.
	ic := solved(83.83, -5.40)
	ic.Object = "Running Man"
	must(t, st.UpsertFrame("/nas/RM_sub/Light_1.fit", ic))
	must(t, st.UpsertFrame("/nas/M 42_sub/Light_2.fit", solved(83.82, -5.39)))
	must(t, st.RejectFrame("/nas/M 42_sub/Light_2.fit", "clouds"))
	// No WCS, known object: estimated from the catalog.
	must(t, st.UpsertFrame("/nas/M 100_sub/Light_1.fit", store.Frame{
		FrameType: store.FrameTypeLight, Telescope: "S50 Pro_8dc7900b", Object: "M 100",
	}))
	// No WCS, unknown object: unplaced.
	must(t, st.UpsertFrame("/nas/Comet_sub/Light_1.fit", store.Frame{
		FrameType: store.FrameTypeLight, Telescope: "S50 Pro_8dc7900b", Object: "C/2026 Q1",
	}))
	// Not a light: ignored.
	must(t, st.UpsertFrame("/nas/M 42/Stacked.fit", store.Frame{FrameType: store.FrameTypeStacked, RA: ptr(83.8), Dec: ptr(-5.4)}))

	got, err := a.GetLightCoverage("/nas")
	if err != nil {
		t.Fatalf("GetLightCoverage: %v", err)
	}

	if len(got.Clusters) != 2 {
		t.Fatalf("clusters = %+v, want 2 (M 42 framing + estimated M 100)", got.Clusters)
	}
	var m42, m100 CoverageCluster
	for _, c := range got.Clusters {
		if c.Approx {
			m100 = c
		} else {
			m42 = c
		}
	}
	if m42.Frames != 2 || len(m42.Objects) != 2 || m42.PixelScale != 2.37 {
		t.Errorf("M 42 cluster = %+v, want 2 frames, 2 objects, scale 2.37", m42)
	}
	if m42.ID != "/nas/M 42_sub/Light_1.fit" {
		t.Errorf("ID = %q, want the first path", m42.ID)
	}
	if m100.Frames != 1 || m100.Objects[0].Name != "M 100" || m100.RA == 0 {
		t.Errorf("M 100 cluster = %+v, want 1 estimated frame at the catalog position", m100)
	}

	if len(got.Unplaced) != 1 || got.Unplaced[0].Objects[0].Name != "C/2026 Q1" {
		t.Errorf("unplaced = %+v, want the comet", got.Unplaced)
	}

	if len(got.Scopes) != 2 {
		t.Fatalf("scopes = %+v, want 2", got.Scopes)
	}
	labels := map[string]string{}
	for _, s := range got.Scopes {
		labels[s.Scope] = s.Label
	}
	if labels["S50_e4256f5d"] != "S50" || labels["S50 Pro_8dc7900b"] != "S50 Pro" {
		t.Errorf("labels = %v", labels)
	}
}

func TestCoverageScopes_DisambiguatesSharedLabels(t *testing.T) {
	got := coverageScopes(map[string]int{"S50_aaaa1111": 5, "S50_bbbb2222": 3})
	if got[0].Label != "S50_aaaa1111" || got[1].Label != "S50_bbbb2222" {
		t.Errorf("labels = %+v, want raw names when the short label is shared", got)
	}
}
