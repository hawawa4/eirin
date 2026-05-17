package importer

import "testing"

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
