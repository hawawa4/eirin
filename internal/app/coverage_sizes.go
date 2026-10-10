package app

import (
	"log/slog"
	"sync"

	"github.com/TaruDesigns/eirin/internal/fits"
	"github.com/TaruDesigns/eirin/internal/indexer"
)

// sizeCache remembers image sizes (pixels) read from files, by key.
type sizeCache struct {
	mu    sync.Mutex
	sizes map[string][2]int
}

func (c *sizeCache) get(key string) ([2]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sz, ok := c.sizes[key]
	return sz, ok
}

func (c *sizeCache) put(key string, sz [2]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sizes == nil {
		c.sizes = map[string][2]int{}
	}
	c.sizes[key] = sz
}

// maxSizeProbes caps the headers read per scope when earlier ones fail.
const maxSizeProbes = 3

// scopeSize returns the scope's sensor size, reading the first readable FITS
// header among paths on a cache miss. Zero when none could be read.
func (a *App) scopeSize(scope string, paths []string) [2]int {
	if sz, ok := a.scopeSizes.get(scope); ok {
		return sz
	}
	for _, p := range paths {
		hdr, err := fits.ReadFITSHeader(p)
		if err != nil || hdr.Width <= 0 || hdr.Height <= 0 {
			slog.Warn("coverage: read sensor size", "path", p, "err", err)
			continue
		}
		sz := [2]int{hdr.Width, hdr.Height}
		a.scopeSizes.put(scope, sz)
		return sz
	}
	return [2]int{}
}

// frameSize returns one image's size (FITS header or raster dimensions),
// cached by path. Zero when it can't be read.
func (a *App) frameSize(path string) [2]int {
	if sz, ok := a.frameSizes.get(path); ok {
		return sz
	}
	var w, h int
	var err error
	if indexer.IsRasterFile(path) {
		w, h, err = fits.RasterSize(path)
	} else {
		var hdr *fits.FITSHeader
		hdr, err = fits.ReadFITSHeader(path)
		if err == nil {
			w, h = hdr.Width, hdr.Height
		}
	}
	if err != nil || w <= 0 || h <= 0 {
		slog.Warn("coverage: read image size", "path", path, "err", err)
		return [2]int{}
	}
	sz := [2]int{w, h}
	a.frameSizes.put(path, sz)
	return sz
}
