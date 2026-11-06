package main

import (
	_ "embed"
	"log"

	"github.com/hawawa4/eirin/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/appicon.png
var appIcon []byte

func main() {
	a := app.NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Eirin",
		Description: "Astrophotography library manager",
		Icon:        appIcon,
		Linux: application.LinuxOptions{
			// Wayland takes the window's app_id from this and matches it to
			// eirin.desktop for the taskbar icon (window icons are ignored there).
			ProgramName: "eirin",
		},
		Services: services(a),
		Assets: application.AssetOptions{
			Handler:    application.BundledAssetFileServer(assets()),
			Middleware: app.AssetMiddleware(a),
		},
	})

	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "Eirin",
		Width:            1600,
		Height:           900,
		BackgroundColour: application.NewRGBA(15, 17, 26, 255),
		URL:              "/",
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}
