package main

// Which app is in front, on KDE Plasma (Wayland or X11).
//
// Wayland gives ordinary programs no way to ask for the active window, so a
// small KWin script (below) runs inside the compositor and calls this app over
// D-Bus each time the active window or its title changes. This app owns the
// session-bus name org.anticater.Knob; the same name keeps a second copy from
// starting (the second copy asks the first to show its settings instead).

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	busName      = "org.anticater.Knob"
	busPath      = "/org/anticater/Knob"
	busIface     = "org.anticater.Knob"
	kwinScript   = "anticater-knob-active-window"
	kwinRaiseTmp = "anticater-knob-raise"
)

const kwinScriptJS = `// Installed by Anticater Knob: reports the active window to the app over D-Bus.
const S = "org.anticater.Knob", P = "/org/anticater/Knob", I = "org.anticater.Knob";
function kind(w) {
    if (!w) return "none";
    if (w.desktopWindow) return "desktop";
    if (w.normalWindow || w.dialog) return "normal";
    return "other";
}
function report(w) {
    callDBus(S, P, I, "Active", w ? String(w.resourceClass || "") : "", w ? String(w.caption || "") : "",
        w ? String(w.pid || 0) : "0", kind(w));
}
let watched = null;
function onCaption() { report(watched); }
function onActivated(w) {
    if (watched) { try { watched.captionChanged.disconnect(onCaption); } catch (e) {} }
    watched = w;
    if (w) { try { w.captionChanged.connect(onCaption); } catch (e) {} }
    report(w);
}
function seen(w) {
    if (w && w.normalWindow && !w.skipTaskbar)
        callDBus(S, P, I, "Seen", String(w.resourceClass || ""), String(w.caption || ""));
}
workspace.windowActivated.connect(onActivated);
workspace.windowAdded.connect(seen);
workspace.windowList().forEach(seen);
onActivated(workspace.activeWindow);
`

// kwinRaiseJS brings the settings window to the front.
const kwinRaiseJS = `workspace.windowList().forEach(function (w) {
    if (String(w.caption) === %q || String(w.caption).indexOf(%q) === 0) {
        if (w.minimized) w.minimized = false;
        workspace.activeWindow = w;
    }
});
`

var bus *dbus.Conn

// knobService is the object KWin's script (and a second copy of this app) call.
type knobService struct{}

func (knobService) Active(class, caption, pid, kind string) *dbus.Error {
	mu.Lock()
	defer mu.Unlock()
	onActiveWindow(class, caption, kind)
	return nil
}

func (knobService) Seen(class, caption string) *dbus.Error {
	mu.Lock()
	defer mu.Unlock()
	noteApp(class, caption)
	return nil
}

func (knobService) ShowSettings() *dbus.Error {
	go openUI()
	return nil
}

// claimBus connects to the session bus and takes the app's name. It returns
// false when another copy already has it; that copy has then been asked to
// show its settings window.
func claimBus() (bool, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return false, err
	}
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return false, err
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Object(busName, busPath).Call(busIface+".ShowSettings", 0)
		conn.Close()
		return false, nil
	}
	if err := conn.Export(knobService{}, busPath, busIface); err != nil {
		return false, err
	}
	bus = conn
	return true, nil
}

func cacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "anticater-knob")
}

// runKWinScript loads a script file into KWin under name and starts it.
func runKWinScript(name, js string) error {
	dir := cacheDir()
	os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, name+".js")
	if err := os.WriteFile(path, []byte(js), 0o644); err != nil {
		return err
	}
	kw := bus.Object("org.kde.KWin", "/Scripting")
	kw.Call("org.kde.kwin.Scripting.unloadScript", 0, name)
	var id int32
	if err := kw.Call("org.kde.kwin.Scripting.loadScript", 0, path, name).Store(&id); err != nil {
		return fmt.Errorf("KWin loadScript: %v", err)
	}
	if id < 0 {
		return fmt.Errorf("KWin refused the script (id %d)", id)
	}
	// Plasma 6 names the script object /Scripting/Script<id>, Plasma 5 /<id>.
	for _, p := range []string{fmt.Sprintf("/Scripting/Script%d", id), fmt.Sprintf("/%d", id)} {
		if err := bus.Object("org.kde.KWin", dbus.ObjectPath(p)).Call("org.kde.kwin.Script.run", 0).Err; err == nil {
			return nil
		}
	}
	return fmt.Errorf("KWin: could not start script %d", id)
}

func kwinScriptLoaded() bool {
	var loaded bool
	err := bus.Object("org.kde.KWin", "/Scripting").Call("org.kde.kwin.Scripting.isScriptLoaded", 0, kwinScript).Store(&loaded)
	return err == nil && loaded
}

var kwinErr string

// startWindowWatch installs the active-window script. Called again from
// housekeeping when KWin has lost it (KWin restarted).
func startWindowWatch() {
	if bus == nil {
		return
	}
	if err := runKWinScript(kwinScript, kwinScriptJS); err != nil {
		if kwinErr != err.Error() {
			logf("active window: %v (only the global profile will apply)", err)
		}
		kwinErr = err.Error()
		return
	}
	kwinErr = ""
	logf("active window: KWin script running")
}

func stopWindowWatch() {
	if bus != nil {
		bus.Object("org.kde.KWin", "/Scripting").Call("org.kde.kwin.Scripting.unloadScript", 0, kwinScript)
	}
}

// raiseSettingsWindow asks KWin to bring the settings window to the front.
func raiseSettingsWindow() {
	if bus == nil {
		return
	}
	if err := runKWinScript(kwinRaiseTmp, fmt.Sprintf(kwinRaiseJS, uiTitle, uiTitle+" ")); err != nil {
		logf("raise settings: %v", err)
	}
	go func() {
		time.Sleep(2 * time.Second)
		bus.Object("org.kde.KWin", "/Scripting").Call("org.kde.kwin.Scripting.unloadScript", 0, kwinRaiseTmp)
	}()
}

// isUIWindow reports whether a window is the settings page itself: its app
// window is titled exactly uiTitle; as a browser tab it is "Anticater Knob - Browser".
func isUIWindow(title string) bool {
	return title == uiTitle || strings.HasPrefix(title, uiTitle+" - ") || strings.HasPrefix(title, uiTitle+" — ")
}
