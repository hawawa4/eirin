package indexer

import "testing"

func TestIsFitsFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"image.fits", true},
		{"image.fit", true},
		{"image.FITS", true},
		{"image.FIT", true},
		{"image.Fits", true},
		{"image.Fit", true},
		{"image.jpg", false},
		{"image.fits.bak", false},
		{"fits", false},
		{"fit", false},
		{"", false},
		{"noextension", false},
	}

	for _, tt := range tests {
		if got := IsFitsFile(tt.name); got != tt.want {
			t.Errorf("IsFitsFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
