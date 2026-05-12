//go:build darwin

package main

type darwinHotkeyManager struct{}

func newPlatformHotkeyManager() HotkeyManager {
	return &darwinHotkeyManager{}
}

func (m *darwinHotkeyManager) Register(entityID string, mods []string, key string) error {
	return nil
}

func (m *darwinHotkeyManager) Unregister(entityID string) error {
	return nil
}

func (m *darwinHotkeyManager) UnregisterAll() {}

func (m *darwinHotkeyManager) Supported() bool {
	return false
}
