//go:build !server

package main

import (
	"embed"
	"io/fs"

	"github.com/hawawa4/eirin/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var desktopAssets embed.FS

// assets is the app's frontend (frontend/dist).
func assets() fs.FS { return desktopAssets }

// services are the Go services the frontend can call: the whole app.
func services(a *app.App) []application.Service {
	return []application.Service{application.NewService(a)}
}
