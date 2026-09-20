package indexer

import "strings"

// ProgressEvent describes the current state of an index run.
type ProgressEvent struct {
	Phase   string `json:"phase"`   // "scanning" | "indexing" | "done" | "cancelled"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Indexed int    `json:"indexed"` // files newly read and added to cache
	Errors  int    `json:"errors"`
	Current string `json:"current"` // short display name of file being processed
}

// IsFitsFile reports whether the given filename has a FITS extension.
func IsFitsFile(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".fits") || strings.HasSuffix(l, ".fit")
}

// IsRasterFile reports whether the given filename is a plain raster image
// (PNG, TIFF, JPEG) that should be indexed as a processed frame without
// parsing a FITS header.
func IsRasterFile(name string) bool {
	l := strings.ToLower(name)
	for _, ext := range []string{".png", ".tif", ".tiff", ".jpg", ".jpeg"} {
		if strings.HasSuffix(l, ext) {
			return true
		}
	}
	return false
}
