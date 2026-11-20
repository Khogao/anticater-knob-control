package main

// The colour of the knob's lights follows the app in front, as set in the
// "led" section of the config:
//
//	"led": { "brightness": 45, "auto": true,
//	         "apps": { "desktop": "FFFFFF", "explorer.exe": "FFC800",
//	                   "msedge.exe": "00A7E1", "chrome.exe": "FF0000" } }
//
// Keys in "apps" match like profile keys: an exe name, or "exe|text" to match
// only when the window title contains text. "desktop" is the Windows desktop
// itself (explorer.exe's desktop window); "explorer.exe" then only covers File
// Explorer windows. A colour is RRGGBB or "off".
//
// An app without a colour gets, with "auto": its brand colour when it is a
// well-known app, otherwise a fixed colour made from its exe name; without
// "auto": the "global" colour, and an empty "global" leaves the lights alone.
//
// The Bluetooth write is done by knob-led.ps1 (embedded below), which
// Windows PowerShell runs once, hidden, for the life of the app; this side
// only writes one line per colour change to its standard input. Windows
// PowerShell reaches the WinRT Bluetooth API without cgo or extra libraries.

import (
	"bufio"
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed knob-led.ps1
var ledScript []byte

type LEDConfig struct {
	Brightness int               `json:"brightness"`
	Auto       bool              `json:"auto,omitempty"`
	Global     string            `json:"global,omitempty"`
	Apps       map[string]string `json:"apps,omitempty"`
}

var ledColorRe = regexp.MustCompile(`^(?i:[0-9a-f]{6}|off)$`)

func (l *LEDConfig) validate() error {
	if l.Brightness < 0 || l.Brightness > 100 {
		return fmt.Errorf("led.brightness must be 0..100, not %d", l.Brightness)
	}
	if l.Brightness == 0 {
		l.Brightness = 45
	}
	check := func(where, c string) error {
		if c != "" && !ledColorRe.MatchString(strings.TrimPrefix(c, "#")) {
			return fmt.Errorf("%s: colour must be RRGGBB or \"off\", not %q", where, c)
		}
		return nil
	}
	if err := check("led.global", l.Global); err != nil {
		return err
	}
	for k, c := range l.Apps {
		if err := check("led.apps."+k, c); err != nil {
			return err
		}
	}
	return nil
}

// colour returns the line for knob-led.ps1 for this app, or "" to leave the
// lights as they are.
func (l *LEDConfig) colour(exe, title string) string {
	lexe, ltitle := strings.ToLower(exe), strings.ToLower(title)
	if lexe == "desktop" {
		if c, ok := l.Apps["desktop"]; ok {
			return l.line(c)
		}
		return l.line(l.Global)
	}
	best, bestLen := "", -1
	for key := range l.Apps {
		name, want, hasTitle := strings.Cut(key, "|")
		if lexe == "" || strings.ToLower(strings.TrimSpace(name)) != lexe {
			continue
		}
		score := 0
		if hasTitle {
			want = strings.ToLower(strings.TrimSpace(want))
			if want == "" || !strings.Contains(ltitle, want) {
				continue
			}
			score = 1 + len(want)
		}
		if score > bestLen || (score == bestLen && key < best) {
			best, bestLen = key, score
		}
	}
	c := l.Global
	switch {
	case best != "":
		c = l.Apps[best]
	case l.Auto:
		c = autoColour(lexe)
	}
	return l.line(c)
}

func (l *LEDConfig) line(c string) string {
	c = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(c), "#"))
	switch c {
	case "":
		return ""
	case "OFF":
		return "off"
	}
	return fmt.Sprintf("%s %d", c, l.Brightness)
}

// brandColours are the colours people know these apps by.
var brandColours = map[string]string{
	// Microsoft Office and Windows
	"winword.exe": "2B579A", "excel.exe": "217346", "powerpnt.exe": "D24726", "outlook.exe": "0078D4",
	"onenote.exe": "7719AA", "msaccess.exe": "A4373A", "visio.exe": "3955A3", "winproj.exe": "31752F",
	"mspub.exe": "077568", "ms-teams.exe": "6264A7", "teams.exe": "6264A7", "notepad.exe": "FFFFFF",
	"windowsterminal.exe": "FFFFFF", "mspaint.exe": "0078D4", "taskmgr.exe": "0078D4",
	// Office alternatives and PDF
	"wps.exe": "C9282D", "et.exe": "2E8B57", "wpp.exe": "D24726", "acrobat.exe": "EC1C24", "acrord32.exe": "EC1C24",
	"foxitpdfreader.exe": "F04D23", "foxitpdfeditor.exe": "F04D23",
	// Chat
	"zalo.exe": "0068FF", "whatsapp.exe": "25D366", "whatsapp.root.exe": "25D366", "telegram.exe": "26A5E4",
	"messenger.exe": "0084FF", "skype.exe": "00AFF0", "viber.exe": "7360F2", "line.exe": "06C755",
	"wechat.exe": "07C160", "weixin.exe": "07C160", "slack.exe": "4A154B", "discord.exe": "5865F2", "zoom.exe": "2D8CFF",
	// Browsers other than Edge and Chrome
	"firefox.exe": "FF7139", "brave.exe": "FB542B", "opera.exe": "FF1B2D", "coccoc.exe": "1DB954", "vivaldi.exe": "EF3939",
	// CAD and 3D
	"acad.exe": "E51937", "revit.exe": "0696D7", "3dsmax.exe": "37A5CC", "sketchup.exe": "005F9E",
	"rhino.exe": "8A2BE2", "blender.exe": "F5792A", "inventor.exe": "F2A900", "civil3d.exe": "E51937",
	// Adobe and design
	"photoshop.exe": "31A8FF", "illustrator.exe": "FF9A00", "indesign.exe": "FF3366", "adobe premiere pro.exe": "9999FF",
	"afterfx.exe": "9999FF", "lightroom.exe": "31A8FF", "figma.exe": "A259FF", "canva.exe": "00C4CC",
	"resolve.exe": "FF5A36", "capcut.exe": "FFFFFF",
	// Code
	"code.exe": "007ACC", "cursor.exe": "FFFFFF", "devenv.exe": "5C2D91", "idea64.exe": "FE315D", "pycharm64.exe": "21D789",
	"notepad++.exe": "90E59A", "sublime_text.exe": "FF9800", "githubdesktop.exe": "6E40C9", "postman.exe": "FF6C37",
	"claude.exe": "D97757", "chatgpt.exe": "10A37F",
	// Notes, media, other
	"obsidian.exe": "7C3AED", "notion.exe": "FFFFFF", "evernote.exe": "00A82D", "spotify.exe": "1DB954",
	"vlc.exe": "FF8800", "potplayermini64.exe": "FFCC00", "steam.exe": "1A9FFF", "tradingview.exe": "2962FF",
}

// autoColour picks a fixed, saturated colour for an app: its brand colour, or
// one made from a hash of the exe name so the same app always gets the same.
func autoColour(lexe string) string {
	if c, ok := brandColours[lexe]; ok {
		return vivid(c)
	}
	h := uint32(2166136261)
	for i := 0; i < len(lexe); i++ {
		h = (h ^ uint32(lexe[i])) * 16777619
	}
	r, g, b := hsvToRGB(float64(h%360), 1, 1)
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

// vivid keeps a colour's hue but makes it fully saturated and bright. The
// knob's lights wash out muted colours: Claude's #D97757 showed nearly white.
// Grey and white (no hue) stay as they are.
func vivid(hex string) string {
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
		return hex
	}
	mx, mn := max(r, g, b), min(r, g, b)
	if mx-mn < 24 {
		return hex
	}
	var h float64
	d := float64(mx - mn)
	switch mx {
	case r:
		h = mod(float64(g-b)/d+6, 6) * 60
	case g:
		h = (float64(b-r)/d + 2) * 60
	default:
		h = (float64(r-g)/d + 4) * 60
	}
	// The knob's blue light is strong: a green with any blue in it (Excel's
	// #217346 sits at hue 146) shows as turquoise. Brand greens become pure green.
	if h >= 90 && h <= 165 {
		h = 120
	}
	R, G, B := hsvToRGB(h, 1, 1)
	return fmt.Sprintf("%02X%02X%02X", R, G, B)
}

func hsvToRGB(h, s, v float64) (byte, byte, byte) {
	c := v * s
	x := c * (1 - abs(mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	return byte((r + m) * 255), byte((g + m) * 255), byte((b + m) * 255)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func mod(a, b float64) float64 { return a - b*float64(int(a/b)) }

var (
	ledMu       sync.Mutex
	ledIn       io.WriteCloser
	ledCmd      *exec.Cmd
	ledLast     string
	ledResults  []string // lines from the script, logged on the main thread
	ledHook     uintptr
	pSetWinHook = user32.NewProc("SetWinEventHook")
)

const eventSystemForeground = 3

var pGetClassNameW = user32.NewProc("GetClassNameW")

func windowClass(hwnd uintptr) string {
	buf := make([]uint16, 256)
	n, _, _ := pGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:n])
}

// ledStart hooks foreground changes; call once on the main thread.
func ledStart() {
	ledHook, _, _ = pSetWinHook.Call(eventSystemForeground, eventSystemForeground, 0,
		syscall.NewCallback(func(hook, event, hwnd uintptr, idObject, idChild int32, thread, time uint32) uintptr {
			ledUpdate()
			return 0
		}), 0, 0, 0) // WINEVENT_OUTOFCONTEXT: called on this thread's message loop
	ledUpdate()
}

// ledUpdate sends the colour for the app in front if it changed.
func ledUpdate() {
	if cfg == nil || cfg.LED == nil || paused {
		return
	}
	hwnd, _, _ := pGetForegroundWindow.Call()
	if hwnd == 0 {
		return
	}
	exe, title := windowApp(hwnd)
	if exe == "" || strings.EqualFold(exe, selfExeName()) || isUIWindow(exe, title) {
		return // keep the colour of the app the settings were opened from
	}
	if strings.EqualFold(exe, "explorer.exe") {
		switch windowClass(hwnd) {
		case "Progman", "WorkerW":
			exe = "desktop"
		case "CabinetWClass", "ExploreWClass":
		default:
			return // taskbar, Start, tray flyouts: keep the colour
		}
	}
	line := cfg.LED.colour(exe, title)
	if line == "" || line == ledLast {
		return
	}
	if err := ledSend(line); err != nil {
		logf("led %s for %s: %v", line, exe, err)
		return
	}
	ledLast = line
	logf("led %s for %s", line, exe)
}

func ledSend(line string) error {
	ledMu.Lock()
	defer ledMu.Unlock()
	if ledIn == nil {
		if err := ledLaunch(); err != nil {
			return err
		}
	}
	if _, err := io.WriteString(ledIn, line+"\n"); err != nil {
		ledIn.Close()
		ledIn = nil
		return err
	}
	return nil
}

// ledLaunch writes knob-led.ps1 next to the exe and starts it. ledMu is held.
func ledLaunch() error {
	path := filepath.Join(filepath.Dir(configPath()), "knob-led.ps1")
	if old, err := os.ReadFile(path); err != nil || !bytes.Equal(old, ledScript) {
		if err := os.WriteFile(path, ledScript, 0o644); err != nil {
			return err
		}
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return err
	}
	ledIn, ledCmd = in, cmd
	go func() {
		sc := bufio.NewScanner(out)
		for sc.Scan() {
			ledMu.Lock()
			ledResults = append(ledResults, sc.Text())
			if strings.HasPrefix(sc.Text(), "error") {
				ledLast = "" // try again on the next change
			}
			ledMu.Unlock()
		}
		ledMu.Lock()
		if ledCmd == cmd {
			ledIn, ledCmd, ledLast = nil, nil, ""
		}
		ledMu.Unlock()
		cmd.Wait()
	}()
	return nil
}

// ledHousekeeping logs what the script answered; main thread.
func ledHousekeeping() {
	ledMu.Lock()
	res := ledResults
	ledResults = nil
	ledMu.Unlock()
	for _, r := range res {
		logf("led script: %s", r)
	}
}

// ledStop ends the script when the app exits.
func ledStop() {
	ledMu.Lock()
	defer ledMu.Unlock()
	if ledIn != nil {
		ledIn.Close() // the script ends when its input closes
		ledIn = nil
	}
}
