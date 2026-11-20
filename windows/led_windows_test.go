package main

import "testing"

func TestVivid(t *testing.T) {
	if c, f := vivid("FFFFFF"); c != "FFFFFF" || f != 1 {
		t.Errorf("white: %s %v", c, f)
	}
	// Excel's green becomes a pure green (red and blue equal), dimmer than Zalo's bright blue.
	c, fExcel := vivid("217346")
	if c[0:2] != c[4:6] || c[2:4] != "FF" {
		t.Errorf("Excel green: %s", c)
	}
	if _, fZalo := vivid("0068FF"); fZalo <= fExcel {
		t.Errorf("Zalo %v should be brighter than Excel %v", fZalo, fExcel)
	}
	// Same hue, different brands must not give the same light.
	w, fw := vivid("2B579A")
	z, fz := vivid("0068FF")
	if w == z && fw == fz {
		t.Errorf("Word and Zalo give the same light %s %v", w, fw)
	}
}
