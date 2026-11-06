//go:build server

package main

import (
	"embed"
	"io/fs"

	"github.com/hawawa4/eirin/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist-gallery
var galleryAssets embed.FS

// assets is the public gallery's frontend (frontend/dist-gallery), not the
// desktop app's: the server doesn't ship the Library or Settings at all.
func assets() fs.FS { return galleryAssets }

// services are the Go services visitors can call: only the gallery. The app
// itself is not bound, so none of its methods are reachable.
func services(a *app.App) []application.Service {
	return []application.Service{application.NewService(app.NewGallery(a))}
}
