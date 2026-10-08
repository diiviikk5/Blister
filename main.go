package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/diiviikk5/Blister/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a, err := app.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Blister failed to start:", err)
		os.Exit(1)
	}
	args := os.Args[1:]
	effect := a.Settings().WindowEffect
	backdrop := map[string]windows.BackdropType{
		"mica": windows.Mica, "acrylic": windows.Acrylic, "tabbed": windows.Tabbed,
	}[effect]
	translucent := backdrop != 0
	minimized := slices.Contains(args, "--minimized")

	err = wails.Run(&options.App{
		Title:             "Blister",
		Width:             1240,
		Height:            780,
		MinWidth:          880,
		MinHeight:         560,
		Frameless:         true,
		StartHidden:       minimized,
		HideWindowOnClose: a.Settings().CloseToTray,
		BackgroundColour:  bg(translucent),
		AssetServer:       &assetserver.Options{Assets: assets},
		OnStartup: func(ctx context.Context) {
			a.Startup(ctx)
			a.HandleArgs(args)
		},
		OnDomReady: a.DomReady,
		OnShutdown: a.Shutdown,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.blister.app",
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
				a.ShowWindow()
				a.HandleArgs(d.Args)
			},
		},
		DragAndDrop: &options.DragAndDrop{EnableFileDrop: true},
		Bind:        []any{a},
		Windows: &windows.Options{
			WebviewIsTransparent: translucent,
			WindowIsTranslucent:  translucent,
			BackdropType:         backdrop,
			DisableWindowIcon:    false,
			Theme:                windows.SystemDefault,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "Blister:", err)
		os.Exit(1)
	}
}

// bg is the window colour behind the page; clear when a backdrop shows through.
func bg(translucent bool) *options.RGBA {
	if translucent {
		return &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	return &options.RGBA{R: 13, G: 9, B: 19, A: 255}
}
