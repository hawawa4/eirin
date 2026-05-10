package main

import (
	"embed"

	"github.com/TaruDesigns/eirin/internal/app"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.NewApp()

	err := wails.Run(&options.App{
		Title:  "Eirin",
		Width:  1600,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 17, B: 26, A: 1},
		OnStartup:        a.Startup,
		OnShutdown:       a.Shutdown,
		Bind:             []interface{}{a},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
