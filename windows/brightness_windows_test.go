package main

import (
	"os"
	"testing"
	"time"
)

// Talks to the real monitors, so it only runs when asked:
//
//	KNOB_TEST_DDC=1 go test ./windows -run Brightness -v
//
// It raises the brightness one step and puts it back.
func TestBrightnessRoundTrip(t *testing.T) {
	if os.Getenv("KNOB_TEST_DDC") == "" {
		t.Skip("set KNOB_TEST_DDC=1 to change the brightness of the attached monitors")
	}
	applyBrightness(0)
	if len(brightnessLevels) == 0 {
		t.Fatal("no physical monitors found")
	}
	before := append([]monitorLevel(nil), brightnessLevels...)
	for i, l := range before {
		t.Logf("monitor %d: ddc=%v level %d (range %d..%d)", i, l.ok, l.cur, l.min, l.max)
	}
	step := brightnessStep
	if before[0].ok && before[0].cur+step > before[0].max {
		step = -step
	}
	applyBrightness(step)
	reread()
	for i, l := range brightnessLevels {
		t.Logf("monitor %d after %+d: %d", i, step, l.cur)
		if before[i].ok && l.cur == before[i].cur && before[i].cur+step <= before[i].max && before[i].cur+step >= before[i].min {
			t.Errorf("monitor %d did not move", i)
		}
	}
	applyBrightness(-step)
	reread()
	for i, l := range brightnessLevels {
		t.Logf("monitor %d restored: %d", i, l.cur)
		if before[i].ok && l.cur != before[i].cur {
			t.Errorf("monitor %d is at %d, was %d", i, l.cur, before[i].cur)
		}
	}
}

// reread fetches the levels from the hardware instead of trusting the cache.
func reread() {
	time.Sleep(200 * time.Millisecond) // some monitors report the old value for a moment
	for i := range brightnessLevels {
		brightnessLevels[i].readAt = time.Time{}
	}
	applyBrightness(0)
}
