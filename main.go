package main

import (
	"log"

	"goscout/internal/ui"

	"fyne.io/fyne/v2/app"
)

func main() {
	goscout := app.New()
	fyneWindow := goscout.NewWindow("GoScout")
	cfg, err := ui.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	ui.SetupWindow(fyneWindow, cfg)
	fyneWindow.ShowAndRun()
}
