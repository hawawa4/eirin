package fits

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/tiff"
)

// writeRaster writes a w×h gradient image to dir/name, encoded by extension.
func writeRaster(t *testing.T, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	var err error
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		err = png.Encode(&buf, img)
	case ".jpg", ".jpeg":
		err = jpeg.Encode(&buf, img, nil)
	case ".tif", ".tiff":
		err = tiff.Encode(&buf, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// decodeDataURL splits a data URL into its MIME type and decoded image.
func decodeDataURL(t *testing.T, url string) (string, []byte, image.Image) {
	t.Helper()
	mime, b64, ok := strings.Cut(strings.TrimPrefix(url, "data:"), ";base64,")
	if !ok {
		t.Fatalf("not a base64 data URL: %.40s", url)
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %s: %v", mime, err)
	}
	return mime, data, img
}

func TestRasterDataURLPassesPNGAndJPEGThrough(t *testing.T) {
	for name, want := range map[string]string{"a.png": "image/png", "b.jpg": "image/jpeg", "c.JPEG": "image/jpeg"} {
		path := writeRaster(t, name, 40, 30)
		url, err := RasterDataURL(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		mime, data, _ := decodeDataURL(t, url)
		orig, _ := os.ReadFile(path)
		if mime != want || !bytes.Equal(data, orig) {
			t.Errorf("%s: got %s (%d bytes), want %s with the original %d bytes", name, mime, len(data), want, len(orig))
		}
	}
}

func TestRasterDataURLConvertsTIFF(t *testing.T) {
	url, err := RasterDataURL(writeRaster(t, "a.tif", 40, 30))
	if err != nil {
		t.Fatal(err)
	}
	mime, _, img := decodeDataURL(t, url)
	if mime != "image/png" || img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 {
		t.Errorf("got %s %v, want a 40x30 PNG", mime, img.Bounds())
	}
}

func TestRenderRasterPreviewDownscales(t *testing.T) {
	for _, name := range []string{"big.png", "big.tiff", "big.jpg"} {
		p, err := RenderRasterPreview(writeRaster(t, name, 200, 100), 64)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		mime, _, img := decodeDataURL(t, p.DataURL)
		if mime != "image/jpeg" || p.Width != 64 || p.Height != 32 || img.Bounds().Dx() != 64 {
			t.Errorf("%s: got %s %dx%d (image %v), want a 64x32 JPEG", name, mime, p.Width, p.Height, img.Bounds())
		}
	}
}

func TestRenderRasterPreviewKeepsSmallJPEG(t *testing.T) {
	path := writeRaster(t, "small.jpg", 50, 40)
	p, err := RenderRasterPreview(path, 64)
	if err != nil {
		t.Fatal(err)
	}
	_, data, _ := decodeDataURL(t, p.DataURL)
	orig, _ := os.ReadFile(path)
	if !bytes.Equal(data, orig) || p.Width != 50 || p.Height != 40 {
		t.Errorf("small JPEG re-encoded or wrong size: %dx%d", p.Width, p.Height)
	}
}

func TestRasterUnreadable(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.png")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderRasterPreview(bad, 64); err == nil {
		t.Error("RenderRasterPreview of a broken file succeeded")
	}
	if _, err := RasterDataURL(filepath.Join(t.TempDir(), "missing.tif")); err == nil {
		t.Error("RasterDataURL of a missing file succeeded")
	}
}

func TestRasterSize(t *testing.T) {
	for _, name := range []string{"a.png", "b.jpg", "c.tiff"} {
		w, h, err := RasterSize(writeRaster(t, name, 30, 20))
		if err != nil || w != 30 || h != 20 {
			t.Errorf("%s: RasterSize = %dx%d, %v; want 30x20", name, w, h, err)
		}
	}
	if _, _, err := RasterSize(filepath.Join(t.TempDir(), "missing.png")); err == nil {
		t.Error("RasterSize of a missing file succeeded")
	}
}
