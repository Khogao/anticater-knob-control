package main

// Bridge between the knob core and the settings page served by package ui
// (the same page as the Windows app). Requests arrive on HTTP goroutines and
// take mu, which guards all state.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "embed"

	"anticater-knob/ui"
)

const (
	uiTitle = "Anticater Knob" // the page's <title>, which is also its window title
	uiPort  = 47815            // preferred; any free port is used when it is taken
)

//go:embed anticater-knob.svg
var iconSVG []byte

var (
	uiServer *ui.Server
	// debugURLFile is empty in normal builds; a test build can set it with
	// -ldflags "-X main.debugURLFile=path" to get the page address written out.
	debugURLFile string
)

type uiState struct {
	Platform      string            `json:"platform"`
	Language      string            `json:"language"`
	Paused        bool              `json:"paused"`
	HookInstalled bool              `json:"hookInstalled"`
	Devices       []string          `json:"devices"`
	KnobAgoMs     int64             `json:"knobAgoMs"`
	FrontApp      string            `json:"frontApp"`
	FrontTitle    string            `json:"frontTitle"`
	Effective     string            `json:"effective"`
	Raw           int               `json:"raw"`
	Recognized    int               `json:"recognized"`
	Swallowed     int               `json:"swallowed"`
	Resent        int               `json:"resent"`
	Gestures      map[string]int    `json:"gestures"`
	LastEvent     string            `json:"lastEvent"`
	LastRaw       string            `json:"lastRaw"`
	Autostart     bool              `json:"autostart"`
	ConfigPath    string            `json:"configPath"`
	ConfigRev     int               `json:"configRev"`
	LEDColours    map[string]string `json:"ledColours,omitempty"`
	KnobNodes     int               `json:"knobNodes"`
}

type appInfo struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

type uiBackend struct{}

func (uiBackend) State() any {
	mu.Lock()
	defer mu.Unlock()
	st := uiState{
		Platform: "linux", Language: languageCode(), Paused: paused,
		// On Linux the knob is grabbed: "hook" means at least one knob node is held.
		HookInstalled: len(knobs) > 0 && out != nil,
		Devices:       cfg.Devices, KnobAgoMs: -1, FrontApp: lastAppExe, FrontTitle: lastAppTitle,
		Effective: cfg.profileFor(lastAppExe, lastAppTitle),
		Raw:       rawCount, Recognized: recognizedCount, Swallowed: recognizedCount - passedCount, Resent: passedCount,
		Gestures: map[string]int{}, LastEvent: lastEventText, LastRaw: lastRawText,
		Autostart: autostartEnabled(), ConfigPath: configPath(), ConfigRev: configRev, KnobNodes: len(knobs),
	}
	if cfg.LED != nil {
		st.LEDColours = map[string]string{}
		keys := []string{"desktop", "settings"}
		for k := range cfg.Apps {
			keys = append(keys, k)
		}
		for k := range seenApps {
			keys = append(keys, k)
		}
		for _, k := range keys {
			exe, title, _ := strings.Cut(k, "|")
			c, _ := cfg.LED.pick(exe, title)
			st.LEDColours[k] = strings.ToUpper(strings.TrimPrefix(c, "#"))
		}
	}
	if !lastKnobAt.IsZero() {
		st.KnobAgoMs = time.Since(lastKnobAt).Milliseconds()
	}
	for g, n := range gestureCounts {
		st.Gestures[g] = n
	}
	return st
}

func (uiBackend) Config() ([]byte, error) {
	mu.Lock()
	defer mu.Unlock()
	return json.Marshal(cfg)
}

func (uiBackend) SaveConfig(data []byte) error {
	mu.Lock()
	defer mu.Unlock()
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}
	if err := c.validate(); err != nil {
		return err
	}
	if err := writeConfig(&c); err != nil {
		return err
	}
	return reloadConfig()
}

func (uiBackend) Apps() any {
	mu.Lock()
	defer mu.Unlock()
	apps := sortedApps()
	if apps == nil {
		apps = []appInfo{}
	}
	return apps
}

func (uiBackend) Command(name, arg string) error {
	if name == "openConfig" || name == "openLog" {
		p := configPath()
		if name == "openLog" {
			p = logPath()
		}
		shellOpen(p)
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	switch name {
	case "pause":
		paused = !paused
		ledLast = ""
		if !paused {
			ledUpdate()
		}
	case "reload":
		return reloadConfig()
	case "clearCounters":
		rawCount, recognizedCount, passedCount = 0, 0, 0
		gestureCounts = map[string]int{}
		recentRaw, lastRawText = nil, ""
	case "diagnostics":
		writeDiagnostics()
	case "ledPreview":
		ledPreview(arg)
	case "autostart":
		setAutostart(!autostartEnabled())
	case "language":
		if arg != "vi" && arg != "en" {
			return fmt.Errorf("unknown language %q", arg)
		}
		switchLanguage(arg)
		trayRefresh()
	default:
		return fmt.Errorf("unknown command %q", name)
	}
	return nil
}

func (uiBackend) Icon() []byte { return nil }

// writeConfig stores c as the config file, indented and without HTML escapes.
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
	os.MkdirAll(configDir(), 0o755)
	return os.WriteFile(configPath(), buf.Bytes(), 0o644)
}

// openUI shows the settings page: raises the window when one is open,
// otherwise opens the page as an app window of a Chromium browser (no tabs, no
// address bar), or as a tab of the default browser when there is none.
func openUI() {
	if uiServer == nil {
		shellOpen(configPath())
		return
	}
	if uiServer.Active() {
		raiseSettingsWindow()
		// The page can be polling from a browser tab that is not the selected
		// one; then there is no window titled for it and nothing came to the
		// front. Open it again in that case.
		time.Sleep(1500 * time.Millisecond)
		mu.Lock()
		shown := isUI(fgClass, fgCaption)
		mu.Unlock()
		if shown {
			return
		}
	}
	target, err := launchPage(uiServer.URL())
	if err != nil {
		logf("settings page: %v", err)
		return
	}
	for _, b := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable", "brave-browser"} {
		path, err := exec.LookPath(b)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, "--app="+target, "--window-size=1180,820")
		if err := cmd.Start(); err == nil {
			go cmd.Wait()
			return
		}
	}
	shellOpen(target)
}

// launchPage writes a small page that forwards to url, readable only by this
// user, and returns its file:// address. The page address carries the API's
// access token; handing it to the browser as a command-line argument would
// show it to every local user in the process list.
func launchPage(url string) (string, error) {
	dir := filepath.Join(cacheDir(), "launch")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	os.Chmod(dir, 0o700)
	path := filepath.Join(dir, "settings.html")
	js, _ := json.Marshal(url)
	page := "<!doctype html><meta charset=utf-8><title>" + uiTitle + "</title><script>location.replace(" + string(js) + ")</script>"
	if err := os.WriteFile(path, []byte(page), 0o600); err != nil {
		return "", err
	}
	os.Chmod(path, 0o600)
	go func() { // the browser has read it by then
		time.Sleep(20 * time.Second)
		os.Remove(path)
	}()
	return "file://" + path, nil
}
