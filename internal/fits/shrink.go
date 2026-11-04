package fits

import (
	"image"

	"golang.org/x/image/draw"
)

// Shrink returns img scaled down to at most maxSize pixels on its longest
// side by area averaging (each output pixel is the mean of the source pixels
// it covers), or img itself when it already fits. RGBA and Gray images keep
// their type; anything else is converted to RGBA first.
func Shrink(img image.Image, maxSize int) image.Image {
	b := img.Bounds()
	w, h := fitSize(b.Dx(), b.Dy(), maxSize)
	if w == b.Dx() && h == b.Dy() {
		return img
	}
	switch src := img.(type) {
	case *image.Gray:
		dst := image.NewGray(image.Rect(0, 0, w, h))
		shrinkBytes(src.Pix, src.Stride, b.Dx(), b.Dy(), dst.Pix, dst.Stride, w, h, 1)
		return dst
	case *image.RGBA:
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		shrinkBytes(src.Pix, src.Stride, b.Dx(), b.Dy(), dst.Pix, dst.Stride, w, h, 4)
		return dst
	default:
		return Shrink(toRGBA(img), maxSize)
	}
}

// toRGBA converts any image to *image.RGBA with its origin at (0,0).
func toRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok && rgba.Rect.Min == (image.Point{}) {
		return rgba
	}
	b := img.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	return rgba
}

// shrinkBytes box-averages a w×h image of bpp interleaved 8-bit components
// into an outW×outH one (the same bounds as downsampleArea).
func shrinkBytes(src []byte, srcStride, w, h int, dst []byte, dstStride, outW, outH, bpp int) {
	sums := make([]uint32, bpp)
	for oy := range outH {
		y0 := oy * h / outH
		y1 := max(y0+1, (oy+1)*h/outH)
		for ox := range outW {
			x0 := ox * w / outW
			x1 := max(x0+1, (ox+1)*w/outW)
			clear(sums)
			for y := y0; y < y1; y++ {
				row := src[y*srcStride+x0*bpp : y*srcStride+x1*bpp]
				for i := 0; i < len(row); i += bpp {
					for c := range bpp {
						sums[c] += uint32(row[i+c])
					}
				}
			}
			n := uint32((y1 - y0) * (x1 - x0))
			o := oy*dstStride + ox*bpp
			for c := range bpp {
				dst[o+c] = uint8((sums[c] + n/2) / n)
			}
		}
	}
}
