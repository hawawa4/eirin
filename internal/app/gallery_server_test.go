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
	frame := store.Frame{NasPath: real, FrameType: store.FrameTypeImage, Public: true}
	a.snapshots.gallery.state.Store(buildGallery(gallery.Default(), []store.Frame{frame}))
	return gallery.PublicPath(real)
}

// frame is a library frame the user marked public.
func frame(path, typ, object string) store.Frame {
	return store.Frame{NasPath: path, FrameType: typ, Object: object, Public: true}
}

func TestBuildGalleryShowsOnlyFinals(t *testing.T) {
	r := frame("/lib/m42.jpg", store.FrameTypeImage, "M 42")
	rejected := frame("/lib/bad.jpg", store.FrameTypeImage, "Bad")
	rejected.Rejected = true
	unmarked := frame("/lib/hidden.jpg", store.FrameTypeImage, "Hidden")
	unmarked.Public = false
	st := buildGallery(gallery.Default(), []store.Frame{
		r,
		rejected,
		unmarked,
		frame("/lib/light.fit", store.FrameTypeLight, "M 42"),
		frame("/lib/stack.fit", store.FrameTypeStacked, "M 42"),
		frame("/lib/final.fit", store.FrameTypeProcessed, "M 42"),
	})
	// Both renderings of the picture are offered; the page picks one at a time.
	if len(st.info.Images) != 2 || st.info.Images[0].Title != "M 42" || st.info.Images[1].Title != "M 42" {
		t.Fatalf("images = %+v, want the M 42 FITS and its export", st.info.Images)
	}
	if st.info.DefaultFormat != gallery.FormatRaster {
		t.Errorf("default format = %q, want the export (auto)", st.info.DefaultFormat)
	}
	if _, ok := st.index.Real(gallery.PublicPath("/lib/light.fit")); ok {
		t.Error("a light frame must not be reachable")
	}
	if _, ok := st.index.Real(gallery.PublicPath("/lib/bad.jpg")); ok {
		t.Error("a rejected image must not be reachable")
	}
	if _, ok := st.index.Real(gallery.PublicPath("/lib/hidden.jpg")); ok {
		t.Error("an image the user didn't mark public must not be reachable")
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

func TestBuildGallerySkyHasSolvedPicturesOfBothFormats(t *testing.T) {
	ra, scale := 10.0, 1.2
	solvedFits := frame("/lib/a.fit", store.FrameTypeProcessed, "M 31")
	solvedFits.RA, solvedFits.PixelScale = &ra, &scale
	solvedPNG := frame("/lib/b.png", store.FrameTypeImage, "M 33")
	solvedPNG.RA, solvedPNG.PixelScale = &ra, &scale
	st := buildGallery(gallery.Default(), []store.Frame{
		frame("/lib/a.jpg", store.FrameTypeImage, "M 31"), solvedFits, solvedPNG,
	})
	if len(st.info.Images) != 3 {
		t.Fatalf("images = %+v, want all three", st.info.Images)
	}
	if len(st.info.Sky) != 2 {
		t.Fatalf("sky = %+v, want the two plate-solved pictures", st.info.Sky)
	}
	for _, img := range st.info.Sky {
		if _, ok := st.index.Real(img.Path); !ok {
			t.Errorf("sky picture %s must be reachable", img.Path)
		}
	}
}

func TestBuildGalleryDefaultFormat(t *testing.T) {
	fits := frame("/lib/a.fit", store.FrameTypeProcessed, "M 31")
	png := frame("/lib/a.png", store.FrameTypeImage, "M 31")
	cfg := gallery.Default()
	if got := buildGallery(cfg, []store.Frame{fits, png}).info.DefaultFormat; got != gallery.FormatRaster {
		t.Errorf("auto with exports: %q", got)
	}
	cfg.Format = gallery.FormatFITS
	if got := buildGallery(cfg, []store.Frame{fits, png}).info.DefaultFormat; got != gallery.FormatFITS {
		t.Errorf("forced fits: %q", got)
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
