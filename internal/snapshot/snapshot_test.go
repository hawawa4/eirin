package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestPath(t *testing.T) {
	if got, want := Path("/nas/Astro"), filepath.Join("/nas/Astro", ".eirin", "library.db"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestPublishWritesSnapshot(t *testing.T) {
	root := t.TempDir()
	s := desktopStore(t, root)
	must(t, Publish(s, root))

	if _, err := os.Stat(Path(root) + ".tmp"); err == nil {
		t.Error("temp file left behind")
	}
	c, err := store.NewStoreAt(Path(root))
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer c.Close()
	if n, _ := c.FrameCount(); n != 2 {
		t.Errorf("snapshot has %d frames, want 2", n)
	}
}

func TestPublishOverwrites(t *testing.T) {
	root := t.TempDir()
	s := desktopStore(t, root)
	must(t, Publish(s, root))
	must(t, s.UpsertFrame(filepath.Join(root, "M42", "stack.fits"), store.Frame{FrameType: store.FrameTypeStacked}))
	must(t, Publish(s, root))

	c, err := store.NewStoreAt(Path(root))
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	defer c.Close()
	if n, _ := c.FrameCount(); n != 3 {
		t.Errorf("snapshot has %d frames after republish, want 3", n)
	}
}

func TestPublishCleansUpLocalTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	orig := localTempDir
	localTempDir = func() string { return tmpDir }
	t.Cleanup(func() { localTempDir = orig })

	root := t.TempDir()
	must(t, Publish(desktopStore(t, root), root))

	left, err := os.ReadDir(tmpDir)
	must(t, err)
	if len(left) != 0 {
		t.Errorf("local temp files left behind: %v", left)
	}
	entries, err := os.ReadDir(filepath.Dir(Path(root)))
	must(t, err)
	if len(entries) != 1 || entries[0].Name() != "library.db" {
		t.Errorf("snapshot folder = %v, want only library.db", entries)
	}
}

func TestPublishWithoutRoot(t *testing.T) {
	if err := Publish(newTestStore(t), ""); err == nil {
		t.Error("Publish with no root succeeded")
	}
}
