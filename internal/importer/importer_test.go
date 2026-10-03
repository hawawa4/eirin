package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchesExtensions(t *testing.T) {
	tests := []struct {
		name string
		exts []string
		want bool
	}{
		// Basic matches
		{"image.fits", []string{"fits"}, true},
		{"image.fit", []string{"fit"}, true},
		// Case-insensitive filename extension
		{"image.FITS", []string{"fits"}, true},
		{"image.FIT", []string{"fit", "fits"}, true},
		// Case-insensitive filter extension
		{"image.jpg", []string{"JPG"}, true},
		// Multiple extensions in filter
		{"image.fits", []string{"jpg", "fits", "png"}, true},
		// No match
		{"image.jpg", []string{"fits", "fit"}, false},
		// No extension in filename
		{"image", []string{"fits"}, false},
		// Empty extension list matches everything
		{"image.fits", []string{}, true},
		{"image.jpg", []string{}, true},
		{"image", []string{}, true},
		// Extension mismatch
		{"image.fits", []string{"jpg", "png"}, false},
	}

	for _, tt := range tests {
		got := MatchesExtensions(tt.name, tt.exts)
		if got != tt.want {
			t.Errorf("MatchesExtensions(%q, %v) = %v, want %v", tt.name, tt.exts, got, tt.want)
		}
	}
}

func TestHashFileFull(t *testing.T) {
	dir := t.TempDir()
	a, b := prefixCollision()
	pa, pb := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	writeFile(t, pa, a)
	writeFile(t, pb, b)

	pha, _ := HashFilePrefix(pa)
	phb, _ := HashFilePrefix(pb)
	if pha != phb {
		t.Fatal("test setup: prefixes should collide")
	}
	fa, err := HashFileFull(pa)
	if err != nil {
		t.Fatal(err)
	}
	fb, _ := HashFileFull(pb)
	if fa == fb {
		t.Error("full hashes of different files must differ")
	}
	// sha256("") is well known.
	empty := filepath.Join(dir, "empty")
	writeFile(t, empty, nil)
	if h, _ := HashFileFull(empty); h != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("empty hash = %s", h)
	}
	if _, err := HashFileFull(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing file should error")
	}
}

func TestCopyFileIfNotExists(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.fit")
	dst := filepath.Join(dir, "nested", "dst.fit")
	writeFile(t, src, []byte("data"))

	skipped, err := CopyFileIfNotExists(src, dst)
	if err != nil || skipped {
		t.Fatalf("first copy: skipped=%v err=%v", skipped, err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "data" {
		t.Errorf("dst = %q", got)
	}
	writeFile(t, src, []byte("changed"))
	skipped, err = CopyFileIfNotExists(src, dst)
	if err != nil || !skipped {
		t.Fatalf("second copy: skipped=%v err=%v", skipped, err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "data" {
		t.Error("existing destination was overwritten")
	}
	if _, err := CopyFileIfNotExists(filepath.Join(dir, "missing"), filepath.Join(dir, "x")); err == nil {
		t.Error("missing source should error")
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); err == nil {
		t.Error("failed copy left a destination file behind")
	}
}
