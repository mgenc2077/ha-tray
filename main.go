package main

import (
	"encoding/json"
	"errors"
	"fmt"
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
	HaURL           string                    `json:"ha_url"`
	HaToken         string                    `json:"ha_token"`
	EnabledEntities map[string]bool           `json:"enabled_entities"`
	Hotkeys         map[string]*HotkeyBinding `json:"hotkeys"`
}

var config AppConfig

func loadConfig() {
	config.EnabledEntities = make(map[string]bool)

	_ = godotenv.Load()
	if envURL := os.Getenv("haURL"); envURL != "" {
		config.HaURL = envURL
	}
	if envToken := os.Getenv("haToken"); envToken != "" {
		config.HaToken = envToken
	}

	data, err := os.ReadFile("config.json")
	if err == nil {
		_ = json.Unmarshal(data, &config)
	} else if os.IsNotExist(err) {
		saveConfig()
	}
	if config.EnabledEntities == nil {
		config.EnabledEntities = make(map[string]bool)
	}
	if config.Hotkeys == nil {
		config.Hotkeys = make(map[string]*HotkeyBinding)
	}
}

func saveConfig() {
	data, err := json.MarshalIndent(config, "", "  ")
	if err == nil {
		os.WriteFile("config.json", data, 0644)
	}
}

func updateTrayMenu(desk desktop.App, w fyne.Window, hkManager HotkeyManager) {
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

	m.Items = append(m.Items, fyne.NewMenuItemSeparator())
	m.Items = append(m.Items, fyne.NewMenuItem("Quit", func() {
		hkManager.UnregisterAll()
		fyne.CurrentApp().Quit()
	}))

	desk.SetSystemTrayMenu(m)
}

func main() {
	loadConfig()

	if len(os.Args) >= 3 && os.Args[1] == "-trigger" {
		entityID := os.Args[2]
		if err := toggleEntityWs(entityID); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("toggled", entityID)
		os.Exit(0)
	}

	hkManager := NewHotkeyManager()
	for entityID, binding := range config.Hotkeys {
		if config.EnabledEntities[entityID] && binding != nil {
			if err := hkManager.Register(entityID, binding.Modifiers, binding.Key); err != nil {
				log.Printf("hotkey register error for %s: %v", entityID, err)
			}
		}
	}

	a := app.New()
	w := a.NewWindow("HA Tray")
	w.Resize(fyne.NewSize(500, 120))

	var desk desktop.App
	var okDesk bool
	if desk, okDesk = a.(desktop.App); okDesk {
		updateTrayMenu(desk, w, hkManager)
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
		// Uses a short debounce (500ms) before triggering the save logic
		debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
			config.HaURL = haURLEntry.Text
			config.HaToken = haTokenEntry.Text

			log.Println("New config:")
			log.Println("HA URL:", config.HaURL)
			log.Println("HA Token:", config.HaToken)
			saveConfig()
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
						return len(entities), 4
					},
					func() fyne.CanvasObject {
						return container.NewStack(
							widget.NewLabel("Wide Content Placeholder"),
							widget.NewCheck("", nil),
							widget.NewEntry(),
						)
					},
					func(id widget.TableCellID, obj fyne.CanvasObject) {
						stack := obj.(*fyne.Container)
						lbl := stack.Objects[0].(*widget.Label)
						chk := stack.Objects[1].(*widget.Check)
						entry := stack.Objects[2].(*widget.Entry)

						switch id.Col {
						case 0:
							chk.Hide()
							entry.Hide()
							lbl.Show()
							lbl.SetText(entities[id.Row].EntityID)
						case 1:
							chk.Hide()
							entry.Hide()
							lbl.Show()
							lbl.SetText(entities[id.Row].State)
						case 2:
							lbl.Hide()
							entry.Hide()
							chk.Show()
							eID := entities[id.Row].EntityID
							chk.Checked = config.EnabledEntities[eID]
							chk.OnChanged = func(checked bool) {
								if checked {
									config.EnabledEntities[eID] = true
									if binding, ok := config.Hotkeys[eID]; ok && binding != nil {
										hkManager.Register(eID, binding.Modifiers, binding.Key)
									}
								} else {
									delete(config.EnabledEntities, eID)
									hkManager.Unregister(eID)
								}
								saveConfig()
								if okDesk {
									updateTrayMenu(desk, w, hkManager)
								}
							}
							chk.Refresh()
						case 3:
							lbl.Hide()
							chk.Hide()
							entry.Show()
							eID := entities[id.Row].EntityID
							if hkManager.Supported() {
								if b, ok := config.Hotkeys[eID]; ok && b != nil {
									entry.SetText(FormatHotkeyBinding(b))
								} else {
									entry.SetText("")
								}
								entry.SetPlaceHolder("Ctrl+Alt+K")
								entry.OnChanged = func(text string) {
									if text == "" {
										delete(config.Hotkeys, eID)
										hkManager.Unregister(eID)
										saveConfig()
										return
									}
									mods, key, ok := ParseHotkeyString(text)
									if !ok {
										return
									}
									config.Hotkeys[eID] = &HotkeyBinding{
										Modifiers: mods,
										Key:       key,
									}
									hkManager.Unregister(eID)
									if err := hkManager.Register(eID, mods, key); err != nil {
										log.Printf("hotkey register error for %s: %v", eID, err)
									}
									saveConfig()
								}
							} else {
								entry.SetText("N/A")
								entry.Disable()
							}
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
					case 3:
						lbl.SetText("Hotkey")
					default:
						lbl.SetText("")
					}
				}

				originalSize := w.Canvas().Size()
				w.Resize(fyne.NewSize(1200, 800))
				table.SetColumnWidth(0, 500)
				table.SetColumnWidth(1, 400)
				table.SetColumnWidth(2, 100)
				table.SetColumnWidth(3, 120)

				tableContainer := container.NewGridWrap(fyne.NewSize(1100, 600), table)

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
	a.Run()
}
