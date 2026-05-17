package fits

import (
	"encoding/binary"
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

// ── Pixel normalisation ───────────────────────────────────────────────────────

func TestNormalizeToUnitEmpty(t *testing.T) {
	if got := normalizeToUnit(nil); got != nil {
		t.Errorf("normalizeToUnit(nil) = %v, want nil", got)
	}
	if got := normalizeToUnit([]float64{}); got != nil {
		t.Errorf("normalizeToUnit([]) = %v, want nil", got)
	}
}

func TestNormalizeToUnitUniform(t *testing.T) {
	// All identical values → all zeros (hi <= lo).
	out := normalizeToUnit([]float64{5, 5, 5, 5})
	for i, v := range out {
		if v != 0 {
			t.Errorf("uniform input: out[%d] = %f, want 0", i, v)
		}
	}
}

func TestNormalizeToUnitLinear(t *testing.T) {
	// [0, 1, 2, 3, 4] → [0, 0.25, 0.5, 0.75, 1.0]
	in := []float64{0, 1, 2, 3, 4}
	out := normalizeToUnit(in)
	want := []float64{0, 0.25, 0.5, 0.75, 1.0}
	for i, w := range want {
		if math.Abs(out[i]-w) > 1e-9 {
			t.Errorf("out[%d] = %f, want %f", i, out[i], w)
		}
	}
}

func TestNormalizeToUnitClipsBelow(t *testing.T) {
	// Negative pixel values are clamped to 0 after normalisation.
	out := normalizeToUnit([]float64{-10, 0, 10})
	if out[0] != 0 {
		t.Errorf("expected negative pixel clamped to 0, got %f", out[0])
	}
}

// ── Global normalisation ──────────────────────────────────────────────────────

func TestGlobalNormalizeEmpty(t *testing.T) {
	if got := globalNormalize(nil); got != nil {
		t.Error("globalNormalize(nil) should return nil")
	}
}

func TestGlobalNormalizePreservesColorBalance(t *testing.T) {
	// One bright channel, one dim channel.
	// Per-channel normalisation would amplify the dim channel independently;
	// global normalisation must not do that.
	bright := []float64{0, 1.0}
	dim := []float64{0, 0.5}
	out := globalNormalize([][]float64{bright, dim})

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

// ── Stretch functions ─────────────────────────────────────────────────────────

func TestLinearAutoStretchOutputRange(t *testing.T) {
	pixels := make([]float64, 1000)
	for i := range pixels {
		pixels[i] = float64(i) / 999.0
	}
	out := linearAutoStretch(pixels)
	for i, v := range out {
		if v < 0 || v > 1 {
			t.Errorf("linearAutoStretch: out[%d] = %f outside [0,1]", i, v)
		}
	}
}

func TestAutoStretchOutputRange(t *testing.T) {
	pixels := make([]float64, 1000)
	for i := range pixels {
		pixels[i] = float64(i) / 999.0
	}
	out := autoStretch(pixels, -2.80, 0.25)
	for i, v := range out {
		if v < 0 || v > 1 {
			t.Errorf("autoStretch: out[%d] = %f outside [0,1]", i, v)
		}
	}
}

// ── Scale helpers ─────────────────────────────────────────────────────────────

func TestApplyScale8(t *testing.T) {
	out := applyScale8([]int8{0, 100, -100}, 1.0, 0.0)
	if out[0] != 0 || out[1] != 100 || out[2] != -100 {
		t.Errorf("applyScale8 identity: %v", out)
	}
}

func TestApplyScale16WithOffset(t *testing.T) {
	// scale=1, zero=32768 (typical FITS unsigned short encoding)
	out := applyScale16([]int16{-32768, 0, 32767}, 1.0, 32768.0)
	if out[0] != 0 {
		t.Errorf("out[0] = %f, want 0", out[0])
	}
	if out[1] != 32768 {
		t.Errorf("out[1] = %f, want 32768", out[1])
	}
}

func TestApplyScaleF64Identity(t *testing.T) {
	src := []float64{1.0, 2.0, 3.0}
	out := applyScaleF64(src, 1.0, 0.0)
	// Identity case returns the src slice directly (no allocation).
	if &out[0] != &src[0] {
		t.Error("applyScaleF64 identity should return the same underlying slice")
	}
}

func TestApplyScaleF64WithTransform(t *testing.T) {
	out := applyScaleF64([]float64{1.0, 2.0}, 2.0, 10.0)
	if out[0] != 12.0 || out[1] != 14.0 {
		t.Errorf("applyScaleF64: %v, want [12, 14]", out)
	}
}

// ── Misc helpers ──────────────────────────────────────────────────────────────

func TestToU8(t *testing.T) {
	tests := []struct {
		in   float64
		want uint8
	}{
		{0.0, 0},
		{1.0, 255},
		{0.5, 127},
		{-1.0, 0},
		{2.0, 255},
	}
	for _, tt := range tests {
		if got := toU8(tt.in); got != tt.want {
			t.Errorf("toU8(%f) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestPackF32(t *testing.T) {
	dst := make([]byte, 4)
	packF32(dst, 0, 1.0)

	// IEEE 754: float32(1.0) = 0x3F800000, stored little-endian.
	bits := binary.LittleEndian.Uint32(dst)
	if bits != math.Float32bits(1.0) {
		t.Errorf("packed bits = 0x%08X, want 0x%08X", bits, math.Float32bits(1.0))
	}
}

func TestPackF32Zero(t *testing.T) {
	dst := make([]byte, 4)
	packF32(dst, 0, 0.0)
	for i, b := range dst {
		if b != 0 {
			t.Errorf("packF32(0): byte[%d] = %d, want 0", i, b)
		}
	}
}

func TestPackF32Offset(t *testing.T) {
	// Verify writing at a non-zero offset does not corrupt surrounding bytes.
	dst := make([]byte, 12)
	dst[0] = 0xFF
	dst[1] = 0xFF
	dst[2] = 0xFF
	dst[3] = 0xFF
	packF32(dst, 4, 2.0) // write at offset 4
	dst[8] = 0xFF
	dst[9] = 0xFF
	dst[10] = 0xFF
	dst[11] = 0xFF

	bits := binary.LittleEndian.Uint32(dst[4:8])
	if bits != math.Float32bits(2.0) {
		t.Errorf("offset write: bits = 0x%08X, want 0x%08X", bits, math.Float32bits(2.0))
	}
	// Surrounding bytes must be untouched.
	for i := 0; i < 4; i++ {
		if dst[i] != 0xFF {
			t.Errorf("byte before offset corrupted: dst[%d] = 0x%02X", i, dst[i])
		}
	}
}
