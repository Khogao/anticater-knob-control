package main

// A brightness flyout in the style of the one Windows 11 shows for its own
// brightness keys. Windows only shows its flyout when it handles a brightness
// key itself; this app changes the brightness directly (brightness_windows.go),
// so it draws its own: a rounded panel with a sun icon and a level bar,
// centred near the bottom of the screen that was changed, hidden after a
// moment. The window never takes focus and lets clicks through.
//
// Everything is drawn in Go into a 32-bit premultiplied bitmap and handed to
// UpdateLayeredWindow, which gives smooth rounded corners without GDI+.

import (
	"math"
	"syscall"
	"unsafe"
)

const (
	osdTimer  = 3
	osdHideMs = 1500

	// Size in pixels at 100 % scaling; multiplied by the screen's DPI scale.
	osdWidth  = 196
	osdHeight = 48
	osdRadius = 8
	osdBottom = 24 // gap above the bottom of the work area (above the taskbar)

	wsExLayered     = 0x00080000
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	wsExTransparent = 0x00000020
	wsPopup         = 0x80000000
	swShowNoActive  = 4
	swHide          = 0
	ulwAlpha        = 2
	acSrcAlpha      = 1
)

var (
	gdi32                    = syscall.NewLazyDLL("gdi32.dll")
	shcore                   = syscall.NewLazyDLL("shcore.dll")
	dwmapi                   = syscall.NewLazyDLL("dwmapi.dll")
	pCreateCompatibleDC      = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection        = gdi32.NewProc("CreateDIBSection")
	pSelectObject            = gdi32.NewProc("SelectObject")
	pDeleteObject            = gdi32.NewProc("DeleteObject")
	pDeleteDC                = gdi32.NewProc("DeleteDC")
	pGetDC                   = user32.NewProc("GetDC")
	pReleaseDC               = user32.NewProc("ReleaseDC")
	pUpdateLayeredWindow     = user32.NewProc("UpdateLayeredWindow")
	pGetDpiForMonitor        = shcore.NewProc("GetDpiForMonitor")
	pDwmGetColorizationColor = dwmapi.NewProc("DwmGetColorizationColor")

	osdHwnd uintptr
)

type bitmapInfoHeader struct {
	Size          uint32
	Width, Height int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPels, YPels  int32
	ClrUsed       uint32
	ClrImportant  uint32
}

type blendFunction struct{ Op, Flags, Alpha, Format byte }

type rgb struct{ r, g, b float64 }

// osdShow is called on the main thread with the new level (0..100) of the
// screen hMonitor.
func osdShow(hMonitor uintptr, level int) {
	if osdHwnd == 0 {
		cls := u16("AnticaterKnobOSD")
		inst, _, _ := pGetModuleHandleW.Call(0)
		wc := wndClassEx{WndProc: syscall.NewCallback(osdProc), Instance: inst, ClassName: cls}
		wc.Size = uint32(unsafe.Sizeof(wc))
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
		osdHwnd, _, _ = pCreateWindowExW.Call(wsExLayered|wsExTopmost|wsExToolWindow|wsExNoActivate|wsExTransparent,
			uintptr(unsafe.Pointer(cls)), 0, wsPopup, 0, 0, 0, 0, 0, 0, inst, 0)
		if osdHwnd == 0 {
			return
		}
	}
	mi := monitorInfoEx{Size: uint32(unsafe.Sizeof(monitorInfoEx{}))}
	if ok, _, _ := pGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi))); ok == 0 {
		return
	}
	scale := 1.0
	var dpiX, dpiY uint32
	if r, _, _ := pGetDpiForMonitor.Call(hMonitor, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY))); r == 0 && dpiX > 0 {
		scale = float64(dpiX) / 96
	}
	w, h := int(math.Round(osdWidth*scale)), int(math.Round(osdHeight*scale))
	x := (mi.Work[0]+mi.Work[2])/2 - int32(w/2)
	y := mi.Work[3] - int32(math.Round(osdBottom*scale)) - int32(h)

	hdcScreen, _, _ := pGetDC.Call(0)
	defer pReleaseDC.Call(0, hdcScreen)
	mem, _, _ := pCreateCompatibleDC.Call(hdcScreen)
	defer pDeleteDC.Call(mem)
	bi := bitmapInfoHeader{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi))
	var bits unsafe.Pointer
	dib, _, _ := pCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 {
		return
	}
	defer pDeleteObject.Call(dib)
	old, _, _ := pSelectObject.Call(mem, dib)
	defer pSelectObject.Call(mem, old)

	pixels := unsafe.Slice((*byte)(bits), w*h*4)
	copy(pixels, osdRender(w, h, scale, level, systemDarkMode(), accentColor()))

	size := [2]int32{int32(w), int32(h)}
	pos := [2]int32{x, y}
	src := [2]int32{0, 0}
	blend := blendFunction{Alpha: 255, Format: acSrcAlpha}
	pUpdateLayeredWindow.Call(osdHwnd, hdcScreen, uintptr(unsafe.Pointer(&pos)), uintptr(unsafe.Pointer(&size)),
		mem, uintptr(unsafe.Pointer(&src)), 0, uintptr(*(*uint32)(unsafe.Pointer(&blend))), ulwAlpha)
	pShowWindow.Call(osdHwnd, swShowNoActive)
	pSetTimer.Call(osdHwnd, osdTimer, osdHideMs, 0)
}

func osdProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	if message == wmTimer && wParam == osdTimer {
		pKillTimer.Call(hwnd, osdTimer)
		pShowWindow.Call(hwnd, swHide)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

// systemDarkMode follows Settings > Personalization > Colors > "Choose your mode"
// for Windows itself (taskbar and flyouts), not for apps.
func systemDarkMode() bool {
	var v, n uint32 = 1, 4
	pRegGetValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`))),
		uintptr(unsafe.Pointer(u16("SystemUsesLightTheme"))), 0x10, 0, uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n)))
	return v == 0
}

func accentColor() rgb {
	var c uint32
	var opaque int32
	if r, _, _ := pDwmGetColorizationColor.Call(uintptr(unsafe.Pointer(&c)), uintptr(unsafe.Pointer(&opaque))); r == 0 {
		return rgb{float64(c>>16&0xFF) / 255, float64(c>>8&0xFF) / 255, float64(c&0xFF) / 255}
	}
	return rgb{0, 0.47, 0.83}
}

// osdRender draws the flyout into premultiplied BGRA pixels, top row first.
func osdRender(w, h int, scale float64, level int, dark bool, accent rgb) []byte {
	bg, border, fg, track := rgb{0.976, 0.976, 0.976}, rgb{0.85, 0.85, 0.85}, rgb{0.1, 0.1, 0.1}, rgb{0.55, 0.55, 0.55}
	if dark {
		bg, border, fg, track = rgb{0.17, 0.17, 0.17}, rgb{0.3, 0.3, 0.3}, rgb{1, 1, 1}, rgb{0.62, 0.62, 0.62}
	}
	px := make([]byte, w*h*4)
	const ss = 4 // samples per pixel side, for smooth edges
	fw, fh := float64(w), float64(h)
	rad := osdRadius * scale
	// Layout, in pixels at this scale.
	iconCX, iconCY := 26*scale, fh/2
	barX0, barX1, barY := 52*scale, fw-24*scale, fh/2
	barH := 4 * scale
	fill := barX0 + (barX1-barX0)*float64(level)/100
	thumbR := 9 * scale

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					fx := float64(x) + (float64(sx)+0.5)/ss
					fy := float64(y) + (float64(sy)+0.5)/ss
					c, ca := samplePanel(fx, fy, fw, fh, rad, scale, bg, border)
					if ca == 0 {
						continue
					}
					// Sun icon: a disc and eight short rays.
					if d := math.Hypot(fx-iconCX, fy-iconCY); d <= 4.2*scale {
						c = fg
					} else if d >= 6.3*scale && d <= 9*scale {
						ang := math.Atan2(fy-iconCY, fx-iconCX)
						k := math.Mod(ang+2*math.Pi, math.Pi/4)
						if math.Min(k, math.Pi/4-k)*d <= 0.9*scale {
							c = fg
						}
					}
					// Bar: accent up to the level, grey after it, then the thumb.
					if math.Abs(fy-barY) <= barH/2 && fx >= barX0-barH/2 && fx <= barX1+barH/2 {
						if fx <= fill {
							c = accent
						} else {
							c = track
						}
					}
					if d := math.Hypot(fx-fill, fy-barY); d <= thumbR {
						if d <= thumbR*0.55 {
							c = accent
						} else if d <= thumbR-1*scale {
							c = bg
							if dark {
								c = rgb{0.27, 0.27, 0.27}
							}
						} else {
							c = border
						}
					}
					r, g, b, a = r+c.r*ca, g+c.g*ca, b+c.b*ca, a+ca
				}
			}
			n := float64(ss * ss)
			i := (y*w + x) * 4
			px[i+0] = byte(b / n * 255)
			px[i+1] = byte(g / n * 255)
			px[i+2] = byte(r / n * 255)
			px[i+3] = byte(a / n * 255)
		}
	}
	return px
}

// samplePanel returns the panel colour at (x, y) and its coverage (0 or 1):
// a rounded rectangle with a one-pixel border.
func samplePanel(x, y, w, h, rad, scale float64, bg, border rgb) (rgb, float64) {
	dx := math.Max(math.Max(rad-x, x-(w-rad)), 0)
	dy := math.Max(math.Max(rad-y, y-(h-rad)), 0)
	d := math.Hypot(dx, dy)
	if d > rad {
		return rgb{}, 0
	}
	edge := math.Min(math.Min(x, w-x), math.Min(y, h-y))
	if d > rad-scale || (dx == 0 && dy == 0 && edge < scale) {
		return border, 1
	}
	return bg, 1
}
