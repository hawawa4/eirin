package app

import (
	"path/filepath"
	"testing"
)

func TestGetAtlasFrameSizes(t *testing.T) {
	a, png := newPreviewApp(t) // a 1200×400 PNG in the library
	missing := filepath.Join(t.TempDir(), "gone.fit")
	got := a.GetAtlasFrameSizes([]string{png, missing})
	if sz, ok := got[png]; !ok || sz.Width != 1200 || sz.Height != 400 {
		t.Errorf("size of %s = %+v (present %v), want 1200×400", png, sz, ok)
	}
	if _, ok := got[missing]; ok {
		t.Error("an unreadable file has a size")
	}
	if len(a.GetAtlasFrameSizes(nil)) != 0 {
		t.Error("no paths should give no sizes")
	}
}
