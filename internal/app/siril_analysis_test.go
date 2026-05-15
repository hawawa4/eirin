package app

import (
	"math"
	"testing"
)

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		input   string
		want    float64
		wantErr bool
	}{
		{"3.14", 3.14, false},
		{"3,14", 3.14, false},
		{"0", 0, false},
		{"1234.5678", 1234.5678, false},
		{"invalid", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDecimal(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseDecimal(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("parseDecimal(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// sirilFullOutput is a representative snippet of Siril combined output.
const sirilFullOutput = `
Loading file: Light_001.fits
Found 371 Gaussian profile stars in image, channel #0 (FWHM 3.383251)
Image center: alpha: 06 45 51.505, delta: -20 46 52.259
Resolution:      3.672 arcsec/px
Up is +180.17 deg CounterclockWise wrt. N
Statistics:
  median:        1234.5
  bgnoise:        25.3
  sigma:          30.0
`

func TestParseSirilOutput(t *testing.T) {
	q, err := parseSirilOutput(sirilFullOutput)
	if err != nil {
		t.Fatalf("parseSirilOutput error: %v", err)
	}
	if q.StarCount != 371 {
		t.Errorf("StarCount = %d, want 371", q.StarCount)
	}
	if math.Abs(q.FWHM-3.383251) > 1e-5 {
		t.Errorf("FWHM = %f, want 3.383251", q.FWHM)
	}
	if q.FWHMUnit != "px" {
		t.Errorf("FWHMUnit = %q, want px", q.FWHMUnit)
	}
	if math.Abs(q.Background-1234.5) > 1e-6 {
		t.Errorf("Background = %f, want 1234.5", q.Background)
	}
	// bgnoise should be preferred over sigma
	if math.Abs(q.Noise-25.3) > 1e-6 {
		t.Errorf("Noise = %f, want 25.3 (bgnoise preferred over sigma)", q.Noise)
	}
	wantSNR := 1234.5 / 25.3
	if math.Abs(q.SNR-wantSNR) > 1e-6 {
		t.Errorf("SNR = %f, want %f", q.SNR, wantSNR)
	}
}

func TestParseSirilOutputCommaDecimalSeparator(t *testing.T) {
	output := "Found 100 Gaussian profile stars in image, channel #0 (FWHM 2,500)\nmedian: 1000,0\nbgnoise: 10,0\n"
	q, err := parseSirilOutput(output)
	if err != nil {
		t.Fatalf("parseSirilOutput error: %v", err)
	}
	if q.StarCount != 100 {
		t.Errorf("StarCount = %d, want 100", q.StarCount)
	}
	if math.Abs(q.FWHM-2.5) > 1e-6 {
		t.Errorf("FWHM = %f, want 2.5", q.FWHM)
	}
}

func TestParseSirilOutputNoRecognisableData(t *testing.T) {
	_, err := parseSirilOutput("Siril 1.4.0\nloading...\n")
	if err == nil {
		t.Error("expected error for output with no recognizable data")
	}
}

func TestParseSirilOutputFallsBackToSigma(t *testing.T) {
	// No bgnoise line — sigma should be used as fallback
	output := "Found 50 Gaussian profile stars in image, channel #0 (FWHM 4.0)\nmedian: 800\nsigma: 8.0\n"
	q, err := parseSirilOutput(output)
	if err != nil {
		t.Fatalf("parseSirilOutput error: %v", err)
	}
	if math.Abs(q.Noise-8.0) > 1e-9 {
		t.Errorf("Noise = %f, want 8.0 (sigma fallback)", q.Noise)
	}
}

func TestParseSirilOutputSNRZeroWhenNoiseMissing(t *testing.T) {
	output := "Found 10 Gaussian profile stars in image, channel #0 (FWHM 2.0)\nmedian: 500\n"
	q, err := parseSirilOutput(output)
	if err != nil {
		t.Fatalf("parseSirilOutput error: %v", err)
	}
	if q.SNR != 0 {
		t.Errorf("SNR should be 0 when noise is missing, got %f", q.SNR)
	}
}

// ── Plate solve parser ────────────────────────────────────────────────────────

func TestParsePlateSolveOutput(t *testing.T) {
	wcs, ok := parsePlateSolveOutput(sirilFullOutput)
	if !ok {
		t.Fatal("expected plate solve to succeed")
	}
	// RA: 06h 45m 51.505s → decimal degrees
	wantRA := (6.0 + 45.0/60.0 + 51.505/3600.0) * 15.0
	if math.Abs(wcs.RA-wantRA) > 1e-3 {
		t.Errorf("RA = %f, want %f", wcs.RA, wantRA)
	}
	// Dec: -20° 46' 52.259"
	wantDec := -(20.0 + 46.0/60.0 + 52.259/3600.0)
	if math.Abs(wcs.Dec-wantDec) > 1e-3 {
		t.Errorf("Dec = %f, want %f", wcs.Dec, wantDec)
	}
	if math.Abs(wcs.PixelScale-3.672) > 1e-6 {
		t.Errorf("PixelScale = %f, want 3.672", wcs.PixelScale)
	}
	if math.Abs(wcs.Rotation-180.17) > 1e-6 {
		t.Errorf("Rotation = %f, want 180.17", wcs.Rotation)
	}
}

func TestParsePlateSolveOutputPositiveDec(t *testing.T) {
	output := "alpha: 10 30 00.000, delta: 41 16 09.000\n"
	wcs, ok := parsePlateSolveOutput(output)
	if !ok {
		t.Fatal("expected plate solve to succeed")
	}
	wantDec := 41.0 + 16.0/60.0 + 9.0/3600.0
	if math.Abs(wcs.Dec-wantDec) > 1e-3 {
		t.Errorf("Dec = %f, want %f (positive dec)", wcs.Dec, wantDec)
	}
}

func TestParsePlateSolveOutputMissingDec(t *testing.T) {
	output := "alpha: 06 45 51.505\n" // no delta line
	_, ok := parsePlateSolveOutput(output)
	if ok {
		t.Error("expected plate solve to fail without Dec")
	}
}

func TestParsePlateSolveOutputMissingRA(t *testing.T) {
	output := "delta: -20 46 52.259\n" // no alpha line
	_, ok := parsePlateSolveOutput(output)
	if ok {
		t.Error("expected plate solve to fail without RA")
	}
}

func TestParsePlateSolveOutputEmpty(t *testing.T) {
	_, ok := parsePlateSolveOutput("")
	if ok {
		t.Error("expected plate solve to fail on empty output")
	}
}
