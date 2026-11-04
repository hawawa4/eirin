package app

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hawawa4/eirin/internal/fits"
	"github.com/hawawa4/eirin/internal/previews"
	"github.com/hawawa4/eirin/internal/store"
)

// newPreviewApp returns a test app with a memory-only preview service and a
// 1200×400 PNG in its library.
func newPreviewApp(t *testing.T) (*App, string) {
	t.Helper()
	a := newTestApp(t)
	a.images.Store(previews.New("", 0))
	path := filepath.Join(t.TempDir(), "final.png")
	img := image.NewRGBA(image.Rect(0, 0, 1200, 400))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	img.Set(0, 0, color.White)
	f, err := os.Create(path)
	must(t, err)
	must(t, png.Encode(f, img))
	must(t, f.Close())
	must(t, a.store().UpsertFrame(path, store.Frame{FrameType: store.FrameTypeImage}))
	return a, path
}

func previewURL(path string, params string) string {
	return PreviewRoute + "?path=" + url.QueryEscape(path) + params
}

func TestServePreview(t *testing.T) {
	a, path := newPreviewApp(t)
	h := AssetMiddleware(a)(http.NotFoundHandler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", previewURL(path, "&size=500"), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q", ct)
	}
	cfg, err := jpeg.DecodeConfig(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 512 || cfg.Height != 170 {
		t.Errorf("got %d×%d, want the 512 level (512×170)", cfg.Width, cfg.Height)
	}

	// The browser revalidates with the ETag and gets a 304.
	tag := rec.Header().Get("ETag")
	if tag == "" {
		t.Fatal("no ETag")
	}
	req := httptest.NewRequest("GET", previewURL(path, "&size=500"), nil)
	req.Header.Set("If-None-Match", tag)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("revalidation: status %d, want 304", rec.Code)
	}
}

func TestServePreviewRejects(t *testing.T) {
	a, path := newPreviewApp(t)
	h := AssetMiddleware(a)(http.NotFoundHandler())
	cases := map[string]int{
		previewURL(path, "&size=-1"):                              http.StatusBadRequest,
		previewURL(path, "&size=big"):                             http.StatusBadRequest,
		previewURL("relative/final.png", ""):                      http.StatusBadRequest,
		previewURL(filepath.Join(t.TempDir(), "notes.txt"), ""):   http.StatusBadRequest,
		previewURL(filepath.Join(t.TempDir(), "missing.fit"), ""): http.StatusNotFound,
	}
	for u, want := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", u, nil))
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d", u, rec.Code, want)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", previewURL(path, ""), nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: status %d, want 405", rec.Code)
	}
	// Other routes go to Wails.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/index.html", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("fallthrough: status %d, want the next handler's 404", rec.Code)
	}
}

func TestServePreviewServerRefusesOutsideLibrary(t *testing.T) {
	if desktopModeEnabled {
		t.Skip("server builds only")
	}
	a, _ := newPreviewApp(t)
	other := filepath.Join(t.TempDir(), "other.png")
	must(t, os.WriteFile(other, []byte("x"), 0o644))
	rec := httptest.NewRecorder()
	AssetMiddleware(a)(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", previewURL(other, ""), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404 for a file outside the library", rec.Code)
	}
}

// writeTestFITS writes a w×h mono float FITS file into the library of a.
func writeTestFITS(t *testing.T, a *App, w, h int) string {
	t.Helper()
	var hdr []byte
	card := func(s string) { hdr = append(hdr, fmt.Sprintf("%-80s", s)...) }
	card(fmt.Sprintf("%-8s= %20s", "SIMPLE", "T"))
	card(fmt.Sprintf("%-8s= %20d", "BITPIX", -32))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS", 2))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS1", w))
	card(fmt.Sprintf("%-8s= %20d", "NAXIS2", h))
	card("END")
	for len(hdr)%2880 != 0 {
		hdr = append(hdr, ' ')
	}
	data := make([]byte, 4*w*h)
	for i := range w * h {
		binary.BigEndian.PutUint32(data[i*4:], math.Float32bits(float32(10+i%w)))
	}
	for len(data)%2880 != 0 {
		data = append(data, 0)
	}
	path := filepath.Join(t.TempDir(), "stack.fit")
	must(t, os.WriteFile(path, append(hdr, data...), 0o644))
	must(t, a.store().UpsertFrame(path, store.Frame{FrameType: store.FrameTypeStacked}))
	return path
}

func rawURL(path, params string) string {
	return RawPreviewRoute + "?path=" + url.QueryEscape(path) + params
}

func TestServeRawPreview(t *testing.T) {
	a := newTestApp(t)
	path := writeTestFITS(t, a, 300, 200)
	h := AssetMiddleware(a)(http.NotFoundHandler())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", rawURL(path, "&size=150"), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	b := rec.Body.Bytes()
	n := int(binary.LittleEndian.Uint32(b))
	var meta fits.RawPreviewMeta
	must(t, json.Unmarshal(b[4:4+n], &meta))
	if meta.Width != 150 || meta.Height != 100 || meta.Channels != 1 {
		t.Errorf("meta = %+v, want 150×100 mono", meta)
	}
	if meta.Header == nil || meta.Header.Width != 300 {
		t.Errorf("header = %+v, want the file's (300 wide)", meta.Header)
	}
	if got := len(b) - 4 - n; got != 150*100*2 {
		t.Errorf("%d sample bytes, want %d", got, 150*100*2)
	}

	req := httptest.NewRequest("GET", rawURL(path, "&size=150"), nil)
	req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Errorf("revalidation: status %d, want 304", rec.Code)
	}

	for u, want := range map[string]int{
		rawURL(path, "&size=-3"):                            http.StatusBadRequest,
		rawURL(strings.TrimSuffix(path, ".fit")+".png", ""): http.StatusBadRequest,
		rawURL(filepath.Join(t.TempDir(), "gone.fit"), ""):  http.StatusNotFound,
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", u, nil))
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d", u, rec.Code, want)
		}
	}
}

func TestServeRawPreviewDropsAbandonedRequests(t *testing.T) {
	a := newTestApp(t)
	path := writeTestFITS(t, a, 64, 64)
	// Every slot busy: a request whose client has gone must not wait for one.
	for range cap(rawSlots) {
		rawSlots <- struct{}{}
	}
	defer func() {
		for range cap(rawSlots) {
			<-rawSlots
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		AssetMiddleware(a)(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", rawURL(path, ""), nil).WithContext(ctx))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("an abandoned request waited for a render slot")
	}
	if rec.Body.Len() != 0 {
		t.Error("an abandoned request was rendered")
	}
}
