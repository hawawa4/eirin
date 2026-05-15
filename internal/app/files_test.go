package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRejectFile(t *testing.T) {
	a := newTestApp(t)
	if err := a.RejectFile("/nas/root/Light_001.fits"); err != nil {
		t.Fatalf("RejectFile: %v", err)
	}
}

func TestUnrejectFile(t *testing.T) {
	a := newTestApp(t)
	path := "/nas/root/Light_001.fits"
	a.RejectFile(path)
	if err := a.UnrejectFile(path); err != nil {
		t.Fatalf("UnrejectFile: %v", err)
	}
}

func TestRejectFileNilPrefs(t *testing.T) {
	a := &App{} // no prefs — should be a no-op
	if err := a.RejectFile("/any/path.fits"); err != nil {
		t.Errorf("RejectFile with nil prefs should return nil, got: %v", err)
	}
}

func TestUnrejectFileNilPrefs(t *testing.T) {
	a := &App{}
	if err := a.UnrejectFile("/any/path.fits"); err != nil {
		t.Errorf("UnrejectFile with nil prefs should return nil, got: %v", err)
	}
}

func TestHardDeleteFileRemovesFromDisk(t *testing.T) {
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
	a := newTestApp(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "frame.fits")
	os.WriteFile(path, []byte(""), 0644)

	// Index the frame so a DB record exists.
	a.prefs.RejectFrame(path, "test") // creates a row

	if err := a.HardDeleteFile(path); err != nil {
		t.Fatalf("HardDeleteFile: %v", err)
	}
	frames, _ := a.prefs.GetFrames([]string{path})
	if _, ok := frames[path]; ok {
		t.Error("DB record should be removed after HardDeleteFile")
	}
}

func TestHardDeleteFileNonExistentErrors(t *testing.T) {
	a := newTestApp(t)
	if err := a.HardDeleteFile("/nonexistent/eirin-test.fits"); err == nil {
		t.Error("expected error when deleting non-existent file")
	}
}
