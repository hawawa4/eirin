package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestCheckServable(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	known := filepath.Join(dir, "stack.fits")
	unknown := filepath.Join(dir, "light.fits")
	must(t, a.store().UpsertFrame(known, store.Frame{FrameType: store.FrameTypeStacked}))

	if err := a.checkServable(known); err != nil {
		t.Errorf("checkServable(library frame) = %v", err)
	}
	err := a.checkServable(unknown)
	if desktopModeEnabled && err != nil {
		t.Errorf("desktop build: checkServable(any path) = %v, want nil", err)
	}
	if !desktopModeEnabled && !errors.Is(err, errNotInLibrary) {
		t.Errorf("server build: checkServable(unknown) = %v, want errNotInLibrary", err)
	}
}

// TestServerRefusesPathsOutsideLibrary checks the path-taking read methods
// refuse files that aren't final images in the loaded snapshot.
func TestServerRefusesPathsOutsideLibrary(t *testing.T) {
	if desktopModeEnabled {
		t.Skip("server builds only")
	}
	a := newTestApp(t)
	dir := t.TempDir()
	other := filepath.Join(dir, "light.png")
	must(t, os.WriteFile(other, []byte("x"), 0o644))

	calls := map[string]func() error{
		"ReadFITSHeader":          func() error { _, err := a.ReadFITSHeader(other); return err },
		"GeneratePreview":         func() error { _, err := a.GeneratePreview(other, 2); return err },
		"GeneratePreviewRawSized": func() error { _, err := a.GeneratePreviewRawSized(other, 0); return err },
		"LoadRasterImage":         func() error { _, err := a.LoadRasterImage(other); return err },
		"GetAtlasFrameSize":       func() error { _, err := a.GetAtlasFrameSize(other); return err },
		"GetViewerPreview":        func() error { _, err := a.GetViewerPreview(other, 2); return err },
		"ListDirectory":           func() error { _, err := a.ListDirectory(dir); return err },
		"ListDirectoryEnriched":   func() error { _, err := a.ListDirectoryEnriched(dir); return err },
	}
	for name, call := range calls {
		if err := call(); !errors.Is(err, errNotInLibrary) {
			t.Errorf("%s() = %v, want errNotInLibrary", name, err)
		}
	}
}
