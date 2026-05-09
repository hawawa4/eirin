package fits

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sort"

	fitsio "codeberg.org/astrogo/fitsio"
)

// FITSHeader holds FITS metadata relevant to astrophotography.
// Multiple keyword aliases are tried for each field (e.g. EXPTIME vs EXPOSURE)
// to cover different camera conventions.
type FITSHeader struct {
	Width      int                    `json:"width"`
	Height     int                    `json:"height"`
	Channels   int                    `json:"channels"`
	BitPix     int                    `json:"bitpix"`
	Object     string                 `json:"object"`
	Telescope  string                 `json:"telescope"`
	Instrument string                 `json:"instrument"`
	Filter     string                 `json:"filter"`
	ExpTime    float64                `json:"exptime"`
	DateObs    string                 `json:"dateObs"`
	Gain       float64                `json:"gain"`
	Offset     float64                `json:"offset"`
	CCDTemp    float64                `json:"ccdTemp"`
	RA         float64                `json:"ra"`
	Dec        float64                `json:"dec"`
	XBinning   int                    `json:"xbinning"`
	YBinning   int                    `json:"ybinning"`
	FocalLen   float64                `json:"focalLen"`
	SiteElev   float64                `json:"siteElev"`
	SiteLat    float64                `json:"siteLat"`
	SiteLong   float64                `json:"siteLong"`
	Extra      map[string]interface{} `json:"extra"`
}

// ReadHeader returns the parsed FITS header for the given file path.
func ReadHeader(path string) (*FITSHeader, error) {
	f, err := openFITS(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	hdu := f.HDU(0)
	if hdu == nil {
		return nil, fmt.Errorf("no HDU in %s", path)
	}
	img, ok := hdu.(fitsio.Image)
	if !ok {
		return nil, fmt.Errorf("primary HDU is not an image")
	}
	hdr := img.Header()
	axes := hdr.Axes()

	w, h, ch := 0, 0, 1
	if len(axes) >= 1 {
		w = axes[0]
	}
	if len(axes) >= 2 {
		h = axes[1]
	}
	if len(axes) >= 3 {
		ch = axes[2]
	}

	// Collect all keys not explicitly parsed into Extra
	knownKeys := map[string]bool{
		"SIMPLE": true, "BITPIX": true, "NAXIS": true,
		"NAXIS1": true, "NAXIS2": true, "NAXIS3": true,
		"BSCALE": true, "BZERO": true, "EXTEND": true,
		"COMMENT": true, "HISTORY": true, "END": true,
		"OBJECT": true, "TELESCOP": true, "INSTRUME": true,
		"FILTER": true, "EXPTIME": true, "EXPOSURE": true,
		"DATE-OBS": true, "GAIN": true, "OFFSET": true,
		"PEDESTAL": true, "CCD-TEMP": true, "CCD_TEMP": true,
		"RA": true, "DEC": true, "OBJCTRA": true, "OBJCTDEC": true,
		"XBINNING": true, "YBINNING": true,
		"FOCALLEN": true, "SITEELEV": true, "SITELAT": true, "SITELONG": true,
	}
	extra := make(map[string]interface{})
	for _, key := range hdr.Keys() {
		if knownKeys[key] {
			continue
		}
		if c := hdr.Get(key); c != nil {
			extra[key] = c.Value
		}
	}

	return &FITSHeader{
		Width:      w,
		Height:     h,
		Channels:   ch,
		BitPix:     hdr.Bitpix(),
		Object:     cardStr(hdr, "OBJECT"),
		Telescope:  cardStr(hdr, "TELESCOP"),
		Instrument: cardStr(hdr, "INSTRUME"),
		Filter:     cardStr(hdr, "FILTER"),
		ExpTime:    cardF64(hdr, 0, "EXPTIME", "EXPOSURE"),
		DateObs:    cardStr(hdr, "DATE-OBS"),
		Gain:       cardF64(hdr, 0, "GAIN"),
		Offset:     cardF64(hdr, 0, "OFFSET", "PEDESTAL"),
		CCDTemp:    cardF64(hdr, 0, "CCD-TEMP", "CCD_TEMP"),
		RA:         cardF64(hdr, 0, "RA", "OBJCTRA"),
		Dec:        cardF64(hdr, 0, "DEC", "OBJCTDEC"),
		XBinning:   cardInt(hdr, 1, "XBINNING"),
		YBinning:   cardInt(hdr, 1, "YBINNING"),
		FocalLen:   cardF64(hdr, 0, "FOCALLEN"),
		SiteElev:   cardF64(hdr, 0, "SITEELEV"),
		SiteLat:    cardF64(hdr, 0, "SITELAT"),
		SiteLong:   cardF64(hdr, 0, "SITELONG"),
		Extra:      extra,
	}, nil
}

// GeneratePreview reads a FITS file, applies autostretch, and returns a
// base64-encoded PNG data URL scaled to maxSize pixels on the longest side.
// StretchLevel controls autostretch aggressiveness:
//
//	0 = none (linear normalisation only)
//	1 = gentle   (shadows −1.25σ, target background 0.10)
//	2 = normal   (shadows −2.80σ, target background 0.25 — Siril default)
//	3 = strong   (shadows −4.00σ, target background 0.40)
type stretchPreset struct{ shadows, targetBG float64 }

var stretchPresets = []stretchPreset{
	{0, 0},         // 0: identity (unused — handled separately)
	{-1.25, 0.10},  // 1: gentle
	{-2.80, 0.25},  // 2: normal
	{-4.00, 0.40},  // 3: strong
}

func makeStretcher(level int) func([]float64) []float64 {
	if level <= 0 {
		return linearAutoStretch
	}
	if level >= len(stretchPresets) {
		level = len(stretchPresets) - 1
	}
	pr := stretchPresets[level]
	return func(p []float64) []float64 { return autoStretch(p, pr.shadows, pr.targetBG) }
}

// GeneratePreview reads a FITS file, applies autostretch, and returns a
// base64-encoded PNG data URL scaled to maxSize pixels on the longest side.
func GeneratePreview(path string, maxSize, stretchLevel int) (string, error) {
	f, err := openFITS(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hdu := f.HDU(0)
	if hdu == nil {
		return "", fmt.Errorf("no HDU in %s", path)
	}
	img, ok := hdu.(fitsio.Image)
	if !ok {
		return "", fmt.Errorf("primary HDU is not an image")
	}
	hdr := img.Header()
	axes := hdr.Axes()
	if len(axes) < 2 {
		return "", fmt.Errorf("not a 2D image")
	}

	w, h := axes[0], axes[1]
	channels := 1
	if len(axes) >= 3 {
		channels = axes[2]
	}

	bscale := cardF64(hdr, 1.0, "BSCALE")
	bzero := cardF64(hdr, 0.0, "BZERO")

	pixels, err := readPixelsAsFloat64(img, bscale, bzero)
	if err != nil {
		return "", fmt.Errorf("read pixels: %w", err)
	}

	stretch := makeStretcher(stretchLevel)
	var channelData [][]float64

	bayerpat := cardStr(hdr, "BAYERPAT", "COLORTYP")
	if bayerpat != "" && channels == 1 {
		// Single-plane Bayer mosaic: demosaic into R/G/B planes (half resolution).
		rCh, gCh, bCh, dw, dh := debayerBlocks(pixels, w, h, bayerpat)
		w, h = dw, dh
		channels = 3
		channelData = [][]float64{
			stretch(normalizeToUnit(rCh)),
			stretch(normalizeToUnit(gCh)),
			stretch(normalizeToUnit(bCh)),
		}
	} else {
		planeSize := w * h
		channelData = make([][]float64, channels)
		for c := 0; c < channels; c++ {
			plane := make([]float64, planeSize)
			copy(plane, pixels[c*planeSize:(c+1)*planeSize])
			channelData[c] = stretch(normalizeToUnit(plane))
		}
	}

	// Compute output dimensions preserving aspect ratio
	outW, outH := w, h
	if w > maxSize || h > maxSize {
		if w >= h {
			outW = maxSize
			outH = max(1, h*maxSize/w)
		} else {
			outH = maxSize
			outW = max(1, w*maxSize/h)
		}
	}

	// FITS pixel (0,0) is bottom-left; Go image (0,0) is top-left — flip Y
	var outImg image.Image
	if channels == 3 {
		rgba := image.NewRGBA(image.Rect(0, 0, outW, outH))
		for y := 0; y < outH; y++ {
			srcY := h - 1 - (y*h/outH)
			for x := 0; x < outW; x++ {
				srcX := x * w / outW
				idx := srcY*w + srcX
				rgba.SetRGBA(x, y, color.RGBA{
					R: toU8(channelData[0][idx]),
					G: toU8(channelData[1][idx]),
					B: toU8(channelData[2][idx]),
					A: 255,
				})
			}
		}
		outImg = rgba
	} else {
		gray := image.NewGray(image.Rect(0, 0, outW, outH))
		for y := 0; y < outH; y++ {
			srcY := h - 1 - (y*h/outH)
			for x := 0; x < outW; x++ {
				srcX := x * w / outW
				gray.SetGray(x, y, color.Gray{Y: toU8(channelData[0][srcY*w+srcX])})
			}
		}
		outImg = gray
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, outImg); err != nil {
		return "", fmt.Errorf("encode PNG: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func openFITS(path string) (*fitsio.File, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	f, err := fitsio.Open(r)
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("parse FITS %s: %w", path, err)
	}
	return f, nil
}

// readPixelsAsFloat64 reads all FITS pixels into float64 and applies BSCALE/BZERO.
//
// fitsio.Image.Read calls reflect.Value.SetLen(n) on the slice we pass in, which
// panics when n > cap (i.e. for any nil/var-declared slice).  Pre-allocating with
// make avoids the panic.
func readPixelsAsFloat64(img fitsio.Image, bscale, bzero float64) ([]float64, error) {
	hdr := img.Header()
	bitpix := hdr.Bitpix()

	n := 1
	for _, dim := range hdr.Axes() {
		n *= dim
	}
	if n <= 0 {
		return nil, fmt.Errorf("image has zero pixels")
	}

	switch bitpix {
	case 8:
		raw := make([]int8, n)
		if err := img.Read(&raw); err != nil {
			return nil, err
		}
		return applyScale8(raw, bscale, bzero), nil
	case 16:
		raw := make([]int16, n)
		if err := img.Read(&raw); err != nil {
			return nil, err
		}
		return applyScale16(raw, bscale, bzero), nil
	case 32:
		raw := make([]int32, n)
		if err := img.Read(&raw); err != nil {
			return nil, err
		}
		return applyScale32(raw, bscale, bzero), nil
	case 64:
		raw := make([]int64, n)
		if err := img.Read(&raw); err != nil {
			return nil, err
		}
		return applyScale64(raw, bscale, bzero), nil
	case -32:
		raw := make([]float32, n)
		if err := img.Read(&raw); err != nil {
			return nil, err
		}
		return applyScaleF32(raw, bscale, bzero), nil
	case -64:
		raw := make([]float64, n)
		if err := img.Read(&raw); err != nil {
			return nil, err
		}
		return applyScaleF64(raw, bscale, bzero), nil
	default:
		return nil, fmt.Errorf("unsupported BITPIX %d", bitpix)
	}
}

func applyScale8(src []int8, s, z float64) []float64 {
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = s*float64(v) + z
	}
	return out
}
func applyScale16(src []int16, s, z float64) []float64 {
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = s*float64(v) + z
	}
	return out
}
func applyScale32(src []int32, s, z float64) []float64 {
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = s*float64(v) + z
	}
	return out
}
func applyScale64(src []int64, s, z float64) []float64 {
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = s*float64(v) + z
	}
	return out
}
func applyScaleF32(src []float32, s, z float64) []float64 {
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = s*float64(v) + z
	}
	return out
}
func applyScaleF64(src []float64, s, z float64) []float64 {
	if s == 1.0 && z == 0.0 {
		return src
	}
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = s*v + z
	}
	return out
}

// normalizeToUnit maps pixel values to [0,1].
// The white point is set at the 99.9th percentile to clip hot pixels.
func normalizeToUnit(pixels []float64) []float64 {
	if len(pixels) == 0 {
		return nil
	}
	sorted := make([]float64, len(pixels))
	copy(sorted, pixels)
	sort.Float64s(sorted)

	lo := sorted[0]
	hiIdx := int(float64(len(sorted)) * 0.999)
	hi := sorted[hiIdx]

	if hi <= lo {
		return make([]float64, len(pixels))
	}
	rng := hi - lo
	out := make([]float64, len(pixels))
	for i, p := range pixels {
		v := (p - lo) / rng
		if v < 0 {
			v = 0
		} else if v > 1 {
			v = 1
		}
		out[i] = v
	}
	return out
}

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
			// 2×2 block: a=top-left, b_=top-right, c=bottom-left, d=bottom-right
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

// linearAutoStretch applies the same shadow-clipping statistics as the normal
// autoStretch preset but maps the result linearly (no MTF curve).
// This is used for stretchLevel=0 ("no stretch") so the sky appears dark and
// the pixel values are physically linear — unlike the raw normalizeToUnit output
// which maps the sky background to ~90% brightness.
func linearAutoStretch(pixels []float64) []float64 {
	const shadowsFactor = -2.80

	sample := pixels
	if len(pixels) > 65536 {
		step := len(pixels) / 65536
		s := make([]float64, 0, 65536)
		for i := 0; i < len(pixels); i += step {
			s = append(s, pixels[i])
		}
		sample = s
	}

	sorted := make([]float64, len(sample))
	copy(sorted, sample)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]

	devs := make([]float64, len(sorted))
	for i, v := range sorted {
		devs[i] = math.Abs(v - median)
	}
	sort.Float64s(devs)
	sigma := devs[len(devs)/2] * 1.4826

	shadowClip := median + shadowsFactor*sigma
	if shadowClip < 0 {
		shadowClip = 0
	}
	scale := 1.0 - shadowClip

	out := make([]float64, len(pixels))
	for i, p := range pixels {
		x := p - shadowClip
		if x < 0 {
			x = 0
		}
		if scale > 0 {
			x /= scale
		}
		if x > 1 {
			x = 1
		}
		out[i] = x
	}
	return out
}

// autoStretch applies a Siril-compatible MTF autostretch to normalised [0,1] data.
// shadowsFactor controls shadow clipping aggressiveness (e.g. −2.80 is Siril default).
// targetBG is the desired output brightness of the background midtone (e.g. 0.25).
func autoStretch(pixels []float64, shadowsFactor, targetBG float64) []float64 {

	// Sub-sample for statistics on very large images (65k pixels is enough)
	sample := pixels
	if len(pixels) > 65536 {
		step := len(pixels) / 65536
		s := make([]float64, 0, 65536)
		for i := 0; i < len(pixels); i += step {
			s = append(s, pixels[i])
		}
		sample = s
	}

	sorted := make([]float64, len(sample))
	copy(sorted, sample)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]

	devs := make([]float64, len(sorted))
	for i, v := range sorted {
		devs[i] = math.Abs(v - median)
	}
	sort.Float64s(devs)
	sigma := devs[len(devs)/2] * 1.4826

	shadowClip := median + shadowsFactor*sigma
	if shadowClip < 0 {
		shadowClip = 0
	}

	scale := 1.0 - shadowClip
	var m float64
	if scale > 0 {
		newMedian := math.Max(0, median-shadowClip) / scale
		m = mtfMidtone(targetBG, newMedian)
	}

	out := make([]float64, len(pixels))
	for i, p := range pixels {
		x := p - shadowClip
		if x < 0 {
			x = 0
		}
		if scale > 0 {
			x /= scale
		}
		if x > 1 {
			x = 1
		}
		out[i] = mtf(m, x)
	}
	return out
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

// ── header card helpers ───────────────────────────────────────────────────────

// cardStr returns the string value of the first matching keyword, or "".
func cardStr(hdr *fitsio.Header, keys ...string) string {
	for _, k := range keys {
		if c := hdr.Get(k); c != nil {
			if s, ok := c.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

// cardF64 returns the float64 value of the first matching keyword, or defaultVal.
func cardF64(hdr *fitsio.Header, defaultVal float64, keys ...string) float64 {
	for _, k := range keys {
		c := hdr.Get(k)
		if c == nil {
			continue
		}
		switch v := c.Value.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

// cardInt returns the int value of the first matching keyword, or defaultVal.
func cardInt(hdr *fitsio.Header, defaultVal int, keys ...string) int {
	for _, k := range keys {
		c := hdr.Get(k)
		if c == nil {
			continue
		}
		switch v := c.Value.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

func toU8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v * 255)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
