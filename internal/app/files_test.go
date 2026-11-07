package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRejectFile(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	if err := a.RejectFile("/nas/root/Light_001.fits"); err != nil {
		t.Fatalf("RejectFile: %v", err)
	}
}

func TestUnrejectFile(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	path := "/nas/root/Light_001.fits"
	a.RejectFile(path)
	if err := a.UnrejectFile(path); err != nil {
		t.Fatalf("UnrejectFile: %v", err)
	}
}

func TestRejectFileNilPrefs(t *testing.T) {
	skipInServer(t)
	a := &App{} // no prefs — should be a no-op
	if err := a.RejectFile("/any/path.fits"); err != nil {
		t.Errorf("RejectFile with nil prefs should return nil, got: %v", err)
	}
}

func TestUnrejectFileNilPrefs(t *testing.T) {
	skipInServer(t)
	a := &App{}
	if err := a.UnrejectFile("/any/path.fits"); err != nil {
		t.Errorf("UnrejectFile with nil prefs should return nil, got: %v", err)
	}
}

func TestHardDeleteFileRemovesFromDisk(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	path := filepath.Join(t.TempDir(), "to_delete.fits")
	if err := os.WriteFile(path, []byte("fake fits"), 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	if err := a.HardDeleteFile(path); err != nil {
		t.Fatalf("HardDeleteFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist on disk after HardDeleteFile")
	}
}

func TestHardDeleteFileAlsoRemovesDBRecord(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.fits")
	os.WriteFile(path, []byte(""), 0644)

	// Index the frame so a DB record exists.
	a.store().RejectFrame(path, "test") // creates a row

	if err := a.HardDeleteFile(path); err != nil {
		t.Fatalf("HardDeleteFile: %v", err)
	}
	frames, _ := a.store().GetFrames([]string{path})
	if _, ok := frames[path]; ok {
		t.Error("DB record should be removed after HardDeleteFile")
	}
}

// A file that vanished from disk must still be removable from the library.
func TestHardDeleteFileMissingOnDiskRemovesDBRecord(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	path := filepath.Join(t.TempDir(), "gone.fits") // never created
	a.store().RejectFrame(path, "test")             // creates a row

	if err := a.HardDeleteFile(path); err != nil {
		t.Fatalf("HardDeleteFile on a missing file: %v", err)
	}
	frames, _ := a.store().GetFrames([]string{path})
	if _, ok := frames[path]; ok {
		t.Error("stale DB record should be removed")
	}
}

func TestBatchHardDeleteFilesMissingOnDiskRemovesDBRecords(t *testing.T) {
	skipInServer(t)
	a := newTestApp(t)
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone.fits")
	there := filepath.Join(dir, "there.fits")
	os.WriteFile(there, []byte("x"), 0644)
	a.store().RejectFrame(gone, "test")
	a.store().RejectFrame(there, "test")

	if err := a.BatchHardDeleteFiles([]string{gone, there}); err != nil {
		t.Fatalf("BatchHardDeleteFiles: %v", err)
	}
	frames, _ := a.store().GetFrames([]string{gone, there})
	if len(frames) != 0 {
		t.Errorf("expected both records removed, got %d", len(frames))
	}
}
