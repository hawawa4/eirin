package fits

import "math"

// ── Linked autostretch ───────────────────────────────────────────────────────
//
// Siril/PixInsight-style MTF autostretch, *linked*: one black point and one
// midtone for every channel (averaged over the channels, as Siril's linked
// mode does), applied after the background-balance gains. Keep in sync with
// computeStretch in frontend/src/lib/stretchPreview.ts.

type stretchPreset struct{ shadows, targetBG float64 }

// stretchPresets are indexed by stretch level; level 0 is linear (black point
// only, no midtone curve and no balance — the camera's raw colour).
var stretchPresets = []stretchPreset{
	{-2.80, 0},    // 0: linear
	{-1.25, 0.10}, // 1: gentle
	{-2.80, 0.25}, // 2: normal (Siril default)
	{-4.00, 0.40}, // 3: strong
}

type stretchParams struct {
	gains    []float64
	shadows  float64
	midtone  float64
	linear   bool
	disabled bool // no usable range: everything maps to black
}

// linkedStretch computes the stretch for `level` from per-channel stats of the
// normalised data and the background-balance gains (ignored at level 0).
func linkedStretch(stats []ChannelStats, gains []float64, level int) stretchParams {
	level = min(max(level, 0), len(stretchPresets)-1)
	pr := stretchPresets[level]
	g := make([]float64, len(stats))
	for i := range g {
		g[i] = 1
		if level > 0 && i < len(gains) && gains[i] > 0 {
			g[i] = gains[i]
		}
	}
	if len(stats) == 0 {
		return stretchParams{gains: g, midtone: 0.5, linear: true}
	}

	var med, c0 float64
	for i, s := range stats {
		m := s.Median * g[i]
		med += m
		c0 += m + pr.shadows*s.Sigma*g[i]
	}
	n := float64(len(stats))
	med /= n
	c0 = max(0, c0/n)

	p := stretchParams{gains: g, shadows: c0, midtone: 0.5, linear: level == 0}
	scale := 1 - c0
	if scale <= 0 {
		p.disabled = true
		return p
	}
	if !p.linear {
		p.midtone = mtfMidtone(pr.targetBG, max(0, med-c0)/scale)
	}
	return p
}

// apply stretches value v of channel c to [0,1].
func (p stretchParams) apply(c int, v float64) float64 {
	if p.disabled {
		return 0
	}
	x := math.Min(1, math.Max(0, (v*p.gains[c]-p.shadows)/(1-p.shadows)))
	if p.linear {
		return x
	}
	return mtf(p.midtone, x)
}

// mtf is the midtones transfer function: MTF(m, x) from PixInsight/Siril.
func mtf(m, x float64) float64 {
	switch {
	case x <= 0:
		return 0
	case x >= 1:
		return 1
	case m <= 0:
		return 0
	case m >= 1:
		return 1
	}
	return (m - 1) * x / ((2*m-1)*x - m)
}

// mtfMidtone solves MTF(m, x) = target for m analytically.
func mtfMidtone(target, x float64) float64 {
	if x == 0 {
		return 0
	}
	d := x*(1-2*target) + target
	if d == 0 {
		return 0
	}
	return x * (1 - target) / d
}
