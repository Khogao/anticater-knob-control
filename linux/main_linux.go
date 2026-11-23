// Anticater VK-01 knob remapper for Linux (KDE Plasma, Wayland or X11).
//
// The knob's input nodes are grabbed, so its volume / mute / brightness keys
// reach this app only. Each key is turned into a gesture, the gesture is looked
// up in the profile of the app in front (reported by a KWin script, see
// kwin_linux.go), and the action runs through a virtual input device. A
// gesture left as "passthrough" is sent on unchanged, so Plasma still handles
// volume and brightness the usual way.
//
// Same config format, gestures and settings page as the Windows app; see README.md.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"anticater-knob/ui"
)

const (
	everyResetMs = 500
	maxLogBytes  = 1 << 20
	scanEvery    = 2 * time.Second
)

var (
	// mu guards everything below and the config; reader goroutines, D-Bus
	// calls and the settings page all take it.
	mu sync.Mutex

	cfg        *Config
	cfgModTime time.Time
	configRev  int
	paused     bool

	knobs    = map[string]*knobReader{} // by /dev/input path
	knobErrs = map[string]string{}      // last error per node, so it is logged once

	// Modifier keys the knob itself pressed (it holds Shift while it sends F13
	// for one gesture). They are held back until the key that follows shows
	// whether they belong to a gesture, and stay in knobMods until released.
	knobMods     = map[uint16]bool{} // physically down on the knob
	knobModsSent = map[uint16]bool{} // down, passed on to the virtual device
	forwarded    = map[uint16]bool{} // knob keys passed on whose release must follow

	lastTooltip string // tooltip text last shown in the tray

	rawCount, recognizedCount, passedCount int
	gestureCounts                          = map[string]int{}
	recentRaw                              []string
	lastRawText                            string
	lastEventText                          = "no knob input yet"
	lastKnobAt                             time.Time

	everyGesture string
	everyCount   int
	everyAt      time.Time

	fgClass, fgCaption, fgKind string
	lastAppExe, lastAppTitle   string
	seenApps                   = map[string]string{} // class -> latest title

	logFile *os.File
)

var (
	logMu sync.Mutex
	// logOn mirrors cfg.Log so logf can be called without mu.
	logOn atomic.Bool
)

func init() { logOn.Store(true) }

func logf(format string, args ...any) {
	if !logOn.Load() {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	line := fmt.Sprintf("%s %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
	if isTerminal {
		os.Stderr.WriteString(line)
	}
	if logFile == nil {
		if st, err := os.Stat(logPath()); err == nil && st.Size() > maxLogBytes {
			os.Remove(logPath())
		}
		os.MkdirAll(configDir(), 0o755)
		f, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return
		}
		logFile = f
	}
	logFile.WriteString(line)
}

var isTerminal = func() bool {
	st, err := os.Stderr.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}()

// ---- gestures -----------------------------------------------------------------

func gestureForKey(code uint16) string {
	switch code {
	case keyVolumeUp:
		return "rotate_right"
	case keyVolumeDown:
		return "rotate_left"
	case keyMute, keyPlayPause: // Mute on some firmware, Play/Pause on others
		return "press"
	case keyBrightnessUp:
		return "press_rotate_right"
	case keyBrightnessDown:
		return "press_rotate_left"
	case keyF13:
		// A knob reassigned in the vendor app for Windows: hold + rotate right
		// sends F13, hold + rotate left Shift + F13.
		if knobMods[keyLeftShift] || knobMods[keyRightShift] {
			return "press_rotate_left"
		}
		return "press_rotate_right"
	}
	return ""
}

// gestureApp is the app whose profile a gesture uses: the window in front,
// except the settings window, which counts as no app (global profile).
func gestureApp() (string, string) {
	if isUI(fgClass, fgCaption) {
		return "", ""
	}
	return fgClass, fgCaption
}

// onKnobEvent handles one event from a grabbed knob node.
func onKnobEvent(n evdevNode, e inputEvent) {
	mu.Lock()
	defer mu.Unlock()
	switch e.Type {
	case evKey:
		onKnobKey(e)
	case evRel:
		// Knob revisions that report rotation as a mouse wheel: pass on as is.
		out.write([3]int32{int32(e.Type), int32(e.Code), e.Value})
	}
}

func onKnobKey(e inputEvent) {
	code, down, up := e.Code, e.Value == 1, e.Value == 0
	if down {
		rawCount++
		lastKnobAt = time.Now()
		lastRawText = fmt.Sprintf("%s  key %d down", time.Now().Format("15:04:05.000"), code)
		recentRaw = append(recentRaw, lastRawText)
		if len(recentRaw) > 30 {
			recentRaw = recentRaw[len(recentRaw)-30:]
		}
	}

	if isModifier(code) {
		switch {
		case down:
			knobMods[code] = true
		case up:
			delete(knobMods, code)
			if knobModsSent[code] {
				delete(knobModsSent, code)
				out.key(code, false)
			}
		}
		return
	}

	gesture := gestureForKey(code)
	if gesture == "" || paused {
		// Not a gesture (or paused): pass it on, with any modifiers the knob holds.
		switch {
		case down:
			flushKnobMods()
			forwarded[code] = true
			out.key(code, true)
			if gesture == "" {
				logf("knob key %d (not a gesture) passed on", code)
			}
		case up:
			delete(forwarded, code)
			out.key(code, false)
		default:
			out.write([3]int32{evKey, int32(code), e.Value})
		}
		return
	}

	switch {
	case down:
		if runGesture(gesture, fmt.Sprintf("key %d", code)) {
			// Passthrough: the knob's key goes on as it came, with its modifiers
			// (Shift + F13 stays Shift + F13).
			flushKnobMods()
			forwarded[code] = true
			passedCount++
			out.key(code, true)
		}
		// Otherwise the knob's held modifiers belong to this gesture and are not
		// passed on; they stay in knobMods until the knob releases them.
	case up:
		if forwarded[code] {
			delete(forwarded, code)
			out.key(code, false)
		}
	default: // auto-repeat
		if forwarded[code] {
			out.write([3]int32{evKey, int32(code), e.Value})
		}
	}
}

// flushKnobMods passes on the knob's held modifiers that have not been sent yet.
func flushKnobMods() {
	for m := range knobMods {
		if !knobModsSent[m] {
			knobModsSent[m] = true
			out.key(m, true)
		}
	}
}

// releaseKnobKeys lets go of every key passed on for the knob, so nothing stays
// held on the virtual device when the knob disconnects. mu is held.
func releaseKnobKeys() {
	for k := range forwarded {
		out.key(k, false)
	}
	for m := range knobModsSent {
		out.key(m, false)
	}
	forwarded, knobModsSent, knobMods = map[uint16]bool{}, map[uint16]bool{}, map[uint16]bool{}
}

// runGesture runs the action for a gesture; it returns true when the gesture
// is passthrough, i.e. the knob's own key should be passed on.
func runGesture(gesture, origin string) bool {
	recognizedCount++
	gestureCounts[gesture]++
	exe, title := gestureApp()
	act, source := cfg.resolve(gesture, exe, title)
	lastEventText = fmt.Sprintf("%s in %s -> %s", gesture, orDash(exe), describe(act, source))
	logf("%s  %s  app=%s  -> %s", origin, gesture, exe, describe(act, source))
	if act.isPassthrough() {
		return true
	}
	if act.Type == "noop" {
		return false
	}
	now := time.Now()
	if act.Every > 1 {
		if gesture != everyGesture || now.Sub(everyAt) > everyResetMs*time.Millisecond {
			everyGesture, everyCount = gesture, 0
		}
		everyAt = now
		everyCount++
		if everyCount%act.Every != 0 {
			return false
		}
	}
	perform(act)
	return false
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

// ---- the window in front (called from kwin_linux.go with mu held) -----------

func onActiveWindow(class, caption, kind string) {
	fgClass, fgCaption, fgKind = class, caption, kind
	if kind == "normal" && class != "" && !isUI(class, caption) {
		lastAppExe, lastAppTitle = class, caption
		noteApp(class, caption)
	}
	ledUpdate()
}

func noteApp(class, caption string) {
	if class == "" || isUI(class, caption) {
		return
	}
	seenApps[class] = caption
}

// ---- knob devices ---------------------------------------------------------------

// scanKnobs opens and grabs every input node of a configured knob that is not
// open yet. Runs at start and every scanEvery, so a knob that is plugged in or
// reconnects over Bluetooth later is picked up.
func scanKnobs() {
	mu.Lock()
	defer mu.Unlock()
	if out == nil {
		return // without the virtual device a grabbed knob would do nothing at all
	}
	added := false
	defer func() {
		if added {
			ledLast = "" // a knob that just connected gets the current colour
			ledUpdate()
		}
	}()
	for _, n := range listEvdev() {
		if _, open := knobs[n.Path]; open || !cfg.matchesDevice(n.Vendor, n.Product) {
			continue
		}
		k, err := openKnob(n)
		if err != nil {
			msg := err.Error()
			if errors.Is(err, os.ErrPermission) {
				msg += " (run install.sh, then log out and back in)"
			} else if strings.Contains(msg, "busy") {
				msg += " (another program holds the knob: xremap? stop it)"
			}
			if knobErrs[n.Path] != msg {
				logf("knob %s %q: %s", n.Path, n.Name, msg)
				knobErrs[n.Path] = msg
			}
			continue
		}
		delete(knobErrs, n.Path)
		knobs[n.Path] = k
		added = true
		logf("knob %s %q (%04X:%04X) grabbed", n.Path, n.Name, n.Vendor, n.Product)
		go func() {
			err := k.run(onKnobEvent)
			mu.Lock()
			delete(knobs, k.node.Path)
			if len(knobs) == 0 {
				releaseKnobKeys()
			}
			mu.Unlock()
			k.close()
			logf("knob %s gone: %v", k.node.Path, err)
		}()
	}
}

// ---- config -------------------------------------------------------------------

// reloadConfig loads the config file; mu is held.
func reloadConfig() error {
	c, err := loadConfig()
	if st, serr := os.Stat(configPath()); serr == nil {
		cfgModTime = st.ModTime()
	}
	if err != nil {
		logf("config error: %v", err)
		notify(L("Lỗi cấu hình", "Config error"), err.Error())
		if cfg == nil {
			cfg = &Config{Devices: []string{"05AC:022C"}, Global: map[string]Action{}, Apps: map[string]map[string]Action{}, Log: true}
		}
		return err
	}
	cfg = c
	configRev++
	logOn.Store(cfg.Log)
	// Let go of knobs no longer listed in "devices"; their readers end and clean up.
	for _, k := range knobs {
		if !cfg.matchesDevice(k.node.Vendor, k.node.Product) {
			k.close()
		}
	}
	setLanguage(cfg.Language)
	logf("config loaded: %d app entries, devices %v", len(cfg.Apps), cfg.Devices)
	ledLast = ""
	ledUpdate()
	return nil
}

func housekeeping() {
	mu.Lock()
	if st, err := os.Stat(configPath()); err == nil && !st.ModTime().Equal(cfgModTime) {
		reloadConfig()
	}
	ledHousekeeping()
	// Retry a colour that did not reach the knob, every few seconds.
	if cfg.LED != nil && ledLast == "" && time.Since(ledTriedAt) > 5*time.Second {
		ledUpdate()
	}
	if lastEventText != lastTooltip {
		lastTooltip = lastEventText
		trayRefresh()
	}
	mu.Unlock()
}

// switchLanguage changes the UI language and stores it, touching only the
// "language" key of the file. mu is held.
func switchLanguage(code string) {
	data, err := os.ReadFile(configPath())
	if err == nil {
		var raw map[string]json.RawMessage
		if err = json.Unmarshal(data, &raw); err == nil {
			raw["language"], _ = json.Marshal(code)
			var b []byte
			if b, err = json.MarshalIndent(raw, "", "  "); err == nil {
				err = os.WriteFile(configPath(), b, 0o644)
			}
		}
	}
	if err != nil {
		logf("language: %v", err)
	}
	setLanguage(code)
	reloadConfig()
	setLanguage(code)
}

// writeDiagnostics writes knob-diagnostic.txt and opens it. mu is held.
func writeDiagnostics() {
	var b strings.Builder
	fmt.Fprintf(&b, "Anticater Knob diagnostic report (Linux)\nGenerated: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "Config file: %s\nDevices wanted: %v\nPaused: %v\nVirtual output device: %v\n", configPath(), cfg.Devices, paused, out != nil)
	fmt.Fprintf(&b, "Desktop: %s, session %s\nActive-window script: %s\n", os.Getenv("XDG_CURRENT_DESKTOP"), os.Getenv("XDG_SESSION_TYPE"), orDash(map[bool]string{true: "running", false: "not running " + kwinErr}[bus != nil && kwinScriptLoaded()]))
	fmt.Fprintf(&b, "Window in front: %s | %s (%s)\nLast app: %s\n\n", orDash(fgClass), fgCaption, fgKind, orDash(lastAppExe))
	fmt.Fprintf(&b, "Knob key presses: %d\nRecognized gestures: %d\nPassed on to the system: %d\nLast event: %s\n\n", rawCount, recognizedCount, passedCount, lastEventText)
	b.WriteString("Knob input nodes grabbed:\n")
	if len(knobs) == 0 {
		b.WriteString("  none\n")
	}
	for p, k := range knobs {
		fmt.Fprintf(&b, "  %s  %q  %04X:%04X\n", p, k.node.Name, k.node.Vendor, k.node.Product)
	}
	for p, e := range knobErrs {
		fmt.Fprintf(&b, "  %s  error: %s\n", p, e)
	}
	b.WriteString("\nAll input devices:\n")
	for _, n := range listEvdev() {
		fmt.Fprintf(&b, "  %s  %04X:%04X  %s\n", n.Path, n.Vendor, n.Product, n.Name)
	}
	b.WriteString("\nLights: " + ledStatus() + "\n")
	b.WriteString("\nRecent knob keys:\n")
	if len(recentRaw) == 0 {
		b.WriteString("  none\n")
	}
	for _, r := range recentRaw {
		b.WriteString("  " + r + "\n")
	}
	path := configDir() + "/knob-diagnostic.txt"
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		logf("diagnostic: %v", err)
		return
	}
	shellOpen(path)
}

// ---- desktop helpers --------------------------------------------------------------

func shellOpen(target string) {
	cmd := exec.Command("xdg-open", target)
	if err := cmd.Start(); err == nil {
		go cmd.Wait()
	}
}

// notify shows a desktop notification (best effort).
func notify(title, text string) {
	if bus == nil {
		return
	}
	bus.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications").Go(
		"org.freedesktop.Notifications.Notify", 0, nil,
		"Anticater Knob", uint32(0), "anticater-knob", title, text, []string{}, map[string]any{}, int32(6000))
}

func autostartPath() string {
	return strings.TrimSuffix(configDir(), "anticater-knob") + "autostart/anticater-knob.desktop"
}

func autostartEnabled() bool {
	_, err := os.Stat(autostartPath())
	return err == nil
}

func setAutostart(on bool) {
	if !on {
		os.Remove(autostartPath())
		return
	}
	exe, _ := os.Executable()
	os.MkdirAll(strings.TrimSuffix(autostartPath(), "/anticater-knob.desktop"), 0o755)
	entry := "[Desktop Entry]\nType=Application\nName=Anticater Knob\nExec=\"" + exe + "\" --background\nIcon=anticater-knob\nX-GNOME-Autostart-enabled=true\n"
	if err := os.WriteFile(autostartPath(), []byte(entry), 0o644); err != nil {
		logf("autostart: %v", err)
	}
}

// ---- main ---------------------------------------------------------------------------

func main() {
	background := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--background":
			background = true
		case "--list-devices":
			for _, n := range listEvdev() {
				fmt.Printf("%s  %04X:%04X  %s\n", n.Path, n.Vendor, n.Product, n.Name)
			}
			return
		case "--version", "-h", "--help":
			fmt.Println("Anticater Knob for Linux. Options: --background (no settings window at start), --list-devices")
			return
		}
	}

	mu.Lock()
	reloadConfig()
	mu.Unlock()

	first, err := claimBus()
	if err != nil {
		logf("session bus: %v (no active-window detection, no tray icon)", err)
	} else if !first {
		fmt.Println("Anticater Knob is already running; asked it to show its settings window.")
		return
	}

	if out, err = openUinput(); err != nil {
		logf("cannot create the virtual input device: %v (run install.sh, then log out and back in)", err)
		notify("Anticater Knob", L("Không tạo được thiết bị nhập ảo. Chạy install.sh rồi đăng nhập lại.",
			"Cannot create the virtual input device. Run install.sh, then log out and back in."))
	}
	defer func() { out.close() }()

	startWindowWatch()
	defer stopWindowWatch()
	trayStart()

	if srv, err := ui.Start(uiBackend{}, uiPort); err == nil {
		uiServer = srv
		if debugURLFile != "" {
			os.WriteFile(debugURLFile, []byte(srv.URL()), 0o600)
		}
	} else {
		logf("settings page unavailable: %v", err)
	}

	scanKnobs()
	mu.Lock()
	n := len(knobs)
	mu.Unlock()
	logf("started; config %s; %d knob nodes grabbed", configPath(), n)
	if !background {
		go openUI()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	scanAt := time.Now()
	scriptAt := time.Now()
	for {
		select {
		case <-sig:
			logf("exiting")
			mu.Lock()
			ledStop()
			mu.Unlock()
			return
		case <-quitCh:
			logf("exiting (quit from the menu)")
			mu.Lock()
			ledStop()
			mu.Unlock()
			return
		case <-tick.C:
			housekeeping()
			if time.Since(scanAt) >= scanEvery {
				if out == nil {
					if o, err := openUinput(); err == nil {
						mu.Lock()
						out = o
						mu.Unlock()
						logf("virtual input device created")
					}
				}
				scanKnobs()
				scanAt = time.Now()
			}
			if bus != nil && time.Since(scriptAt) >= 10*time.Second {
				scriptAt = time.Now()
				if !kwinScriptLoaded() {
					startWindowWatch() // KWin restarted, or it failed earlier
				}
				trayEnsure() // plasmashell restarted: register the icon again
			}
		}
	}
}

var quitCh = make(chan struct{}, 1)

func sortedApps() []appInfo {
	var apps []appInfo
	for c, t := range seenApps {
		apps = append(apps, appInfo{c, t})
	}
	sort.Slice(apps, func(i, j int) bool { return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name) })
	if len(apps) > 60 {
		apps = apps[:60]
	}
	return apps
}
