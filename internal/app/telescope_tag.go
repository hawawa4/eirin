package app

import (
	"log/slog"

	"github.com/hawawa4/eirin/internal/telescopes"
)

// tagTelescopeByFOV best-effort tags path's telescope from its field of view,
// when a plate solve has given it a pixel scale and it doesn't already have a
// telescope (from a FITS header, a manual edit, or a previous tag). This is
// how stacked/processed outputs — which rarely carry a TELESCOP header, and
// may have inherited an empty or wrong one from their source project's light
// frames — get a telescope label. Mosaics are a best-effort case: a mosaic's
// per-sub pixel scale still matches its originating model, but eirin cannot
// tell a wide-field composite from a single sub, so it doesn't try to.
func (a *App) tagTelescopeByFOV(nasPath string) {
	frames, err := a.store().GetFrames([]string{nasPath})
	if err != nil {
		slog.Warn("telescope tag: get frame", "path", nasPath, "err", err)
		return
	}
	f, ok := frames[nasPath]
	if !ok || f.Telescope != "" || f.PixelScale == nil || *f.PixelScale <= 0 {
		return
	}
	size := a.frameSize(nasPath)
	model, _, ok := telescopes.GuessFromFOV(size[0], size[1], *f.PixelScale)
	if !ok {
		return
	}
	if _, err := a.store().SetTelescopeIfEmpty(nasPath, model); err != nil {
		slog.Warn("telescope tag: write", "path", nasPath, "err", err)
	}
}
