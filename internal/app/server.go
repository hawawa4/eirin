package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/TaruDesigns/eirin/internal/server"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) startServer() {
	port := server.Port()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/frames", a.handleFrames)

	a.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			runtime.LogErrorf(a.ctx, "server: %v", err)
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
