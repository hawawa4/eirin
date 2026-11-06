package gallery

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
)

// Prefix is the folder opaque image paths live in.
const Prefix = "/g/"

// PublicPath is the name a library file is known by to visitors: a stable
// hash of its real path, keeping the extension so the frontend (and the
// preview route) still tell FITS from raster images. It never reveals where
// the file is or what it is called, and is the same after a restart, so links
// to a picture keep working.
func PublicPath(real string) string {
	sum := sha256.Sum256([]byte(real))
	return Prefix + hex.EncodeToString(sum[:8]) + strings.ToLower(filepath.Ext(real))
}

// Index maps the public paths of one library snapshot back to the real ones.
// It is immutable once built, so it can be shared between requests.
type Index struct {
	real map[string]string
}

// NewIndex builds the index of the given real paths.
func NewIndex(paths []string) *Index {
	ix := &Index{real: make(map[string]string, len(paths))}
	for _, p := range paths {
		ix.real[PublicPath(p)] = p
	}
	return ix
}

// Real returns the library path behind a public path. Only files in the
// gallery resolve: anything else (including a real path) is not found.
func (ix *Index) Real(public string) (string, bool) {
	if ix == nil {
		return "", false
	}
	p, ok := ix.real[public]
	return p, ok
}

// Len is the number of files in the index.
func (ix *Index) Len() int {
	if ix == nil {
		return 0
	}
	return len(ix.real)
}
