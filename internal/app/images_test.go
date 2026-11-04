package app

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

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
