package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/public
var assets embed.FS

func main() {
	app := NewApp() // Defined in app.go

	err := wails.Run(&options.App{
		Title:  "MLC Analyzer GO",
		Width:  720,
		Height: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 46, G: 46, B: 46, A: 255}, // #2e2e2e
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		log.Fatal("Error running Wails app: ", err.Error())
	}
}
