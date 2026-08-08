# Home Assistant Tray

A cross-platform native system tray application for toggling Home Assistant entities. Inspired by [home-assistant-tray-menu](https://github.com/PascalLuginbuehl/home-assistant-tray-menu).

![Settings](screenshots/SettingsMenu.png)

![Devices](screenshots/DevicesMenu.png)

## Features

- System tray icon with per-entity toggle menu
- Entity discovery and selection via HA REST API
- Entity toggle via HA WebSocket API
- Global hotkeys (Windows only — type `Ctrl+Alt+K` style bindings per entity)
- Debounced auto-save configuration

## Tech Stack

- **Go 1.24+** — single `package main`, no internal packages
- **[Fyne](https://fyne.io/) v2** — cross-platform GUI framework (system tray, tables, dialogs)
- **[gorilla/websocket](https://github.com/gorilla/websocket)** — HA WebSocket API client
- **[godotenv](https://github.com/joho/godotenv)** — `.env` file loading
- `logger.go` — structured JSON logging via `log/slog`
- **[golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys)** — Win32 syscall wrappers for global hotkeys
- **[log/slog](https://pkg.go.dev/log/slog)** — structured JSON file logging (stdlib)

## Prerequisites

- Go 1.24+
- C compiler (GCC on Linux, MSVC on Windows — CGO is required by Fyne)
- Display server (the app opens a GUI window and registers a system tray icon)

**Linux** — install development headers:
```bash
sudo apt-get install gcc libgl1-mesa-dev libwayland-dev libx11-dev libxkbcommon-dev xorg-dev
```

**Windows** — no extra dependencies. CGO uses the built-in MSVC toolchain.

## Install

### Arch Linux

Download `ha-tray-*.pkg.tar.zst` from the [latest release](https://github.com/mgenc2077/ha-tray/releases) and install it:

```bash
sudo pacman -U ha-tray-*.pkg.tar.zst
```

This installs `/usr/bin/ha-tray` plus a desktop entry, so the app appears in your launcher. Not published to the AUR — the package is built in CI from the tagged source ([`packaging/PKGBUILD`](packaging/PKGBUILD)). To build it yourself from a clone:

```bash
cd packaging && makepkg -si
```

### Other platforms

Grab the Linux tarball or `ha-tray.exe` from the release page, or build locally.

## Build Locally

```bash
go build -o ha-tray .
```

Or with the Fyne CLI (includes icon and metadata embedding):
```bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os linux -icon Icon.png -release   # produces ha-tray.tar.gz
fyne package -os windows -icon Icon.png -release  # produces ha-tray.exe
```

## Run

```bash
go run .
```

On first launch the app creates `config.json` and opens the settings window. Enter your Home Assistant URL and long-lived access token.

## Configuration

The app reads configuration from two sources (in order):

1. **`.env`** file in the working directory — set `haURL` and `haToken` environment variables
2. **`config.json`** — auto-created runtime config, edited via the settings UI

Both files contain credentials and are gitignored.

### File locations

`config.json` is looked up in two places:

| Mode | Condition | Config | Log |
|---|---|---|---|
| Portable | `config.json` exists in the working directory | `./config.json` | `./ha-tray.log` |
| Per-user | otherwise | `~/.config/ha-tray/config.json` | `~/.local/state/ha-tray/ha-tray.log` |

Portable mode keeps everything next to the binary — this is what `go run .` in a clone and the unpacked Windows zip both use. A system-wide install (`/usr/bin/ha-tray`) has no config beside it, so it falls back to the per-user paths, which is also what makes `ha-tray -trigger` work when launched from a keyboard shortcut with an arbitrary working directory.

The XDG environment variables `XDG_CONFIG_HOME` and `XDG_STATE_HOME` are honoured when set. On Windows the per-user location is `%AppData%\ha-tray\`. Setting `log_file` to an absolute path overrides the log location in either mode.

### Example `config.json`

```json
{
  "ha_url": "http://192.168.1.103:8123",
  "ha_token": "your-long-lived-access-token",
  "enabled_entities": {
    "switch.living_room": true
  },
  "hotkeys": {
    "switch.living_room": {
      "modifiers": ["ctrl", "alt"],
      "key": "l"
    },
    "switch.bedroom": {
      "modifiers": ["ctrl", "alt"],
      "key": "b",
      "enabled": false
    }
  },
  "log_level": "info",
  "log_file": "ha-tray.log"
}
```

### Global Hotkeys

Hotkeys are supported on **Windows only**. On Linux and macOS the hotkey column shows "N/A".

Format: `Ctrl+Alt+L`, `Ctrl+Shift+F5`, etc. At least one modifier is required.

Set `"enabled": false` on a hotkey binding to disable it without deleting the configuration. Omitting `enabled` defaults to `true`.

### Logging

All application events are written to a structured JSON log file. Configure in `config.json`:

| Field | Values | Default |
|---|---|---|
| `log_level` | `debug`, `info`, `warn`, `error` | `info` |
| `log_file` | absolute path, or a name resolved per [File locations](#file-locations) | `ha-tray.log` |

Log files are gitignored (`*.log`).

### CLI Trigger (Linux hotkey alternative)

The `-trigger` flag toggles an entity and exits immediately — no GUI, no display server needed. This is the recommended way to bind hotkeys on Linux:

```bash
ha-tray -trigger switch.living_room
```

Bind it in your desktop environment's keyboard shortcuts (e.g. GNOME Settings → Keyboard → Custom Shortcuts):

```
Name:     Toggle Living Room
Command:  /usr/bin/ha-tray -trigger switch.living_room
Shortcut: Ctrl+Alt+L
```

Exit code 0 on success, 1 on failure. Prints `toggled <entity>` to stdout or an error message to stderr.

## Cross-Compilation (Windows from Linux)

Uses [fyne-cross](https://github.com/fyne-io/fyne-cross) with Docker:

```bash
go install github.com/fyne-io/fyne-cross@latest
fyne-cross windows
```

Output: `fyne-cross/dist/windows-amd64/ha-tray.exe.zip`

## Releases

Pushing a `v*` tag triggers the [GitHub Actions release workflow](.github/workflows/release.yml):

```bash
git tag v1.0.0
git push origin v1.0.0
```

The workflow builds the Linux tarball and Windows `.exe` with `fyne package`, and the Arch `.pkg.tar.zst` with `makepkg` in an `archlinux:latest` container, then creates a GitHub Release with all three attached.

It can also be run manually from the Actions tab ("Build Release" → "Run workflow") with an existing tag as input — useful for re-running after a transient build failure without cutting a new tag. Note that `pkgver` forbids hyphens, so a tag like `v1.2-rc1` will fail the Arch job.

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).
