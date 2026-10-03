package fits

import (
	"math"
	"testing"
)

// ── MTF math ──────────────────────────────────────────────────────────────────

func TestMtfEdgeCases(t *testing.T) {
	// Boundary inputs map directly to boundary outputs.
	if v := mtf(0.5, 0); v != 0 {
		t.Errorf("mtf(0.5, 0) = %f, want 0", v)
	}
	if v := mtf(0.5, 1); v != 1 {
		t.Errorf("mtf(0.5, 1) = %f, want 1", v)
	}
	if v := mtf(0, 0.5); v != 0 {
		t.Errorf("mtf(0, 0.5) = %f, want 0", v)
	}
	if v := mtf(1, 0.5); v != 1 {
		t.Errorf("mtf(1, 0.5) = %f, want 1", v)
	}
}

func TestMtfSymmetry(t *testing.T) {
	// MTF(0.5, 0.5) == 0.5 — midpoint is its own midtone.
	if v := mtf(0.5, 0.5); math.Abs(v-0.5) > 1e-12 {
		t.Errorf("mtf(0.5, 0.5) = %f, want 0.5", v)
	}
}

func TestMtfFormula(t *testing.T) {
	// Verify the formula: (m-1)*x / ((2m-1)*x - m)
	m, x := 0.3, 0.6
	want := (m-1)*x / ((2*m-1)*x - m)
	if v := mtf(m, x); math.Abs(v-want) > 1e-12 {
		t.Errorf("mtf(%f, %f) = %f, want %f", m, x, v, want)
	}
}

func TestMtfMidtoneInverse(t *testing.T) {
	// mtf(mtfMidtone(target, x), x) should equal target.
	target, x := 0.25, 0.10
	m := mtfMidtone(target, x)
	got := mtf(m, x)
	if math.Abs(got-target) > 1e-12 {
		t.Errorf("mtf(mtfMidtone(%f, %f), %f) = %f, want %f", target, x, x, got, target)
	}
}

func TestMtfMidtoneZeroX(t *testing.T) {
	// x=0 → m=0 (no valid midtone)
	if v := mtfMidtone(0.25, 0); v != 0 {
		t.Errorf("mtfMidtone(0.25, 0) = %f, want 0", v)
	}
}

// ── Linked stretch ────────────────────────────────────────────────────────────

func TestLinkedStretchNeutralisesBackground(t *testing.T) {
	// A sky with a strong colour cast: after balancing, every channel's
	// background lands on the same grey (the normal preset's 0.25 target).
	stats := []ChannelStats{{0.30, 0.020}, {0.15, 0.008}, {0.10, 0.012}}
	p := linkedStretch(stats, balanceGains(stats), 2)
	for c, s := range stats {
		if v := p.apply(c, s.Median); math.Abs(v-0.25) > 1e-9 {
			t.Errorf("channel %d background = %f, want 0.25", c, v)
		}
	}
}

func TestLinkedStretchLinearIgnoresBalance(t *testing.T) {
	stats := []ChannelStats{{0.30, 0.02}, {0.15, 0.01}, {0.10, 0.01}}
	p := linkedStretch(stats, []float64{0.5, 2, 3}, 0)
	if !p.linear {
		t.Fatal("level 0 should be linear")
	}
	for c, g := range p.gains {
		if g != 1 {
			t.Errorf("gain[%d] = %f, want 1 at level 0", c, g)
		}
	}
}

func TestLinkedStretchOutputRange(t *testing.T) {
	stats := []ChannelStats{{0.2, 0.05}}
	for level := range len(stretchPresets) + 1 {
		p := linkedStretch(stats, nil, level)
		for i := 0; i <= 100; i++ {
			if v := p.apply(0, float64(i)/100); v < 0 || v > 1 {
				t.Fatalf("level %d: apply(%f) = %f outside [0,1]", level, float64(i)/100, v)
			}
		}
	}
}

func TestLinkedStretchNoRange(t *testing.T) {
	// Background clipped at or above white: nothing sensible to show.
	p := linkedStretch([]ChannelStats{{1, 0}}, nil, 2)
	if v := p.apply(0, 1); v != 0 {
		t.Errorf("apply = %f, want 0 when the stretch has no range", v)
	}
}
