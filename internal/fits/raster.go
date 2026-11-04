package fits

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg" // registers the JPEG decoder for image.Decode
	_ "image/png"  // registers the PNG decoder for image.Decode
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/tiff" // registers the TIFF decoder for image.Decode
)

// Raster (non-FITS) images: PNG, JPEG and TIFF finals.

// RenderedPreview is a preview image ready to show, with its pixel size.
type RenderedPreview struct {
	DataURL string `json:"dataUrl"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

func isTIFF(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tif" || ext == ".tiff"
}

// RenderRaster decodes a PNG, JPEG or TIFF file as an 8-bit image (Gray or
// RGBA) at most maxSize pixels on its longest side (0 = native size, capped
// at MaxRenderSize).
func RenderRaster(path string, maxSize int) (image.Image, error) {
	if maxSize <= 0 || maxSize > MaxRenderSize {
		maxSize = MaxRenderSize
	}
	img, err := decodeImage(path)
	if err != nil {
		return nil, err
	}
	switch img.(type) {
	case *image.Gray, *image.RGBA:
	default:
		img = toRGBA(img)
	}
	return Shrink(img, maxSize), nil
}

func decodeImage(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil && isTIFF(path) {
		// x/image/tiff has no floating-point support; try our own decoder.
		if fimg, ferr := decodeFloatTIFF(data); ferr == nil {
			return fimg, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// RasterSize returns the pixel size of a PNG, JPEG or TIFF without decoding
// its pixels (floating-point TIFFs: from the TIFF directory).
func RasterSize(path string) (int, int, error) {
	cfg, err := decodeConfig(path)
	if err == nil {
		return cfg.Width, cfg.Height, nil
	}
	if isTIFF(path) {
		if data, rerr := os.ReadFile(path); rerr == nil {
			if w, h, terr := tiffSize(data); terr == nil {
				return w, h, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("decode config: %w", err)
}

func decodeConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return cfg, err
}

func dataURL(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
