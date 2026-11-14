package main

// Two UI languages: Vietnamese and English. Every user-visible string is written
// inline as L("tiếng Việt", "English"), so both versions sit next to each other.

import "syscall"

var pGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// langEN is true while the UI is in English.
var langEN bool

// L picks the string for the current language.
func L(vi, en string) string {
	if langEN {
		return en
	}
	return vi
}

// text is a label kept in both languages, for tables built at package level.
type text struct{ vi, en string }

func (t text) String() string { return L(t.vi, t.en) }

// setLanguage applies "vi" or "en". Anything else follows the Windows display
// language: Vietnamese when Windows is in Vietnamese, English otherwise.
func setLanguage(code string) {
	switch code {
	case "vi":
		langEN = false
	case "en":
		langEN = true
	default:
		id, _, _ := pGetUserDefaultUILanguage.Call()
		langEN = id&0x3FF != 0x2A // primary language id 0x2A is Vietnamese
	}
}

func languageCode() string {
	if langEN {
		return "en"
	}
	return "vi"
}
