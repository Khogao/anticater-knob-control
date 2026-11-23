package main

// Icon in the Plasma system tray (StatusNotifierItem over D-Bus). Clicking it,
// left or right, opens the settings window; everything else (pause, language,
// start with the system, diagnostics) is on that page. The tooltip shows the
// last gesture.

import (
	"fmt"
	"os"
	"path/filepath"

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
	IconName string
	Pixmaps  []struct {
		W, H int32
		Data []byte
	}
	Title, Text string
}

func trayToolTip() toolTip {
	return toolTip{IconName: "anticater-knob", Title: "Anticater Knob", Text: L("Gần nhất: ", "Last: ") + lastEventText}
}

// iconDir holds the app's icon as <dir>/hicolor/scalable/apps/anticater-knob.svg
// so the tray can find it through IconThemePath without an installed icon.
func iconDir() string {
	dir := filepath.Join(cacheDir(), "icons")
	p := filepath.Join(dir, "hicolor", "scalable", "apps", "anticater-knob.svg")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, iconSVG, 0o644)
	return dir
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
		"Category":           str("Hardware"),
		"Id":                 str("anticater-knob"),
		"Title":              str("Anticater Knob"),
		"Status":             str("Active"),
		"WindowId":           str(int32(0)),
		"IconName":           str("anticater-knob"),
		"IconThemePath":      str(iconDir()),
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
	err = bus.Object("org.kde.StatusNotifierWatcher", "/StatusNotifierWatcher").
		Call("org.kde.StatusNotifierWatcher.RegisterStatusNotifierItem", 0, name).Err
	if err != nil {
		logf("tray: no system tray to register with: %v", err)
	}
}

// trayRefresh updates the tooltip; mu is held.
func trayRefresh() {
	if trayProps == nil {
		return
	}
	trayProps.SetMust(sniIface, "ToolTip", trayToolTip())
	bus.Emit(sniPath, sniIface+".NewToolTip")
}
