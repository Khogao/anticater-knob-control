package main

import (
	"os"
	"syscall"
	"testing"
	"time"
)

// Talks to the real screens, so it only runs when asked:
//
//	KNOB_TEST_DDC=1 go test ./windows -run Brightness -v
//
// For every screen it moves the brightness one step and puts it back.
func TestBrightnessRoundTrip(t *testing.T) {
	if os.Getenv("KNOB_TEST_DDC") == "" {
		t.Skip("set KNOB_TEST_DDC=1 to change the brightness of the attached screens")
	}
	var monitors []uintptr
	enum := syscall.NewCallback(func(h, _, _, _ uintptr) uintptr { monitors = append(monitors, h); return 1 })
	user32.NewProc("EnumDisplayMonitors").Call(0, 0, enum, 0)
	if len(monitors) == 0 {
		t.Fatal("no screens found")
	}
	internal := internalDisplays()
	t.Logf("built-in displays: %v", internal)
	for _, h := range monitors {
		device := monitorDevice(h)
		before, ok := level(h, device)
		if !ok {
			t.Logf("%s: no brightness control (DDC/CI off, or not supported)", device)
			continue
		}
		step := brightnessStep
		if before.cur+step > before.max {
			step = -step
		}
		applyBrightness(h, step)
		after, _ := level(h, device)
		applyBrightness(h, -step)
		restored, _ := level(h, device)
		t.Logf("%s (built-in=%v): %d -> %d -> %d", device, internal[device], before.cur, after.cur, restored.cur)
		if after.cur != before.cur+step {
			t.Errorf("%s did not move by %d", device, step)
		}
		if restored.cur != before.cur {
			t.Errorf("%s is at %d, was %d", device, restored.cur, before.cur)
		}
	}
}

// level reads a screen's level from the hardware instead of trusting the cache.
func level(h uintptr, device string) (monitorLevel, bool) {
	time.Sleep(time.Second) // screens and the panel driver report the old value for a moment
	panelLevel.readAt = time.Time{}
	for i := range brightnessLevels[device] {
		brightnessLevels[device][i].readAt = time.Time{}
	}
	if _, ok := applyBrightness(h, 0); !ok {
		return monitorLevel{}, false
	}
	if internalDisplays()[device] {
		return panelLevel, true
	}
	for _, l := range brightnessLevels[device] {
		if l.ok {
			return l, true
		}
	}
	return monitorLevel{}, false
}
