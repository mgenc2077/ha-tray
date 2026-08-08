package main

import (
	"os"
	"path/filepath"
	"runtime"
)

const appDirName = "ha-tray"

var (
	// portable is true when a config.json sits in the working directory. The
	// app then keeps every runtime file there — the `go run .` dev workflow and
	// the unpacked-zip Windows workflow both rely on this. Otherwise files go to
	// the per-user XDG locations, which is what a /usr/bin install needs.
	portable bool
	// cfgPath is the file the app reads and writes its configuration from.
	// Resolved once by resolvePaths so load and save can never disagree.
	cfgPath string
)

// resolvePaths decides between portable and per-user locations. Called once at
// the top of loadConfig, before anything touches the filesystem.
func resolvePaths() {
	if info, err := os.Stat("config.json"); err == nil && !info.IsDir() {
		portable = true
		cfgPath = "config.json"
		return
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		// No HOME (or %AppData%) to work with — fall back to the working directory.
		portable = true
		cfgPath = "config.json"
		return
	}
	cfgPath = filepath.Join(dir, appDirName, "config.json")
}

// stateDir returns the base directory for logs and other non-config state.
// $XDG_STATE_HOME is the freedesktop location for this; Go's stdlib has no
// equivalent of os.UserConfigDir for it, so it is resolved by hand.
func stateDir() string {
	if runtime.GOOS != "windows" {
		if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
			return dir
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "state")
		}
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return dir
	}
	return "."
}

// logPath resolves the configured log_file value to a usable location.
// Absolute paths are honoured verbatim; a relative name (the default) lands
// beside the config in portable mode, or under the state directory otherwise.
func logPath(configured string) string {
	if configured == "" {
		configured = "ha-tray.log"
	}
	if filepath.IsAbs(configured) || portable {
		return configured
	}
	return filepath.Join(stateDir(), appDirName, configured)
}
