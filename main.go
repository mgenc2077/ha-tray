package main

import (
	"errors"
	"log"
	"os"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/joho/godotenv"
)

type AppConfig struct {
	HaURL           string
	HaToken         string
	EnabledEntities map[string]bool
}

var config AppConfig

func updateTrayMenu(desk desktop.App, w fyne.Window) {
	m := fyne.NewMenu("HA Tray",
		fyne.NewMenuItem("Show", func() {
			w.Show()
		}))

	m.Items = append(m.Items, fyne.NewMenuItem("Discovery", func() {
		go func() {
			_, _ = discovery()
		}()
	}))

	for entityID, enabled := range config.EnabledEntities {
		if enabled {
			eID := entityID
			m.Items = append(m.Items, fyne.NewMenuItem(eID, func() {
				_ = toggleEntityWs(eID)
			}))
		}
	}

	desk.SetSystemTrayMenu(m)
}

func main() {
	config.EnabledEntities = make(map[string]bool)

	a := app.New()
	w := a.NewWindow("HA Tray")
	w.Resize(fyne.NewSize(1000, 200))

	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	config.HaURL = os.Getenv("haURL")
	config.HaToken = os.Getenv("haToken")

	var desk desktop.App
	var okDesk bool
	if desk, okDesk = a.(desktop.App); okDesk {
		updateTrayMenu(desk, w)
	}

	haURLEntry := widget.NewEntry()
	haURLEntry.SetText(config.HaURL)

	haTokenEntry := widget.NewPasswordEntry()
	haTokenEntry.SetText(config.HaToken)

	form := widget.NewForm(
		widget.NewFormItem("HA URL", haURLEntry),
		widget.NewFormItem("HA Token", haTokenEntry),
	)
	var debounceTimer *time.Timer
	debounceSave := func(s string) {
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		// Uses a long debounce (2 seconds) before triggering the save logic
		debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
			config.HaURL = haURLEntry.Text
			config.HaToken = haTokenEntry.Text

			log.Println("New config:")
			log.Println("HA URL:", config.HaURL)
			log.Println("HA Token:", config.HaToken)
		})
	}

	haURLEntry.OnChanged = debounceSave
	haTokenEntry.OnChanged = debounceSave

	deviceBtn := widget.NewButton("Devices", func() {
		done := make(chan struct{})
		var loadingDialog dialog.Dialog

		// Trigger loading animation if discovery is slow
		time.AfterFunc(500*time.Millisecond, func() {
			select {
			case <-done:
			default:
				fyne.Do(func() {
					select {
					case <-done:
					default:
						activity := widget.NewActivity()
						activity.Start()
						content := container.NewPadded(activity)
						loadingDialog = dialog.NewCustomWithoutButtons("Connecting to HA...", content, w)
						loadingDialog.Show()
					}
				})
			}
		})

		go func() {
			defer func() {
				close(done)
				fyne.Do(func() {
					if loadingDialog != nil {
						loadingDialog.Hide()
					}
				})
			}()

			entities, err := discovery()
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(errors.New("cant connect to home assistant"), w)
				})
				return
			}

			fyne.Do(func() {
				table := widget.NewTable(
					func() (int, int) {
						return len(entities), 3
					},
					func() fyne.CanvasObject {
						return container.NewStack(
							widget.NewLabel("Wide Content Placeholder"),
							widget.NewCheck("", nil),
						)
					},
					func(id widget.TableCellID, obj fyne.CanvasObject) {
						stack := obj.(*fyne.Container)
						lbl := stack.Objects[0].(*widget.Label)
						chk := stack.Objects[1].(*widget.Check)

						switch id.Col {
						case 0:
							chk.Hide()
							lbl.Show()
							lbl.SetText(entities[id.Row].EntityID)
						case 1:
							chk.Hide()
							lbl.Show()
							lbl.SetText(entities[id.Row].State)
						case 2:
							lbl.Hide()
							chk.Show()
							eID := entities[id.Row].EntityID
							chk.Checked = config.EnabledEntities[eID]
							chk.OnChanged = func(checked bool) {
								config.EnabledEntities[eID] = checked
								if okDesk {
									updateTrayMenu(desk, w)
								}
							}
							chk.Refresh()
						}
					},
				)

				table.ShowHeaderRow = true
				table.CreateHeader = func() fyne.CanvasObject {
					return widget.NewLabel("Header Placeholder")
				}
				table.UpdateHeader = func(id widget.TableCellID, obj fyne.CanvasObject) {
					lbl := obj.(*widget.Label)
					switch id.Col {
					case 0:
						lbl.SetText("Entities")
					case 1:
						lbl.SetText("State")
					case 2:
						lbl.SetText("Enabled")
					default:
						lbl.SetText("")
					}
				}

				originalSize := w.Canvas().Size()
				w.Resize(fyne.NewSize(1000, 800))
				table.SetColumnWidth(0, 400)
				table.SetColumnWidth(1, 300)
				table.SetColumnWidth(2, 50)

				tableContainer := container.NewGridWrap(fyne.NewSize(800, 600), table)

				d := dialog.NewCustom("Discovered Devices", "Close", tableContainer, w)
				d.SetOnClosed(func() {
					w.Resize(originalSize)
				})
				d.Show()
			})
		}()
	})

	w.SetContent(container.NewVBox(form, deviceBtn))
	w.SetCloseIntercept(func() {
		w.Hide()
	})
	w.ShowAndRun()
}
