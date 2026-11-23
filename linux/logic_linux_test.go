package main

import (
	"encoding/json"
	"testing"
)

func TestLEDPacket(t *testing.T) {
	p, err := ledPacket("00A7E1 45")
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0x03, 0xFD, 0x00, 0x06, 0x00, 0xA7, 0xE1, 45, 1}
	if len(p) != 64 {
		t.Fatalf("len %d", len(p))
	}
	for i, b := range want {
		if p[i] != b {
			t.Fatalf("byte %d = %02X, want %02X", i, p[i], b)
		}
	}
	off, _ := ledPacket("off")
	if off[4]|off[5]|off[6]|off[7]|off[8] != 0 {
		t.Fatalf("off packet % X", off[:9])
	}
}

func TestVendorOutputReport(t *testing.T) {
	// Keyboard collection with an LED output report, then a vendor collection
	// (usage page FF00) with report id 3: 64 bytes out.
	desc := []byte{
		0x05, 0x01, 0x09, 0x06, 0xA1, 0x01, 0x85, 0x01,
		0x05, 0x08, 0x75, 0x01, 0x95, 0x05, 0x91, 0x02, 0xC0,
		0x06, 0x00, 0xFF, 0x09, 0x01, 0xA1, 0x01, 0x85, 0x03,
		0x75, 0x08, 0x95, 0x40, 0x91, 0x02, 0xC0,
	}
	id, size, ok := vendorOutputReport(desc)
	if !ok || id != 3 || size != 64 {
		t.Fatalf("got id %d size %d ok %v", id, size, ok)
	}
	if _, _, ok := vendorOutputReport(desc[:17]); ok {
		t.Fatal("keyboard LED report taken as vendor report")
	}
}

func TestGestureForKey(t *testing.T) {
	cases := map[uint16]string{
		keyVolumeUp: "rotate_right", keyVolumeDown: "rotate_left", keyMute: "press", keyPlayPause: "press",
		keyBrightnessUp: "press_rotate_right", keyBrightnessDown: "press_rotate_left", keyF13: "press_rotate_right",
	}
	for k, g := range cases {
		if got := gestureForKey(k); got != g {
			t.Errorf("key %d: %q, want %q", k, got, g)
		}
	}
	knobMods[keyLeftShift] = true
	defer delete(knobMods, keyLeftShift)
	if got := gestureForKey(keyF13); got != "press_rotate_left" {
		t.Errorf("shift+F13: %q", got)
	}
}

func TestDefaultConfig(t *testing.T) {
	var c Config
	if err := json.Unmarshal(defaultConfigJSON, &c); err != nil {
		t.Fatal(err)
	}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if a, src := c.resolve("press_rotate_right", "Google-Chrome", "Video - YouTube - Google Chrome"); src != "google-chrome|YouTube" || a.Keys[0] != "right" {
		t.Fatalf("resolve: %v from %s", a, src)
	}
	for _, k := range []string{"ctrl", "shift", "tab", "f13", "a", "0", "volume_up", "brightness_down", "["} {
		if _, ok := lookupKey(k); !ok {
			t.Errorf("key %q unknown", k)
		}
	}
}
