package main

// Sending the colour to the knob.
//
// The command is the vendor's (read from its ANTICATER app, see
// windows/knob-led.ps1): 03 FD 00 06 R G B BRIGHTNESS ON, padded with zeros.
// One colour for the whole knob.
//
//   - Over Bluetooth it is written, without response, to characteristic AE41
//     of the knob's service AE40, through BlueZ on the system D-Bus.
//   - Over USB or the 2.4G dongle it is written as a HID output report to the
//     knob's /dev/hidrawN. The packet's first byte, 03, is the report id.
//
// Sends run on their own goroutine; only the newest colour is kept when the
// knob is slower than the app switches.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

var (
	ledLast        string // last line sent (mu)
	ledPreviewLine string // colour shown while the settings window is in front (mu)
	ledResults     []string
	ledVia         string // how the last send went out, for the diagnostic
	ledQueue       = make(chan string, 1)
	ledStarted     bool
)

// ledPreview handles the settings page's "ledPreview" command; mu is held.
func ledPreview(arg string) {
	if cfg == nil || cfg.LED == nil {
		return
	}
	ledPreviewLine = ""
	switch {
	case strings.HasPrefix(arg, "key:"):
		exe, title, _ := strings.Cut(strings.TrimPrefix(arg, "key:"), "|")
		c, f := cfg.LED.pick(exe, title)
		ledPreviewLine = cfg.LED.lineAt(c, f)
	case strings.HasPrefix(arg, "colour:"):
		if c := strings.TrimPrefix(strings.TrimPrefix(arg, "colour:"), "#"); ledColorRe.MatchString(c) {
			ledPreviewLine = cfg.LED.lineAt(c, 1)
		}
	}
	ledLast = ""
	ledUpdate()
}

// ledUpdate sends the colour for the window in front if it changed; mu is held.
func ledUpdate() {
	if cfg == nil || cfg.LED == nil || paused {
		return
	}
	exe, title := fgClass, fgCaption
	switch {
	case isUIWindow(fgCaption):
		exe = "settings"
	case fgKind == "desktop":
		exe = "desktop"
	case fgKind != "normal" || exe == "":
		return // panel, launcher, OSD, notifications: keep the colour of the app before
	}
	line := cfg.LED.colour(exe, title)
	if exe == "settings" && ledPreviewLine != "" {
		line = ledPreviewLine
	}
	if line == "" || line == ledLast {
		return
	}
	ledLast = line
	logf("led %s for %s", line, exe)
	if !ledStarted {
		ledStarted = true
		go ledWorker()
	}
	select {
	case <-ledQueue: // drop the older colour still waiting
	default:
	}
	ledQueue <- line
}

func ledWorker() {
	for line := range ledQueue {
		via, err := ledSend(line)
		mu.Lock()
		if err != nil {
			ledResults = append(ledResults, "error "+line+": "+err.Error())
			ledLast = "" // try again on the next change
		} else {
			ledVia = via
		}
		mu.Unlock()
	}
}

// ledHousekeeping logs send errors; mu is held.
func ledHousekeeping() {
	for _, r := range ledResults {
		logf("led: %s", r)
	}
	ledResults = nil
}

func ledStop() {}

func ledStatus() string {
	if cfg.LED == nil {
		return "off in the config"
	}
	if ledVia == "" {
		return "nothing sent successfully yet; last colour " + orDash(ledLast)
	}
	return "last sent " + orDash(ledLast) + " via " + ledVia
}

// ledPacket is the vendor's colour command for a line "RRGGBB brightness" or "off".
func ledPacket(line string) ([]byte, error) {
	pkt := make([]byte, 64)
	pkt[0], pkt[1], pkt[2], pkt[3] = 0x03, 0xFD, 0x00, 0x06
	if line == "off" {
		return pkt, nil // colour 000000, brightness 0, off
	}
	var r, g, b, bright int
	if _, err := fmt.Sscanf(line, "%02X%02X%02X %d", &r, &g, &b, &bright); err != nil {
		return nil, fmt.Errorf("bad colour line %q", line)
	}
	pkt[4], pkt[5], pkt[6], pkt[7], pkt[8] = byte(r), byte(g), byte(b), byte(bright), 1
	return pkt, nil
}

func ledSend(line string) (string, error) {
	pkt, err := ledPacket(line)
	if err != nil {
		return "", err
	}
	var errs []string
	if via, err := ledSendBluetooth(pkt); err == nil {
		return via, nil
	} else if !errors.Is(err, errNoKnob) {
		errs = append(errs, err.Error())
	}
	if via, err := ledSendHidraw(pkt); err == nil {
		return via, nil
	} else if !errors.Is(err, errNoKnob) {
		errs = append(errs, err.Error())
	}
	if len(errs) == 0 {
		return "", errors.New("knob not found (neither a Bluetooth knob with service AE40 nor a USB / dongle knob)")
	}
	return "", errors.New(strings.Join(errs, "; "))
}

var errNoKnob = errors.New("no knob")

// ---- Bluetooth (BlueZ) --------------------------------------------------------

var (
	sysBus     *dbus.Conn
	btCharPath dbus.ObjectPath
)

func ledSendBluetooth(pkt []byte) (string, error) {
	if sysBus == nil {
		c, err := dbus.ConnectSystemBus()
		if err != nil {
			return "", errNoKnob
		}
		sysBus = c
	}
	if btCharPath == "" {
		p, err := findAE41()
		if err != nil {
			return "", err
		}
		btCharPath = p
	}
	err := sysBus.Object("org.bluez", btCharPath).Call("org.bluez.GattCharacteristic1.WriteValue", 0,
		pkt, map[string]dbus.Variant{"type": dbus.MakeVariant("command")}).Err
	if err != nil {
		btCharPath = "" // look it up again next time (knob reconnected)
		return "", fmt.Errorf("bluetooth write: %v", err)
	}
	return "Bluetooth " + string(btCharPath), nil
}

// findAE41 finds characteristic AE41 of service AE40 on a connected device
// named ANTICATER*.
func findAE41() (dbus.ObjectPath, error) {
	var objs map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	if err := sysBus.Object("org.bluez", "/").Call("org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objs); err != nil {
		return "", errNoKnob
	}
	str := func(v dbus.Variant) string { s, _ := v.Value().(string); return s }
	for path, ifaces := range objs {
		ch, ok := ifaces["org.bluez.GattCharacteristic1"]
		if !ok || !strings.HasPrefix(strings.ToLower(str(ch["UUID"])), "0000ae41-") {
			continue
		}
		// .../dev_XX/serviceNNNN/charNNNN: check the device's name and connection.
		devPath := dbus.ObjectPath(filepath.Dir(filepath.Dir(string(path))))
		dev := objs[devPath]["org.bluez.Device1"]
		name := strings.ToUpper(str(dev["Name"]) + str(dev["Alias"]))
		connected, _ := dev["Connected"].Value().(bool)
		if connected && strings.Contains(name, "ANTICATER") {
			return path, nil
		}
	}
	return "", errNoKnob
}

// ---- USB / 2.4G dongle (hidraw) ---------------------------------------------------

type hidrawNode struct {
	path   string
	id     byte // output report id
	length int  // bytes to write, report id included
}

var hidNode *hidrawNode

func ledSendHidraw(pkt []byte) (string, error) {
	if hidNode == nil {
		n, err := findHidraw()
		if err != nil {
			return "", err
		}
		hidNode = n
	}
	n := hidNode
	buf := make([]byte, n.length)
	if n.id == pkt[0] {
		copy(buf, pkt) // the command's 03 is the report id
	} else {
		buf[0] = n.id
		copy(buf[1:], pkt)
	}
	f, err := os.OpenFile(n.path, os.O_WRONLY, 0)
	if err != nil {
		hidNode = nil
		if errors.Is(err, os.ErrPermission) {
			return "", fmt.Errorf("%s: no permission (run install.sh, then unplug and replug the knob)", n.path)
		}
		return "", fmt.Errorf("%s: %v", n.path, err)
	}
	defer f.Close()
	f.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := f.Write(buf); err != nil {
		hidNode = nil
		return "", fmt.Errorf("%s: %v", n.path, err)
	}
	return fmt.Sprintf("%s (report %02X, %d bytes)", n.path, n.id, n.length), nil
}

// findHidraw picks the knob's hidraw node that has a vendor-defined output
// report, and that report's id and size, from the HID report descriptor.
func findHidraw() (*hidrawNode, error) {
	dirs, _ := filepath.Glob("/sys/class/hidraw/hidraw*")
	for _, d := range dirs {
		ue, _ := os.ReadFile(filepath.Join(d, "device", "uevent"))
		var bus, vendor, product uint32
		for _, l := range strings.Split(string(ue), "\n") {
			if v, ok := strings.CutPrefix(l, "HID_ID="); ok {
				fmt.Sscanf(v, "%x:%x:%x", &bus, &vendor, &product)
			}
		}
		if cfg == nil || !cfg.matchesDevice(uint16(vendor), uint16(product)) {
			continue
		}
		desc, err := os.ReadFile(filepath.Join(d, "device", "report_descriptor"))
		if err != nil {
			continue
		}
		if id, size, ok := vendorOutputReport(desc); ok {
			return &hidrawNode{path: "/dev/" + filepath.Base(d), id: id, length: 1 + size}, nil
		}
	}
	return nil, errNoKnob
}

// vendorOutputReport walks a HID report descriptor and returns the id and
// byte size of the first Output report in a vendor-defined usage page
// (0xFF00 and up). Output reports elsewhere (keyboard LEDs) are skipped.
func vendorOutputReport(d []byte) (id byte, size int, ok bool) {
	var usagePage uint32
	var reportSize, reportCount uint32
	var reportID byte
	sizes := map[byte]int{}
	var order []byte
	for i := 0; i < len(d); {
		b := d[i]
		if b == 0xFE { // long item
			if i+1 >= len(d) {
				break
			}
			i += 3 + int(d[i+1])
			continue
		}
		n := int(b & 3)
		if n == 3 {
			n = 4
		}
		if i+1+n > len(d) {
			break
		}
		var v uint32
		switch n {
		case 1:
			v = uint32(d[i+1])
		case 2:
			v = uint32(binary.LittleEndian.Uint16(d[i+1:]))
		case 4:
			v = binary.LittleEndian.Uint32(d[i+1:])
		}
		switch b & 0xFC {
		case 0x04: // Usage Page
			usagePage = v
		case 0x74: // Report Size
			reportSize = v
		case 0x94: // Report Count
			reportCount = v
		case 0x84: // Report ID
			reportID = byte(v)
		case 0x90: // Output
			if usagePage >= 0xFF00 {
				if _, seen := sizes[reportID]; !seen {
					order = append(order, reportID)
				}
				sizes[reportID] += int(reportSize * reportCount / 8)
			}
		}
		i += 1 + n
	}
	if len(order) == 0 {
		return 0, 0, false
	}
	// Prefer report 03, the id the vendor command starts with.
	if s, has := sizes[0x03]; has {
		return 0x03, s, true
	}
	return order[0], sizes[order[0]], true
}
