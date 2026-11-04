package fits

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"strings"
)

const previewJPEGQuality = 90

// MaxRenderSize caps the longest side of any rendered image, "native" ones
// included: beyond it the 8-bit image alone would take hundreds of MB.
const MaxRenderSize = 8192

// previewImage is a FITS image prepared for display: debayered, chroma-
// smoothed (raw subs), sized to fit and normalised to [0,1] with one shared
// range. Rows are still in FITS order (bottom-up).
type previewImage struct {
	ph       *primaryHeader
	channels [][]float32
	w, h     int
	stats    []ChannelStats
	gains    []float64
}

// loadPreviewImage reads and prepares path at most maxSize pixels on its
// longest side (0 = native size, capped at MaxRenderSize).
func loadPreviewImage(path string, maxSize int) (previewImage, error) {
	if maxSize <= 0 || maxSize > MaxRenderSize {
		maxSize = MaxRenderSize
	}
	ph, pixels, err := readPixels(path)
	if err != nil {
		return previewImage{}, err
	}
	if len(ph.axes) < 2 {
		return previewImage{}, fmt.Errorf("not a 2D image")
	}

	w, h := ph.axes[0], ph.axes[1]
	nch := 1
	if len(ph.axes) >= 3 {
		nch = max(1, ph.axes[2])
	}

	var channels [][]float32
	bayerpat := strings.ToUpper(strings.TrimSpace(cardStr(ph.hdr, "BAYERPAT", "COLORTYP")))
	isRawSub := bayerpat != "" && nch == 1
	if isRawSub {
		r, g, b, dw, dh := debayerBlocks(pixels, w, h, bayerpat)
		w, h = dw, dh
		channels = [][]float32{r, g, b}
	} else {
		planeSize := w * h
		channels = make([][]float32, nch)
		for c := range nch {
			channels[c] = pixels[c*planeSize : (c+1)*planeSize]
		}
	}

	outW, outH := fitSize(w, h, maxSize)
	for c := range channels {
		channels[c] = downsampleArea(channels[c], w, h, outW, outH)
	}
	if isRawSub {
		// Smoothing after downscaling is ~4× cheaper; scale the radius so it
		// covers the same patch of sky.
		radius := max(1, int(math.Round(float64(chromaRadius)*float64(outW)/float64(w))))
		channels = reduceChroma(channels, outW, outH, radius)
	}

	// Balance gains are ratios of the sky medians, so they can be taken before
	// normalising; normalising is linear, so the stats carry over too.
	raw := make([]ChannelStats, len(channels))
	for c, ch := range channels {
		raw[c].Median, raw[c].Sigma = channelMedianSigma(ch)
	}
	channels, nr := globalNormalize(channels, balanceGains(raw))
	stats := make([]ChannelStats, len(channels))
	for c := range raw {
		stats[c] = nr.stats(raw[c])
	}
	return previewImage{
		ph:       ph,
		channels: channels,
		w:        outW,
		h:        outH,
		stats:    stats,
		gains:    balanceGains(stats),
	}, nil
}

// RenderFITS renders the FITS image at path as an 8-bit image (RGBA for
// colour, Gray for mono) at most maxSize pixels on its longest side (0 =
// native size, capped at MaxRenderSize), stretched at stretchLevel:
// 0=linear, 1=gentle, 2=normal, 3=strong.
func RenderFITS(path string, maxSize, stretchLevel int) (image.Image, error) {
	pi, err := loadPreviewImage(path, maxSize)
	if err != nil {
		return nil, err
	}
	return pi.render(stretchLevel), nil
}

// render stretches the prepared image to 8 bits, flipping it to top-down rows
// (FITS pixel (0,0) is bottom-left; Go image (0,0) is top-left).
func (pi previewImage) render(stretchLevel int) image.Image {
	p := linkedStretch(pi.stats, pi.gains, stretchLevel)
	w, h := pi.w, pi.h
	// Large images go through a lookup table per channel: far cheaper than
	// the stretch per pixel, and fine enough to be exact in 8 bits.
	useLUT := w*h > stretchLUTSize
	tables := make([][]uint8, min(3, len(pi.channels)))
	for c := range tables {
		if useLUT {
			tables[c] = p.table(c)
		}
	}
	px := func(c int, v float32) uint8 {
		if useLUT {
			return tables[c][int(v*(stretchLUTSize-1)+0.5)]
		}
		return toU8(p.apply(c, float64(v)))
	}

	if len(pi.channels) >= 3 {
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		r, g, b := pi.channels[0], pi.channels[1], pi.channels[2]
		for y := range h {
			src := (h - 1 - y) * w
			row := rgba.Pix[y*rgba.Stride : y*rgba.Stride+w*4]
			for x := range w {
				i := src + x
				o := x * 4
				row[o] = px(0, r[i])
				row[o+1] = px(1, g[i])
				row[o+2] = px(2, b[i])
				row[o+3] = 255
			}
		}
		return rgba
	}
	gray := image.NewGray(image.Rect(0, 0, w, h))
	ch := pi.channels[0]
	for y := range h {
		src := (h - 1 - y) * w
		row := gray.Pix[y*gray.Stride : y*gray.Stride+w]
		for x := range w {
			row[x] = px(0, ch[src+x])
		}
	}
	return gray
}

// EncodeJPEG encodes img as a JPEG at the given quality (1–100).
func EncodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// GeneratePreview renders a stretched JPEG preview (data URL) at most maxSize
// pixels on its longest side. stretchLevel: 0=linear, 1=gentle, 2=normal, 3=strong.
// JPEG rather than PNG: ~4× faster to encode and ~5× smaller to send, which
// matters when blinking; quality 90 is indistinguishable for culling.
func GeneratePreview(path string, maxSize, stretchLevel int) (string, error) {
	p, err := RenderPreview(path, maxSize, stretchLevel)
	return p.DataURL, err
}

// RenderPreview is GeneratePreview with the preview's pixel size, which is
// the same as GeneratePreviewRaw's for the same maxSize.
func RenderPreview(path string, maxSize, stretchLevel int) (RenderedPreview, error) {
	img, err := RenderFITS(path, maxSize, stretchLevel)
	if err != nil {
		return RenderedPreview{}, err
	}
	data, err := EncodeJPEG(img, previewJPEGQuality)
	if err != nil {
		return RenderedPreview{}, err
	}
	b := img.Bounds()
	return RenderedPreview{DataURL: dataURL("image/jpeg", data), Width: b.Dx(), Height: b.Dy()}, nil
}

// RawPreviewMeta describes the samples EncodeRawPreview returns.
type RawPreviewMeta struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	// Channels is the number of samples per pixel: 1 = mono, 3 = colour.
	Channels int            `json:"channels"`
	Stats    []ChannelStats `json:"stats"`
	// Balance holds per-channel gains that neutralise the sky background
	// (all 1 for mono). Applied only when stretching.
	Balance []float64   `json:"balance"`
	Header  *FITSHeader `json:"header"`
}

// EncodeRawPreview returns the normalised, unstretched preview of path (at
// most maxSize pixels on its longest side, 0 = native) for a client that
// stretches it itself — the desktop Library, whose stretch is adjustable:
//
//	uint32 LE n | n bytes of RawPreviewMeta JSON | samples
//
// The JSON is space-padded so the samples start 4-byte aligned. Samples are
// uint16 LE (v/65535, v in [0,1]), pixel-interleaved, rows top-down. All
// channels share one normalisation (black at 0), so the camera's raw colour
// is kept; the client multiplies by Balance and applies the linked stretch
// from Stats (see stretch.go and frontend/src/lib/stretchPreview.ts).
func EncodeRawPreview(path string, maxSize int) ([]byte, error) {
	pi, err := loadPreviewImage(path, maxSize)
	if err != nil {
		return nil, err
	}
	channels := 1
	if len(pi.channels) >= 3 {
		channels = 3
	}
	meta, err := json.Marshal(RawPreviewMeta{
		Width:    pi.w,
		Height:   pi.h,
		Channels: channels,
		Stats:    pi.stats[:channels],
		Balance:  pi.gains[:channels],
		Header:   headerOf(pi.ph),
	})
	if err != nil {
		return nil, err
	}
	for (4+len(meta))%4 != 0 {
		meta = append(meta, ' ')
	}

	w, h := pi.w, pi.h
	out := make([]byte, 4+len(meta)+w*h*channels*2)
	binary.LittleEndian.PutUint32(out, uint32(len(meta)))
	copy(out[4:], meta)
	samples := out[4+len(meta):]
	o := 0
	for y := range h {
		src := (h - 1 - y) * w // FITS rows are bottom-up
		for x := range w {
			for c := range channels {
				v := pi.channels[c][src+x] // normalised to [0,1]
				binary.LittleEndian.PutUint16(samples[o:], uint16(v*65535+0.5))
				o += 2
			}
		}
	}
	return out, nil
}
