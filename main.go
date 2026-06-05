package main

import (
	"embed"
	"log"

	"github.com/TaruDesigns/eirin/internal/app"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.NewApp()

	wailsApp := application.New(application.Options{
		Name:        "Eirin",
		Description: "Astrophotography library manager",
		Services: []application.Service{
			application.NewService(a),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
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
