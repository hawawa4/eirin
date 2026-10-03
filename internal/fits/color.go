package fits

import (
	"math"
	"slices"
)

// ── Colour & resampling for previews ─────────────────────────────────────────
//
// Raw one-shot-colour subs (e.g. Seestar) have no white balance and a strong
// sky cast. Stretching each channel on its own noise level (Siril's "unlinked"
// autostretch) makes anything that varies across the frame — vignetting, sky
// gradients, nebulae — take the colour of the channel with the best SNR
// (green under IRCUT, red under the LP filter). Instead the preview pipeline:
//
//  1. averages chroma noise away (reduceChroma), luminance untouched;
//  2. neutralises the sky by scaling each channel to the same background
//     (balanceGains), assuming a black level of 0 — Seestar subs are already
//     bias-subtracted, so the median is pure sky;
//  3. applies one linked stretch to all channels (stretch.go).

// chromaRadius is the box radius (in debayered pixels) used to smooth colour
// noise in raw subs: 5×5 removes the red/blue speckle without visibly
// bleeding star colours.
const chromaRadius = 2

// maxBalanceGain caps background neutralisation so a nearly empty channel
// can't be amplified into pure noise.
const maxBalanceGain = 16.0

// debayerBlocks demosaics a single-plane Bayer image using 2×2 block averaging.
// Each 2×2 super-pixel becomes one RGB output pixel (output is w/2 × h/2).
// Supported patterns: RGGB, BGGR, GRBG, GBRG (defaults to RGGB).
func debayerBlocks(bayer []float64, w, h int, pat string) (r, g, b []float64, outW, outH int) {
	outW, outH = w/2, h/2
	size := outW * outH
	r = make([]float64, size)
	g = make([]float64, size)
	b = make([]float64, size)

	for by := 0; by < outH; by++ {
		y0 := by * 2
		y1 := y0 + 1
		if y1 >= h {
			y1 = y0
		}
		for bx := 0; bx < outW; bx++ {
			x0 := bx * 2
			x1 := x0 + 1
			if x1 >= w {
				x1 = x0
			}
			// 2×2 block: a=top-left, bv=top-right, c=bottom-left, d=bottom-right
			a := bayer[y0*w+x0]
			bv := bayer[y0*w+x1]
			c := bayer[y1*w+x0]
			d := bayer[y1*w+x1]

			idx := by*outW + bx
			switch pat {
			case "BGGR": // B G / G R
				b[idx] = a
				g[idx] = (bv + c) / 2
				r[idx] = d
			case "GRBG": // G R / B G
				g[idx] = (a + d) / 2
				r[idx] = bv
				b[idx] = c
			case "GBRG": // G B / R G
				g[idx] = (a + d) / 2
				b[idx] = bv
				r[idx] = c
			default: // RGGB: R G / G B
				r[idx] = a
				g[idx] = (bv + c) / 2
				b[idx] = d
			}
		}
	}
	return
}

// reduceChroma smooths colour noise: each channel becomes the luminance
// (channel mean) plus a box-blurred difference from it, so detail and noise
// in brightness are kept and only the per-pixel colour is averaged. Modifies
// and returns channels.
func reduceChroma(channels [][]float64, w, h, radius int) [][]float64 {
	if len(channels) < 2 || radius <= 0 {
		return channels
	}
	n := len(channels[0])
	lum := make([]float64, n)
	for _, ch := range channels {
		for i, v := range ch {
			lum[i] += v
		}
	}
	inv := 1 / float64(len(channels))
	for i := range lum {
		lum[i] *= inv
	}
	diff := make([]float64, n)
	for _, ch := range channels {
		for i, v := range ch {
			diff[i] = v - lum[i]
		}
		smooth := boxBlur(diff, w, h, radius)
		for i := range ch {
			ch[i] = lum[i] + smooth[i]
		}
	}
	return channels
}

// boxBlur averages each pixel over a (2r+1)² square, clamping at the edges.
// Separable running sums make it O(pixels) whatever the radius.
func boxBlur(src []float64, w, h, r int) []float64 {
	n := float64(2*r + 1)
	tmp := make([]float64, len(src))
	for y := 0; y < h; y++ {
		row := src[y*w : (y+1)*w]
		var sum float64
		for k := -r; k <= r; k++ {
			sum += row[clampIdx(k, w)]
		}
		for x := 0; x < w; x++ {
			tmp[y*w+x] = sum / n
			sum += row[clampIdx(x+r+1, w)] - row[clampIdx(x-r, w)]
		}
	}
	out := make([]float64, len(src))
	for x := 0; x < w; x++ {
		var sum float64
		for k := -r; k <= r; k++ {
			sum += tmp[clampIdx(k, h)*w+x]
		}
		for y := 0; y < h; y++ {
			out[y*w+x] = sum / n
			sum += tmp[clampIdx(y+r+1, h)*w+x] - tmp[clampIdx(y-r, h)*w+x]
		}
	}
	return out
}

func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// fitSize scales (w, h) down to fit within maxSize, keeping the aspect ratio.
func fitSize(w, h, maxSize int) (int, int) {
	if maxSize <= 0 || (w <= maxSize && h <= maxSize) {
		return w, h
	}
	if w >= h {
		return maxSize, max(1, h*maxSize/w)
	}
	return max(1, w*maxSize/h), maxSize
}

// downsampleArea shrinks an image by averaging every source pixel that falls
// in each output pixel. Unlike nearest-neighbour sampling this lowers noise
// instead of aliasing it.
func downsampleArea(src []float64, w, h, outW, outH int) []float64 {
	if outW == w && outH == h {
		return src
	}
	out := make([]float64, outW*outH)
	for oy := 0; oy < outH; oy++ {
		y0 := oy * h / outH
		y1 := max(y0+1, (oy+1)*h/outH)
		for ox := 0; ox < outW; ox++ {
			x0 := ox * w / outW
			x1 := max(x0+1, (ox+1)*w/outW)
			var sum float64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					sum += src[y*w+x]
				}
			}
			out[oy*outW+ox] = sum / float64((y1-y0)*(x1-x0))
		}
	}
	return out
}

// globalNormalize maps all channels to [0,1] with ONE shared range, so the
// ratios between channels (the camera's raw colour) are preserved. The black
// point is 0 for non-negative data — subtracting the minimum would break the
// background neutralisation in balanceGains. The white point is the 99.9th
// percentile across all channels *after* multiplying by gains (nil = 1), so
// balanced channels reach white together and star cores stay white instead of
// one channel clipping early. The returned range maps an original value v to
// (v - lo) / scale, so statistics taken before normalising can be carried over.
func globalNormalize(channels [][]float64, gains []float64) ([][]float64, normRange) {
	identity := normRange{lo: 0, scale: 1}
	if len(channels) == 0 {
		return channels, identity
	}

	totalLen := 0
	for _, ch := range channels {
		totalLen += len(ch)
	}

	// Sub-sample for statistics (65k points per channel is plenty).
	maxSamples := 65536 * len(channels)
	step := max(1, totalLen/maxSamples)
	sample := make([]float64, 0, min(totalLen, maxSamples))
	i := 0
	lo := 0.0
	for c, ch := range channels {
		g := 1.0
		if c < len(gains) && gains[c] > 0 {
			g = gains[c]
		}
		for _, v := range ch {
			lo = min(lo, v)
			if i%step == 0 {
				sample = append(sample, v*g)
			}
			i++
		}
	}

	slices.Sort(sample)
	hi := sample[int(float64(len(sample))*0.999)]

	if hi <= lo {
		return channels, identity
	}
	rng := hi - lo

	out := make([][]float64, len(channels))
	for c, ch := range channels {
		norm := make([]float64, len(ch))
		for j, v := range ch {
			norm[j] = math.Min(1, math.Max(0, (v-lo)/rng))
		}
		out[c] = norm
	}
	return out, normRange{lo: lo, scale: rng}
}

// normRange is the linear map globalNormalize applied: v → (v - lo) / scale.
type normRange struct{ lo, scale float64 }

// stats maps statistics of the original values onto the normalised ones.
// Exact for unclipped values; the median and MAD of sky-dominated frames
// are far from both clip points.
func (r normRange) stats(s ChannelStats) ChannelStats {
	return ChannelStats{Median: (s.Median - r.lo) / r.scale, Sigma: s.Sigma / r.scale}
}

// channelMedianSigma returns the median and MAD-based sigma for a pixel array.
func channelMedianSigma(pixels []float64) (median, sigma float64) {
	sample := pixels
	if len(pixels) > 65536 {
		step := len(pixels) / 65536
		s := make([]float64, 0, 65536)
		for i := 0; i < len(pixels); i += step {
			s = append(s, pixels[i])
		}
		sample = s
	}

	sorted := slices.Clone(sample)
	slices.Sort(sorted)
	median = sorted[len(sorted)/2]

	devs := make([]float64, len(sorted))
	for i, v := range sorted {
		devs[i] = math.Abs(v - median)
	}
	slices.Sort(devs)
	sigma = devs[len(devs)/2] * 1.4826
	return
}

// balanceGains returns per-channel gains that lift every channel's background
// (median) to the brightest one, neutralising the sky colour. Mono images and
// channels without a usable background get a gain of 1.
func balanceGains(stats []ChannelStats) []float64 {
	gains := make([]float64, len(stats))
	ref := 0.0
	for _, s := range stats {
		ref = max(ref, s.Median)
	}
	for i, s := range stats {
		gains[i] = 1
		if len(stats) > 1 && s.Median > 0 {
			gains[i] = min(ref/s.Median, maxBalanceGain)
		}
	}
	return gains
}
