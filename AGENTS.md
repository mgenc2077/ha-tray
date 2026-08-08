# AGENTS.md

(`CLAUDE.md` is a symlink to this file.)

## Project

Single-package Go system tray app for toggling Home Assistant entities. Built with [Fyne](https://fyne.io/) v2.7 (GUI) and gorilla/websocket. Module: `github.com/mgenc2077/ha-tray`, Go 1.25.

## Architecture

- `main.go` — config load/save, Fyne window, system tray menu, config UI (debounced save), device discovery table (4 columns), `-trigger` CLI mode
- `ha.go` — Home Assistant REST and WebSocket API calls (discovery, toggle)
- `hotkey.go` — `HotkeyManager` interface, `HotkeyBinding` struct, hotkey string parsing/formatting
- `hotkey_windows.go` — Win32 `RegisterHotKey`/`UnregisterHotKey` via `golang.org/x/sys/windows` lazy DLL procs (no CGO); each registration runs a `GetMessage` goroutine that calls `toggleEntityWs` on `WM_HOTKEY`
- `hotkey_linux.go` / `hotkey_darwin.go` — No-op stubs, `Supported()` returns `false`
- `logger.go` — structured JSON file logging via `log/slog`, installed as `slog.Default()`
- `paths.go` — resolves where `config.json` and the log file live (see below)
- `packaging/` — `PKGBUILD` + `.desktop` entry for the Arch package built in CI
- `FyneApp.toml` — app metadata (name, ID, version) read by `fyne package`
- `config.json` — runtime config (HA URL, token, enabled entities, hotkey bindings, log settings). Gitignored; auto-created on first run
- `.env` — alternative source for `haURL` / `haToken` env vars. Gitignored

All code is in `package main`. No internal packages or libraries. Platform-specific code uses Go build tags (`//go:build windows`, etc.).

`.opencode/skills/ha-tray-context/SKILL.md` holds a longer reference (Fyne widget patterns, HA WebSocket handshake, hotkey flow) — read it before non-trivial UI or protocol changes.

## Data flow

`loadConfig()` calls `resolvePaths()` first, then reads `.env` (via godotenv), then `config.json` overwrites those fields. `config` is a package-level global written directly by UI callbacks; every mutation calls `saveConfig()` and, when entity enablement changes, `updateTrayMenu()` to rebuild the tray from `config.EnabledEntities`.

`paths.go` picks between two modes, once per process:

- **portable** — a `config.json` exists in the working directory: everything stays there. This is what `go run .` in a clone and the unpacked Windows zip use.
- **per-user** — otherwise: `os.UserConfigDir()/ha-tray/config.json` and `$XDG_STATE_HOME` (or `~/.local/state`)`/ha-tray/ha-tray.log`. This is the path a `/usr/bin` install takes, and it is what makes `-trigger` work from a keyboard shortcut with an arbitrary cwd.

`cfgPath` and `portable` are resolved once so load and save can never disagree — don't call `os.Stat("config.json")` again elsewhere.

Toggling always goes through `toggleEntityWs()` (WebSocket: `auth_required` → `auth` → `auth_ok` → `call_service` → `result`). The REST `toggleEntity()` in `ha.go` is currently dead code.

## Commands

```bash
go run .                 # run the app (needs a display server)
go build -o ha-tray .
go vet ./...
```

Task runner is [Taskfile](https://taskfile.dev/), but the file is named `taskfile2.yml` and is **gitignored**, so `task` will not auto-discover it — pass it explicitly:

| Command | Description |
|---|---|
| `task -t taskfile2.yml run` | `go run .` |
| `task -t taskfile2.yml build-win` | Cross-compile for Windows (`fyne-cross windows`, requires Docker) |
| `task -t taskfile2.yml deploy-win` | Build + SCP + remote extract on a Windows host |
| `task -t taskfile2.yml deploy-drive` | Build + upload via rclone (`rclone.local.conf`, gitignored) |

Packaging with icon/metadata uses the Fyne CLI (`go install fyne.io/tools/cmd/fyne@latest`):

```bash
fyne package -os linux   -icon Icon.png -release
fyne package -os windows -icon Icon.png -release
```

## Headless CLI mode

`ha-tray -trigger <entity_id>` toggles an entity via the WebSocket API and exits (0 on success, 1 on failure). No display server needed — parsed from `os.Args` at the top of `main()` before `app.New()`, so it must be the first argument (the `flag` package is not used). This is the Linux alternative to global hotkeys: users bind it as a custom desktop-environment shortcut.

## CI

`.github/workflows/release.yml` runs on `v*` tags, or manually via `workflow_dispatch` with a `tag` input (used to re-cut a release for an existing tag without re-tagging).

A `prepare` job resolves the tag once — `inputs.tag` on a manual run, `github.ref_name` on a tag push — rejects unsafe values, and verifies the tag exists via `git ls-remote` before anything builds. Every other job depends on it: the three build jobs check out `needs.prepare.outputs.tag`, and `release` passes it as `tag_name` (required on a manual run, where `github.ref` is a branch). **Never read `github.ref_name` directly in a job** — it is the branch name on a manual run.

- `build-linux` / `build-windows` — `fyne package` on `ubuntu-latest` / `windows-latest`
- `build-arch` — `makepkg` inside an `archlinux:latest` container. The job stamps `pkgver` into `packaging/PKGBUILD` from the tag, then `makepkg` fetches that tag's tarball from GitHub (`sha256sums=('SKIP')`). It runs as an unprivileged `builder` user because makepkg refuses to run as root.

There is no build/test/lint workflow on push or PR.

Packaging notes: the Arch build is **X11-only** (no wayland build tag) and glfw `dlopen`s the X extensions, so `ldd` under-reports the real dependencies — check `strings` for sonames before touching `depends`. `pkgver` forbids hyphens, so `v1.2-rc1` is not a usable tag.

## Important constraints

- **Fyne requires a display server.** Except for `-trigger` mode, the app opens a GUI window and registers a system tray icon. Verifying changes requires a desktop environment.
- **UI updates from goroutines must be wrapped in `fyne.Do()`.** Discovery runs in a goroutine; every dialog/widget mutation from it goes through `fyne.Do`. Skipping this causes races and crashes.
- **No tests exist.** There are no `_test.go` files. `hotkey.go` (parsing/formatting) is the pure-logic part that is testable without a display.
- **Global hotkeys are Windows-only.** Linux/macOS stubs make the hotkey column render as a disabled "N/A" entry.
- **`fyne-cross` requires Docker.** Cross-compilation (Windows builds) wraps Docker and fails without it.
- **`config.json` is runtime state.** Do not commit it. It holds HA credentials and is auto-generated on first launch, in the repo directory (portable mode) or `~/.config/ha-tray/`. Never log the token — existing code logs `has_token: bool` instead.
- **Licensed GPL-3.0-or-later** (`LICENSE`). Keep `license=` in `packaging/PKGBUILD` in sync if that ever changes.
