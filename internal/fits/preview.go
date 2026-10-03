package fits

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"

	fitsio "codeberg.org/astrogo/fitsio"
)

// previewImage is a FITS image prepared for display: debayered, chroma-
// smoothed (raw subs), sized to fit and normalised to [0,1] with one shared
// range. Rows are still in FITS order (bottom-up).
type previewImage struct {
	channels [][]float64
	w, h     int
	stats    []ChannelStats
	gains    []float64
}

func loadPreviewImage(path string, maxSize int) (previewImage, error) {
	f, err := openFITS(path)
	if err != nil {
		return previewImage{}, err
	}
	defer f.Close()

	hdu := f.HDU(0)
	if hdu == nil {
		return previewImage{}, fmt.Errorf("no HDU in %s", path)
	}
	img, ok := hdu.(fitsio.Image)
	if !ok {
		return previewImage{}, fmt.Errorf("primary HDU is not an image")
	}
	hdr := img.Header()
	axes := hdr.Axes()
	if len(axes) < 2 {
		return previewImage{}, fmt.Errorf("not a 2D image")
	}

	w, h := axes[0], axes[1]
	nch := 1
	if len(axes) >= 3 {
		nch = axes[2]
	}

	pixels, err := readPixelsAsFloat64(img, cardF64(hdr, 1.0, "BSCALE"), cardF64(hdr, 0.0, "BZERO"))
	if err != nil {
		return previewImage{}, fmt.Errorf("read pixels: %w", err)
	}

	var channels [][]float64
	bayerpat := strings.ToUpper(strings.TrimSpace(cardStr(hdr, "BAYERPAT", "COLORTYP")))
	if bayerpat != "" && nch == 1 {
		r, g, b, dw, dh := debayerBlocks(pixels, w, h, bayerpat)
		w, h = dw, dh
		channels = reduceChroma([][]float64{r, g, b}, w, h, chromaRadius)
	} else {
		planeSize := w * h
		channels = make([][]float64, nch)
		for c := range nch {
			channels[c] = pixels[c*planeSize : (c+1)*planeSize]
		}
	}

	outW, outH := fitSize(w, h, maxSize)
	for c := range channels {
		channels[c] = downsampleArea(channels[c], w, h, outW, outH)
	}
	// Balance gains are ratios of the sky medians, so they can be taken before
	// normalising; normalising is linear with black at 0 and keeps them.
	raw := make([]ChannelStats, len(channels))
	for c, ch := range channels {
		raw[c].Median, raw[c].Sigma = channelMedianSigma(ch)
	}
	channels = globalNormalize(channels, balanceGains(raw))

	stats := make([]ChannelStats, len(channels))
	for c, ch := range channels {
		med, sig := channelMedianSigma(ch)
		stats[c] = ChannelStats{Median: med, Sigma: sig}
	}
	return previewImage{
		channels: channels,
		w:        outW,
		h:        outH,
		stats:    stats,
		gains:    balanceGains(stats),
	}, nil
}

// GeneratePreview renders a stretched PNG preview (data URL) at most maxSize
// pixels on its longest side. stretchLevel: 0=linear, 1=gentle, 2=normal, 3=strong.
func GeneratePreview(path string, maxSize, stretchLevel int) (string, error) {
	pi, err := loadPreviewImage(path, maxSize)
	if err != nil {
		return "", err
	}
	p := linkedStretch(pi.stats, pi.gains, stretchLevel)
	w, h := pi.w, pi.h

	// FITS pixel (0,0) is bottom-left; Go image (0,0) is top-left — flip Y.
	var outImg image.Image
	if len(pi.channels) >= 3 {
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			src := (h - 1 - y) * w
			for x := range w {
				i := src + x
				rgba.SetRGBA(x, y, color.RGBA{
					R: toU8(p.apply(0, pi.channels[0][i])),
					G: toU8(p.apply(1, pi.channels[1][i])),
					B: toU8(p.apply(2, pi.channels[2][i])),
					A: 255,
				})
			}
		}
		outImg = rgba
	} else {
		gray := image.NewGray(image.Rect(0, 0, w, h))
		for y := range h {
			src := (h - 1 - y) * w
			for x := range w {
				gray.SetGray(x, y, color.Gray{Y: toU8(p.apply(0, pi.channels[0][src+x]))})
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

// GeneratePreviewRaw returns the normalised (unstretched) preview pixels plus
// the stats and balance gains the frontend needs to stretch them.
func GeneratePreviewRaw(path string, maxSize int) (RawPreviewData, error) {
	pi, err := loadPreviewImage(path, maxSize)
	if err != nil {
		return RawPreviewData{}, err
	}
	w, h := pi.w, pi.h
	colour := len(pi.channels) >= 3

	// Pack as RGBA float32, interleaved, row-major; flip Y so row 0 = visual top.
	raw := make([]byte, w*h*16) // 4 components × 4 bytes each
	for y := range h {
		src := (h - 1 - y) * w
		for x := range w {
			i := src + x
			base := (y*w + x) * 16
			r := float32(pi.channels[0][i])
			g, b := r, r
			if colour {
				g = float32(pi.channels[1][i])
				b = float32(pi.channels[2][i])
			}
			packF32(raw, base+0, r)
			packF32(raw, base+4, g)
			packF32(raw, base+8, b)
			packF32(raw, base+12, 1.0)
		}
	}

	channels := 1
	if colour {
		channels = 3
	}
	return RawPreviewData{
		Data:     base64.StdEncoding.EncodeToString(raw),
		Width:    w,
		Height:   h,
		Channels: channels,
		Stats:    pi.stats[:channels],
		Balance:  pi.gains[:channels],
	}, nil
}
