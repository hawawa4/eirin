package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/TaruDesigns/eirin/internal/server"
)

func (a *App) startServer() {
	port := server.Port()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/frames", a.handleFrames)
	mux.HandleFunc("/api/image", a.handleImage)

	a.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
		}
	}()
}

func (a *App) stopServer() {
	if a.server == nil {
		return
	}
	_ = a.server.Shutdown(context.Background())
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rootFolder := a.store.Load().RootFolder
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":      "ok",
		"root_folder": rootFolder,
	})
}

func (a *App) handleFrames(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	rootFolder := a.store.Load().RootFolder
	if rootFolder == "" {
		http.Error(w, `{"error":"no root folder configured"}`, http.StatusBadRequest)
		return
	}
	frames, err := a.store.GetAllFramesUnder(rootFolder)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(frames)
}

// handleImage serves a local raster file (PNG/TIFF) by absolute path.
// The path must be under the configured NAS root folder.
func (a *App) handleImage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "missing path", http.StatusBadRequest)
		return
	}

	// Security: only serve files under the configured NAS root.
	rootFolder := a.store.Load().RootFolder
	absPath := filepath.Clean(path)
	absRoot := filepath.Clean(rootFolder)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, absPath)
}
