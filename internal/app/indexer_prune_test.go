package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPruneMissingFrames(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	root := t.TempDir()
	kept := filepath.Join(root, "kept.fits")
	gone := filepath.Join(root, "gone.fits")
	outside := filepath.Join(t.TempDir(), "outside.fits") // missing, but not under root
	if err := os.WriteFile(kept, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{kept, gone, outside} {
		a.store().RejectFrame(p, "test")
	}

	n := a.pruneMissingFrames(context.Background(), root, []string{kept})
	if n != 1 {
		t.Fatalf("pruned %d, want 1", n)
	}
	frames, _ := a.store().GetFrames([]string{kept, gone, outside})
	if _, ok := frames[gone]; ok {
		t.Error("record of the missing file should be removed")
	}
	if _, ok := frames[kept]; !ok {
		t.Error("record of an existing file must stay")
	}
	if _, ok := frames[outside]; !ok {
		t.Error("records outside the root must stay")
	}
}

// An empty scan (e.g. an unmounted NAS) must never wipe the library.
func TestPruneMissingFramesEmptyScanIsNoop(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	root := t.TempDir()
	p := filepath.Join(root, "gone.fits")
	a.store().RejectFrame(p, "test")

	if n := a.pruneMissingFrames(context.Background(), root, nil); n != 0 {
		t.Fatalf("pruned %d on an empty scan, want 0", n)
	}
	if frames, _ := a.store().GetFrames([]string{p}); len(frames) != 1 {
		t.Error("record must survive an empty scan")
	}
}

// A file the walk missed (e.g. behind a symlinked folder) but that still exists stays.
func TestPruneMissingFramesKeepsExistingFilesTheScanMissed(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	root := t.TempDir()
	seen := filepath.Join(root, "seen.fits")
	hidden := filepath.Join(root, "hidden.fits")
	os.WriteFile(hidden, []byte("x"), 0644)
	a.store().RejectFrame(hidden, "test")

	if n := a.pruneMissingFrames(context.Background(), root, []string{seen}); n != 0 {
		t.Fatalf("pruned %d, want 0", n)
	}
}
