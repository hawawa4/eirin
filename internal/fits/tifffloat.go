package fits

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
)

// decodeFloatTIFF decodes 32-bit floating-point TIFFs (Siril's 32-bit
// export), which golang.org/x/image/tiff doesn't support. Handles grey or
// RGB(A) samples, interleaved, in strips, uncompressed or Deflate-compressed
// without a predictor. Values are taken as 0..1 and returned as 16-bit RGB:
// a few overshooting pixels (hot pixels, star cores) are clipped, but if a
// sizeable share is above 1 the data uses another range and is scaled to fit.
func decodeFloatTIFF(data []byte) (image.Image, error) {
	bo, tags, err := tiffDirectory(data)
	if err != nil {
		return nil, err
	}
	one := func(tag uint16, def uint32) uint32 {
		if v := tags[tag]; len(v) > 0 {
			return v[0]
		}
		return def
	}

	w, h := int(one(256, 0)), int(one(257, 0))
	spp := int(one(277, 1))
	compression := one(259, 1)
	switch {
	case w <= 0 || h <= 0 || w > 1<<16 || h > 1<<16:
		return nil, fmt.Errorf("tiff: bad size %dx%d", w, h)
	case spp != 1 && spp != 3 && spp != 4:
		return nil, fmt.Errorf("tiff: %d samples per pixel", spp)
	case !allEqual(tags[258], 32) || !allEqual(tags[339], 3):
		return nil, errors.New("tiff: not 32-bit floating point")
	case compression != 1 && compression != 8 && compression != 32946:
		return nil, fmt.Errorf("tiff: unsupported compression %d", compression)
	case one(317, 1) != 1:
		return nil, errors.New("tiff: predictors are not supported")
	case one(284, 1) != 1:
		return nil, errors.New("tiff: planar layout is not supported")
	case len(tags[322]) > 0:
		return nil, errors.New("tiff: tiled images are not supported")
	}

	offsets, counts := tags[273], tags[279]
	if len(offsets) == 0 || len(offsets) != len(counts) {
		return nil, errors.New("tiff: missing strip offsets")
	}
	need := w * h * spp * 4
	pix := make([]byte, 0, need)
	for i, off := range offsets {
		end := uint64(off) + uint64(counts[i])
		if end > uint64(len(data)) {
			return nil, errors.New("tiff: strip outside the file")
		}
		strip := data[off:end]
		if compression != 1 {
			zr, err := zlib.NewReader(bytes.NewReader(strip))
			if err != nil {
				return nil, fmt.Errorf("tiff: deflate: %w", err)
			}
			strip, err = io.ReadAll(zr)
			if err != nil {
				return nil, fmt.Errorf("tiff: deflate: %w", err)
			}
		}
		pix = append(pix, strip...)
	}
	if len(pix) < need {
		return nil, errors.New("tiff: pixel data is short")
	}

	sample := func(i int) float64 {
		return float64(math.Float32frombits(bo.Uint32(pix[i*4:])))
	}
	colours := min(spp, 3) // ignore alpha
	peak, over := 1.0, 0
	for p := range w * h {
		for c := range colours {
			if v := sample(p*spp + c); v > 1 && !math.IsInf(v, 1) {
				over++
				peak = max(peak, v)
			}
		}
	}
	if over*1000 < w*h*colours { // under 0.1% above 1: clip them
		peak = 1
	}
	to16 := func(v float64) uint16 {
		v /= peak
		if !(v > 0) { // also catches NaN
			return 0
		}
		return uint16(math.Min(v, 1)*65535 + 0.5)
	}
	img := image.NewRGBA64(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			p := (y*w + x) * spp
			r := to16(sample(p))
			g, b := r, r
			if colours == 3 {
				g, b = to16(sample(p+1)), to16(sample(p+2))
			}
			img.SetRGBA64(x, y, color.RGBA64{R: r, G: g, B: b, A: 0xffff})
		}
	}
	return img, nil
}

// tiffSize reads the image size from a TIFF's first directory, for TIFFs
// image.DecodeConfig rejects (floating point).
func tiffSize(data []byte) (int, int, error) {
	_, tags, err := tiffDirectory(data)
	if err != nil {
		return 0, 0, err
	}
	if len(tags[256]) == 0 || len(tags[257]) == 0 {
		return 0, 0, errors.New("tiff: no image size")
	}
	return int(tags[256][0]), int(tags[257][0]), nil
}

// tiffDirectory reads a classic TIFF's byte order and first directory.
func tiffDirectory(data []byte) (binary.ByteOrder, map[uint16][]uint32, error) {
	if len(data) < 8 {
		return nil, nil, errors.New("tiff: file too short")
	}
	var bo binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return nil, nil, errors.New("tiff: not a TIFF file")
	}
	if bo.Uint16(data[2:4]) != 42 {
		return nil, nil, errors.New("tiff: not a classic TIFF")
	}
	tags, err := readIFD(data, bo, bo.Uint32(data[4:8]))
	return bo, tags, err
}

// readIFD reads the SHORT and LONG tags of the image file directory at off.
func readIFD(data []byte, bo binary.ByteOrder, off uint32) (map[uint16][]uint32, error) {
	if uint64(off)+2 > uint64(len(data)) {
		return nil, errors.New("tiff: directory outside the file")
	}
	n := int(bo.Uint16(data[off:]))
	if uint64(off)+2+uint64(n)*12 > uint64(len(data)) {
		return nil, errors.New("tiff: directory outside the file")
	}
	tags := make(map[uint16][]uint32, n)
	for i := range n {
		e := data[int(off)+2+i*12:]
		tag, typ, count := bo.Uint16(e), bo.Uint16(e[2:]), bo.Uint32(e[4:])
		size := map[uint16]int{3: 2, 4: 4}[typ] // SHORT, LONG; others are skipped
		if size == 0 || count == 0 || count > 1<<24 {
			continue
		}
		raw := e[8:12]
		if total := uint64(count) * uint64(size); total > 4 {
			at := uint64(bo.Uint32(e[8:]))
			if at+total > uint64(len(data)) {
				return nil, fmt.Errorf("tiff: tag %d outside the file", tag)
			}
			raw = data[at : at+total]
		}
		vals := make([]uint32, count)
		for j := range vals {
			if size == 2 {
				vals[j] = uint32(bo.Uint16(raw[j*2:]))
			} else {
				vals[j] = bo.Uint32(raw[j*4:])
			}
		}
		tags[tag] = vals
	}
	return tags, nil
}

func allEqual(vals []uint32, want uint32) bool {
	if len(vals) == 0 {
		return false
	}
	for _, v := range vals {
		if v != want {
			return false
		}
	}
	return true
}
