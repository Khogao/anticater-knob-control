package main

// The app's logo: the same AnticaterKnob.ico as the Windows app (generated
// from assets/logo.svg), embedded here as anticater-knob.ico. It holds PNG
// images from 16 to 256 px.
//
// It is used three ways: as the tray icon (sent as pixels, so no icon theme
// lookup is involved), as the settings page's favicon (Chrome shows it as the
// app window's icon), and installed under ~/.local/share/icons for the menu
// entry, the autostart entry and notifications ("anticater-knob").

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed anticater-knob.ico
var iconICO []byte

type icoImage struct {
	size int
	png  []byte
}

// icoImages lists the PNG images inside the .ico.
func icoImages() []icoImage {
	d := iconICO
	if len(d) < 6 {
		return nil
	}
	n := int(binary.LittleEndian.Uint16(d[4:]))
	var out []icoImage
	for i := 0; i < n && 6+16*(i+1) <= len(d); i++ {
		e := d[6+16*i:]
		size := int(e[0])
		if size == 0 {
			size = 256
		}
		length := int(binary.LittleEndian.Uint32(e[8:]))
		off := int(binary.LittleEndian.Uint32(e[12:]))
		if off+length > len(d) || !bytes.HasPrefix(d[off:], []byte("\x89PNG")) {
			continue
		}
		out = append(out, icoImage{size, d[off : off+length]})
	}
	return out
}

// sniPixmap is one tray icon image: width, height and ARGB32 pixels in network byte order.
type sniPixmap struct {
	W, H int32
	Data []byte
}

// trayPixmaps decodes the .ico's images up to 128 px for the tray.
func trayPixmaps() []sniPixmap {
	var out []sniPixmap
	for _, im := range icoImages() {
		if im.size > 128 {
			continue
		}
		img, err := png.Decode(bytes.NewReader(im.png))
		if err != nil {
			continue
		}
		out = append(out, toARGB(img))
	}
	return out
}

func toARGB(img image.Image) sniPixmap {
	b := img.Bounds()
	p := sniPixmap{W: int32(b.Dx()), H: int32(b.Dy()), Data: make([]byte, 0, 4*b.Dx()*b.Dy())}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA() // premultiplied, 16-bit
			un := func(c uint32) byte {
				if a == 0 {
					return 0
				}
				return byte(c * 0xffff / a >> 8)
			}
			p.Data = append(p.Data, byte(a>>8), un(r), un(g), un(bl))
		}
	}
	return p
}

// installIcons writes the logo into the user's icon theme folder
// (~/.local/share/icons/hicolor/<N>x<N>/apps/anticater-knob.png) when it is
// missing or different, so desktop entries and notifications find it.
func installIcons() error {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	changed := false
	for _, im := range icoImages() {
		p := filepath.Join(base, "icons", "hicolor", fmt.Sprintf("%dx%d", im.size, im.size), "apps", "anticater-knob.png")
		if old, err := os.ReadFile(p); err == nil && bytes.Equal(old, im.png) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, im.png, 0o644); err != nil {
			return err
		}
		changed = true
	}
	// An older install put an SVG of the previous artwork here; it would win over the PNGs.
	os.Remove(filepath.Join(base, "icons", "hicolor", "scalable", "apps", "anticater-knob.svg"))
	if changed {
		logf("icons installed in %s", filepath.Join(base, "icons", "hicolor"))
	}
	return nil
}

// settingsWindowClass is the class Chrome gives the settings app window: it is
// made from the address the window was opened with (the launch page, see
// launchPage), e.g. chrome-__home_phi_.cache_anticater-knob_launch_settings.html-Default.
func settingsWindowClass() string {
	return "chrome-_" + strings.ReplaceAll(launchPagePath(), "/", "_") + "-Default"
}

// installSettingsEntry writes a hidden desktop entry that ties the settings
// window's class to the app's icon, so the Plasma taskbar shows the logo for
// that window instead of a generic browser icon.
func installSettingsEntry() error {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	// Named after the window class itself: on Wayland Plasma first looks for a
	// desktop file whose name is the window's app id.
	os.Remove(filepath.Join(base, "applications", "anticater-knob-settings.desktop")) // older name
	p := filepath.Join(base, "applications", settingsWindowClass()+".desktop")
	entry := "[Desktop Entry]\nType=Application\nName=Anticater Knob\nIcon=anticater-knob\nExec=anticater-knob\nNoDisplay=true\nStartupWMClass=" + settingsWindowClass() + "\n"
	if old, err := os.ReadFile(p); err == nil && string(old) == entry {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(entry), 0o644); err != nil {
		return err
	}
	// Plasma reads desktop files from a cache; rebuild it so the entry counts now.
	for _, tool := range []string{"kbuildsycoca6", "kbuildsycoca5"} {
		if path, err := exec.LookPath(tool); err == nil {
			exec.Command(path).Run()
			break
		}
	}
	return nil
}
