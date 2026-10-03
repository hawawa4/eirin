package fits

import (
	"math"
	"testing"
)

// ── Global normalisation ──────────────────────────────────────────────────────

func TestGlobalNormalizeEmpty(t *testing.T) {
	if got, _ := globalNormalize(nil, nil); got != nil {
		t.Error("globalNormalize(nil) should return nil")
	}
}

func TestGlobalNormalizePreservesColorBalance(t *testing.T) {
	// One bright channel, one dim channel.
	// Per-channel normalisation would amplify the dim channel independently;
	// global normalisation must not do that.
	bright := []float64{0, 1.0}
	dim := []float64{0, 0.5}
	out, _ := globalNormalize([][]float64{bright, dim}, nil)

	// Bright channel: max maps to 1.0.
	if math.Abs(out[0][1]-1.0) > 1e-9 {
		t.Errorf("bright channel max = %f, want 1.0", out[0][1])
	}
	// Dim channel max must be < 1.0 (colour balance preserved).
	if out[1][1] >= 1.0 {
		t.Errorf("dim channel max = %f, should be < 1.0 (colour balance)", out[1][1])
	}
}

// ── channelMedianSigma ────────────────────────────────────────────────────────

func TestChannelMedianSigmaKnownValues(t *testing.T) {
	// [0, 0.25, 0.5, 0.75, 1.0] → median = 0.5, sigma = 0.25 * 1.4826
	pixels := []float64{0, 0.25, 0.5, 0.75, 1.0}
	med, sig := channelMedianSigma(pixels)

	if math.Abs(med-0.5) > 1e-9 {
		t.Errorf("median = %f, want 0.5", med)
	}
	wantSig := 0.25 * 1.4826
	if math.Abs(sig-wantSig) > 1e-9 {
		t.Errorf("sigma = %f, want %f", sig, wantSig)
	}
}

func TestChannelMedianSigmaUniform(t *testing.T) {
	pixels := []float64{1, 1, 1, 1, 1}
	med, sig := channelMedianSigma(pixels)
	if med != 1 {
		t.Errorf("median = %f, want 1", med)
	}
	if sig != 0 {
		t.Errorf("sigma = %f, want 0 (uniform)", sig)
	}
}

// ── Debayer ───────────────────────────────────────────────────────────────────

func TestDebayerBlocksRGGB(t *testing.T) {
	// 2×2 Bayer input, RGGB: [R, G / G, B]
	bayer := []float64{1.0, 2.0, 3.0, 4.0}
	r, g, b, w, h := debayerBlocks(bayer, 2, 2, "RGGB")

	if w != 1 || h != 1 {
		t.Errorf("output size = %d×%d, want 1×1", w, h)
	}
	if r[0] != 1.0 {
		t.Errorf("R = %f, want 1.0", r[0])
	}
	if math.Abs(g[0]-2.5) > 1e-9 { // (2+3)/2
		t.Errorf("G = %f, want 2.5", g[0])
	}
	if b[0] != 4.0 {
		t.Errorf("B = %f, want 4.0", b[0])
	}
}

func TestDebayerBlocksBGGR(t *testing.T) {
	// 2×2 BGGR: [B, G / G, R]
	bayer := []float64{1.0, 2.0, 3.0, 4.0}
	r, g, b, w, h := debayerBlocks(bayer, 2, 2, "BGGR")

	if w != 1 || h != 1 {
		t.Fatalf("output size = %d×%d, want 1×1", w, h)
	}
	if b[0] != 1.0 {
		t.Errorf("B = %f, want 1.0", b[0])
	}
	if math.Abs(g[0]-2.5) > 1e-9 {
		t.Errorf("G = %f, want 2.5", g[0])
	}
	if r[0] != 4.0 {
		t.Errorf("R = %f, want 4.0", r[0])
	}
}

func TestDebayerBlocksOutputDimensions(t *testing.T) {
	// 4×4 input → 2×2 output
	bayer := make([]float64, 16)
	_, _, _, w, h := debayerBlocks(bayer, 4, 4, "RGGB")
	if w != 2 || h != 2 {
		t.Errorf("output size = %d×%d, want 2×2", w, h)
	}
}

func TestGlobalNormalizeKeepsBlackAtZero(t *testing.T) {
	// A bright-sky frame never reaches 0, but its black level still is 0:
	// subtracting the minimum would break the channel ratios.
	out, _ := globalNormalize([][]float64{{100, 200, 400}}, nil)
	if math.Abs(out[0][0]-0.25) > 1e-9 || math.Abs(out[0][1]-0.5) > 1e-9 {
		t.Errorf("out = %v, want ratios kept (0.25, 0.5, 1)", out[0])
	}
}

func TestGlobalNormalizeWhitePointAfterBalance(t *testing.T) {
	// Unbalanced values are returned, but the white point is the balanced
	// peak: with gain 4 the dim channel reaches white exactly when balanced.
	out, _ := globalNormalize([][]float64{{0, 0.5}, {0, 0.5}}, []float64{1, 4})
	if math.Abs(out[1][1]*4-1) > 1e-9 {
		t.Errorf("balanced peak = %f, want 1 (white point taken after balance)", out[1][1]*4)
	}
	if math.Abs(out[0][1]-0.25) > 1e-9 {
		t.Errorf("unbalanced value = %f, want 0.25", out[0][1])
	}
}

func TestNormRangeStatsMatchNormalisedData(t *testing.T) {
	ch := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 1000}
	med, sig := channelMedianSigma(ch)
	out, nr := globalNormalize([][]float64{ch}, nil)
	gotMed, gotSig := channelMedianSigma(out[0])
	want := nr.stats(ChannelStats{Median: med, Sigma: sig})
	if math.Abs(want.Median-gotMed) > 1e-12 || math.Abs(want.Sigma-gotSig) > 1e-12 {
		t.Errorf("carried stats %+v, measured median %f sigma %f", want, gotMed, gotSig)
	}
}

// ── Balance, chroma & resampling ──────────────────────────────────────────────

func TestBalanceGains(t *testing.T) {
	g := balanceGains([]ChannelStats{{Median: 0.2}, {Median: 0.4}, {Median: 0.1}})
	want := []float64{2, 1, 4}
	for i := range want {
		if math.Abs(g[i]-want[i]) > 1e-9 {
			t.Errorf("gain[%d] = %f, want %f", i, g[i], want[i])
		}
	}
}

func TestBalanceGainsMonoAndEdgeCases(t *testing.T) {
	if g := balanceGains([]ChannelStats{{Median: 0.3}}); g[0] != 1 {
		t.Errorf("mono gain = %f, want 1", g[0])
	}
	g := balanceGains([]ChannelStats{{Median: 0.5}, {Median: 0}, {Median: 0.001}})
	if g[1] != 1 {
		t.Errorf("empty channel gain = %f, want 1", g[1])
	}
	if g[2] != maxBalanceGain {
		t.Errorf("dim channel gain = %f, want capped at %f", g[2], maxBalanceGain)
	}
}

func TestBoxBlurKeepsConstant(t *testing.T) {
	src := make([]float64, 5*4)
	for i := range src {
		src[i] = 3
	}
	for i, v := range boxBlur(src, 5, 4, 2) {
		if math.Abs(v-3) > 1e-12 {
			t.Fatalf("out[%d] = %f, want 3", i, v)
		}
	}
}

func TestBoxBlurAveragesNeighbourhood(t *testing.T) {
	// A single 9 in the middle of a 5×5 zero image, radius 1 → 1 over a 3×3 block.
	src := make([]float64, 25)
	src[12] = 9
	out := boxBlur(src, 5, 5, 1)
	for y := range 5 {
		for x := range 5 {
			want := 0.0
			if x >= 1 && x <= 3 && y >= 1 && y <= 3 {
				want = 1
			}
			if math.Abs(out[y*5+x]-want) > 1e-12 {
				t.Errorf("out(%d,%d) = %f, want %f", x, y, out[y*5+x], want)
			}
		}
	}
}

func TestReduceChromaKeepsLuminanceAndSmoothsColour(t *testing.T) {
	// Checkerboard colour noise on a flat grey: luminance is unchanged and the
	// per-pixel colour difference shrinks.
	const w, h = 8, 8
	r, g, b := make([]float64, w*h), make([]float64, w*h), make([]float64, w*h)
	for i := range r {
		d := 0.1
		if (i/w+i%w)%2 == 0 {
			d = -0.1
		}
		r[i], g[i], b[i] = 0.5+d, 0.5, 0.5-d
	}
	out := reduceChroma([][]float64{r, g, b}, w, h, 1)
	maxChroma := 0.0
	for i := range w * h {
		lum := (out[0][i] + out[1][i] + out[2][i]) / 3
		if math.Abs(lum-0.5) > 1e-12 {
			t.Fatalf("luminance at %d = %f, want 0.5", i, lum)
		}
		maxChroma = max(maxChroma, math.Abs(out[0][i]-out[2][i]))
	}
	if maxChroma > 0.1 {
		t.Errorf("max R-B difference = %f, want well below the original 0.2", maxChroma)
	}
}

func TestFitSize(t *testing.T) {
	cases := []struct{ w, h, max, ww, wh int }{
		{1080, 1920, 2048, 1080, 1920},
		{1080, 1920, 1024, 576, 1024},
		{4000, 2000, 1000, 1000, 500},
		{100, 100, 0, 100, 100},
	}
	for _, c := range cases {
		if w, h := fitSize(c.w, c.h, c.max); w != c.ww || h != c.wh {
			t.Errorf("fitSize(%d,%d,%d) = %d×%d, want %d×%d", c.w, c.h, c.max, w, h, c.ww, c.wh)
		}
	}
}

func TestDownsampleAreaAverages(t *testing.T) {
	src := []float64{
		1, 3, 0, 0,
		5, 7, 0, 4,
		2, 2, 1, 1,
		2, 2, 1, 1,
	}
	got := downsampleArea(src, 4, 4, 2, 2)
	want := []float64{4, 1, 2, 1}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-12 {
			t.Errorf("out[%d] = %f, want %f", i, got[i], want[i])
		}
	}
}
