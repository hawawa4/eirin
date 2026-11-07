package app

import (
	"testing"

	"github.com/hawawa4/eirin/internal/store"
)

func TestSetFilesPublic(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	one, two := "/nas/root/m42.fits", "/nas/root/m31.fits"
	for _, p := range []string{one, two} {
		must(t, a.store().UpsertFrame(p, store.Frame{FrameType: store.FrameTypeProcessed}))
	}
	isPublic := func(p string) bool {
		t.Helper()
		frames, err := a.GetLibraryFrames("/nas/root")
		must(t, err)
		for _, f := range frames {
			if f.NasPath == p {
				return f.IsPublic
			}
		}
		t.Fatalf("%s not in the library", p)
		return false
	}

	if isPublic(one) {
		t.Fatal("a new frame must not be public")
	}
	must(t, a.SetFilesPublic([]string{one}, true))
	if !isPublic(one) || isPublic(two) {
		t.Errorf("after marking one: one=%v two=%v, want true/false", isPublic(one), isPublic(two))
	}
	// Re-indexing keeps the mark.
	must(t, a.store().UpsertFrame(one, store.Frame{FrameType: store.FrameTypeProcessed, Object: "M 42"}))
	if !isPublic(one) {
		t.Error("re-indexing a frame cleared its public mark")
	}
	must(t, a.SetFilesPublic([]string{one, two}, false))
	if isPublic(one) || isPublic(two) {
		t.Error("unmarking did not clear the marks")
	}
	// Paths the library doesn't know are ignored, not an error.
	must(t, a.SetFilesPublic([]string{"/nowhere.fits"}, true))
}
