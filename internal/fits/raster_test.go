package fits

import (
	"bytes"
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

func TestRenderRasterDownscales(t *testing.T) {
	for _, name := range []string{"big.png", "big.tiff", "big.jpg"} {
		img, err := RenderRaster(writeRaster(t, name, 200, 100), 64)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 32 {
			t.Errorf("%s: got %v, want 64x32", name, b)
		}
	}
}

func TestRenderRasterNative(t *testing.T) {
	img, err := RenderRaster(writeRaster(t, "a.tif", 40, 30), 0)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 40 || b.Dy() != 30 {
		t.Errorf("got %v, want the native 40x30", b)
	}
}

func TestRasterUnreadable(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.png")
	if err := os.WriteFile(bad, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderRaster(bad, 64); err == nil {
		t.Error("RenderRaster of a broken file succeeded")
	}
	if _, err := RenderRaster(filepath.Join(t.TempDir(), "missing.tif"), 0); err == nil {
		t.Error("RenderRaster of a missing file succeeded")
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
