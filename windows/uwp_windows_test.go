package main

import (
	"strings"
	"syscall"
	"testing"
)

// A Store app (Settings, Calculator...) open on screen must be reported as its
// own exe, not ApplicationFrameHost.exe. Skips when none is open.
func TestUWPWindowApp(t *testing.T) {
	var frames []uintptr
	cb := syscall.NewCallback(func(h, _ uintptr) uintptr {
		if vis, _, _ := pIsWindowVisible.Call(h); vis != 0 {
			if ic, _, _ := pIsIconic.Call(h); ic != 0 {
				return 1 // minimised: the app's window is detached from its frame then
			}
			if exe, _ := windowProcess(h); strings.EqualFold(exe, "ApplicationFrameHost.exe") {
				frames = append(frames, h)
			}
		}
		return 1
	})
	pEnumWindows.Call(cb, 0)
	found := false
	for _, h := range frames {
		exe, title := windowApp(h)
		if title == "" {
			continue
		}
		t.Logf("%q -> %s", title, exe)
		if !strings.EqualFold(exe, "ApplicationFrameHost.exe") {
			found = true
		}
	}
	if len(frames) == 0 {
		t.Skip("no Store app window open")
	}
	if !found {
		t.Error("no frame host window resolved to its app")
	}
}
