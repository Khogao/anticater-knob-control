package main

// Screen brightness of the screen under the mouse pointer, as on macOS:
// external monitors over DDC/CI (dxva2.dll), the built-in panel of a laptop
// through the video driver's \\.\LCD device.
//
// The knob's own brightness reports (usages 0x6F / 0x70) are handled by Windows
// itself (README 9.5): they always move the built-in panel and show the
// brightness flyout, whatever this app does. For the shortcut keys
// "brightness_up" / "brightness_down" to be the only thing that moves the
// brightness, the knob's hold + turn must send F13 / Shift + F13 instead
// (set with the vendor's configuration app; see knobKeyGesture).
//
// A DDC/CI call takes about 60 ms per monitor, far too slow for the thread that
// runs the keyboard hook, so the work happens on a goroutine that folds a burst
// of knob ticks into one change.

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
	brightnessStale = 4 * time.Second // re-read a screen's level after this long
)

var (
	dxva2                    = syscall.NewLazyDLL("dxva2.dll")
	pMonitorFromPoint        = user32.NewProc("MonitorFromPoint")
	pGetMonitorInfoW         = user32.NewProc("GetMonitorInfoW")
	pGetDisplayConfigBufSize = user32.NewProc("GetDisplayConfigBufferSizes")
	pQueryDisplayConfig      = user32.NewProc("QueryDisplayConfig")
	pDisplayConfigGetInfo    = user32.NewProc("DisplayConfigGetDeviceInfo")
	pGetNumberOfPhysMonitors = dxva2.NewProc("GetNumberOfPhysicalMonitorsFromHMONITOR")
	pGetPhysicalMonitors     = dxva2.NewProc("GetPhysicalMonitorsFromHMONITOR")
	pDestroyPhysicalMonitors = dxva2.NewProc("DestroyPhysicalMonitors")
	pGetMonitorBrightness    = dxva2.NewProc("GetMonitorBrightness")
	pSetMonitorBrightness    = dxva2.NewProc("SetMonitorBrightness")

	brightnessPending atomic.Int32
	brightnessWake    = make(chan struct{}, 1)
	brightnessOnce    sync.Once

	// Owned by the worker goroutine. Keyed by the display's GDI name
	// (\\.\DISPLAY1...), which stays put while monitor handles may be reissued.
	brightnessLevels = map[string][]monitorLevel{}
	// Main thread only: the level last shown in the flyout, per screen.
	osdLevels  = map[uintptr]int{}
	panelLevel monitorLevel
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

type monitorInfoEx struct {
	Size    uint32
	Monitor [4]int32
	Work    [4]int32
	Flags   uint32
	Device  [32]uint16
}

// adjustBrightness asks for the screen under the mouse pointer to move by delta
// percent. It returns at once.
func adjustBrightness(delta int) {
	brightnessOnce.Do(func() { go brightnessWorker() })
	// A DDC/CI change takes about 65 ms, as long as the gap between two knob
	// ticks, so the worker folds ticks together and the screen moves in uneven
	// steps. The flyout runs ahead of it from the last level seen, one step per
	// tick, so at least the bar moves evenly.
	if h := monitorUnderCursor(); h != 0 {
		if last, ok := osdLevels[h]; ok {
			osdLevels[h] = clamp(last+delta, 0, 100)
			osdShow(h, osdLevels[h])
		}
	}
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
			h := monitorUnderCursor()
			if level, ok := applyBrightness(h, delta); ok {
				pPostMessageW.Call(hwndMain, wmBrightnessShown, uintptr(level), h)
			} else {
				brightnessUnsupported.Store(true)
			}
		}
	}
}

func monitorUnderCursor() uintptr {
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	const monitorDefaultToNearest = 2
	h, _, _ := pMonitorFromPoint.Call(uintptr(*(*uint64)(unsafe.Pointer(&pt))), monitorDefaultToNearest)
	return h
}

func monitorDevice(hMonitor uintptr) string {
	mi := monitorInfoEx{Size: uint32(unsafe.Sizeof(monitorInfoEx{}))}
	if ok, _, _ := pGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(mi.Device[:])
}

// applyBrightness moves one screen by delta percent. It returns the new level
// in percent, and whether the screen took the change (or is already at the end
// of its range).
func applyBrightness(hMonitor uintptr, delta int) (int, bool) {
	device := monitorDevice(hMonitor)
	if device != "" && internalDisplays()[device] {
		return adjustPanel(delta)
	}
	return adjustDDC(hMonitor, device, delta)
}

func adjustDDC(hMonitor uintptr, device string, delta int) (int, bool) {
	var count uint32
	if ok, _, _ := pGetNumberOfPhysMonitors.Call(hMonitor, uintptr(unsafe.Pointer(&count))); ok == 0 || count == 0 {
		return 0, false
	}
	physical := make([]physicalMonitor, count)
	if ok, _, _ := pGetPhysicalMonitors.Call(hMonitor, uintptr(count), uintptr(unsafe.Pointer(&physical[0]))); ok == 0 {
		return 0, false
	}
	defer pDestroyPhysicalMonitors.Call(uintptr(count), uintptr(unsafe.Pointer(&physical[0])))
	levels := brightnessLevels[device]
	for len(levels) < len(physical) {
		levels = append(levels, monitorLevel{})
	}
	levels = levels[:len(physical)]
	brightnessLevels[device] = levels

	changed, percent := false, 0
	for i, p := range physical {
		level := &levels[i]
		if time.Since(level.readAt) > brightnessStale {
			// The level can also be changed with the monitor's own buttons.
			// DDC/CI reads fail now and then (seen about one in three on the
			// test monitor), so try again before giving up, and keep the last
			// good level rather than skipping the tick.
			for try := 0; try < 3; try++ {
				var lo, cur, hi uint32
				ok, _, _ := pGetMonitorBrightness.Call(p.Handle, uintptr(unsafe.Pointer(&lo)), uintptr(unsafe.Pointer(&cur)), uintptr(unsafe.Pointer(&hi)))
				if ok != 0 && hi > lo {
					*level = monitorLevel{min: int(lo), cur: int(cur), max: int(hi), ok: true, readAt: time.Now()}
					break
				}
				time.Sleep(40 * time.Millisecond)
			}
		}
		if !level.ok {
			continue // DDC/CI switched off in the monitor's menu
		}
		want := clamp(level.cur+delta, level.min, level.max)
		if want != level.cur {
			set := false
			for try := 0; try < 2 && !set; try++ {
				ok, _, _ := pSetMonitorBrightness.Call(p.Handle, uintptr(want))
				set = ok != 0
			}
			if !set {
				level.readAt = time.Time{} // read it again next time
				continue
			}
			level.cur, level.readAt = want, time.Now()
		}
		if !changed {
			percent = (level.cur - level.min) * 100 / (level.max - level.min)
		}
		changed = true
	}
	return percent, changed
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---- built-in panel ----------------------------------------------------------

const (
	qdcOnlyActivePaths          = 2
	getSourceName               = 1
	outputTechInternal          = 0x80000000
	outputTechDisplayPortEmbed  = 11
	outputTechUDIEmbedded       = 13
	ioctlVideoQueryDisplayLevel = 0x230498 // IOCTL_VIDEO_QUERY_DISPLAY_BRIGHTNESS
	ioctlVideoSetDisplayLevel   = 0x23049C // IOCTL_VIDEO_SET_DISPLAY_BRIGHTNESS
	displayPolicyBoth           = 3        // set the level for both mains and battery
	displayPolicyDC             = 2        // a query answers this when on battery
)

type displayConfigPath struct {
	SourceAdapter    [8]byte
	SourceID         uint32
	SourceMode       uint32
	SourceStatus     uint32
	TargetAdapter    [8]byte
	TargetID         uint32
	TargetMode       uint32
	OutputTechnology uint32
	Rotation         uint32
	Scaling          uint32
	RefreshNum       uint32
	RefreshDen       uint32
	ScanLineOrdering uint32
	TargetAvailable  uint32
	TargetStatus     uint32
	Flags            uint32
}

type displayConfigSourceName struct {
	Type    uint32
	Size    uint32
	Adapter [8]byte
	ID      uint32
	Device  [32]uint16
}

// internalDisplays returns the GDI names (\\.\DISPLAYn) of the displays that
// show the built-in panel of a laptop.
func internalDisplays() map[string]bool {
	out := map[string]bool{}
	var nPaths, nModes uint32
	if r, _, _ := pGetDisplayConfigBufSize.Call(qdcOnlyActivePaths, uintptr(unsafe.Pointer(&nPaths)), uintptr(unsafe.Pointer(&nModes))); r != 0 || nPaths == 0 {
		return out
	}
	paths := make([]displayConfigPath, nPaths)
	modes := make([]byte, 64*int(nModes)+64) // DISPLAYCONFIG_MODE_INFO is 64 bytes
	if r, _, _ := pQueryDisplayConfig.Call(qdcOnlyActivePaths, uintptr(unsafe.Pointer(&nPaths)), uintptr(unsafe.Pointer(&paths[0])),
		uintptr(unsafe.Pointer(&nModes)), uintptr(unsafe.Pointer(&modes[0])), 0); r != 0 {
		return out
	}
	for _, p := range paths[:nPaths] {
		switch p.OutputTechnology {
		case outputTechInternal, outputTechDisplayPortEmbed, outputTechUDIEmbedded:
		default:
			continue
		}
		name := displayConfigSourceName{Type: getSourceName, Size: uint32(unsafe.Sizeof(displayConfigSourceName{})), Adapter: p.SourceAdapter, ID: p.SourceID}
		if r, _, _ := pDisplayConfigGetInfo.Call(uintptr(unsafe.Pointer(&name))); r == 0 {
			out[syscall.UTF16ToString(name.Device[:])] = true
		}
	}
	return out
}

type displayBrightness struct{ Policy, AC, DC uint8 }

// adjustPanel moves the built-in panel by delta percent through \\.\LCD.
func adjustPanel(delta int) (int, bool) {
	name, _ := syscall.UTF16PtrFromString(`\\.\LCD`)
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0, false // the driver does not offer it
	}
	defer syscall.CloseHandle(h)
	var n uint32
	if time.Since(panelLevel.readAt) > brightnessStale {
		// Windows' own slider and keys change it too. Right after a set the driver
		// still answers a query with the old level, so a fresh cached value wins.
		var b displayBrightness
		if err := syscall.DeviceIoControl(h, ioctlVideoQueryDisplayLevel, nil, 0, (*byte)(unsafe.Pointer(&b)), 3, &n, nil); err != nil {
			return 0, false
		}
		cur := b.AC
		if b.Policy == displayPolicyDC {
			cur = b.DC
		}
		panelLevel = monitorLevel{min: 0, cur: int(cur), max: 100, ok: true, readAt: time.Now()}
	}
	want := clamp(panelLevel.cur+delta, 0, 100)
	if want == panelLevel.cur {
		return want, true
	}
	b := displayBrightness{Policy: displayPolicyBoth, AC: uint8(want), DC: uint8(want)}
	if err := syscall.DeviceIoControl(h, ioctlVideoSetDisplayLevel, (*byte)(unsafe.Pointer(&b)), 3, nil, 0, &n, nil); err != nil {
		panelLevel.readAt = time.Time{}
		return 0, false
	}
	panelLevel.cur, panelLevel.readAt = want, time.Now()
	return want, true
}

// brightnessUnsupported is set when the screen under the pointer did not accept
// a brightness change; the main thread reports it once.
var brightnessUnsupported atomic.Bool
var brightnessWarned bool

func warnBrightnessOnce() {
	if brightnessWarned || !brightnessUnsupported.Load() {
		return
	}
	brightnessWarned = true
	balloon("Anticater Knob", L("Màn hình đang có con trỏ chuột không nhận lệnh độ sáng. Với màn hình ngoài, bật DDC/CI trong menu của màn hình.",
		"The screen under the pointer did not accept the brightness change. For an external monitor, turn on DDC/CI in the monitor's own menu."))
}
