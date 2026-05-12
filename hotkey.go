package main

type HotkeyBinding struct {
	Modifiers []string `json:"modifiers"`
	Key       string   `json:"key"`
}

type HotkeyManager interface {
	Register(entityID string, mods []string, key string) error
	Unregister(entityID string) error
	UnregisterAll()
	Supported() bool
}

func NewHotkeyManager() HotkeyManager {
	return newPlatformHotkeyManager()
}

func ParseHotkeyString(s string) (mods []string, key string, ok bool) {
	if s == "" {
		return nil, "", false
	}
	parts := splitHotkeyString(s)
	if len(parts) == 0 {
		return nil, "", false
	}
	key = parts[len(parts)-1]
	if !isValidKey(key) {
		return nil, "", false
	}
	mods = parts[:len(parts)-1]
	for _, m := range mods {
		if !isValidModifier(m) {
			return nil, "", false
		}
	}
	if len(mods) == 0 {
		return nil, "", false
	}
	return mods, key, true
}

func splitHotkeyString(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == '+' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func isValidModifier(m string) bool {
	switch lower(m) {
	case "ctrl", "control", "alt", "shift", "win", "super":
		return true
	}
	return false
}

func isValidKey(k string) bool {
	l := lower(k)
	if len(l) == 1 && l[0] >= 'a' && l[0] <= 'z' {
		return true
	}
	switch l {
	case "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12",
		"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
		"escape", "esc",
		"tab", "enter", "return",
		"space",
		"backspace",
		"insert", "delete", "del",
		"home", "end",
		"pageup", "pagedown",
		"up", "down", "left", "right",
		"numpad0", "numpad1", "numpad2", "numpad3", "numpad4",
		"numpad5", "numpad6", "numpad7", "numpad8", "numpad9":
		return true
	}
	return false
}

func lower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func FormatHotkeyBinding(b *HotkeyBinding) string {
	if b == nil {
		return ""
	}
	s := ""
	for i, m := range b.Modifiers {
		if i > 0 {
			s += "+"
		}
		s += capitalize(m)
	}
	if s != "" {
		s += "+"
	}
	s += capitalize(b.Key)
	return s
}

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	l := lower(s)
	return string(l[0]-32) + l[1:]
}
