package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.fits"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "b.jpg"), []byte(""), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	entries, err := listDirectory(dir)
	if err != nil {
		t.Fatalf("listDirectory: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}

	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name] = true
		wantPath := filepath.Join(dir, e.Name)
		if e.Path != wantPath {
			t.Errorf("entry %q: Path = %q, want %q", e.Name, e.Path, wantPath)
		}
	}
	if !names["a.fits"] {
		t.Error("a.fits not found in listing")
	}
	if !names["b.jpg"] {
		t.Error("b.jpg not found in listing")
	}
	if !names["subdir"] {
		t.Error("subdir not found in listing")
	}
}

func TestListDirectoryIsDir(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte(""), 0644)

	entries, err := listDirectory(dir)
	if err != nil {
		t.Fatalf("listDirectory: %v", err)
	}

	for _, e := range entries {
		switch e.Name {
		case "sub":
			if !e.IsDir {
				t.Error("sub should be IsDir=true")
			}
		case "file.txt":
			if e.IsDir {
				t.Error("file.txt should be IsDir=false")
			}
		}
	}
}

func TestListDirectoryEmpty(t *testing.T) {
	entries, err := listDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("listDirectory (empty dir): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestListDirectoryNonExistent(t *testing.T) {
	_, err := listDirectory("/nonexistent/eirin-test-path")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestListDirectoryModTimeAndSize(t *testing.T) {
	dir := t.TempDir()
	content := []byte("fits data")
	os.WriteFile(filepath.Join(dir, "frame.fits"), content, 0644)

	entries, _ := listDirectory(dir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Size != int64(len(content)) {
		t.Errorf("Size = %d, want %d", e.Size, len(content))
	}
	if e.ModTime.IsZero() {
		t.Error("ModTime should not be zero")
	}
}
