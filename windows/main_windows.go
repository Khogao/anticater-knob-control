// Anticater VK-01 knob remapper for Windows.
//
// Reads the knob's consumer-control HID reports through Raw Input (so it knows
// the events come from the knob, not another keyboard), looks up the foreground
// app, and runs the action configured for that app in knob-config.json. A
// low-level keyboard hook swallows the knob's native volume/mute keys whenever
// the gesture is remapped.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"anticater-knob/ui"
)

const (
	timerHousekeeping = 1
	timerPending      = 2

	knobWindowMs   = 60 // a volume key stamped this close to a knob report is the knob's
	pendingHoldMs  = 50 // how long an unattributed volume key waits for a knob report
	everyResetMs   = 500
	fgCacheMs      = 150
	maxLogBytes    = 1 << 20
	cmdPause       = 100
	cmdOpenConfig  = 101
	cmdReload      = 102
	cmdAddApp      = 103
	cmdOpenLog     = 104
	cmdAutostart   = 105
	cmdExit        = 106
	cmdSettings    = 107
	cmdDiagnostics = 108
	cmdLanguage    = 109
	trayIconID     = 1

	mainClassName = "AnticaterKnobWindow"
	idiApplication = 32512
)

type pendingKey struct {
	vk     uint16
	up     bool
	time   uint32 // the key event's own timestamp
	heldAt uint64
}

var (
	cfg        *Config
	cfgModTime time.Time
	hwndMain   uintptr
	hookHandle uintptr
	paused     bool
	nid        notifyIconData
	msgTaskbar uint32

	// Timestamps (GetTickCount base) of recent knob reports. Keys are matched
	// against these, not against arrival order: the macOS port showed that
	// arrival times drift far apart when the thread is busy running actions.
	knobTimes  []uint32
	lastKnobMs uint64
	pending    []pendingKey

	rawCount, recognizedCount, swallowedCount, resentCount, hookCalls int
	recentRaw                                              []string
	devicesSeen                                            []string
	deviceKnown                                            = map[uintptr]bool{}

	everyGesture string
	everyCount   int
	everyAt      uint64

	fgExe, fgTitle string
	fgAt           uint64
	lastAppExe     string
	lastEventText  = "no knob input yet"

	logFile *os.File
)

func logf(format string, args ...any) {
	if cfg == nil || !cfg.Log {
		return
	}
	if logFile == nil {
		if st, err := os.Stat(logPath()); err == nil && st.Size() > maxLogBytes {
			os.Remove(logPath())
		}
		f, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		logFile = f
	}
	fmt.Fprintf(logFile, "%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}

func currentApp() (string, string) {
	now := tickMs()
	if now-fgAt > fgCacheMs {
		fgExe, fgTitle = foregroundApp()
		fgAt = now
	}
	return fgExe, fgTitle
}

func gestureForUsage(usage uint16) string {
	switch usage {
	case 0xE9:
		return "rotate_right"
	case 0xEA:
		return "rotate_left"
	case 0xE2, 0xCD: // Mute on some firmware, Play/Pause on others
		return "press"
	case 0x6F:
		return "press_rotate_right"
	case 0x70:
		return "press_rotate_left"
	}
	return ""
}

func gestureForVK(vk uint32) string {
	switch vk {
	case vkVolumeUp:
		return "rotate_right"
	case vkVolumeDown:
		return "rotate_left"
	case vkVolumeMute, vkMediaPlayPause:
		return "press"
	}
	return ""
}

// ---- knob input (Raw Input) -------------------------------------------------

func noteKnobReport(t uint32) {
	knobTimes = append(knobTimes, t)
	if len(knobTimes) > 64 {
		knobTimes = knobTimes[len(knobTimes)-64:]
	}
}

// knobReportedNear reports whether a knob report carries a timestamp within
// knobWindowMs of t. Tick counts wrap, so compare as signed differences.
func knobReportedNear(t uint32) bool {
	for _, k := range knobTimes {
		d := int32(k - t)
		if d < 0 {
			d = -d
		}
		if d <= knobWindowMs {
			return true
		}
	}
	return false
}

func onRawInput(lParam uintptr, msgTime uint32) {
	var size uint32
	hdr := uint32(unsafe.Sizeof(rawInputHeader{}))
	pGetRawInputData.Call(lParam, ridInput, 0, uintptr(unsafe.Pointer(&size)), uintptr(hdr))
	if size < hdr+8 || size > 4096 {
		return
	}
	buf := make([]byte, size)
	n, _, _ := pGetRawInputData.Call(lParam, ridInput, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), uintptr(hdr))
	if uint32(n) != size {
		return
	}
	h := (*rawInputHeader)(unsafe.Pointer(&buf[0]))
	if h.Type != rimTypeHID {
		return
	}
	known, seen := deviceKnown[h.Device]
	if !seen {
		name := rawDeviceName(h.Device)
		known = cfg.matchesDevice(name)
		deviceKnown[h.Device] = known
		logf("device %s  knob=%v", name, known)
		if len(devicesSeen) < 40 {
			devicesSeen = append(devicesSeen, fmt.Sprintf("knob=%v  %s", known, name))
		}
	}
	if !known {
		return
	}
	sizeHid := *(*uint32)(unsafe.Pointer(&buf[hdr]))
	count := *(*uint32)(unsafe.Pointer(&buf[hdr+4]))
	data := buf[hdr+8:]
	for i := uint32(0); i < count; i++ {
		if sizeHid == 0 || uint32(len(data)) < (i+1)*sizeHid {
			break
		}
		onKnobReport(data[i*sizeHid:(i+1)*sizeHid], msgTime)
	}
}

func onKnobReport(report []byte, msgTime uint32) {
	lastKnobMs = tickMs()
	noteKnobReport(msgTime)
	rawCount++
	lastRawText = fmt.Sprintf("%s  % x", time.Now().Format("15:04:05.000"), report)
	recentRaw = append(recentRaw, lastRawText)
	if len(recentRaw) > 30 {
		recentRaw = recentRaw[len(recentRaw)-30:]
	}
	var usage uint16
	switch {
	case len(report) >= 3:
		usage = uint16(report[1]) | uint16(report[2])<<8
	case len(report) == 2:
		usage = uint16(report[1])
	}
	if usage == 0 {
		return // release report
	}
	gesture := gestureForUsage(usage)
	if gesture == "" {
		logf("report % x  time=%d  usage 0x%04x  (unmapped)", report, msgTime, usage)
		return
	}
	if paused {
		return
	}
	recognizedCount++
	gestureCounts[gesture]++
	exe, title := currentApp()
	act, source := cfg.resolve(gesture, exe, title)
	lastEventText = fmt.Sprintf("%s in %s -> %s", gesture, orDash(exe), describe(act, source))
	logf("report % x  time=%d  %s  app=%s  -> %s", report, msgTime, gesture, exe, describe(act, source))
	if act.isPassthrough() || act.Type == "noop" {
		return
	}
	if act.Every > 1 {
		if gesture != everyGesture || lastKnobMs-everyAt > everyResetMs {
			everyGesture, everyCount = gesture, 0
		}
		everyAt = lastKnobMs
		everyCount++
		if everyCount%act.Every != 0 {
			return
		}
	}
	perform(act)
}

func describe(a Action, source string) string {
	d := a.Description
	if d == "" {
		d = a.Type
		if d == "" {
			d = "passthrough"
		}
	}
	return d + " [" + source + "]"
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ---- suppressing the knob's native keys (low-level keyboard hook) -----------

func hookProc(code int32, wParam, lParam uintptr) uintptr {
	if code >= 0 && !paused && cfg != nil {
		k := (*kbdLLHook)(unsafe.Pointer(lParam))
		hookCalls++
		// The knob's own volume keys can arrive flagged as injected (seen on a
		// Bluetooth LE knob: Windows synthesizes them from the consumer-control
		// report), so the flag cannot be used to skip them. Only keys this app
		// sent itself, recognized by their mark, are let through untouched.
		if k.ExtraInfo != ownInputMark {
			// Diagnostic: a key outside the typing range that coincides with a
			// knob report is probably one the knob produced and is not handled.
			if vk := k.VkCode; gestureForVK(vk) == "" && (vk < 0x08 || (vk > 0xA5 && vk < 0xBA) || vk > 0xE2) && knobReportedNear(k.Time) {
				logf("hook other vk=0x%02x scan=0x%02x flags=0x%02x time=%d (near a knob report, not handled)", vk, k.ScanCode, k.Flags, k.Time)
			}
			if gesture := gestureForVK(k.VkCode); gesture != "" {
				exe, title := currentApp()
				if act, _ := cfg.resolve(gesture, exe, title); !act.isPassthrough() {
					if knobReportedNear(k.Time) {
						swallowedCount++
						logf("hook vk=0x%02x time=%d -> swallowed (knob report nearby)", k.VkCode, k.Time)
					} else {
						logf("hook vk=0x%02x time=%d -> held (no knob report within %d ms yet)", k.VkCode, k.Time, knobWindowMs)
						// Its knob report may not have arrived yet, or this is
						// a real keyboard's volume key: hold it back briefly;
						// it is re-sent unless a knob report claims it.
						up := wParam != wmKeyDown && wParam != wmSysKeyDown
						pending = append(pending, pendingKey{uint16(k.VkCode), up, k.Time, tickMs()})
						pSetTimer.Call(hwndMain, timerPending, pendingHoldMs, 0)
					}
					return 1
				}
			}
		}
	}
	r, _, _ := pCallNextHookEx.Call(0, uintptr(code), wParam, lParam)
	return r
}

func flushPending() {
	pKillTimer.Call(hwndMain, timerPending)
	now := tickMs()
	keep := pending[:0]
	var resend []pendingKey
	for _, p := range pending {
		switch {
		case now-p.heldAt < pendingHoldMs-5:
			keep = append(keep, p)
		case knobReportedNear(p.time):
			swallowedCount++ // the knob's own key after all
			logf("held vk=0x%02x time=%d -> swallowed (knob report arrived)", p.vk, p.time)
		default:
			resend = append(resend, p)
		}
	}
	pending = keep
	for _, p := range resend {
		resentCount++
		logf("held vk=0x%02x time=%d -> re-sent (not from the knob)", p.vk, p.time)
		sendKey(p.vk, p.up) // carries ownInputMark, so the hook lets it through
	}
	if len(pending) > 0 {
		pSetTimer.Call(hwndMain, timerPending, pendingHoldMs, 0)
	}
}

// ---- config -----------------------------------------------------------------

func reloadConfig(announce bool) error {
	c, err := loadConfig()
	if st, serr := os.Stat(configPath()); serr == nil {
		cfgModTime = st.ModTime()
	}
	if err != nil {
		balloon(L("Lỗi cấu hình", "Config error"), err.Error())
		if cfg == nil {
			cfg = &Config{Devices: []string{"05AC:022C"}, Global: map[string]Action{}, Apps: map[string]map[string]Action{}}
		}
		return err
	}
	cfg = c
	configRev++
	setLanguage(cfg.Language)
	deviceKnown = map[uintptr]bool{}
	logf("config loaded: %d app entries, devices %v", len(cfg.Apps), cfg.Devices)
	if announce {
		balloon("Anticater Knob", fmt.Sprintf(L("Đã nạp lại cấu hình (%d hồ sơ app).", "Config reloaded (%d app entries)."), len(cfg.Apps)))
	}
	return nil
}

func housekeeping() {
	if st, err := os.Stat(configPath()); err == nil && !st.ModTime().Equal(cfgModTime) {
		reloadConfig(true)
	}
	warnBrightnessOnce()
	exe, title := foregroundApp()
	if exe != "" && !strings.EqualFold(exe, "explorer.exe") && !strings.EqualFold(exe, selfExeName()) && !isUIWindow(exe, title) {
		lastAppExe, lastAppTitle = exe, title
	}
}

func selfExeName() string {
	p, _ := os.Executable()
	if i := strings.LastIndexAny(p, `\/`); i >= 0 {
		return p[i+1:]
	}
	return p
}

// addAppToConfig appends an entry for exe, pre-filled from global so it only
// needs editing. Rewrites the file through a raw map to keep unknown keys.
func addAppToConfig(exe string) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		balloon(L("Lỗi cấu hình", "Config error"), err.Error())
		return
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		balloon(L("Lỗi cấu hình", "Config error"), err.Error())
		return
	}
	apps := map[string]json.RawMessage{}
	if a, ok := raw["apps"]; ok {
		json.Unmarshal(a, &apps)
	}
	for key := range apps {
		if strings.EqualFold(key, exe) {
			balloon("Anticater Knob", exe+L(" đã có trong cấu hình.", " is already in the config."))
			shellOpen(configPath())
			return
		}
	}
	entry, _ := json.Marshal(cfg.Global)
	apps[exe] = entry
	raw["apps"], _ = json.Marshal(apps)
	out, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile(configPath(), out, 0o644); err != nil {
		balloon(L("Lỗi cấu hình", "Config error"), err.Error())
		return
	}
	reloadConfig(false)
	balloon("Anticater Knob", L("Đã thêm "+exe+" (chép từ hồ sơ mặc định). Sửa trong file cấu hình hoặc cửa sổ Cài đặt.",
		"Added "+exe+" (copied from global). Edit it in the config file or the Settings window."))
	shellOpen(configPath())
}

// writeDiagnostics dumps what the app has seen so far to knob-diagnostic.txt
// and opens it. On macOS the equivalent report was what located every problem.
func writeDiagnostics() {
	exe, title := foregroundApp()
	var b strings.Builder
	fmt.Fprintf(&b, "Anticater Knob diagnostic report\r\nGenerated: %s\r\n\r\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Config file: %s\r\nDevices wanted: %v\r\nPaused: %v\r\nKeyboard hook installed: %v\r\n", configPath(), cfg.Devices, paused, hookHandle != 0)
	fmt.Fprintf(&b, "Last non-console app: %s\r\nForeground now: %s | %s\r\n\r\n", orDash(lastAppExe), orDash(exe), title)
	fmt.Fprintf(&b, "Knob reports received: %d\r\nRecognized gestures: %d\r\nNative keys swallowed: %d\r\nKeys re-sent (not from the knob): %d\r\nKeyboard hook calls: %d\r\nLast event: %s\r\n\r\n",
		rawCount, recognizedCount, swallowedCount, resentCount, hookCalls, lastEventText)
	b.WriteString("Consumer-control devices seen (only those that sent input):\r\n")
	if len(devicesSeen) == 0 {
		b.WriteString("  none yet - turn the knob, or press a media key on a keyboard\r\n")
	}
	for _, d := range devicesSeen {
		b.WriteString("  " + d + "\r\n")
	}
	b.WriteString("\r\nRecent knob reports (raw bytes):\r\n")
	if len(recentRaw) == 0 {
		b.WriteString("  none\r\n")
	}
	for _, r := range recentRaw {
		b.WriteString("  " + r + "\r\n")
	}
	b.WriteString("\r\nHints\r\n" +
		"- No devices listed: Windows is not delivering the knob's reports to this app.\r\n" +
		"- A device is listed with knob=false: add its VID:PID to \"devices\" in the config.\r\n" +
		"- Reports arrive but recognized stays 0: send the raw bytes above.\r\n" +
		"- Gestures recognized but the volume still changes: \"Native keys swallowed\" should grow; if it does not, the hook is not seeing the knob's keys.\r\n")
	path := strings.TrimSuffix(configPath(), "knob-config.json") + "knob-diagnostic.txt"
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		balloon("Anticater Knob", L("Không ghi được báo cáo: ", "Could not write the report: ")+err.Error())
		return
	}
	shellOpen(path)
}

// switchLanguage changes the UI language now and stores it in the config file,
// touching only the "language" key so the rest of the file is kept as written.
func switchLanguage(code string) {
	data, err := os.ReadFile(configPath())
	if err == nil {
		var raw map[string]json.RawMessage
		if err = json.Unmarshal(data, &raw); err == nil {
			raw["language"], _ = json.Marshal(code)
			var out []byte
			if out, err = json.MarshalIndent(raw, "", "  "); err == nil {
				err = os.WriteFile(configPath(), out, 0o644)
			}
		}
	}
	if err != nil {
		balloon(L("Lỗi cấu hình", "Config error"), err.Error())
	}
	setLanguage(code)
	reloadConfig(false)
	setLanguage(code) // even when the file could not be written
}

// ---- tray -------------------------------------------------------------------

func trayAdd() {
	nid = notifyIconData{Hwnd: hwndMain, ID: trayIconID, Flags: nifMessage | nifIcon | nifTip, CallbackMessage: wmTray}
	nid.Size = uint32(unsafe.Sizeof(nid))
	nid.Icon = appIcon(true)
	copyU16(nid.Tip[:], "Anticater Knob")
	pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
}

func balloon(title, text string) {
	logf("%s: %s", title, text)
	if hwndMain == 0 {
		return
	}
	n := nid
	n.Flags = nifInfo
	copyU16(n.InfoTitle[:], title)
	copyU16(n.Info[:], text)
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
}

func showMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	add := func(id uintptr, flags uintptr, text string) {
		pAppendMenuW.Call(menu, flags, id, uintptr(unsafe.Pointer(u16(text))))
	}
	check := func(on bool) uintptr {
		if on {
			return mfChecked
		}
		return mfString
	}
	add(0, mfGrayed, L("Gần nhất: ", "Last: ")+lastEventText)
	add(0, mfSeparator, "")
	add(cmdSettings, mfString, L("Cài đặt…", "Settings…"))
	add(cmdPause, check(paused), L("Tạm dừng", "Paused"))
	add(cmdOpenConfig, mfString, L("Mở file cấu hình", "Open config file"))
	add(cmdReload, mfString, L("Nạp lại cấu hình", "Reload config"))
	if lastAppExe != "" {
		add(cmdAddApp, mfString, L("Thêm "+lastAppExe+" vào cấu hình", "Add "+lastAppExe+" to config"))
	}
	add(cmdOpenLog, mfString, L("Mở log", "Open log"))
	add(cmdDiagnostics, mfString, L("Báo cáo chẩn đoán", "Diagnostic report"))
	add(cmdAutostart, check(autostartEnabled()), L("Khởi động cùng Windows", "Start with Windows"))
	add(cmdLanguage, mfString, L("Language: English", "Ngôn ngữ: Tiếng Việt"))
	add(0, mfSeparator, "")
	add(cmdExit, mfString, L("Thoát", "Exit"))

	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(hwndMain)
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(pt.X), uintptr(pt.Y), 0, hwndMain, 0)
	pDestroyMenu.Call(menu)

	switch cmd {
	case cmdSettings:
		openUI()
	case cmdPause:
		paused = !paused
		pending = pending[:0]
	case cmdOpenConfig:
		shellOpen(configPath())
	case cmdReload:
		reloadConfig(true)
	case cmdAddApp:
		addAppToConfig(lastAppExe)
	case cmdOpenLog:
		shellOpen(logPath())
	case cmdDiagnostics:
		writeDiagnostics()
	case cmdLanguage:
		switchLanguage(L("en", "vi"))
	case cmdAutostart:
		exe, _ := os.Executable()
		setAutostart(!autostartEnabled(), exe)
	case cmdExit:
		pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		pPostQuitMessage.Call(0)
	}
}

// ---- window -----------------------------------------------------------------

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case wmInput:
		t, _, _ := pGetMessageTime.Call()
		onRawInput(lParam, uint32(t))
	case wmTimer:
		switch wParam {
		case timerPending:
			flushPending()
		case timerHousekeeping:
			housekeeping()
		}
		return 0
	case wmUICall:
		runUICalls()
		return 0
	case wmShowUI:
		openUI()
		return 0
	case wmTray:
		switch lParam {
		case wmLButtonUp:
			openUI()
		case wmRButtonUp:
			showMenu()
		}
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	default:
		if message == msgTaskbar && msgTaskbar != 0 {
			trayAdd() // Explorer restarted
		}
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return r
}

func main() {
	runtime.LockOSThread()

	_, _, lastErr := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(u16("Local\\AnticaterKnobWindows"))))
	if errno, ok := lastErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		// Already running: starting the app again opens its settings page.
		if other, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(u16(mainClassName))), 0); other != 0 {
			pPostMessageW.Call(other, wmShowUI, 0, 0)
			return
		}
		setLanguage("")
		messageBox("Anticater Knob", L("App đang chạy rồi (tìm biểu tượng ở khay hệ thống).", "Already running (look for the icon in the system tray)."))
		return
	}

	reloadConfig(false)

	inst, _, _ := pGetModuleHandleW.Call(0)
	className := u16(mainClassName)
	wc := wndClassEx{WndProc: syscall.NewCallback(wndProc), Instance: inst, ClassName: className}
	wc.Size = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	hwndMain, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(u16("Anticater Knob"))),
		0, 0, 0, 0, 0, 0, 0, inst, 0)
	if hwndMain == 0 {
		messageBox("Anticater Knob", L("Không tạo được cửa sổ.", "Could not create the window."))
		return
	}

	r, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(u16("TaskbarCreated"))))
	msgTaskbar = uint32(r)
	trayAdd()

	// Consumer Control top-level collection (usage page 0x0C, usage 0x01).
	rid := rawInputDevice{UsagePage: 0x0C, Usage: 0x01, Flags: ridevInputSink, Target: hwndMain}
	if ok, _, err := pRegisterRawInputDevices.Call(uintptr(unsafe.Pointer(&rid)), 1, unsafe.Sizeof(rid)); ok == 0 {
		messageBox("Anticater Knob", fmt.Sprintf(L("Đăng ký Raw Input thất bại: %v", "Raw Input registration failed: %v"), err))
		return
	}

	hookHandle, _, _ = pSetWindowsHookExW.Call(whKeyboardLL, syscall.NewCallback(hookProc), inst, 0)
	if hookHandle == 0 {
		balloon("Anticater Knob", L("Không cài được hook bàn phím: thao tác đã gán lại vẫn sẽ đổi âm lượng.",
			"Keyboard hook failed: remapped gestures will also change the volume."))
	}
	defer pUnhookWindowsHookEx.Call(hookHandle)

	// The settings page. Started after the window exists: its requests are
	// handed to this thread through hwndMain.
	if srv, err := ui.Start(uiBackend{}, uiPort); err == nil {
		uiServer = srv
		if debugURLFile != "" {
			os.WriteFile(debugURLFile, []byte(srv.URL()), 0o600)
		}
	} else {
		logf("settings page unavailable: %v", err)
	}

	pSetTimer.Call(hwndMain, timerHousekeeping, 1000, 0)
	logf("started; config %s", configPath())

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}
