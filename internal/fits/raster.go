package fits

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff" // registers the TIFF decoder for image.Decode
)

// Raster (non-FITS) images: PNG, JPEG and TIFF finals.

// RenderedPreview is a preview image ready to show, with its pixel size.
type RenderedPreview struct {
	DataURL string `json:"dataUrl"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
}

func isJPEG(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg"
}

func isTIFF(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tif" || ext == ".tiff"
}

// RasterDataURL returns a raster image as a data URL a browser can display:
// PNG and JPEG files as they are, TIFF (which browsers can't show) converted
// to an 8-bit PNG at full resolution (browsers draw 8 bits anyway; 16 would
// double the size).
func RasterDataURL(path string) (string, error) {
	if isTIFF(path) {
		img, err := decodeImage(path)
		if err != nil {
			return "", err
		}
		rgba := image.NewRGBA(img.Bounds())
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
		var buf bytes.Buffer
		if err := png.Encode(&buf, rgba); err != nil {
			return "", fmt.Errorf("encode PNG: %w", err)
		}
		return dataURL("image/png", buf.Bytes()), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	mime := "image/png"
	if isJPEG(path) {
		mime = "image/jpeg"
	}
	return dataURL(mime, data), nil
}

// RenderRasterPreview returns a JPEG of the image at most maxSize pixels on
// its longest side. A JPEG that already fits is returned as it is.
func RenderRasterPreview(path string, maxSize int) (RenderedPreview, error) {
	if isJPEG(path) {
		if cfg, err := decodeConfig(path); err == nil && cfg.Width <= maxSize && cfg.Height <= maxSize {
			data, err := os.ReadFile(path)
			if err != nil {
				return RenderedPreview{}, fmt.Errorf("read file: %w", err)
			}
			return RenderedPreview{DataURL: dataURL("image/jpeg", data), Width: cfg.Width, Height: cfg.Height}, nil
		}
	}
	img, err := decodeImage(path)
	if err != nil {
		return RenderedPreview{}, err
	}
	b := img.Bounds()
	w, h := fitSize(b.Dx(), b.Dy(), maxSize)
	if w != b.Dx() || h != b.Dy() {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: previewJPEGQuality}); err != nil {
		return RenderedPreview{}, fmt.Errorf("encode JPEG: %w", err)
	}
	return RenderedPreview{DataURL: dataURL("image/jpeg", buf.Bytes()), Width: w, Height: h}, nil
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
