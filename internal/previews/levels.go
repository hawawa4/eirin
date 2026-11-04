// Package previews serves display images of library files (FITS stretched to
// 8 bits, PNG/JPEG/TIFF as they are) as JPEGs at a few fixed resolutions.
//
// A file is rendered once at its native size; every level is then shrunk
// from that render, so all levels look the same and zooming in only swaps in
// sharper pixels. Results are kept on disk (surviving restarts, shared by
// every viewer of the server) and the most recent ones in memory.
package previews

// Levels are the longest-side sizes previews come in, smallest first. A
// requested size snaps up to the next level; above the largest, or at or
// above the file's own size, the native image is served.
var Levels = []int{512, 1024, 2048, 4096}

// Native is the level of the image at its own size (capped at
// fits.MaxRenderSize).
const Native = 0

// SnapLevel returns the level serving a request for size pixels on the
// longest side (0 or less: native).
func SnapLevel(size int) int {
	if size <= 0 {
		return Native
	}
	for _, l := range Levels {
		if size <= l {
			return l
		}
	}
	return Native
}

// effectiveLevel maps level to Native when the image isn't larger than it,
// so the same pixels are never stored twice.
func effectiveLevel(level, nativeLong int) int {
	if level != Native && nativeLong > 0 && level >= nativeLong {
		return Native
	}
	return level
}

// levelsFor returns every distinct level of an image whose longest side is
// nativeLong: the shrunk levels smaller than it, largest first, then Native.
func levelsFor(nativeLong int) []int {
	var out []int
	for i := len(Levels) - 1; i >= 0; i-- {
		if Levels[i] < nativeLong {
			out = append(out, Levels[i])
		}
	}
	return append(out, Native)
}
