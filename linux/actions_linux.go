package main

import (
	"os/exec"
	"strings"
	"syscall"
)

// Linux key codes (linux/input-event-codes.h).
const (
	keyEsc            = 1
	keyBackspace      = 14
	keyTab            = 15
	keyEnter          = 28
	keyLeftCtrl       = 29
	keyLeftShift      = 42
	keyRightShift     = 54
	keyLeftAlt        = 56
	keySpace          = 57
	keyRightCtrl      = 97
	keyRightAlt       = 100
	keyHome           = 102
	keyUp             = 103
	keyPageUp         = 104
	keyLeft           = 105
	keyRight          = 106
	keyEnd            = 107
	keyDown           = 108
	keyPageDown       = 109
	keyInsert         = 110
	keyDelete         = 111
	keyMute           = 113
	keyVolumeDown     = 114
	keyVolumeUp       = 115
	keyLeftMeta       = 125
	keyRightMeta      = 126
	keyNextSong       = 163
	keyPlayPause      = 164
	keyPreviousSong   = 165
	keyF13            = 183
	keyBrightnessDown = 224
	keyBrightnessUp   = 225
)

var namedKeys = map[string]uint16{
	"ctrl": keyLeftCtrl, "control": keyLeftCtrl, "command": keyLeftCtrl, "cmd": keyLeftCtrl,
	"shift": keyLeftShift, "alt": keyLeftAlt, "option": keyLeftAlt, "opt": keyLeftAlt,
	"win": keyLeftMeta, "super": keyLeftMeta, "meta": keyLeftMeta,

	"enter": keyEnter, "return": keyEnter, "esc": keyEsc, "escape": keyEsc, "space": keySpace, "tab": keyTab,
	"backspace": keyBackspace, "delete": keyDelete, "del": keyDelete, "insert": keyInsert,
	"home": keyHome, "end": keyEnd, "pageup": keyPageUp, "pagedown": keyPageDown,
	"left": keyLeft, "up": keyUp, "right": keyRight, "down": keyDown,

	"[": 26, "]": 27, "-": 12, "minus": 12, "=": 13, "plus": 13, "equal": 13,
	",": 51, ".": 52, "/": 53, ";": 39, "'": 40, "`": 41, "\\": 43,

	"volume_up": keyVolumeUp, "volume_down": keyVolumeDown, "mute": keyMute,
	"play_pause": keyPlayPause, "next_track": keyNextSong, "previous_track": keyPreviousSong,
	// On Linux these are the real brightness keys; Plasma acts on them (laptop
	// panel, and external monitors over DDC/CI when Plasma can reach them).
	"brightness_up": keyBrightnessUp, "brightness_down": keyBrightnessDown,
}

// Letters and digits by position on a US keyboard.
var letterKeys = map[byte]uint16{
	'q': 16, 'w': 17, 'e': 18, 'r': 19, 't': 20, 'y': 21, 'u': 22, 'i': 23, 'o': 24, 'p': 25,
	'a': 30, 's': 31, 'd': 32, 'f': 33, 'g': 34, 'h': 35, 'j': 36, 'k': 37, 'l': 38,
	'z': 44, 'x': 45, 'c': 46, 'v': 47, 'b': 48, 'n': 49, 'm': 50,
	'1': 2, '2': 3, '3': 4, '4': 5, '5': 6, '6': 7, '7': 8, '8': 9, '9': 10, '0': 11,
}

// F1..F24
var fKeys = []uint16{59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 87, 88, 183, 184, 185, 186, 187, 188, 189, 190, 191, 192, 193, 194}

func isModifier(k uint16) bool {
	switch k {
	case keyLeftCtrl, keyRightCtrl, keyLeftShift, keyRightShift, keyLeftAlt, keyRightAlt, keyLeftMeta, keyRightMeta:
		return true
	}
	return false
}

func lookupKey(name string) (uint16, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	if k, ok := namedKeys[n]; ok {
		return k, true
	}
	if len(n) == 1 {
		if k, ok := letterKeys[n[0]]; ok {
			return k, true
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
			return fKeys[num-1], true
		}
	}
	return 0, false
}

// splitKeys separates modifiers (held) from ordinary keys (tapped in order).
func splitKeys(names []string) (mods, keys []uint16) {
	for _, n := range names {
		k, ok := lookupKey(n)
		if !ok {
			continue
		}
		if isModifier(k) {
			mods = append(mods, k)
		} else {
			keys = append(keys, k)
		}
	}
	return
}

func withModifiers(mods []uint16, fn func()) {
	for _, m := range mods {
		out.key(m, true)
	}
	fn()
	for i := len(mods) - 1; i >= 0; i-- {
		out.key(mods[i], false)
	}
}

// perform runs a non-passthrough action.
func perform(a Action) {
	switch a.Type {
	case "shortcut", "key":
		mods, keys := splitKeys(a.Keys)
		withModifiers(mods, func() {
			for _, k := range keys {
				out.tap(k)
			}
		})
	case "scroll":
		mods, _ := splitKeys(a.Keys)
		withModifiers(mods, func() {
			if a.Dy != 0 {
				out.wheel(false, a.Dy)
			}
			if a.Dx != 0 {
				out.wheel(true, a.Dx)
			}
		})
	case "mouse":
		switch strings.ToLower(a.Button) {
		case "right":
			out.click(btnRight)
		case "middle", "center":
			out.click(btnMiddle)
		default:
			out.click(btnLeft)
		}
	case "shell":
		cmd := exec.Command("/bin/sh", "-c", a.Command)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
		} else {
			logf("shell failed: %v", err)
		}
	}
}
