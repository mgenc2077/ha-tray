package main

import (
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	w.Resize(fyne.NewSize(1200, 800))

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
		m.Items = append(m.Items, fyne.NewMenuItem("Office Light(ws)", func() {
			toggleEntityWs("switch.cata_ct_4010_akilli_priz_socket_1")
		}))
		m.Items = append(m.Items, fyne.NewMenuItem("Discovery", func() {
			_, _ = discovery()
		}))
		desk.SetSystemTrayMenu(m)
	}

	haURLEntry := widget.NewEntry()
	haURLEntry.SetText(config.HaURL)

	haTokenEntry := widget.NewPasswordEntry()
	haTokenEntry.SetText(config.HaToken)

	form := widget.NewForm(
		widget.NewFormItem("HA URL", haURLEntry),
		widget.NewFormItem("HA Token", haTokenEntry),
	)
	form.SubmitText = "Save"
	form.OnSubmit = func() {
		config.HaURL = haURLEntry.Text
		config.HaToken = haTokenEntry.Text

		log.Println("New config:")
		log.Println("HA URL:", config.HaURL)
		log.Println("HA Token:", config.HaToken)
	}

	deviceBtn := widget.NewButton("Devices", func() {
		entities, err := discovery()
		if err != nil {
			dialog.ShowError(err, w)
			return
		}

		table := widget.NewTable(
			func() (int, int) {
				return len(entities), 3
			},
			func() fyne.CanvasObject {
				return widget.NewLabel("Wide Content Placeholder")
			},
			func(id widget.TableCellID, obj fyne.CanvasObject) {
				lbl := obj.(*widget.Label)
				switch id.Col {
				case 0:
					lbl.SetText(entities[id.Row].EntityID)
				case 1:
					lbl.SetText(entities[id.Row].State)
				default:
					lbl.SetText("") // "enabled" column placeholder
				}
			},
		)
		table.SetColumnWidth(0, 400)
		table.SetColumnWidth(1, 300)
		table.SetColumnWidth(2, 50)

		tableContainer := container.NewGridWrap(fyne.NewSize(800, 600), table)

		dialog.ShowCustom("Discovered Devices", "Close", tableContainer, w)
	})

	w.SetContent(container.NewVBox(form, deviceBtn))
	w.SetCloseIntercept(func() {
		w.Hide()
	})
	w.ShowAndRun()
}
