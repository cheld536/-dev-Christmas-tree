package main

const (
	wmHotkey           = 0x0312
	fullscreenHotkeyID = 1
	vkF11              = 0x7A
	hotkeyNoRepeat     = 0x4000
	hotkeyShiftWin     = 0x0004 | 0x0008
	hotkeyCtrlShift    = 0x0002 | 0x0004
)

var fullscreenHotkeyModifiers uintptr

func registerFullscreenHotkey(hwnd uintptr) string {
	for _, option := range []struct {
		modifiers uintptr
		label     string
	}{{hotkeyShiftWin, "Win+Shift+F11"}, {hotkeyCtrlShift, "Ctrl+Shift+F11"}} {
		if call(u, "RegisterHotKey", hwnd, fullscreenHotkeyID, option.modifiers|hotkeyNoRepeat, vkF11) != 0 {
			fullscreenHotkeyModifiers = option.modifiers
			return option.label
		}
	}
	return "앱에서 F11"
}

func unregisterFullscreenHotkey(hwnd uintptr) {
	if fullscreenHotkeyModifiers != 0 {
		call(u, "UnregisterHotKey", hwnd, fullscreenHotkeyID)
		fullscreenHotkeyModifiers = 0
	}
}

func handleFullscreenHotkey(id, keys uintptr) {
	if fullscreenHotkeyModifiers == 0 || id != fullscreenHotkeyID ||
		keys&(0xffff&^hotkeyNoRepeat) != fullscreenHotkeyModifiers || keys>>16 != vkF11 {
		return
	}
	if fullScreen {
		stopSaver()
	} else {
		openFull()
	}
}

func plainF11() bool {
	for _, key := range []uintptr{0x10, 0x11, 0x12, 0x5b, 0x5c} {
		if call(u, "GetKeyState", key)&0x8000 != 0 {
			return false
		}
	}
	return true
}
