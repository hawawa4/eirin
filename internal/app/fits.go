package app

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
	Width      int            `json:"width"`
	Height     int            `json:"height"`
	Channels   int            `json:"channels"`
	BitPix     int            `json:"bitpix"`
	Object     string         `json:"object"`
	Telescope  string         `json:"telescope"`
	Instrument string         `json:"instrument"`
	Filter     string         `json:"filter"`
	ExpTime    float64        `json:"exptime"`
	DateObs    string         `json:"dateObs"`
	Gain       float64        `json:"gain"`
	Offset     float64        `json:"offset"`
	CCDTemp    float64        `json:"ccdTemp"`
	RA         float64        `json:"ra"`
	Dec        float64        `json:"dec"`
	XBinning   int            `json:"xbinning"`
	YBinning   int            `json:"ybinning"`
	FocalLen   float64        `json:"focalLen"`
	SiteElev   float64        `json:"siteElev"`
	SiteLat    float64        `json:"siteLat"`
	SiteLong   float64        `json:"siteLong"`
	// WCS-derived pixel scale in arcsec/pixel and field rotation in degrees.
	// Non-zero only when the header contains usable WCS keywords.
	PixelScale float64        `json:"pixelScale"`
	Rotation   float64        `json:"rotation"`
	Extra      map[string]any `json:"extra"`
}

// ChannelStats holds per-channel statistics needed by the frontend to compute
// MTF stretch uniforms without a second backend round-trip.
type ChannelStats struct {
	Median float64 `json:"median"`
	Sigma  float64 `json:"sigma"`
}

// RawPreviewData is returned by GeneratePreviewRaw. The pixel data is a
// base64-encoded little-endian float32 array in RGBA interleaved order
// (R,G,B,1.0 per pixel, row-major, top-left origin). All channels are
// globally normalised to [0,1] so that colour balance is preserved; the
// frontend applies the MTF stretch in a WebGL shader using the Stats.
type RawPreviewData struct {
	Data     string         `json:"data"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	Channels int            `json:"channels"` // 1 = mono, 3 = colour
	Stats    []ChannelStats `json:"stats"`
}

func (a *App) ReadFITSHeader(path string) (*FITSHeader, error) {
	return readFITSHeader(path)
}

// GeneratePreview returns a PNG preview as a base64 data URL, scaled to 1024 px.
// stretchLevel: 0=linear, 1=gentle, 2=normal, 3=strong
func (a *App) GeneratePreview(path string, stretchLevel int) (string, error) {
	return generatePreview(path, 1024, stretchLevel)
}

// GeneratePreviewRaw returns raw float32 RGBA pixel data (base64-encoded) plus
// per-channel statistics for WebGL-based MTF rendering on the frontend.
// All channels are globally normalised so colour balance is preserved.
func (a *App) GeneratePreviewRaw(path string) (RawPreviewData, error) {
	return generatePreviewRaw(path, 768)
}

// GeneratePreviewRawSized is like GeneratePreviewRaw but lets the caller choose the
// maximum dimension. maxSize=0 means native resolution (no downscaling).
func (a *App) GeneratePreviewRawSized(path string, maxSize int) (RawPreviewData, error) {
	if maxSize <= 0 {
		maxSize = 1<<31 - 1 // effectively native
	}
	return generatePreviewRaw(path, maxSize)
}

// ── Implementation ────────────────────────────────────────────────────────────

func readFITSHeader(path string) (*FITSHeader, error) {
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
		"CDELT1": true, "CDELT2": true, "CROTA2": true,
		"CD1_1": true, "CD1_2": true, "CD2_1": true, "CD2_2": true,
		"CRPIX1": true, "CRPIX2": true, "CTYPE1": true, "CTYPE2": true,
		"CRVAL1": true, "CRVAL2": true,
		"PIXSCALE": true, "SCALE": true,
	}
	extra := make(map[string]any)
	for _, key := range hdr.Keys() {
		if knownKeys[key] {
			continue
		}
		if c := hdr.Get(key); c != nil {
			extra[key] = c.Value
		}
	}

	pixelScale, rotation := extractWCS(hdr)

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
		RA:         cardF64(hdr, 0, "RA", "OBJCTRA", "CRVAL1"),
		Dec:        cardF64(hdr, 0, "DEC", "OBJCTDEC", "CRVAL2"),
		XBinning:   cardInt(hdr, 1, "XBINNING"),
		YBinning:   cardInt(hdr, 1, "YBINNING"),
		FocalLen:   cardF64(hdr, 0, "FOCALLEN"),
		SiteElev:   cardF64(hdr, 0, "SITEELEV"),
		SiteLat:    cardF64(hdr, 0, "SITELAT"),
		SiteLong:   cardF64(hdr, 0, "SITELONG"),
		PixelScale: pixelScale,
		Rotation:   rotation,
		Extra:      extra,
	}, nil
}

type stretchPreset struct{ shadows, targetBG float64 }

var stretchPresets = []stretchPreset{
	{0, 0},         // 0: identity (unused — handled separately)
	{-1.25, 0.10},  // 1: gentle
	{-2.80, 0.25},  // 2: normal (Siril default)
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

func generatePreview(path string, maxSize, stretchLevel int) (string, error) {
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

	// FITS pixel (0,0) is bottom-left; Go image (0,0) is top-left — flip Y.
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

func generatePreviewRaw(path string, maxSize int) (RawPreviewData, error) {
	f, err := openFITS(path)
	if err != nil {
		return RawPreviewData{}, err
	}
	defer f.Close()

	hdu := f.HDU(0)
	if hdu == nil {
		return RawPreviewData{}, fmt.Errorf("no HDU in %s", path)
	}
	img, ok := hdu.(fitsio.Image)
	if !ok {
		return RawPreviewData{}, fmt.Errorf("primary HDU is not an image")
	}
	hdr := img.Header()
	axes := hdr.Axes()
	if len(axes) < 2 {
		return RawPreviewData{}, fmt.Errorf("not a 2D image")
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
		return RawPreviewData{}, fmt.Errorf("read pixels: %w", err)
	}

	var channelData [][]float64

	bayerpat := cardStr(hdr, "BAYERPAT", "COLORTYP")
	if bayerpat != "" && channels == 1 {
		rCh, gCh, bCh, dw, dh := debayerBlocks(pixels, w, h, bayerpat)
		w, h = dw, dh
		channels = 3
		channelData = [][]float64{rCh, gCh, bCh}
	} else {
		planeSize := w * h
		channelData = make([][]float64, channels)
		for c := 0; c < channels; c++ {
			plane := make([]float64, planeSize)
			copy(plane, pixels[c*planeSize:(c+1)*planeSize])
			channelData[c] = plane
		}
	}

	// Normalise ALL channels with the same global percentile range so that
	// colour balance is preserved.
	channelData = globalNormalize(channelData)

	stats := make([]ChannelStats, channels)
	for c := 0; c < channels; c++ {
		med, sig := channelMedianSigma(channelData[c])
		stats[c] = ChannelStats{Median: med, Sigma: sig}
	}

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

	// Pack as RGBA float32, interleaved, row-major, top-left origin.
	// FITS pixel (0,0) is bottom-left; flip Y so row 0 = visual top.
	nPixels := outW * outH
	raw := make([]byte, nPixels*4*4) // 4 components × 4 bytes each
	for y := 0; y < outH; y++ {
		srcY := h - 1 - (y*h/outH)
		for x := 0; x < outW; x++ {
			srcX := x * w / outW
			idx := srcY*w + srcX
			base := (y*outW+x) * 16 // 4 floats × 4 bytes
			var r, g, b float32
			if channels == 3 {
				r = float32(channelData[0][idx])
				g = float32(channelData[1][idx])
				b = float32(channelData[2][idx])
			} else {
				v := float32(channelData[0][idx])
				r, g, b = v, v, v
			}
			packF32(raw, base+0, r)
			packF32(raw, base+4, g)
			packF32(raw, base+8, b)
			packF32(raw, base+12, 1.0)
		}
	}

	return RawPreviewData{
		Data:     base64.StdEncoding.EncodeToString(raw),
		Width:    outW,
		Height:   outH,
		Channels: channels,
		Stats:    stats,
	}, nil
}

// ── Pixel readers ─────────────────────────────────────────────────────────────

func openFITS(path string) (*fitsio.File, error) {
	r, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	f, err := fitsio.Open(r)
	if err != nil {
		_ = r.Close()
		return nil, fmt.Errorf("parse FITS %s: %w", path, err)
	}
	return f, nil
}

// readPixelsAsFloat64 reads all FITS pixels into float64 and applies BSCALE/BZERO.
//
// fitsio.Image.Read calls reflect.Value.SetLen(n) on the slice we pass in, which
// panics when n > cap (i.e. for any nil/var-declared slice). Pre-allocating with
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

// ── Image processing ──────────────────────────────────────────────────────────

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

// globalNormalize maps all channels to [0,1] using a SHARED percentile range
// computed across all channels combined. This preserves colour balance —
// unlike per-channel normalisation which amplifies dim channels independently.
func globalNormalize(channels [][]float64) [][]float64 {
	if len(channels) == 0 {
		return channels
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
	for _, ch := range channels {
		for _, v := range ch {
			if i%step == 0 {
				sample = append(sample, v)
			}
			i++
		}
	}

	sort.Float64s(sample)
	lo := sample[0]
	hiIdx := int(float64(len(sample)) * 0.999)
	hi := sample[hiIdx]

	if hi <= lo {
		return channels
	}
	rng := hi - lo

	out := make([][]float64, len(channels))
	for c, ch := range channels {
		norm := make([]float64, len(ch))
		for j, v := range ch {
			nv := (v - lo) / rng
			if nv < 0 {
				nv = 0
			} else if nv > 1 {
				nv = 1
			}
			norm[j] = nv
		}
		out[c] = norm
	}
	return out
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

	sorted := make([]float64, len(sample))
	copy(sorted, sample)
	sort.Float64s(sorted)
	median = sorted[len(sorted)/2]

	devs := make([]float64, len(sorted))
	for i, v := range sorted {
		devs[i] = math.Abs(v - median)
	}
	sort.Float64s(devs)
	sigma = devs[len(devs)/2] * 1.4826
	return
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

// linearAutoStretch applies the same shadow-clipping statistics as the normal
// autoStretch preset but maps the result linearly (no MTF curve).
// Used for stretchLevel=0 so the sky appears dark and values stay physically linear.
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

// ── Header card helpers ───────────────────────────────────────────────────────

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

// extractWCS attempts to derive pixel scale (arcsec/px) and rotation (degrees)
// from WCS keywords. Tries PIXSCALE, then CDELT1, then the CD matrix.
// Returns (0, 0) when no usable WCS is found.
func extractWCS(hdr *fitsio.Header) (pixelScale, rotation float64) {
	// Direct pixel scale keyword (some cameras/stacking tools write this)
	if ps := cardF64(hdr, 0, "PIXSCALE", "SCALE"); ps > 0 {
		crota2 := cardF64(hdr, 0, "CROTA2")
		return ps, crota2
	}

	// Standard WCS: CDELT1 is degrees/pixel (FITS standard).
	// Some nonstandard software writes arcsec/pixel directly; heuristic: if |value| >= 1 it's already arcsec.
	cdelt1 := cardF64(hdr, 0, "CDELT1")
	if cdelt1 != 0 {
		abs := math.Abs(cdelt1)
		var ps float64
		if abs >= 1.0 {
			ps = abs // already arcsec/pixel
		} else {
			ps = abs * 3600.0 // degrees → arcsec
		}
		crota2 := cardF64(hdr, 0, "CROTA2")
		return ps, crota2
	}

	// CD matrix: CD1_1, CD2_1 give the column vector for the RA axis.
	// Standard FITS has CDELT1 < 0 (RA increases right-to-left), so:
	//   CD1_1 = CDELT1*cos(R) < 0,  CD2_1 = CDELT1*sin(R) < 0 for R in (0,π).
	// atan2(-CD2_1, -CD1_1) recovers CROTA2 correctly for CDELT1<0.
	// (The previous atan2(CD2_1,-CD1_1) gave -CROTA2, flipping all non-zero rotations.)
	cd1_1 := cardF64(hdr, 0, "CD1_1")
	cd2_1 := cardF64(hdr, 0, "CD2_1")
	if cd1_1 != 0 || cd2_1 != 0 {
		ps := math.Sqrt(cd1_1*cd1_1+cd2_1*cd2_1) * 3600.0
		rot := math.Atan2(-cd2_1, -cd1_1) * 180.0 / math.Pi
		return ps, rot
	}

	return 0, 0
}

// ── Misc helpers ──────────────────────────────────────────────────────────────

func packF32(dst []byte, offset int, v float32) {
	bits := math.Float32bits(v)
	dst[offset+0] = byte(bits)
	dst[offset+1] = byte(bits >> 8)
	dst[offset+2] = byte(bits >> 16)
	dst[offset+3] = byte(bits >> 24)
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
