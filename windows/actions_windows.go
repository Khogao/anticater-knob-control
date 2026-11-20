package main

import (
	"os/exec"
	"strings"
	"syscall"
)

const (
	vkShift, vkControl, vkMenu, vkLWin                = 0x10, 0x11, 0x12, 0x5B
	vkVolumeMute, vkVolumeDown, vkVolumeUp            = 0xAD, 0xAE, 0xAF
	vkMediaNext, vkMediaPrev, vkMediaPlayPause        = 0xB0, 0xB1, 0xB3
	mouseLeftDown, mouseLeftUp                        = 0x0002, 0x0004
	mouseRightDown, mouseRightUp                      = 0x0008, 0x0010
	mouseMiddleDown, mouseMiddleUp                    = 0x0020, 0x0040
	createNoWindow                             uint32 = 0x08000000
)

var namedKeys = map[string]uint16{
	"ctrl": vkControl, "control": vkControl, "command": vkControl, "cmd": vkControl,
	"shift": vkShift, "alt": vkMenu, "option": vkMenu, "opt": vkMenu,
	"win": vkLWin, "super": vkLWin, "meta": vkLWin,

	"enter": 0x0D, "return": 0x0D, "esc": 0x1B, "escape": 0x1B, "space": 0x20, "tab": 0x09,
	"backspace": 0x08, "delete": 0x2E, "del": 0x2E, "insert": 0x2D,
	"home": 0x24, "end": 0x23, "pageup": 0x21, "pagedown": 0x22,
	"left": 0x25, "up": 0x26, "right": 0x27, "down": 0x28,

	"[": 0xDB, "]": 0xDD, "-": 0xBD, "minus": 0xBD, "=": 0xBB, "plus": 0xBB, "equal": 0xBB,
	",": 0xBC, ".": 0xBE, "/": 0xBF, ";": 0xBA, "'": 0xDE, "`": 0xC0, "\\": 0xDC,

	"volume_up": vkVolumeUp, "volume_down": vkVolumeDown, "mute": vkVolumeMute,
	"play_pause": vkMediaPlayPause, "next_track": vkMediaNext, "previous_track": vkMediaPrev,

	// Not real keys: they set monitor brightness over DDC/CI (brightness_windows.go).
	"brightness_up": keyBrightnessUp, "brightness_down": keyBrightnessDown,
}

var extendedKeys = map[uint16]bool{
	0x2E: true, 0x2D: true, 0x24: true, 0x23: true, 0x21: true, 0x22: true,
	0x25: true, 0x26: true, 0x27: true, 0x28: true, vkLWin: true,
}

func isModifier(vk uint16) bool {
	return vk == vkShift || vk == vkControl || vk == vkMenu || vk == vkLWin
}

func lookupKey(name string) (uint16, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if vk, ok := namedKeys[n]; ok {
		return vk, true
	}
	if len(n) == 1 {
		ch := n[0]
		switch {
		case ch >= 'a' && ch <= 'z':
			return uint16(ch - 'a' + 'A'), true
		case ch >= '0' && ch <= '9':
			return uint16(ch), true
		}
	}
	if len(n) >= 2 && n[0] == 'f' {
		num := 0
		for _, ch := range n[1:] {
			if ch < '0' || ch > '9' {
				return 0, false
			}
			num = num*10 + int(ch-'0')
		}
		if num >= 1 && num <= 24 {
			return uint16(0x70 + num - 1), true
		}
	}
	return 0, false
}

// splitKeys separates modifiers (held) from ordinary keys (tapped in order).
func splitKeys(names []string) (mods, keys []uint16) {
	for _, n := range names {
		vk, ok := lookupKey(n)
		if !ok {
			continue
		}
		if isModifier(vk) {
			mods = append(mods, vk)
		} else {
			keys = append(keys, vk)
		}
	}
	return
}

func withModifiers(mods []uint16, fn func()) {
	for _, m := range mods {
		sendKey(m, false)
	}
	fn()
	for i := len(mods) - 1; i >= 0; i-- {
		sendKey(mods[i], true)
	}
}

// perform runs a non-passthrough action.
func perform(a Action) {
	switch a.Type {
	case "shortcut", "key":
		mods, keys := splitKeys(a.Keys)
		withModifiers(mods, func() {
			for _, k := range keys {
				switch k {
				case keyBrightnessUp:
					adjustBrightness(brightnessStep)
				case keyBrightnessDown:
					adjustBrightness(-brightnessStep)
				default:
					sendKey(k, false)
					sendKey(k, true)
				}
			}
		})
	case "scroll":
		mods, _ := splitKeys(a.Keys)
		withModifiers(mods, func() {
			if a.Dy != 0 {
				sendWheel(false, a.Dy)
			}
			if a.Dx != 0 {
				sendWheel(true, a.Dx)
			}
		})
	case "mouse":
		switch strings.ToLower(a.Button) {
		case "right":
			sendMouseButton(mouseRightDown, mouseRightUp)
		case "middle", "center":
			sendMouseButton(mouseMiddleDown, mouseMiddleUp)
		default:
			sendMouseButton(mouseLeftDown, mouseLeftUp)
		}
	case "shell":
		cmd := exec.Command("cmd.exe", "/c", a.Command)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
		} else {
			logf("shell failed: %v", err)
		}
	}
}
