package app

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
		if got := isFitsFile(tt.name); got != tt.want {
			t.Errorf("isFitsFile(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
