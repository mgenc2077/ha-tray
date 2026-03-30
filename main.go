package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/joho/godotenv"
)

type AppConfig struct {
	HaURL   string
	HaToken string
}

var config AppConfig

func main() {
	a := app.New()
	w := a.NewWindow("HA Tray")

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	config.HaURL = os.Getenv("haURL")
	config.HaToken = os.Getenv("haToken")

	if desk, ok := a.(desktop.App); ok {
		m := fyne.NewMenu("HA Tray",
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}))
		m.Items = append(m.Items, fyne.NewMenuItem("Office Light", func() {
			toggleEntity("switch.cata_ct_4010_akilli_priz_socket_1")
		}))
		desk.SetSystemTrayMenu(m)
	}

	w.SetContent(widget.NewLabel("Fyne System Tray"))
	w.SetCloseIntercept(func() {
		w.Hide()
	})
	w.ShowAndRun()
}
