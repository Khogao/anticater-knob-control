package main

// Screen brightness: external monitors over DDC/CI (dxva2.dll), and the built-in
// panel of a laptop through \\.\LCD.
//
// The knob's own brightness reports (usages 0x6F / 0x70) only move the built-in
// panel of a laptop; Windows ignores them for external monitors. The shortcut
// keys "brightness_up" / "brightness_down" talk to every screen directly.
// A DDC/CI call takes about 60 ms per monitor, far too slow for the thread
// that runs the keyboard hook, so the work happens on a goroutine that folds
// a burst of knob ticks into one change.

import (
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	// Pseudo key codes, above any real virtual-key code.
	keyBrightnessUp   uint16 = 0x1001
	keyBrightnessDown uint16 = 0x1002

	brightnessStep  = 5               // percent per knob tick
	brightnessStale = 4 * time.Second // re-read a monitor's level after this long
)

var (
	dxva2                    = syscall.NewLazyDLL("dxva2.dll")
	pEnumDisplayMonitors     = user32.NewProc("EnumDisplayMonitors")
	pGetNumberOfPhysMonitors = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	pGetPhysicalMonitors     = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	pDestroyPhysicalMonitors = dxva2.NewProc("DestroyPhysicalMonitors")
	pGetMonitorBrightness    = dxva2.NewProc("GetMonitorBrightness")
	pSetMonitorBrightness    = dxva2.NewProc("SetMonitorBrightness")

	brightnessPending atomic.Int32
	brightnessWake    = make(chan struct{}, 1)
	brightnessOnce    sync.Once

	// Owned by the worker goroutine.
	brightnessLevels []monitorLevel
	monitorHandles   []uintptr
)

type physicalMonitor struct {
	Handle      uintptr
	Description [128]uint16
}

type monitorLevel struct {
	min, cur, max int
	ok            bool
	readAt        time.Time
}

// adjustBrightness asks for the brightness of every monitor to move by delta
// percent. It returns at once.
func adjustBrightness(delta int) {
	brightnessOnce.Do(func() { go brightnessWorker() })
	brightnessPending.Add(int32(delta))
	select {
	case brightnessWake <- struct{}{}:
	default:
	}
}

func brightnessWorker() {
	for range brightnessWake {
		for {
			delta := int(brightnessPending.Swap(0))
			if delta == 0 {
				break
			}
			applyBrightness(delta)
		}
	}
}

func monitorEnumProc(hMonitor, _, _, _ uintptr) uintptr {
	monitorHandles = append(monitorHandles, hMonitor)
	return 1
}

var monitorEnumCallback = syscall.NewCallback(monitorEnumProc)

func applyBrightness(delta int) {
	monitorHandles = monitorHandles[:0]
	pEnumDisplayMonitors.Call(0, 0, monitorEnumCallback, 0)

	index, changed := 0, 0
	for _, hMonitor := range monitorHandles {
		var count uint32
		if ok, _, _ := pGetNumberOfPhysMonitors.Call(hMonitor, uintptr(unsafe.Pointer(&count))); ok == 0 || count == 0 {
			continue
		}
		physical := make([]physicalMonitor, count)
		if ok, _, _ := pGetPhysicalMonitors.Call(hMonitor, uintptr(count), uintptr(unsafe.Pointer(&physical[0]))); ok == 0 {
			continue
		}
		for _, p := range physical {
			if index >= len(brightnessLevels) {
				brightnessLevels = append(brightnessLevels, monitorLevel{})
			}
			level := &brightnessLevels[index]
			index++
			if time.Since(level.readAt) > brightnessStale {
				// The level can also be changed with the monitor's own buttons.
				var lo, cur, hi uint32
				ok, _, _ := pGetMonitorBrightness.Call(p.Handle, uintptr(unsafe.Pointer(&lo)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&hi)))
				*level = monitorLevel{min: int(lo), cur: int(cur), max: int(hi), ok: ok != 0 && hi > lo, readAt: time.Now()}
			}
			if !level.ok {
				continue // built-in panel, or a monitor with DDC/CI switched off
			}
			want := level.cur + delta
			if want < level.min {
				want = level.min
			}
			if want > level.max {
				want = level.max
			}
			if want == level.cur {
				changed++ // already at the end of its range
				continue
			}
			if ok, _, _ := pSetMonitorBrightness.Call(p.Handle, uintptr(want)); ok != 0 {
				level.cur, level.readAt = want, time.Now()
				changed++
			} else {
				level.readAt = time.Time{} // read it again next time
			}
		}
		pDestroyPhysicalMonitors.Call(uintptr(count), uintptr(unsafe.Pointer(&physical[0])))
	}
	brightnessLevels = brightnessLevels[:index]
	if adjustPanel(delta) {
		changed++
	}
	if changed == 0 {
		brightnessUnsupported.Store(true)
	}
}

// Built-in laptop panel, through the video driver's \\.\LCD device. It ignores
// DDC/CI, and when the knob's own brightness reports are turned off (README 9.5)
// this is the only way left to move it. Owned by the worker goroutine.
const (
	ioctlVideoQueryDisplayBrightness = 0x230498
	ioctlVideoSetDisplayBrightness   = 0x23049C
	displayPolicyBoth                = 3 // set the level for both mains and battery
)

type displayBrightness struct{ Policy, AC, DC uint8 }

var panelLevel monitorLevel

// adjustPanel moves the built-in panel by delta percent. It reports whether the
// panel exists and took the change (or is already at the end of its range).
func adjustPanel(delta int) bool {
	name, _ := syscall.UTF16PtrFromString(`\\.\LCD`)
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return false // no built-in panel (desktop), or the driver does not offer it
	}
	defer syscall.CloseHandle(h)
	var n uint32
	if time.Since(panelLevel.readAt) > brightnessStale {
		// Windows' own slider and keys change it too. The driver answers a query with
		// the level from before the last set for a moment, so the cached value wins
		// while it is fresh.
		var b displayBrightness
		err := syscall.DeviceIoControl(h, ioctlVideoQueryDisplayBrightness, nil, 0, (*byte)(unsafe.Pointer(&b)), 3, &n, nil)
		if err != nil {
			return false
		}
		cur := b.AC
		if b.Policy == 2 {
			cur = b.DC // on battery
		}
		panelLevel = monitorLevel{min: 0, cur: int(cur), max: 100, ok: true, readAt: time.Now()}
	}
	want := panelLevel.cur + delta
	if want < 0 {
		want = 0
	}
	if want > 100 {
		want = 100
	}
	if want == panelLevel.cur {
		return true
	}
	b := displayBrightness{Policy: displayPolicyBoth, AC: uint8(want), DC: uint8(want)}
	if err := syscall.DeviceIoControl(h, ioctlVideoSetDisplayBrightness, (*byte)(unsafe.Pointer(&b)), 3, nil, 0, &n, nil); err != nil {
		panelLevel.readAt = time.Time{}
		return false
	}
	panelLevel.cur, panelLevel.readAt = want, time.Now()
	return true
}

// brightnessUnsupported is set when no monitor accepted a brightness change;
// the main thread reports it once.
var brightnessUnsupported atomic.Bool
var brightnessWarned bool

func warnBrightnessOnce() {
	if brightnessWarned || !brightnessUnsupported.Load() {
		return
	}
	brightnessWarned = true
	balloon("Anticater Knob", L("Không màn hình nào nhận lệnh độ sáng. Bật DDC/CI trong menu của màn hình.",
		"No monitor accepted the brightness change. Turn on DDC/CI in the monitor's own menu."))
}
