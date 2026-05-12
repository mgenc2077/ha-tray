---
name: ha-tray-context
description: Project architecture, Fyne v2 patterns, Home Assistant API protocol, and codebase conventions for ha-tray
---

## Architecture

Single `package main` Go app (Go 1.24). Source files:

- **`main.go`** — Fyne GUI, system tray, config UI, device discovery table (4 columns)
- **`ha.go`** — Home Assistant REST and WebSocket API clients
- **`hotkey.go`** — `HotkeyManager` interface, `HotkeyBinding` struct, hotkey string parsing/formatting
- **`hotkey_windows.go`** — Win32 `RegisterHotKey`/`UnregisterHotKey` implementation (pure Go via `golang.org/x/sys/windows`)
- **`hotkey_linux.go`** — No-op stub (`Supported() = false`)
- **`hotkey_darwin.go`** — No-op stub (`Supported() = false`)
- **`logger.go`** — `InitLogger()` sets up `slog.JSONHandler` writing to configured log file. Log level and file path from config. Defaults: `info` level, `ha-tray.log`.

Runtime state: `config.json` (gitignored, auto-created) holds `ha_url`, `ha_token`, `enabled_entities`, `hotkeys`, `log_level`, `log_file`. `.env` (gitignored) is an alternative source for `haURL`/`haToken` env vars via `godotenv`. `loadConfig()` reads `.env` first, then `config.json` overrides.

## Fyne v2 patterns used in this project

### System tray

```go
import "fyne.io/fyne/v2/driver/desktop"

// Type-assert app to desktop.App — only works on desktop platforms
if desk, ok := a.(desktop.App); ok {
    m := fyne.NewMenu("HA Tray", fyne.NewMenuItem("Show", func() { w.Show() }))
    desk.SetSystemTrayMenu(m)
}
```

- `desktop.App` interface has: `SetSystemTrayMenu`, `SetSystemTrayIcon`, `SetSystemTrayWindow` (new in 2.7)
- **Icon.png** exists in repo root but is NOT loaded as tray icon — app uses Fyne default
- Window close is intercepted to hide instead of quit:
  ```go
  w.SetCloseIntercept(func() { w.Hide() })
  ```

### Thread safety: fyne.Do()

All UI updates from goroutines MUST be wrapped in `fyne.Do()`:

```go
go func() {
    // ... background work ...
    fyne.Do(func() {
        // safe to update widgets here
        dialog.ShowError(err, w)
    })
}()
```

This project uses this pattern in `main.go` for:
- Discovery loading dialog show/hide
- Error dialogs
- Building the entity table
- Closing the `done` channel signaling

### widget.NewTable

3 callbacks: `length`, `create`, `update`. Currently 4 columns:

| Col | Content |
|---|---|
| 0 | Entity ID (label) |
| 1 | State (label) |
| 2 | Enabled (checkbox) |
| 3 | Hotkey (text entry or "N/A" when unsupported) |

```go
table := widget.NewTable(
    func() (int, int) { return len(entities), 4 },       // rows, cols
    func() fyne.CanvasObject {                            // template cell
        return container.NewStack(
            widget.NewLabel(""),
            widget.NewCheck("", nil),
            widget.NewEntry(),
        )
    },
    func(id widget.TableCellID, obj fyne.CanvasObject) {  // bind data
        // update cell based on id.Row, id.Col
    },
)
```

Headers require separate `CreateHeader` / `UpdateHeader` callbacks. Column widths via `table.SetColumnWidth(col, width)`.

### Dialogs

- `dialog.NewCustom(title, dismiss, content, window)` — dialog with dismiss button
- `dialog.NewCustomWithoutButtons(title, content, window)` — no buttons (used for loading spinner)
- `dialog.ShowError(err, window)` — standard error dialog

### Config form with debounced save

`widget.NewForm` + `widget.NewFormItem` with `widget.NewEntry` / `widget.NewPasswordEntry`. `OnChanged` callback uses `time.AfterFunc(500ms, saveConfig)` to debounce writes.

## Home Assistant API

### REST endpoints

- `GET /api/states` — list all entities (returns `[]HAEntity` with `entity_id` and `state`)
- `POST /api/services/homeassistant/toggle` — toggle entity state (body: `{"entity_id": "..."}`)
- Auth header: `Authorization: Bearer <token>`

### WebSocket protocol (used for toggle)

```
1. Connect to ws(s)://<ha_url>/api/websocket
2. Read: {"type": "auth_required"}
3. Write: {"type": "auth", "access_token": "<token>"}
4. Read: {"type": "auth_ok"}
5. Write: {"id": 1, "type": "call_service", "domain": "homeassistant", "service": "toggle", "service_data": {"entity_id": "..."}}
6. Read: {"type": "result", "success": true}
```

5-second handshake timeout. URL is derived from `config.HaURL` by replacing `http://` → `ws://`, `https://` → `wss://`.

### Dead code note

`toggleEntity()` (REST-based toggle) in `ha.go` is never called — only `toggleEntityWs()` is used from the tray menu. The REST function can be removed or kept as fallback.

## Config data model

```go
type HotkeyBinding struct {
    Modifiers []string `json:"modifiers"` // e.g. ["ctrl", "alt"]
    Key       string   `json:"key"`       // e.g. "l"
    Enabled   *bool    `json:"enabled"`   // nil = true, explicit false disables
}

func (h *HotkeyBinding) IsEnabled() bool { return h.Enabled == nil || *h.Enabled }

type AppConfig struct {
    HaURL           string                    `json:"ha_url"`
    HaToken         string                    `json:"ha_token"`
    EnabledEntities map[string]bool           `json:"enabled_entities"`
    Hotkeys         map[string]*HotkeyBinding `json:"hotkeys"`  // entity_id → binding
    LogLevel        string                    `json:"log_level"` // debug|info|warn|error, default info
    LogFile         string                    `json:"log_file"`  // default ha-tray.log
}
```

`config.json` is the source of truth. Saved with `json.MarshalIndent`. When entities are toggled in the discovery table, `EnabledEntities` is updated and `saveConfig()` is called immediately, followed by `updateTrayMenu()` to refresh the tray.

## Global hotkey system

Platform-specific via build tags. `HotkeyManager` interface in `hotkey.go`:

- **Windows** (`hotkey_windows.go`): Pure Go via `golang.org/x/sys/windows`. Uses Win32 `RegisterHotKey`/`UnregisterHotKey`. Each registration spawns a goroutine running `GetMessage` to catch `WM_HOTKEY`. On trigger, calls `toggleEntityWs()`.
- **Linux/macOS**: No-op stubs. `Supported()` returns `false`. Hotkey column shows "N/A".

Hotkey text format: `Ctrl+Alt+L` — parsed by `ParseHotkeyString()` in `hotkey.go`. Requires at least one modifier + one key.

Flow:
1. On startup, all saved hotkeys for enabled entities (where `IsEnabled() == true`) are registered
2. User types combo in discovery table column 3 → validated → saved → registered
3. Empty entry clears the hotkey
4. Enable/disable checkbox also registers/unregisters the hotkey
5. On quit, `UnregisterAll()` cleans up

### Logging (`logger.go`)

`InitLogger(level, filePath)` called after `loadConfig()` in `main()`. Uses `slog.JSONHandler` writing to a log file (append mode). Sets the logger as `slog.Default()` so all `slog.Info/Error/Debug/Warn` calls go to the file.

All `log.*` calls have been replaced with `slog.*`. The HA token is never logged — only `"has_token": bool`.

The `-trigger` CLI mode also initializes the logger and writes to the same file.

### CLI trigger mode (`-trigger` flag)

`ha-tray -trigger <entity_id>` toggles an entity and exits — no GUI, no display server.

Parsed at the top of `main()` before `app.New()`. Loads `config.json`, calls `toggleEntityWs()`, prints result, `os.Exit()`. Fyne never initializes.

On Linux this replaces global hotkeys — users bind `ha-tray -trigger <entity>` as a custom keyboard shortcut in their desktop environment (GNOME, KDE, etc).

## Build and deploy

- `task run` — `go run .` (requires display server — Fyne opens a GUI window)
- `task build-win` — `fyne-cross windows` (requires Docker)
- `task deploy-win` — builds + SCPs zip to Windows server, extracts remotely
- `task deploy-drive` — builds + uploads via rclone
- Output: `fyne-cross/dist/windows-amd64/ha-tray.exe.zip`

## Gotchas

- **No tests.** No `_test.go` files exist.
- **No CI.**
- **Fyne requires a display server.** Cannot run headless or in CI without a virtual framebuffer.
- **`config.json` and `.env` contain HA credentials.** Never commit. Both are gitignored.
- **`fyne-cross` requires Docker running** or it fails silently.
- **`w.ShowAndRun()` vs `a.Run()`:** This project uses `a.Run()` after `w.SetContent()`. Both work; `ShowAndRun` just calls Show then Run.
- **Global hotkeys are Windows-only.** Linux and macOS are stubbed. No CGO dependency on those platforms.
- **`golang.org/x/sys`** is the only new dependency, used for Windows syscall wrappers.
