package fits

import (
	"fmt"
	"math"
	"os"

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

// ChannelStats holds per-channel statistics of the normalised preview data,
// used to compute the autostretch (here and in the frontend).
type ChannelStats struct {
	Median float64 `json:"median"`
	Sigma  float64 `json:"sigma"`
}

// RawPreviewData is returned by GeneratePreviewRawSized. The pixel data is a
// base64-encoded little-endian float32 array in RGBA interleaved order
// (R,G,B,1.0 per pixel, row-major, top-left origin). All channels share one
// [0,1] normalisation (black at 0), so the camera's raw colour is kept; the
// frontend multiplies by Balance and applies the linked stretch from Stats
// (see stretch.go and frontend/src/lib/stretchPreview.ts).
type RawPreviewData struct {
	Data     string         `json:"data"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	Channels int            `json:"channels"` // 1 = mono, 3 = colour
	Stats    []ChannelStats `json:"stats"`
	// Balance holds per-channel gains that neutralise the sky background
	// (all 1 for mono). Applied only when stretching.
	Balance []float64 `json:"balance"`
}


// ── Implementation ────────────────────────────────────────────────────────────

func ReadFITSHeader(path string) (*FITSHeader, error) {
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
