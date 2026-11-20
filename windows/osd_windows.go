package main

// A brightness flyout drawn like the volume / brightness flyout of Windows 11.
// Windows only shows its own flyout when it handles a brightness key itself;
// this app changes the brightness directly (brightness_windows.go), so it draws
// its own, centred near the bottom of the screen that was changed and hidden
// after a moment. The window never takes focus and lets clicks through.
//
// Sizes and colours were measured on a screenshot of the Windows 11 volume
// flyout (Windows 11 26100, dark mode, 150 % scaling, 21/11/2026): a 192 x 47.3
// panel 14.7 above the work area, #2D2D2D, an icon centred 18 from the left,
// a 4-thick bar from 42 to 152 filled with the accent colour's "light 2" shade
// over #9F9F9F, and the level as a number centred at 171. Light-mode colours are
// not measured; they follow the accent's "dark 1" shade and light greys.
//
// The panel is drawn in Go into a 32-bit premultiplied bitmap, the number with
// GDI on top, and the result is handed to UpdateLayeredWindow, which gives
// smooth rounded corners without GDI+.

import (
	"math"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	osdTimer  = 3
	osdHideMs = 1500

	// Measured sizes, in pixels at 100 % scaling.
	osdWidth   = 192.0
	osdHeight  = 47.33
	osdRadius  = 7.0
	osdBottom  = 14.67 // gap above the bottom of the work area
	osdIconX   = 18.0  // centre of the icon
	osdBarX0   = 42.0
	osdBarX1   = 152.0
	osdBarH    = 4.0
	osdNumberX = 171.0 // centre of the number
	osdFontPx  = 14.0

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

	dpiAwarenessPerMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = -4
)

var (
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	shcore               = syscall.NewLazyDLL("shcore.dll")
	pCreateCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	pSelectObject        = gdi32.NewProc("SelectObject")
	pDeleteObject        = gdi32.NewProc("DeleteObject")
	pDeleteDC            = gdi32.NewProc("DeleteDC")
	pCreateFontW         = gdi32.NewProc("CreateFontW")
	pSetTextColor        = gdi32.NewProc("SetTextColor")
	pSetBkMode           = gdi32.NewProc("SetBkMode")
	pGdiFlush            = gdi32.NewProc("GdiFlush")
	pDrawTextW           = user32.NewProc("DrawTextW")
	pGetDC               = user32.NewProc("GetDC")
	pReleaseDC           = user32.NewProc("ReleaseDC")
	pUpdateLayeredWindow = user32.NewProc("UpdateLayeredWindow")
	pGetDpiForMonitor    = shcore.NewProc("GetDpiForMonitor")

	pSetThreadDpiAwarenessContext = user32.NewProc("SetThreadDpiAwarenessContext")

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

type osdTheme struct{ bg, edge, fg, track, fill rgb }

func currentOSDTheme() osdTheme {
	if systemDarkMode() {
		return osdTheme{bg: hex(0x2D2D2D), edge: hex(0x121212), fg: hex(0xFFFFFF), track: hex(0x9F9F9F), fill: accentShade(1, 0x4CC2FF)}
	}
	return osdTheme{bg: hex(0xF9F9F9), edge: hex(0xD5D5D5), fg: hex(0x1B1B1B), track: hex(0x868686), fill: accentShade(4, 0x0067C0)}
}

func hex(c uint32) rgb {
	return rgb{float64(c>>16&0xFF) / 255, float64(c>>8&0xFF) / 255, float64(c&0xFF) / 255}
}

// accentShade reads one shade of the accent colour from the palette Windows
// keeps in the registry (0 = light 3 ... 3 = base ... 6 = dark 3).
func accentShade(index int, fallback uint32) rgb {
	var buf [32]byte
	n := uint32(len(buf))
	r, _, _ := pRegGetValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(`Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`))),
		uintptr(unsafe.Pointer(u16("AccentPalette"))), 0x08, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))) // RRF_RT_REG_BINARY
	if r != 0 || int(n) < (index+1)*4 {
		return hex(fallback)
	}
	return rgb{float64(buf[index*4]) / 255, float64(buf[index*4+1]) / 255, float64(buf[index*4+2]) / 255}
}

// systemDarkMode follows Settings > Personalization > Colors > "Choose your mode"
// for Windows itself (taskbar and flyouts), not for apps.
func systemDarkMode() bool {
	var v, n uint32 = 1, 4
	pRegGetValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`))),
		uintptr(unsafe.Pointer(u16("SystemUsesLightTheme"))), 0x10, 0, uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n)))
	return v == 0
}

// osdShow is called on the main thread with the new level (0..100) of the
// screen hMonitor.
func osdShow(hMonitor uintptr, level int) {
	if osdHwnd == 0 {
		if old, _, _ := pSetThreadDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2); old != 0 {
			defer pSetThreadDpiAwarenessContext.Call(old) // the window keeps the awareness it was created with
		}
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
	// The rest of the app is not DPI aware, so Windows would stretch this window
	// and report scaled coordinates. Work in real pixels here.
	if old, _, _ := pSetThreadDpiAwarenessContext.Call(dpiAwarenessPerMonitorV2); old != 0 {
		defer pSetThreadDpiAwarenessContext.Call(old)
		pGetMonitorInfoW.Call(hMonitor, uintptr(unsafe.Pointer(&mi)))
	}
	scale := 1.0
	var dpiX, dpiY uint32
	if r, _, _ := pGetDpiForMonitor.Call(hMonitor, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY))); r == 0 && dpiX > 0 {
		scale = float64(dpiX) / 96
	}
	w, h := int(math.Round(osdWidth*scale)), int(math.Round(osdHeight*scale))
	x := (mi.Work[0]+mi.Work[2])/2 - int32(w/2)
	y := mi.Work[3] - int32(math.Round(osdBottom*scale)) - int32(h)
	theme := currentOSDTheme()

	hdcScreen, _, _ := pGetDC.Call(0)
	defer pReleaseDC.Call(0, hdcScreen)
	mem, pixels, done := osdBitmap(hdcScreen, w, h)
	if mem == 0 {
		return
	}
	defer done()
	osdPaint(mem, pixels, w, h, scale, level, theme)

	size := [2]int32{int32(w), int32(h)}
	pos := [2]int32{x, y}
	src := [2]int32{0, 0}
	blend := blendFunction{Alpha: 255, Format: acSrcAlpha}
	pUpdateLayeredWindow.Call(osdHwnd, hdcScreen, uintptr(unsafe.Pointer(&pos)), uintptr(unsafe.Pointer(&size)),
		mem, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
	pShowWindow.Call(osdHwnd, swShowNoActive)
	pSetTimer.Call(osdHwnd, osdTimer, osdHideMs, 0)
}

// osdBitmap makes a memory DC holding a w x h 32-bit top-down bitmap and
// returns its pixels and a function that frees both.
func osdBitmap(hdc uintptr, w, h int) (uintptr, []byte, func()) {
	mem, _, _ := pCreateCompatibleDC.Call(hdc)
	bi := bitmapInfoHeader{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	bi.Size = uint32(unsafe.Sizeof(bi))
	var bits unsafe.Pointer
	dib, _, _ := pCreateDIBSection.Call(mem, uintptr(unsafe.Pointer(&bi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 {
		pDeleteDC.Call(mem)
		return 0, nil, nil
	}
	old, _, _ := pSelectObject.Call(mem, dib)
	return mem, unsafe.Slice((*byte)(bits), w*h*4), func() {
		pSelectObject.Call(mem, old)
		pDeleteObject.Call(dib)
		pDeleteDC.Call(mem)
	}
}

func osdPaint(mem uintptr, pixels []byte, w, h int, scale float64, level int, theme osdTheme) {
	copy(pixels, osdRender(w, h, scale, level, theme))
	osdDrawNumber(mem, pixels, w, h, scale, level, theme)
}

// osdDrawNumber writes the level with GDI. GDI clears the alpha byte of every
// pixel it touches; the number sits on the opaque panel, so alpha goes back to
// 255 over its box afterwards.
func osdDrawNumber(dc uintptr, pixels []byte, w, h int, scale float64, level int, t osdTheme) {
	const fwNormal, antialiased, transparent, dtCenterVcenterSingle = 400, 4, 1, 0x1 | 0x4 | 0x20
	font, _, _ := pCreateFontW.Call(uintptr(-int32(math.Round(osdFontPx*scale))), 0, 0, 0, fwNormal, 0, 0, 0, 1, 0, 0, antialiased, 0,
		uintptr(unsafe.Pointer(u16("Segoe UI Variable Text"))))
	if font == 0 {
		return
	}
	defer pDeleteObject.Call(font)
	oldFont, _, _ := pSelectObject.Call(dc, font)
	defer pSelectObject.Call(dc, oldFont)
	pSetBkMode.Call(dc, transparent)
	pSetTextColor.Call(dc, uintptr(byte(t.fg.r*255))|uintptr(byte(t.fg.g*255))<<8|uintptr(byte(t.fg.b*255))<<16)
	half := 14 * scale
	rect := [4]int32{int32(osdNumberX*scale - half), 0, int32(osdNumberX*scale + half), int32(h)}
	text := u16(strconv.Itoa(level))
	pDrawTextW.Call(dc, uintptr(unsafe.Pointer(text)), ^uintptr(0), uintptr(unsafe.Pointer(&rect)), dtCenterVcenterSingle)
	pGdiFlush.Call()
	for y := int(rect[1]) + 2; y < int(rect[3])-2; y++ {
		for x := int(rect[0]); x < int(rect[2]) && x < w; x++ {
			pixels[(y*w+x)*4+3] = 255
		}
	}
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

// osdRender draws the panel, icon and bar into premultiplied BGRA pixels, top
// row first. The number is added by osdDrawNumber.
func osdRender(w, h int, scale float64, level int, t osdTheme) []byte {
	px := make([]byte, w*h*4)
	const ss = 4 // samples per pixel side, for smooth edges
	fw, fh := float64(w), float64(h)
	rad := osdRadius * scale
	iconCX, iconCY := osdIconX*scale, fh/2
	barX0, barX1, barY, barR := osdBarX0*scale, osdBarX1*scale, fh/2, osdBarH*scale/2
	fill := barX0 + (barX1-barX0)*float64(level)/100
	stroke := 1.0 * scale // icon line width

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					fx := float64(x) + (float64(sx)+0.5)/ss
					fy := float64(y) + (float64(sy)+0.5)/ss
					c, ok := samplePanel(fx, fy, fw, fh, rad, scale, t)
					if !ok {
						continue
					}
					// Sun, outline style: a ring and eight short rays.
					d := math.Hypot(fx-iconCX, fy-iconCY)
					if math.Abs(d-3.2*scale) <= stroke/2 {
						c = t.fg
					} else if d >= 5.2*scale && d <= 7.4*scale {
						ang := math.Atan2(fy-iconCY, fx-iconCX)
						k := math.Mod(ang+2*math.Pi, math.Pi/4)
						if math.Min(k, math.Pi/4-k)*d <= stroke/2 {
							c = t.fg
						}
					}
					// Bar with round ends: fill up to the level, track after it.
					if fy >= barY-barR && fy <= barY+barR {
						cx := math.Max(barX0, math.Min(barX1, fx))
						if math.Hypot(fx-cx, fy-barY) <= barR {
							if fx <= math.Max(fill, barX0) {
								c = t.fill
							} else {
								c = t.track
							}
						}
					}
					r, g, b, a = r+c.r, g+c.g, b+c.b, a+1
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

// samplePanel returns the panel colour at (x, y), or false outside it: a
// rounded rectangle with a one-pixel darker edge.
func samplePanel(x, y, w, h, rad, scale float64, t osdTheme) (rgb, bool) {
	dx := math.Max(math.Max(rad-x, x-(w-rad)), 0)
	dy := math.Max(math.Max(rad-y, y-(h-rad)), 0)
	d := math.Hypot(dx, dy)
	if d > rad {
		return rgb{}, false
	}
	if d > rad-1 || math.Min(math.Min(x, w-x), math.Min(y, h-y)) < 1 {
		return t.edge, true
	}
	return t.bg, true
}
