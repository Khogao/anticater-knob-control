package main

// Thin Win32 bindings using only the standard library (no cgo, no modules).

import (
	"strings"
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pGetMessageTime   = user32.NewProc("GetMessageTime")
	pPostMessageW     = user32.NewProc("PostMessageW")
	pGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	pLoadImageW       = user32.NewProc("LoadImageW")
	pGetSystemMetrics = user32.NewProc("GetSystemMetrics")
	pShowWindow       = user32.NewProc("ShowWindow")
	pIsIconic         = user32.NewProc("IsIconic")
	pFindWindowW      = user32.NewProc("FindWindowW")
	pEnumWindows      = user32.NewProc("EnumWindows")
	pIsWindowVisible  = user32.NewProc("IsWindowVisible")

	pRegisterClassExW          = user32.NewProc("RegisterClassExW")
	pCreateWindowExW           = user32.NewProc("CreateWindowExW")
	pDefWindowProcW            = user32.NewProc("DefWindowProcW")
	pGetMessageW               = user32.NewProc("GetMessageW")
	pTranslateMessage          = user32.NewProc("TranslateMessage")
	pDispatchMessageW          = user32.NewProc("DispatchMessageW")
	pPostQuitMessage           = user32.NewProc("PostQuitMessage")
	pRegisterRawInputDevices   = user32.NewProc("RegisterRawInputDevices")
	pGetRawInputData           = user32.NewProc("GetRawInputData")
	pGetRawInputDeviceInfoW    = user32.NewProc("GetRawInputDeviceInfoW")
	pSetWindowsHookExW         = user32.NewProc("SetWindowsHookExW")
	pCallNextHookEx            = user32.NewProc("CallNextHookEx")
	pUnhookWindowsHookEx       = user32.NewProc("UnhookWindowsHookEx")
	pSendInput                 = user32.NewProc("SendInput")
	pGetForegroundWindow       = user32.NewProc("GetForegroundWindow")
	pGetWindowThreadProcessId  = user32.NewProc("GetWindowThreadProcessId")
	pGetWindowTextW            = user32.NewProc("GetWindowTextW")
	pSetTimer                  = user32.NewProc("SetTimer")
	pKillTimer                 = user32.NewProc("KillTimer")
	pLoadIconW                 = user32.NewProc("LoadIconW")
	pCreatePopupMenu           = user32.NewProc("CreatePopupMenu")
	pAppendMenuW               = user32.NewProc("AppendMenuW")
	pTrackPopupMenu            = user32.NewProc("TrackPopupMenu")
	pDestroyMenu               = user32.NewProc("DestroyMenu")
	pGetCursorPos              = user32.NewProc("GetCursorPos")
	pSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	pRegisterWindowMessageW    = user32.NewProc("RegisterWindowMessageW")
	pMessageBoxW               = user32.NewProc("MessageBoxW")
	pShellNotifyIconW          = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW             = shell32.NewProc("ShellExecuteW")
	pGetModuleHandleW          = kernel32.NewProc("GetModuleHandleW")
	pOpenProcess               = kernel32.NewProc("OpenProcess")
	pCloseHandle               = kernel32.NewProc("CloseHandle")
	pQueryFullProcessImageName = kernel32.NewProc("QueryFullProcessImageNameW")
	pCreateMutexW              = kernel32.NewProc("CreateMutexW")
	pGetTickCount64            = kernel32.NewProc("GetTickCount64")
	pRegSetKeyValueW           = advapi32.NewProc("RegSetKeyValueW")
	pRegDeleteKeyValueW        = advapi32.NewProc("RegDeleteKeyValueW")
	pRegGetValueW              = advapi32.NewProc("RegGetValueW")
)

const (
	wmDestroy         = 0x0002
	wmTimer           = 0x0113
	wmInput           = 0x00FF
	wmApp             = 0x8000
	wmTray            = wmApp + 1
	wmUICall          = wmApp + 2 // the settings page has work queued for this thread
	wmShowUI          = wmApp + 3 // a second launch asks the running app to show settings
	wmKnobKey         = wmApp + 4 // the hook saw the knob's F13; wParam indexes gestureNames
	wmBrightnessShown = wmApp + 5 // a screen took a brightness change; wParam = level, lParam = HMONITOR
	vkF13             = 0x7C
	wmLButtonUp       = 0x0202
	wmRButtonUp       = 0x0205
	wmKeyDown         = 0x0100
	wmSysKeyDown      = 0x0104
	whKeyboardLL      = 13
	ridInput          = 0x10000003
	ridiDeviceName    = 0x20000007
	ridevInputSink    = 0x00000100
	rimTypeHID        = 2

	inputMouse    = 0
	inputKeyboard = 1
	keyUp         = 0x0002
	keyExtended   = 0x0001
	mouseWheel    = 0x0800
	mouseHWheel   = 0x1000

	nimAdd     = 0
	nimModify  = 1
	nimDelete  = 2
	nifMessage = 1
	nifIcon    = 2
	nifTip     = 4
	nifInfo    = 0x10

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfSeparator = 0x0800

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	hkeyCurrentUser = 0x80000001
	regSZ           = 1
	rrfRtRegSZ      = 0x00000002

	processQueryLimited = 0x1000
	errorAlreadyExists  = 183
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type point struct{ X, Y int32 }

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

type rawInputDevice struct {
	UsagePage uint16
	Usage     uint16
	Flags     uint32
	Target    uintptr
}

type rawInputHeader struct {
	Type   uint32
	Size   uint32
	Device uintptr
	WParam uintptr
}

type kbdLLHook struct {
	VkCode    uint32
	ScanCode  uint32
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// ownInputMark is stored in dwExtraInfo of every key this app sends, so the
// keyboard hook can tell its own keys from anyone else's.
const ownInputMark uintptr = 0x414B4E42 // "AKNB"

// INPUT is 40 bytes on amd64; both variants are laid out explicitly.
type keyInput struct {
	Type  uint32
	_     uint32
	Vk    uint16
	Scan  uint16
	Flags uint32
	Time  uint32
	_     uint32
	Extra uintptr
	_     [8]byte
}

type mouseInput struct {
	Type      uint32
	_         uint32
	Dx        int32
	Dy        int32
	MouseData uint32
	Flags     uint32
	Time      uint32
	_         uint32
	Extra     uintptr
}

type notifyIconData struct {
	Size            uint32
	Hwnd            uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            [16]byte
	BalloonIcon     uintptr
}

func u16(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func copyU16(dst []uint16, s string) {
	src, _ := syscall.UTF16FromString(s)
	if len(src) > len(dst) {
		src = src[:len(dst)]
		src[len(src)-1] = 0
	}
	copy(dst, src)
}

func tickMs() uint64 {
	r, _, _ := pGetTickCount64.Call()
	return uint64(r)
}

func messageBox(title, text string) {
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), 0x40)
}

// foregroundApp returns the exe file name and window title of the active window.
func foregroundApp() (exe, title string) {
	hwnd, _, _ := pGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", ""
	}
	return windowApp(hwnd)
}

// windowApp returns the exe file name and title of a top-level window.
func windowApp(hwnd uintptr) (exe, title string) {
	tbuf := make([]uint16, 512)
	n, _, _ := pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&tbuf[0])), uintptr(len(tbuf)))
	title = syscall.UTF16ToString(tbuf[:n])

	var pid uint32
	pGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return "", title
	}
	h, _, _ := pOpenProcess.Call(processQueryLimited, 0, uintptr(pid))
	if h == 0 {
		return "", title
	}
	defer pCloseHandle.Call(h)
	pbuf := make([]uint16, 1024)
	size := uint32(len(pbuf))
	ok, _, _ := pQueryFullProcessImageName.Call(h, 0, uintptr(unsafe.Pointer(&pbuf[0])), uintptr(unsafe.Pointer(&size)))
	if ok == 0 {
		return "", title
	}
	full := syscall.UTF16ToString(pbuf[:size])
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '\\' || full[i] == '/' {
			return full[i+1:], title
		}
	}
	return full, title
}

func rawDeviceName(device uintptr) string {
	var size uint32
	pGetRawInputDeviceInfoW.Call(device, ridiDeviceName, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 || size > 4096 {
		return ""
	}
	buf := make([]uint16, size+1)
	pGetRawInputDeviceInfoW.Call(device, ridiDeviceName, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	return syscall.UTF16ToString(buf)
}

func sendKey(vk uint16, up bool) {
	in := keyInput{Type: inputKeyboard, Vk: vk, Extra: ownInputMark}
	if extendedKeys[vk] {
		in.Flags |= keyExtended
	}
	if up {
		in.Flags |= keyUp
	}
	pSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func sendWheel(horizontal bool, notches int) {
	in := mouseInput{Type: inputMouse, MouseData: uint32(int32(notches * 120)), Flags: mouseWheel}
	if horizontal {
		in.Flags = mouseHWheel
	}
	pSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func sendMouseButton(downFlag, upFlag uint32) {
	for _, f := range []uint32{downFlag, upFlag} {
		in := mouseInput{Type: inputMouse, Flags: f}
		pSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	}
}

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runValue = "AnticaterKnob"

func autostartEnabled() bool {
	var size uint32
	r, _, _ := pRegGetValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))),
		uintptr(unsafe.Pointer(u16(runValue))), rrfRtRegSZ, 0, 0, uintptr(unsafe.Pointer(&size)))
	return r == 0
}

func setAutostart(on bool, exePath string) {
	if !on {
		pRegDeleteKeyValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))), uintptr(unsafe.Pointer(u16(runValue))))
		return
	}
	val, _ := syscall.UTF16FromString(`"` + exePath + `"`)
	pRegSetKeyValueW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))), uintptr(unsafe.Pointer(u16(runValue))),
		regSZ, uintptr(unsafe.Pointer(&val[0])), uintptr(len(val)*2))
}

// appIcon returns the icon for the tray (small) or for windows (large). It is
// read from AnticaterKnob.ico next to the exe when that file exists; the icon is
// not compiled into the exe. Without the file the stock application icon is used.
func appIcon(small bool) uintptr {
	metric := uintptr(11) // SM_CXICON
	if small {
		metric = 49 // SM_CXSMICON
	}
	size, _, _ := pGetSystemMetrics.Call(metric)
	if size == 0 {
		size = 32
	}
	// IMAGE_ICON = 1, LR_LOADFROMFILE = 0x10
	if h, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(u16(iconPath()))), 1, size, size, 0x10); h != 0 {
		return h
	}
	h, _, _ := pLoadIconW.Call(0, idiApplication)
	return h
}

func iconPath() string {
	return strings.TrimSuffix(configPath(), "knob-config.json") + "AnticaterKnob.ico"
}

func shellOpen(path string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(path))), 0, 0, 1)
}
