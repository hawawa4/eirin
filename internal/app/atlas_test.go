package app

import (
	"path/filepath"
	"testing"

	"github.com/hawawa4/eirin/internal/store"
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

func TestGetAtlasIndexOnlyShowsPublicImages(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	solvedFrame := func(typ string) store.Frame {
		return store.Frame{FrameType: typ, RA: ptr(83.8), Dec: ptr(-5.4), PixelScale: ptr(2.4), Rotation: ptr(0)}
	}
	shown, hidden, stack := "/nas/M 42/final.png", "/nas/M 31/final.png", "/nas/M 42/stack.fit"
	must(t, a.store().UpsertFrame(shown, solvedFrame(store.FrameTypeImage)))
	must(t, a.store().UpsertFrame(hidden, solvedFrame(store.FrameTypeImage)))
	must(t, a.store().UpsertFrame(stack, solvedFrame(store.FrameTypeStacked)))
	must(t, a.SetFilesPublic([]string{shown}, true))

	got := a.GetAtlasIndex("/nas")
	if len(got) != 1 || got[0].NasPath != shown {
		t.Fatalf("atlas = %+v, want only the public image", got)
	}

	must(t, a.SetFilesPublic([]string{shown}, false))
	if got := a.GetAtlasIndex("/nas"); len(got) != 0 {
		t.Errorf("atlas = %+v after unmarking, want empty", got)
	}
}
