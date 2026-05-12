//go:build linux

package main

type linuxHotkeyManager struct{}

func newPlatformHotkeyManager() HotkeyManager {
	return &linuxHotkeyManager{}
}

func (m *linuxHotkeyManager) Register(entityID string, mods []string, key string) error {
	return nil
}

func (m *linuxHotkeyManager) Unregister(entityID string) error {
	return nil
}

func (m *linuxHotkeyManager) UnregisterAll() {}

func (m *linuxHotkeyManager) Supported() bool {
	return false
}
