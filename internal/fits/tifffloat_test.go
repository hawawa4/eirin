package fits

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// floatTIFF builds a TIFF of float32 samples (interleaved, spp per pixel),
// split into strips of rowsPerStrip rows, optionally Deflate-compressed.
func floatTIFF(t *testing.T, bo binary.ByteOrder, w, h, spp int, vals []float32, deflate bool, rowsPerStrip int) []byte {
	t.Helper()
	var strips [][]byte
	rowBytes := w * spp * 4
	for y := 0; y < h; y += rowsPerStrip {
		rows := min(rowsPerStrip, h-y)
		raw := make([]byte, rows*rowBytes)
		for i := range rows * w * spp {
			bo.PutUint32(raw[i*4:], math.Float32bits(vals[y*w*spp+i]))
		}
		if deflate {
			var buf bytes.Buffer
			zw := zlib.NewWriter(&buf)
			_, _ = zw.Write(raw)
			_ = zw.Close()
			raw = buf.Bytes()
		}
		strips = append(strips, raw)
	}

	type entry struct {
		tag, typ uint16
		vals     []uint32
	}
	compression := uint32(1)
	if deflate {
		compression = 8
	}
	perSample := func(v uint32) []uint32 {
		out := make([]uint32, spp)
		for i := range out {
			out[i] = v
		}
		return out
	}
	photometric := uint32(1)
	if spp >= 3 {
		photometric = 2
	}
	var offsets, counts []uint32
	entries := []entry{
		{256, 4, []uint32{uint32(w)}},
		{257, 4, []uint32{uint32(h)}},
		{258, 3, perSample(32)},
		{259, 3, []uint32{compression}},
		{262, 3, []uint32{photometric}},
		{273, 4, nil}, // strip offsets, filled in below
		{277, 3, []uint32{uint32(spp)}},
		{278, 4, []uint32{uint32(rowsPerStrip)}},
		{279, 4, nil}, // strip byte counts
		{339, 3, perSample(3)},
	}

	// Layout: header, strips, out-of-line tag values, IFD.
	var out bytes.Buffer
	if bo == binary.LittleEndian {
		out.WriteString("II")
	} else {
		out.WriteString("MM")
	}
	_ = binary.Write(&out, bo, uint16(42))
	_ = binary.Write(&out, bo, uint32(0)) // IFD offset, patched below
	for _, s := range strips {
		offsets = append(offsets, uint32(out.Len()))
		counts = append(counts, uint32(len(s)))
		out.Write(s)
	}
	entries[5].vals, entries[8].vals = offsets, counts

	valueAt := make([]uint32, len(entries))
	for i, e := range entries {
		size := 4
		if e.typ == 3 {
			size = 2
		}
		if len(e.vals)*size <= 4 {
			continue
		}
		valueAt[i] = uint32(out.Len())
		for _, v := range e.vals {
			if size == 2 {
				_ = binary.Write(&out, bo, uint16(v))
			} else {
				_ = binary.Write(&out, bo, v)
			}
		}
	}
	ifd := uint32(out.Len())
	_ = binary.Write(&out, bo, uint16(len(entries)))
	for i, e := range entries {
		_ = binary.Write(&out, bo, e.tag)
		_ = binary.Write(&out, bo, e.typ)
		_ = binary.Write(&out, bo, uint32(len(e.vals)))
		size := 4
		if e.typ == 3 {
			size = 2
		}
		if len(e.vals)*size > 4 {
			_ = binary.Write(&out, bo, valueAt[i])
			continue
		}
		var inline [4]byte
		for j, v := range e.vals {
			if size == 2 {
				bo.PutUint16(inline[j*2:], uint16(v))
			} else {
				bo.PutUint32(inline[:], v)
			}
		}
		out.Write(inline[:])
	}
	_ = binary.Write(&out, bo, uint32(0)) // no next IFD
	data := out.Bytes()
	bo.PutUint32(data[4:], ifd)
	return data
}

func rgba16(img image.Image, x, y int) [3]uint32 {
	r, g, b, _ := img.At(x, y).RGBA()
	return [3]uint32{r, g, b}
}

func TestDecodeFloatTIFFRGB(t *testing.T) {
	// 2×2 RGB: black, white, mid grey, pure red.
	vals := []float32{0, 0, 0, 1, 1, 1, 0.5, 0.5, 0.5, 1, 0, 0}
	for _, deflate := range []bool{false, true} {
		img, err := decodeFloatTIFF(floatTIFF(t, binary.LittleEndian, 2, 2, 3, vals, deflate, 1))
		if err != nil {
			t.Fatalf("deflate=%v: %v", deflate, err)
		}
		if got := img.Bounds().Size(); got != (image.Point{2, 2}) {
			t.Fatalf("size = %v", got)
		}
		want := map[image.Point][3]uint32{
			{0, 0}: {0, 0, 0},
			{1, 0}: {65535, 65535, 65535},
			{0, 1}: {32768, 32768, 32768},
			{1, 1}: {65535, 0, 0},
		}
		for p, w := range want {
			if got := rgba16(img, p.X, p.Y); got != w {
				t.Errorf("deflate=%v: pixel %v = %v, want %v", deflate, p, got, w)
			}
		}
	}
}

func TestDecodeFloatTIFFScalesValuesAboveOne(t *testing.T) {
	// Grey, big-endian, values up to 4: scaled so the peak is white.
	img, err := decodeFloatTIFF(floatTIFF(t, binary.BigEndian, 2, 1, 1, []float32{2, 4}, false, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := rgba16(img, 0, 0); got != [3]uint32{32768, 32768, 32768} {
		t.Errorf("half of peak = %v", got)
	}
	if got := rgba16(img, 1, 0); got != [3]uint32{65535, 65535, 65535} {
		t.Errorf("peak = %v", got)
	}
}

func TestDecodeFloatTIFFClipsRareOvershoot(t *testing.T) {
	// 2000 grey pixels at 0.5 and one hot pixel at 3: the hot pixel is
	// clipped instead of darkening the whole image.
	vals := make([]float32, 2001)
	for i := range vals {
		vals[i] = 0.5
	}
	vals[2000] = 3
	img, err := decodeFloatTIFF(floatTIFF(t, binary.LittleEndian, 2001, 1, 1, vals, false, 1))
	if err != nil {
		t.Fatal(err)
	}
	if got := rgba16(img, 0, 0); got != [3]uint32{32768, 32768, 32768} {
		t.Errorf("0.5 = %v, want unscaled mid grey", got)
	}
	if got := rgba16(img, 2000, 0); got != [3]uint32{65535, 65535, 65535} {
		t.Errorf("hot pixel = %v, want clipped to white", got)
	}
}

func TestDecodeFloatTIFFRejects(t *testing.T) {
	good := floatTIFF(t, binary.LittleEndian, 2, 2, 3, make([]float32, 12), false, 2)
	if _, err := decodeFloatTIFF(good[:len(good)/2]); err == nil {
		t.Error("truncated file decoded")
	}
	if _, err := decodeFloatTIFF([]byte("not a tiff at all")); err == nil {
		t.Error("garbage decoded")
	}
	// An 8-bit TIFF is for x/image/tiff, not this decoder.
	eightBit, err := os.ReadFile(writeRaster(t, "a.tif", 4, 4))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeFloatTIFF(eightBit); err == nil {
		t.Error("8-bit TIFF accepted as floating point")
	}
}

func TestRasterPreviewOfFloatTIFF(t *testing.T) {
	vals := make([]float32, 100*50*3)
	for i := range vals {
		vals[i] = float32(i%300) / 300
	}
	path := filepath.Join(t.TempDir(), "siril.tif")
	if err := os.WriteFile(path, floatTIFF(t, binary.LittleEndian, 100, 50, 3, vals, true, 7), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := RenderRasterPreview(path, 40)
	if err != nil {
		t.Fatal(err)
	}
	if p.Width != 40 || p.Height != 20 {
		t.Errorf("preview %dx%d, want 40x20", p.Width, p.Height)
	}
	url, err := RasterDataURL(path)
	if err != nil {
		t.Fatal(err)
	}
	if mime, _, img := decodeDataURL(t, url); mime != "image/png" || img.Bounds().Dx() != 100 {
		t.Errorf("RasterDataURL: %s %v, want a 100-wide PNG", mime, img.Bounds())
	}
}

func TestRasterSizeOfFloatTIFF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "siril.tif")
	if err := os.WriteFile(path, floatTIFF(t, binary.LittleEndian, 30, 20, 3, make([]float32, 30*20*3), false, 20), 0o644); err != nil {
		t.Fatal(err)
	}
	w, h, err := RasterSize(path)
	if err != nil || w != 30 || h != 20 {
		t.Errorf("RasterSize = %dx%d, %v; want 30x20", w, h, err)
	}
}
