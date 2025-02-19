package main

import (
	"goscout/internal/ui"

	"fyne.io/fyne/v2/app"
)

func main() {
	goscout := app.New()
	fyneWindow := goscout.NewWindow("GoScout")
	cfg, err := ui.LoadConfig()
	if err != nil {
		cfg = ui.DefaultConfig()
	}
	ui.SetupWindow(fyneWindow, cfg)
	fyneWindow.ShowAndRun()
}
