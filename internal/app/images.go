package app

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hawawa4/eirin/internal/indexer"
	"github.com/hawawa4/eirin/internal/previews"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PreviewRoute serves display images over plain HTTP on the frontend's own
// origin (Wails' asset server, desktop and server alike), so they arrive as
// binary JPEGs the browser decodes off the main thread and caches — not as
// base64 strings inside JSON RPC replies.
//
//	GET /eirin/preview?path=<nas path>&size=<px, 0 = native>&stretch=<0–3>
const PreviewRoute = "/eirin/preview"

// Environment overrides for the preview disk cache.
const (
	EnvCacheDir     = "EIRIN_CACHE_DIR"
	EnvCacheMaxMB   = "EIRIN_PREVIEW_CACHE_MB"
	defaultCacheMax = 2048 // MB
)

// AssetMiddleware routes PreviewRoute to a's preview handler and everything
// else to Wails. Pass it as application.AssetOptions.Middleware. (A function,
// not a method: Wails would expose an exported method to the frontend.)
func AssetMiddleware(a *App) application.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == PreviewRoute {
				a.servePreview(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// startPreviews creates the preview service. defaultDir is the build's
// cache location, used unless EIRIN_CACHE_DIR is set.
func (a *App) startPreviews(defaultDir string) {
	dir := defaultDir
	if v := os.Getenv(EnvCacheDir); v != "" {
		dir = filepath.Join(v, "previews")
	}
	maxMB := int64(defaultCacheMax)
	if v, err := strconv.ParseInt(os.Getenv(EnvCacheMaxMB), 10, 64); err == nil && v >= 0 {
		maxMB = v
	}
	if maxMB == 0 {
		dir = "" // disk cache off
	}
	a.images.Store(previews.New(dir, maxMB<<20))
}

// previewRequest validates a preview request's parameters.
func previewRequest(r *http.Request) (previews.Request, bool) {
	q := r.URL.Query()
	path := q.Get("path")
	if path == "" || !filepath.IsAbs(path) {
		return previews.Request{}, false
	}
	req := previews.Request{Path: filepath.Clean(path), Raster: indexer.IsRasterFile(path)}
	if !req.Raster && !indexer.IsFitsFile(path) {
		return previews.Request{}, false
	}
	if v := q.Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return previews.Request{}, false
		}
		req.Size = n
	}
	if v := q.Get("stretch"); v != "" && !req.Raster {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n > 3 {
			return previews.Request{}, false
		}
		req.Stretch = n
	}
	return req, true
}

func (a *App) servePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	svc := a.images.Load()
	if svc == nil || a.store() == nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	req, ok := previewRequest(r)
	if !ok {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// The server viewer serves only its library's final images. The desktop's
	// asset server is reachable from its own webview only.
	if err := a.checkServable(req.Path); err != nil {
		http.Error(w, "not in the library", http.StatusNotFound)
		return
	}

	// "no-cache" lets the browser keep the image but makes it ask whether the
	// file changed (answered from its size and mtime, without rendering).
	w.Header().Set("Cache-Control", "private, no-cache")
	tag, mod, err := svc.Tag(req)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("ETag", tag)
	if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, tag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	img, err := svc.Get(r.Context(), req)
	if err != nil {
		if r.Context().Err() == nil {
			slog.Warn("preview", "path", req.Path, "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, "", mod, bytes.NewReader(img.Data))
}
