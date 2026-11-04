package previews

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// writeFITS writes a w×h mono float FITS image with a gradient, so stretching
// has something to work with.
func writeFITS(t *testing.T, dir string, w, h int) string {
	t.Helper()
	var hdr bytes.Buffer
	card := func(s string) { hdr.WriteString(fmt.Sprintf("%-80s", s)) }
	card(fmt.Sprintf("%-8s= %20s", "SIMPLE", "T"))
	card(fmt.Sprintf("%-8s= %20d", "BITPIX", -32))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS", 2))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS1", w))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS2", h))
	card("END")
	data := hdr.Bytes()
	for len(data)%2880 != 0 {
		data = append(data, ' ')
	}
	px := make([]byte, 4*w*h)
	for i := range w * h {
		binary.BigEndian.PutUint32(px[i*4:], math.Float32bits(float32(100+i%w)))
	}
	data = append(data, px...)
	for len(data)%2880 != 0 {
		data = append(data, 0)
	}
	p := filepath.Join(dir, fmt.Sprintf("img-%dx%d.fit", w, h))
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func writePNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	p := filepath.Join(dir, "img.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

// newService returns a Service that has finished all its background work
// (levels still being encoded after Get returned) by the end of the test.
func newService(t *testing.T, dir string, maxBytes int64) *Service {
	t.Helper()
	s := New(dir, maxBytes)
	t.Cleanup(s.running.Wait)
	return s
}

func jpegSize(t *testing.T, b []byte) (int, int) {
	t.Helper()
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	return cfg.Width, cfg.Height
}

func TestSnapLevel(t *testing.T) {
	cases := map[int]int{-1: Native, 0: Native, 1: 512, 512: 512, 513: 1024, 2000: 2048, 4096: 4096, 4097: Native}
	for in, want := range cases {
		if got := SnapLevel(in); got != want {
			t.Errorf("SnapLevel(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestLevelsFor(t *testing.T) {
	if got := fmt.Sprint(levelsFor(3000)); got != "[2048 1024 512 0]" {
		t.Errorf("levelsFor(3000) = %s", got)
	}
	if got := fmt.Sprint(levelsFor(512)); got != "[0]" {
		t.Errorf("levelsFor(512) = %s (a 512 px image is its own native level)", got)
	}
}

func TestGetLevelsAndDiskCache(t *testing.T) {
	src, cache := t.TempDir(), t.TempDir()
	path := writeFITS(t, src, 1500, 600)
	s := newService(t, cache, 1<<30)
	ctx := context.Background()

	cases := []struct{ size, w, h int }{
		{300, 512, 204},
		{1024, 1024, 409},
		{2048, 1500, 600}, // larger than the image: native
		{0, 1500, 600},
	}
	for _, c := range cases {
		img, err := s.Get(ctx, Request{Path: path, Stretch: 2, Size: c.size})
		if err != nil {
			t.Fatal(err)
		}
		if w, h := jpegSize(t, img.Data); w != c.w || h != c.h {
			t.Errorf("size %d: got %d×%d, want %d×%d", c.size, w, h, c.w, c.h)
		}
	}

	// A new service (a restart) serves the same bytes from disk.
	first, _ := s.Get(ctx, Request{Path: path, Stretch: 2, Size: 512})
	s2 := newService(t, cache, 1<<30)
	key, _, _ := source(Request{Path: path, Stretch: 2})
	if !s2.complete(key) {
		t.Fatal("not every level was written to disk")
	}
	again, err := s2.Get(ctx, Request{Path: path, Stretch: 2, Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Data, again.Data) || first.ETag != again.ETag {
		t.Error("cached level differs after a restart")
	}

	// Another stretch is another image.
	other, _ := s.Get(ctx, Request{Path: path, Stretch: 0, Size: 512})
	if other.ETag == first.ETag {
		t.Error("stretch levels share an ETag")
	}
}

func TestGetChangesWithFile(t *testing.T) {
	src := t.TempDir()
	path := writeFITS(t, src, 64, 32)
	s := newService(t, t.TempDir(), 1<<30)
	a, err := s.Get(context.Background(), Request{Path: path, Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(path)
	path = writeFITS(t, src, 32, 64) // same name, new contents
	b, err := s.Get(context.Background(), Request{Path: path, Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	if a.ETag == b.ETag {
		t.Error("ETag survived a file change")
	}
	if w, h := jpegSize(t, b.Data); w != 32 || h != 64 {
		t.Errorf("stale image served: %d×%d", w, h)
	}
}

func TestGetRaster(t *testing.T) {
	path := writePNG(t, t.TempDir(), 900, 300)
	s := newService(t, "", 0) // memory only
	img, err := s.Get(context.Background(), Request{Path: path, Raster: true, Size: 512})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := jpegSize(t, img.Data); w != 512 || h != 170 {
		t.Errorf("got %d×%d, want 512×170", w, h)
	}
}

func TestGetConcurrentRequestsShareOneRender(t *testing.T) {
	path := writeFITS(t, t.TempDir(), 3000, 200)
	s := newService(t, t.TempDir(), 1<<30)
	var wg sync.WaitGroup
	sizes := []int{512, 1024, 2048, 0, 512, 0}
	errs := make([]error, len(sizes))
	for i, size := range sizes {
		wg.Go(func() {
			_, errs[i] = s.Get(context.Background(), Request{Path: path, Size: size})
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	key, _, _ := source(Request{Path: path})
	if !s.complete(key) {
		t.Error("levels missing after concurrent requests")
	}
}

func TestGetErrors(t *testing.T) {
	s := newService(t, "", 0)
	if _, err := s.Get(context.Background(), Request{Path: filepath.Join(t.TempDir(), "nope.fit")}); err == nil {
		t.Error("expected an error for a missing file")
	}
	bad := filepath.Join(t.TempDir(), "bad.fit")
	os.WriteFile(bad, []byte("garbage"), 0o644)
	if _, err := s.Get(context.Background(), Request{Path: bad}); err == nil {
		t.Error("expected an error for a broken file")
	}
}

func TestWarmFillsTheDisk(t *testing.T) {
	src := t.TempDir()
	reqs := []Request{
		{Path: writeFITS(t, src, 700, 300), Stretch: 2},
		{Path: writePNG(t, src, 100, 50), Raster: true},
	}
	s := newService(t, t.TempDir(), 1<<30)
	s.Warm(context.Background(), reqs)
	for _, r := range reqs {
		key, _, _ := source(r)
		if !s.complete(key) {
			t.Errorf("%s not warmed", r.Path)
		}
	}
}

func TestDiskPruneEvictsLeastRecentlyUsed(t *testing.T) {
	src, cache := t.TempDir(), t.TempDir()
	s := newService(t, cache, 1<<30)
	var keys []string
	for i := range 3 {
		path := writeFITS(t, src, 600+i, 400)
		if _, err := s.Get(context.Background(), Request{Path: path, Size: 512}); err != nil {
			t.Fatal(err)
		}
		key, _, _ := source(Request{Path: path})
		keys = append(keys, key)
		s.running.Wait() // every level written: this image is now the newest
	}
	// Shrink the budget so that only the newest image fits under the 90% a
	// prune brings the cache down to.
	s.disk.mu.Lock()
	s.disk.maxBytes = s.disk.entries[keys[2]].bytes*10/9 + 10
	s.disk.mu.Unlock()
	s.disk.prune()
	if _, err := os.Stat(filepath.Join(cache, keys[0])); !os.IsNotExist(err) {
		t.Error("oldest image survived the prune")
	}
	if _, err := os.Stat(filepath.Join(cache, keys[2])); err != nil {
		t.Error("newest image was evicted")
	}
}

func TestWarmStopsBeforeEvicting(t *testing.T) {
	src := t.TempDir()
	s := newService(t, t.TempDir(), 1<<30)
	first := Request{Path: writeFITS(t, src, 700, 300)}
	s.Warm(context.Background(), []Request{first})
	// Budget now just above what's cached: warming more would evict.
	s.disk.mu.Lock()
	s.disk.maxBytes = s.disk.total + 1
	s.disk.mu.Unlock()
	second := Request{Path: writeFITS(t, src, 300, 700)}
	s.Warm(context.Background(), []Request{second})
	key, _, _ := source(second)
	if s.complete(key) {
		t.Error("warm-up kept going past its share of the cache")
	}
	key, _, _ = source(first)
	if !s.complete(key) {
		t.Error("warm-up evicted what it cached before")
	}
}
