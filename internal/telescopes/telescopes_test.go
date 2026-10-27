package telescopes

import "testing"

func TestGuessFromFOV_S50(t *testing.T) {
	// Same fixture as coverage_test.go's s50 field.
	model, conf, ok := GuessFromFOV(1080, 1920, 2.37)
	if !ok || model != "S50" {
		t.Fatalf("got model=%q ok=%v, want S50", model, ok)
	}
	if conf <= 0 || conf > 1 {
		t.Errorf("confidence = %v, want (0,1]", conf)
	}
}

func TestGuessFromFOV_S50Pro_DisambiguatedBySize(t *testing.T) {
	// S50 and S50 Pro share pixel scale; dimensions must disambiguate.
	model, _, ok := GuessFromFOV(3840, 2160, 2.30)
	if !ok || model != "S50 Pro" {
		t.Fatalf("got model=%q ok=%v, want S50 Pro", model, ok)
	}
	model, _, ok = GuessFromFOV(1920, 1080, 2.30)
	if !ok || model != "S50" {
		t.Fatalf("got model=%q ok=%v, want S50", model, ok)
	}
}

func TestGuessFromFOV_S30(t *testing.T) {
	model, _, ok := GuessFromFOV(1920, 1080, 3.99)
	if !ok || model != "S30" {
		t.Fatalf("got model=%q ok=%v, want S30", model, ok)
	}
}

func TestGuessFromFOV_S30Pro(t *testing.T) {
	model, _, ok := GuessFromFOV(3840, 2160, 3.74)
	if !ok || model != "S30 Pro" {
		t.Fatalf("got model=%q ok=%v, want S30 Pro", model, ok)
	}
}

func TestGuessFromFOV_NoDimensions_StillMatchesOnScaleAlone(t *testing.T) {
	// Mosaic/cropped/stacked outputs may not carry native sensor dimensions.
	model, _, ok := GuessFromFOV(0, 0, 3.99)
	if !ok || model != "S30" {
		t.Fatalf("got model=%q ok=%v, want S30", model, ok)
	}
}

func TestGuessFromFOV_OutOfRange(t *testing.T) {
	if _, _, ok := GuessFromFOV(1920, 1080, 10.0); ok {
		t.Error("want no match for an unrecognized pixel scale")
	}
}

func TestGuessFromFOV_InvalidScale(t *testing.T) {
	if _, _, ok := GuessFromFOV(1920, 1080, 0); ok {
		t.Error("want no match for a non-positive pixel scale")
	}
	if _, _, ok := GuessFromFOV(1920, 1080, -1); ok {
		t.Error("want no match for a negative pixel scale")
	}
}
