package fits

import (
	"testing"
)

// ── Misc helpers ──────────────────────────────────────────────────────────────

func TestToU8(t *testing.T) {
	tests := []struct {
		in   float64
		want uint8
	}{
		{0.0, 0},
		{1.0, 255},
		{0.5, 127},
		{-1.0, 0},
		{2.0, 255},
	}
	for _, tt := range tests {
		if got := toU8(tt.in); got != tt.want {
			t.Errorf("toU8(%f) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
