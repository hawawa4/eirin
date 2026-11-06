//go:build server

package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hawawa4/eirin/internal/gallery"
	"github.com/hawawa4/eirin/internal/store"
)

// publicPath is how the server's frontend names a library file in requests:
// by its opaque id. It publishes a gallery containing that one file.
func publicPath(a *App, real string) string {
	frame := store.Frame{NasPath: real, FrameType: store.FrameTypeImage}
	a.snapshots.gallery.state.Store(buildGallery(gallery.Default(), []store.Frame{frame}))
	return gallery.PublicPath(real)
}

func frame(path, typ, object string) store.Frame {
	return store.Frame{NasPath: path, FrameType: typ, Object: object}
}

func TestBuildGalleryShowsOnlyFinals(t *testing.T) {
	r := frame("/lib/m42.jpg", store.FrameTypeImage, "M 42")
	rejected := frame("/lib/bad.jpg", store.FrameTypeImage, "Bad")
	rejected.Rejected = true
	st := buildGallery(gallery.Default(), []store.Frame{
		r,
		rejected,
		frame("/lib/light.fit", store.FrameTypeLight, "M 42"),
		frame("/lib/stack.fit", store.FrameTypeStacked, "M 42"),
		frame("/lib/final.fit", store.FrameTypeProcessed, "M 42"),
	})
	// Auto format: the export wins over the FITS of the same picture.
	if len(st.info.Images) != 1 || st.info.Images[0].Title != "M 42" || st.info.Images[0].Format != "raster" {
		t.Fatalf("images = %+v, want just the M 42 export", st.info.Images)
	}
	if _, ok := st.index.Real(gallery.PublicPath("/lib/light.fit")); ok {
		t.Error("a light frame must not be reachable")
	}
	if _, ok := st.index.Real(gallery.PublicPath("/lib/bad.jpg")); ok {
		t.Error("a rejected image must not be reachable")
	}
}

func TestBuildGalleryNeverLeaksPaths(t *testing.T) {
	f := frame("/library/Secret Nebula/Stacked_30_Secret_10.0s_S50_20240101-221530.jpg", store.FrameTypeImage, "")
	st := buildGallery(gallery.Default(), []store.Frame{f})
	data, err := json.Marshal(st.info)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"library", "Secret", "Stacked_30"} {
		if strings.Contains(string(data), leak) {
			t.Errorf("GalleryInfo leaks %q: %s", leak, data)
		}
	}
	img := st.info.Images[0]
	if img.Title != "Untitled" || img.Subs != 30 || img.Telescope != "S50" {
		t.Errorf("image = %+v, want an untitled S50 stack of 30", img)
	}
}

func TestBuildGallerySkyFallsBackToSolvedFITS(t *testing.T) {
	ra, scale := 10.0, 1.2
	solvedFits := frame("/lib/a.fit", store.FrameTypeProcessed, "M 31")
	solvedFits.RA, solvedFits.PixelScale = &ra, &scale
	st := buildGallery(gallery.Default(), []store.Frame{frame("/lib/a.jpg", store.FrameTypeImage, "M 31"), solvedFits})
	if len(st.info.Images) != 1 || st.info.Images[0].Format != "raster" {
		t.Fatalf("images = %+v, want the export", st.info.Images)
	}
	if len(st.info.Sky) != 1 || st.info.Sky[0].Format != "fits" {
		t.Fatalf("sky = %+v, want the plate-solved FITS", st.info.Sky)
	}
	if _, ok := st.index.Real(st.info.Sky[0].Path); !ok {
		t.Error("a sky picture must be reachable")
	}
}

func TestPreviewRefusesRealPaths(t *testing.T) {
	a, path := newPreviewApp(t)
	publicPath(a, path)
	h := AssetMiddleware(a)(http.NotFoundHandler())
	for _, p := range []string{path, "/g/0000000000000000.png"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", rawPreviewURL(p, ""), nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", previewURL(a, path, ""), nil))
	if rec.Code != http.StatusOK {
		t.Errorf("by opaque id: status %d, want 200", rec.Code)
	}
}

func TestServerHasNoRawRoute(t *testing.T) {
	a := newTestApp(t)
	path := writeTestFITS(t, a, 64, 64)
	rec := httptest.NewRecorder()
	AssetMiddleware(a)(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", rawURL(path, ""), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

func TestGalleryImageSizesByID(t *testing.T) {
	a, path := newPreviewApp(t)
	id := publicPath(a, path)
	g := NewGallery(a)
	got := g.GetImageSizes([]string{id, path, "/g/ffffffffffffffff.png"})
	if len(got) != 1 || got[id].Width != 1200 || got[id].Height != 400 {
		t.Errorf("GetImageSizes = %+v, want only %s at 1200×400", got, id)
	}
}

func TestGalleryBeforeFirstSnapshot(t *testing.T) {
	a := newTestApp(t)
	info := NewGallery(a).GetGallery()
	if info.Loaded || len(info.Images) != 0 {
		t.Errorf("GetGallery before a snapshot = %+v, want empty and not loaded", info)
	}
}
