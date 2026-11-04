package fits

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// rawFITS builds a FITS file by hand: the given cards (after SIMPLE, BITPIX
// and the NAXIS cards) and big-endian data, both padded to 2880-byte blocks.
func rawFITS(bitpix int, axes []int, cards []string, data []byte) []byte {
	var hdr bytes.Buffer
	card := func(s string) { hdr.WriteString(fmt.Sprintf("%-80s", s)) }
	card(fmt.Sprintf("%-8s= %20s", "SIMPLE", "T"))
	card(fmt.Sprintf("%-8s= %20d", "BITPIX", bitpix))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS", len(axes)))
	for i, a := range axes {
		card(fmt.Sprintf("%-8s= %20d", fmt.Sprintf("NAXIS%d", i+1), a))
	}
	for _, c := range cards {
		card(c)
	}
	card("END")
	pad := func(b []byte, fill byte) []byte {
		for len(b)%fitsBlockSize != 0 {
			b = append(b, fill)
		}
		return b
	}
	return append(pad(hdr.Bytes(), ' '), pad(data, 0)...)
}

func writeTemp(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadFITSHeaderSkipsData(t *testing.T) {
	// A 3000×2000×3 float image whose data unit is missing entirely: the
	// header is all that may be read.
	file := rawFITS(-32, []int{3000, 2000, 3}, []string{
		fmt.Sprintf("%-8s= %-20s", "OBJECT", "'M 42'"),
		fmt.Sprintf("%-8s= %20s", "EXPTIME", "10.0"),
		fmt.Sprintf("%-8s= %-20s", "CUSTOM", "'kept'"),
	}, nil)
	h, err := ReadFITSHeader(writeTemp(t, "big.fit", file))
	if err != nil {
		t.Fatal(err)
	}
	if h.Width != 3000 || h.Height != 2000 || h.Channels != 3 || h.BitPix != -32 {
		t.Errorf("got %d×%d×%d bitpix %d, want 3000×2000×3 bitpix -32", h.Width, h.Height, h.Channels, h.BitPix)
	}
	if h.Object != "M 42" || h.ExpTime != 10 {
		t.Errorf("object %q exptime %v", h.Object, h.ExpTime)
	}
	if h.Extra["CUSTOM"] != "kept" {
		t.Errorf("extra = %v, want CUSTOM=kept", h.Extra)
	}
	if _, ok := h.Extra["NAXIS"]; ok {
		t.Error("NAXIS leaked into extra")
	}
}

func TestReadFITSHeaderRejectsNonFITS(t *testing.T) {
	p := writeTemp(t, "x.fit", bytes.Repeat([]byte("not a fits file "), 400))
	if _, err := ReadFITSHeader(p); err == nil {
		t.Error("expected an error for a non-FITS file")
	}
	if _, err := ReadFITSHeader(filepath.Join(t.TempDir(), "missing.fit")); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestReadPixelsBitpix(t *testing.T) {
	be := func(vals ...any) []byte {
		var b bytes.Buffer
		for _, v := range vals {
			_ = binary.Write(&b, binary.BigEndian, v)
		}
		return b.Bytes()
	}
	cases := []struct {
		name   string
		bitpix int
		cards  []string
		data   []byte
		want   []float32
	}{
		{"uint8", 8, nil, []byte{0, 128, 255, 7}, []float32{0, 128, 255, 7}},
		{"int16 unsigned via BZERO", 16, []string{fmt.Sprintf("%-8s= %20s", "BZERO", "32768")},
			be(int16(-32768), int16(0), int16(32767), int16(1)), []float32{0, 32768, 65535, 32769}},
		{"int32 scaled", 32, []string{fmt.Sprintf("%-8s= %20s", "BSCALE", "0.5"), fmt.Sprintf("%-8s= %20s", "BZERO", "10")},
			be(int32(2), int32(-4), int32(0), int32(100)), []float32{11, 8, 10, 60}},
		{"int64", 64, nil, be(int64(1), int64(2), int64(3), int64(-4)), []float32{1, 2, 3, -4}},
		{"float32", -32, nil, be(float32(0.25), float32(-1), float32(3.5), float32(0)), []float32{0.25, -1, 3.5, 0}},
		{"float64", -64, nil, be(0.5, 1.5, 2.5, -3.0), []float32{0.5, 1.5, 2.5, -3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := writeTemp(t, "img.fit", rawFITS(c.bitpix, []int{2, 2}, c.cards, c.data))
			ph, px, err := readPixels(p)
			if err != nil {
				t.Fatal(err)
			}
			if len(ph.axes) != 2 || ph.axes[0] != 2 || ph.axes[1] != 2 {
				t.Errorf("axes = %v, want [2 2]", ph.axes)
			}
			for i := range c.want {
				if math.Abs(float64(px[i]-c.want[i])) > 1e-6 {
					t.Errorf("pixel %d = %v, want %v", i, px[i], c.want[i])
				}
			}
		})
	}
}

func TestReadPixelsTruncated(t *testing.T) {
	file := rawFITS(-32, []int{100, 100}, nil, make([]byte, 4*10))
	if _, _, err := readPixels(writeTemp(t, "short.fit", file)); err == nil {
		t.Error("expected an error for a truncated data unit")
	}
}
