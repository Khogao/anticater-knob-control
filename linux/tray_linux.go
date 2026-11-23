package main

// Icon in the Plasma system tray (StatusNotifierItem over D-Bus). Clicking it,
// left or right, opens the settings window; everything else (pause, language,
// start with the system, diagnostics) is on that page. The tooltip shows the
// last gesture (refreshed at most once a second, from housekeeping).

import (
	"fmt"
	"os"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	sniPath  = "/StatusNotifierItem"
	sniIface = "org.kde.StatusNotifierItem"
)

var trayProps *prop.Properties

type sniItem struct{}

func (sniItem) Activate(x, y int32) *dbus.Error          { go openUI(); return nil }
func (sniItem) SecondaryActivate(x, y int32) *dbus.Error { go openUI(); return nil }
func (sniItem) ContextMenu(x, y int32) *dbus.Error       { go openUI(); return nil }
func (sniItem) Scroll(delta int32, orientation string) *dbus.Error {
	return nil
}

type toolTip struct {
	IconName    string
	Pixmaps     []sniPixmap
	Title, Text string
}

var trayIcon = trayPixmaps()

func trayToolTip() toolTip {
	return toolTip{Pixmaps: trayIcon, Title: "Anticater Knob", Text: L("Gần nhất: ", "Last: ") + lastEventText}
}

func trayStart() {
	if bus == nil {
		return
	}
	name := fmt.Sprintf("org.kde.StatusNotifierItem-%d-1", os.Getpid())
	if _, err := bus.RequestName(name, dbus.NameFlagDoNotQueue); err != nil {
		logf("tray: %v", err)
		return
	}
	if err := bus.Export(sniItem{}, sniPath, sniIface); err != nil {
		logf("tray: %v", err)
		return
	}
	str := func(v any) *prop.Prop { return &prop.Prop{Value: v, Writable: false, Emit: prop.EmitTrue} }
	props, err := prop.Export(bus, sniPath, prop.Map{sniIface: {
		"Category": str("Hardware"),
		"Id":       str("anticater-knob"),
		"Title":    str("Anticater Knob"),
		"Status":   str("Active"),
		"WindowId": str(int32(0)),
		// The logo by name (from ~/.local/share/icons) and as pixels.
		"IconName":           str("anticater-knob"), // installed by installIcons; IconPixmap is the fallback
		"IconPixmap":         str(trayIcon),
		"IconThemePath":      str(""),
		"OverlayIconName":    str(""),
		"AttentionIconName":  str(""),
		"AttentionMovieName": str(""),
		"ItemIsMenu":         str(false),
		"Menu":               str(dbus.ObjectPath("/NO_DBUSMENU")),
		"ToolTip":            str(trayToolTip()),
	}})
	if err != nil {
		logf("tray: %v", err)
		return
	}
	trayProps = props
	trayName = name
	trayEnsure()
}

var (
	trayName       string
	trayRegistered bool
)

// trayEnsure registers the icon with the tray when the tray does not list it:
// at start, and again from the main loop every few seconds, so the icon comes
// back after plasmashell restarts or when the tray started after this app.
func trayEnsure() {
	if trayProps == nil {
		return
	}
	watcher := bus.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher")
	if v, err := watcher.GetProperty("org.kde.StatusNotifierWatcher.RegisteredStatusNotifierItems"); err == nil {
		if items, ok := v.Value().([]string); ok {
			for _, it := range items {
				if it == trayName || strings.HasPrefix(it, trayName+"/") {
					return
				}
			}
		}
	}
	err := watcher.Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem", 0, trayName).Err
	if err != nil {
		if trayRegistered || trayErr == "" {
			logf("tray: no system tray to register with: %v", err)
		}
		trayErr, trayRegistered = err.Error(), false
		return
	}
	if !trayRegistered {
		logf("tray: icon registered")
	}
	trayErr, trayRegistered = "", true
}

var trayErr string

// trayRefresh updates the tooltip; mu is held.
func trayRefresh() {
	if trayProps == nil {
		return
	}
	trayProps.SetMust(sniIface, "ToolTip", trayToolTip())
	bus.Emit(sniPath, sniIface+".NewToolTip")
}
