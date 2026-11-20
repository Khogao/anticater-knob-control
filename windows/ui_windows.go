package main

// Bridge between the knob core and the settings page served by package ui.
// The page runs in a browser app window; its requests arrive on HTTP
// goroutines and are carried over to the main thread, which owns all state.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"anticater-knob/ui"
)

const (
	uiTitle = "Anticater Knob" // the page's <title>, which is also its window title
	uiPort  = 47815            // preferred; any free port is used when it is taken
)

var (
	uiServer *ui.Server
	uiQueue  struct {
		sync.Mutex
		fns []func()
	}

	// debugURLFile is empty in normal builds. A test build can set it with
	// -ldflags "-X main.debugURLFile=path" to get the page address written out,
	// so the page can be driven by a test browser.
	debugURLFile string

	configRev     int // bumped on every successful config load
	gestureCounts = map[string]int{}
	lastRawText   string
	lastAppTitle  string
)

// onMain runs fn on the main thread and waits for it.
func onMain(fn func()) {
	done := make(chan struct{})
	uiQueue.Lock()
	uiQueue.fns = append(uiQueue.fns, func() { fn(); close(done) })
	uiQueue.Unlock()
	pPostMessageW.Call(hwndMain, wmUICall, 0, 0)
	<-done
}

func runUICalls() {
	uiQueue.Lock()
	fns := uiQueue.fns
	uiQueue.fns = nil
	uiQueue.Unlock()
	for _, fn := range fns {
		fn()
	}
}

type uiState struct {
	Platform      string         `json:"platform"`
	Language      string         `json:"language"`
	Paused        bool           `json:"paused"`
	HookInstalled bool           `json:"hookInstalled"`
	Devices       []string       `json:"devices"`
	KnobAgoMs     int64          `json:"knobAgoMs"` // -1 until the knob has sent something
	FrontApp      string         `json:"frontApp"`
	FrontTitle    string         `json:"frontTitle"`
	Effective     string         `json:"effective"` // apps key in force for FrontApp, "" for global
	Raw           int            `json:"raw"`
	Recognized    int            `json:"recognized"`
	Swallowed     int            `json:"swallowed"`
	Resent        int            `json:"resent"`
	Gestures      map[string]int `json:"gestures"`
	LastEvent     string         `json:"lastEvent"`
	LastRaw       string         `json:"lastRaw"`
	Autostart     bool           `json:"autostart"`
	ConfigPath    string         `json:"configPath"`
	ConfigRev     int            `json:"configRev"`
}

type appInfo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type uiBackend struct{}

func (uiBackend) State() any {
	var st uiState
	onMain(func() {
		st = uiState{
			Platform: "windows", Language: languageCode(), Paused: paused, HookInstalled: hookHandle != 0,
			Devices: cfg.Devices, KnobAgoMs: -1, FrontApp: lastAppExe, FrontTitle: lastAppTitle,
			Effective: cfg.profileFor(lastAppExe, lastAppTitle),
			Raw:       rawCount, Recognized: recognizedCount, Swallowed: swallowedCount, Resent: resentCount,
			Gestures: map[string]int{}, LastEvent: lastEventText, LastRaw: lastRawText,
			Autostart: autostartEnabled(), ConfigPath: configPath(), ConfigRev: configRev,
		}
		if lastKnobMs != 0 {
			st.KnobAgoMs = int64(tickMs() - lastKnobMs)
		}
		for g, n := range gestureCounts {
			st.Gestures[g] = n
		}
	})
	return st
}

func (uiBackend) Config() (data []byte, err error) {
	onMain(func() { data, err = json.Marshal(cfg) })
	return
}

func (uiBackend) SaveConfig(data []byte) (err error) {
	onMain(func() {
		var c Config
		if err = json.Unmarshal(data, &c); err != nil {
			return
		}
		if err = c.validate(); err != nil {
			return
		}
		if err = writeConfig(&c); err != nil {
			return
		}
		err = reloadConfig(false)
	})
	return
}

func (uiBackend) Apps() any {
	var apps []appInfo
	onMain(func() { apps = runningApps() })
	return apps
}

func (uiBackend) Command(name, arg string) (err error) {
	onMain(func() {
		switch name {
		case "pause":
			paused = !paused
			pending = pending[:0]
		case "reload":
			err = reloadConfig(false)
		case "clearCounters":
			rawCount, recognizedCount, swallowedCount, resentCount = 0, 0, 0, 0
			gestureCounts = map[string]int{}
			recentRaw, lastRawText = nil, ""
		case "diagnostics":
			writeDiagnostics()
		case "openConfig":
			shellOpen(configPath())
		case "openLog":
			shellOpen(logPath())
		case "autostart":
			exe, _ := os.Executable()
			setAutostart(!autostartEnabled(), exe)
		case "language":
			if arg != "vi" && arg != "en" {
				err = fmt.Errorf("unknown language %q", arg)
				return
			}
			switchLanguage(arg)
		default:
			err = fmt.Errorf("unknown command %q", name)
		}
	})
	return
}

func (uiBackend) Icon() []byte {
	data, err := os.ReadFile(iconPath())
	if err != nil {
		return nil
	}
	return data
}

// writeConfig stores c as the config file, indented and without the <
// escapes encoding/json applies by default.
func writeConfig(c *Config) error {
	if c.Global == nil {
		c.Global = map[string]Action{}
	}
	if c.Apps == nil {
		c.Apps = map[string]map[string]Action{}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return err
	}
	return os.WriteFile(configPath(), buf.Bytes(), 0o644)
}

// ---- the settings window ----------------------------------------------------

// isUIWindow reports whether a window is the settings page itself, which
// should not count as "the app the knob is being used in".
func isUIWindow(exe, title string) bool {
	// Its own app window is titled exactly uiTitle; when no Edge or Chrome is
	// found the page opens as a browser tab, titled "Anticater Knob - <browser>".
	return (title == uiTitle || strings.HasPrefix(title, uiTitle+" - ")) && !strings.EqualFold(exe, selfExeName())
}

var enumApps []appInfo
var enumUIWindow uintptr

func enumProc(hwnd, _ uintptr) uintptr {
	if vis, _, _ := pIsWindowVisible.Call(hwnd); vis == 0 {
		return 1
	}
	exe, title := windowApp(hwnd)
	if exe == "" || title == "" || strings.EqualFold(exe, selfExeName()) {
		return 1
	}
	if isUIWindow(exe, title) {
		enumUIWindow = hwnd
		return 1
	}
	for _, a := range enumApps {
		if strings.EqualFold(a.Name, exe) {
			return 1
		}
	}
	enumApps = append(enumApps, appInfo{exe, title})
	return 1
}

var enumCallback = syscall.NewCallback(enumProc)

// runningApps lists the apps that have a visible titled window, one entry per
// exe, so a profile can be created with a click.
func runningApps() []appInfo {
	enumApps, enumUIWindow = []appInfo{}, 0
	pEnumWindows.Call(enumCallback, 0)
	sort.Slice(enumApps, func(i, j int) bool { return strings.ToLower(enumApps[i].Name) < strings.ToLower(enumApps[j].Name) })
	if len(enumApps) > 60 {
		enumApps = enumApps[:60]
	}
	return enumApps
}

// openUI shows the settings page: it raises the window when one is open,
// otherwise it starts a browser app window (no tabs, no address bar).
func openUI() {
	if uiServer == nil {
		balloon("Anticater Knob", L("Không mở được trang cài đặt. Sửa trực tiếp file cấu hình.",
			"The settings page is unavailable. Edit the config file instead."))
		shellOpen(configPath())
		return
	}
	if uiServer.Active() {
		runningApps()
		if enumUIWindow != 0 {
			if iconic, _, _ := pIsIconic.Call(enumUIWindow); iconic != 0 {
				pShowWindow.Call(enumUIWindow, 9) // SW_RESTORE
			}
			pSetForegroundWindow.Call(enumUIWindow)
			return
		}
	}
	url := uiServer.URL()
	for _, browser := range appModeBrowsers() {
		if _, err := os.Stat(browser); err != nil {
			continue
		}
		cmd := exec.Command(browser, "--app="+url, "--window-size=1180,820")
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return
		}
	}
	shellOpen(url) // no Edge or Chrome: a tab in the default browser
}

// appModeBrowsers lists Chromium browsers that can show a page as an app
// window. Edge ships with Windows, so the first entry is normally there.
func appModeBrowsers() []string {
	var out []string
	for _, rel := range []string{`Microsoft\Edge\Application\msedge.exe`, `Google\Chrome\Application\chrome.exe`} {
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
			if dir := os.Getenv(env); dir != "" {
				out = append(out, filepath.Join(dir, rel))
			}
		}
	}
	return out
}
