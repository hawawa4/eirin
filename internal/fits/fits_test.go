package fits

import (
	"encoding/binary"
	"math"
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

func TestPackF32(t *testing.T) {
	dst := make([]byte, 4)
	packF32(dst, 0, 1.0)

	// IEEE 754: float32(1.0) = 0x3F800000, stored little-endian.
	bits := binary.LittleEndian.Uint32(dst)
	if bits != math.Float32bits(1.0) {
		t.Errorf("packed bits = 0x%08X, want 0x%08X", bits, math.Float32bits(1.0))
	}
}

func TestPackF32Zero(t *testing.T) {
	dst := make([]byte, 4)
	packF32(dst, 0, 0.0)
	for i, b := range dst {
		if b != 0 {
			t.Errorf("packF32(0): byte[%d] = %d, want 0", i, b)
		}
	}
}

func TestPackF32Offset(t *testing.T) {
	// Verify writing at a non-zero offset does not corrupt surrounding bytes.
	dst := make([]byte, 12)
	dst[0] = 0xFF
	dst[1] = 0xFF
	dst[2] = 0xFF
	dst[3] = 0xFF
	packF32(dst, 4, 2.0) // write at offset 4
	dst[8] = 0xFF
	dst[9] = 0xFF
	dst[10] = 0xFF
	dst[11] = 0xFF

	bits := binary.LittleEndian.Uint32(dst[4:8])
	if bits != math.Float32bits(2.0) {
		t.Errorf("offset write: bits = 0x%08X, want 0x%08X", bits, math.Float32bits(2.0))
	}
	// Surrounding bytes must be untouched.
	for i := 0; i < 4; i++ {
		if dst[i] != 0xFF {
			t.Errorf("byte before offset corrupted: dst[%d] = 0x%02X", i, dst[i])
		}
	}
}
