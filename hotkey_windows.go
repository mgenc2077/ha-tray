//go:build windows

package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazyDLL("user32.dll")
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey    = user32.NewProc("UnregisterHotKey")
	procGetMessage          = user32.NewProc("GetMessageW")
	procPostThreadMessage   = user32.NewProc("PostThreadMessageW")
	procGetCurrentThreadId  = user32.NewProc("GetCurrentThreadId")
)

const (
	WM_HOTKEY          = 0x0312
	WM_USER_QUIT       = WM_HOTKEY + 0x1000
	MOD_ALT     uint32 = 0x0001
	MOD_CONTROL uint32 = 0x0002
	MOD_SHIFT   uint32 = 0x0004
	MOD_WIN     uint32 = 0x0008
)

type hotkeyEntry struct {
	entityID string
	threadID uint32
	cancel   chan struct{}
}

type winHotkeyManager struct {
	mu      sync.Mutex
	nextID  atomic.Uintptr
	entries map[int]*hotkeyEntry
}

func newPlatformHotkeyManager() HotkeyManager {
	m := &winHotkeyManager{
		entries: make(map[int]*hotkeyEntry),
	}
	m.nextID.Store(1)
	return m
}

func (m *winHotkeyManager) Supported() bool {
	return true
}

func (m *winHotkeyManager) Register(entityID string, mods []string, key string) error {
	modFlags := parseModifiers(mods)
	vk, err := parseVirtualKey(key)
	if err != nil {
		return fmt.Errorf("invalid key %q: %w", key, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, entry := range m.entries {
		if entry.entityID == entityID {
			m.unregisterLocked(id)
			break
		}
	}

	hotkeyID := int(m.nextID.Add(1))

	ret, _, err := procRegisterHotKey.Call(
		0, uintptr(hotkeyID), uintptr(modFlags), uintptr(vk),
	)
	if ret == 0 {
		return fmt.Errorf("RegisterHotKey failed (possibly already in use): %w", err)
	}

	cancel := make(chan struct{})
	entry := &hotkeyEntry{
		entityID: entityID,
		cancel:   cancel,
	}
	m.entries[hotkeyID] = entry

	go m.listenThread(hotkeyID, entry)

	return nil
}

func (m *winHotkeyManager) listenThread(hotkeyID int, entry *hotkeyEntry) {
	tidRet, _, _ := procGetCurrentThreadId.Call()
	entry.threadID = uint32(tidRet)

	var msg [48]byte

	for {
		ret, _, _ := procGetMessage.Call(
			uintptr(unsafe.Pointer(&msg[0])),
			0, 0, 0,
		)
		if ret == 0 || int32(ret) == -1 {
			return
		}

		message := *(*uint32)(unsafe.Pointer(&msg[4]))
		wParam := *(*uintptr)(unsafe.Pointer(&msg[8]))

		if message == WM_USER_QUIT {
			return
		}

		if message == WM_HOTKEY && int(wParam) == hotkeyID {
			go func(eid string) {
				_ = toggleEntityWs(eid)
			}(entry.entityID)
		}
	}
}

func (m *winHotkeyManager) Unregister(entityID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, entry := range m.entries {
		if entry.entityID == entityID {
			m.unregisterLocked(id)
			return nil
		}
	}
	return nil
}

func (m *winHotkeyManager) unregisterLocked(id int) {
	entry, ok := m.entries[id]
	if !ok {
		return
	}
	procUnregisterHotKey.Call(0, uintptr(id))
	if entry.threadID != 0 {
		procPostThreadMessage.Call(
			uintptr(entry.threadID),
			WM_USER_QUIT, 0, 0,
		)
	}
	close(entry.cancel)
	delete(m.entries, id)
}

func (m *winHotkeyManager) UnregisterAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id := range m.entries {
		m.unregisterLocked(id)
	}
}

func parseModifiers(mods []string) uint32 {
	var flags uint32
	for _, m := range mods {
		switch lower(m) {
		case "ctrl", "control":
			flags |= MOD_CONTROL
		case "alt":
			flags |= MOD_ALT
		case "shift":
			flags |= MOD_SHIFT
		case "win", "super":
			flags |= MOD_WIN
		}
	}
	return flags
}

func parseVirtualKey(key string) (uint32, error) {
	l := lower(key)
	if len(l) == 1 && l[0] >= 'a' && l[0] <= 'z' {
		return uint32(l[0]) - 32, nil
	}
	switch l {
	case "0":
		return 0x30, nil
	case "1":
		return 0x31, nil
	case "2":
		return 0x32, nil
	case "3":
		return 0x33, nil
	case "4":
		return 0x34, nil
	case "5":
		return 0x35, nil
	case "6":
		return 0x36, nil
	case "7":
		return 0x37, nil
	case "8":
		return 0x38, nil
	case "9":
		return 0x39, nil
	case "f1":
		return 0x70, nil
	case "f2":
		return 0x71, nil
	case "f3":
		return 0x72, nil
	case "f4":
		return 0x73, nil
	case "f5":
		return 0x74, nil
	case "f6":
		return 0x75, nil
	case "f7":
		return 0x76, nil
	case "f8":
		return 0x77, nil
	case "f9":
		return 0x78, nil
	case "f10":
		return 0x79, nil
	case "f11":
		return 0x7A, nil
	case "f12":
		return 0x7B, nil
	case "escape", "esc":
		return 0x1B, nil
	case "tab":
		return 0x09, nil
	case "enter", "return":
		return 0x0D, nil
	case "space":
		return 0x20, nil
	case "backspace":
		return 0x08, nil
	case "insert":
		return 0x2D, nil
	case "delete", "del":
		return 0x2E, nil
	case "home":
		return 0x24, nil
	case "end":
		return 0x23, nil
	case "pageup":
		return 0x21, nil
	case "pagedown":
		return 0x22, nil
	case "up":
		return 0x26, nil
	case "down":
		return 0x28, nil
	case "left":
		return 0x25, nil
	case "right":
		return 0x27, nil
	case "numpad0":
		return 0x60, nil
	case "numpad1":
		return 0x61, nil
	case "numpad2":
		return 0x62, nil
	case "numpad3":
		return 0x63, nil
	case "numpad4":
		return 0x64, nil
	case "numpad5":
		return 0x65, nil
	case "numpad6":
		return 0x66, nil
	case "numpad7":
		return 0x67, nil
	case "numpad8":
		return 0x68, nil
	case "numpad9":
		return 0x69, nil
	}
	return 0, fmt.Errorf("unsupported key: %s", key)
}
