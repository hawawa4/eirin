package fits

import (
	"encoding/base64"
	"encoding/binary"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fitsio "codeberg.org/astrogo/fitsio"
)

// writeBayerSky writes a w×h GRBG raw sub of a vignetted sky with a strong
// colour cast (R:G:B = 300:600:200, like an unbalanced OSC camera) and a small
// white star in the last rows and columns (top-right once displayed), which
// sets the white point.
func writeBayerSky(t *testing.T, w, h int) string {
	t.Helper()
	sky := map[byte]float32{'R': 300, 'G': 600, 'B': 200}
	pattern := [2][2]byte{{'G', 'R'}, {'B', 'G'}} // GRBG
	data := make([]float32, w*h)
	for y := range h {
		for x := range w {
			dx := (float64(x) - float64(w)/2) / float64(w)
			dy := (float64(y) - float64(h)/2) / float64(h)
			vignette := float32(1 - 1.6*(dx*dx+dy*dy))
			data[y*w+x] = sky[pattern[y%2][x%2]] * vignette
			if x >= w-4 && y >= h-4 {
				data[y*w+x] = 20000
			}
		}
	}

	path := filepath.Join(t.TempDir(), "sky.fit")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ff, err := fitsio.Create(f)
	if err != nil {
		t.Fatal(err)
	}
	img := fitsio.NewImage(-32, []int{w, h})
	if err := img.Header().Append(fitsio.Card{Name: "BAYERPAT", Value: "GRBG"}); err != nil {
		t.Fatal(err)
	}
	if err := img.Write(&data); err != nil {
		t.Fatal(err)
	}
	if err := ff.Write(img); err != nil {
		t.Fatal(err)
	}
	if err := ff.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeRawPixels(t *testing.T, d RawPreviewData) []float32 {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(d.Data)
	if err != nil {
		t.Fatal(err)
	}
	px := make([]float32, len(b)/4)
	for i := range px {
		px[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return px
}

func TestGeneratePreviewRawNeutralisesSkyCast(t *testing.T) {
	path := writeBayerSky(t, 512, 384)
	d, err := GeneratePreviewRaw(path, 2048)
	if err != nil {
		t.Fatal(err)
	}
	if d.Channels != 3 || d.Width != 256 || d.Height != 192 {
		t.Fatalf("got %d channels %d×%d, want 3 channels 256×192", d.Channels, d.Width, d.Height)
	}
	for c, want := range []float64{2, 1, 3} { // brightest (green) background / each channel's
		if math.Abs(d.Balance[c]-want)/want > 0.05 {
			t.Errorf("balance[%d] = %f, want ≈ %f", c, d.Balance[c], want)
		}
	}

	px := decodeRawPixels(t, d)
	p := linkedStretch(d.Stats, d.Balance, 2)
	pixel := func(x, y int) [3]float64 {
		i := (y*d.Width + x) * 4
		return [3]float64{
			p.apply(0, float64(px[i])),
			p.apply(1, float64(px[i+1])),
			p.apply(2, float64(px[i+2])),
		}
	}
	// Displayed rows are top-down: the star is top-right, the tested corner bottom-left.
	centre, corner := pixel(128, 96), pixel(4, 180)
	for _, rgb := range [][3]float64{centre, corner} {
		spread := math.Max(rgb[0], math.Max(rgb[1], rgb[2])) - math.Min(rgb[0], math.Min(rgb[1], rgb[2]))
		if spread > 0.02 {
			t.Errorf("pixel %v is tinted (channel spread %f), want neutral grey", rgb, spread)
		}
	}
	if centre[1] <= corner[1] {
		t.Errorf("centre %f should be brighter than the vignetted corner %f", centre[1], corner[1])
	}
}

func TestGeneratePreviewJPEG(t *testing.T) {
	path := writeBayerSky(t, 64, 48)
	url, err := GeneratePreview(path, 16, 2)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/jpeg;base64,"
	if !strings.HasPrefix(url, prefix) {
		t.Fatalf("not a JPEG data URL: %.40s", url)
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(url, prefix))
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(strings.NewReader(string(b)))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Dx(); got != 16 {
		t.Errorf("width = %d, want 16 (fitted to maxSize)", got)
	}
}

func TestRenderPreviewMatchesRawSize(t *testing.T) {
	path := writeBayerSky(t, 64, 48)
	p, err := RenderPreview(path, 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := GeneratePreviewRaw(path, 20)
	if err != nil {
		t.Fatal(err)
	}
	// Annotations are positioned from these sizes in both preview paths.
	if p.Width != raw.Width || p.Height != raw.Height {
		t.Errorf("RenderPreview size %dx%d, raw preview %dx%d", p.Width, p.Height, raw.Width, raw.Height)
	}
	url, err := GeneratePreview(path, 20, 2)
	if err != nil {
		t.Fatal(err)
	}
	if url != p.DataURL {
		t.Error("GeneratePreview and RenderPreview disagree")
	}
}
