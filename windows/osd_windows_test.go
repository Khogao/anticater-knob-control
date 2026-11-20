package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// Writes what the brightness flyout looks like, light and dark, at 150 %:
//
//	KNOB_OSD_PNG=osd.png go test ./windows -run OSDPreview
func TestOSDPreview(t *testing.T) {
	out := os.Getenv("KNOB_OSD_PNG")
	if out == "" {
		t.Skip("set KNOB_OSD_PNG to a file name to write a preview")
	}
	scale := 1.5
	w, h := int(osdWidth*scale), int(osdHeight*scale)
	img := image.NewRGBA(image.Rect(0, 0, w+40, 2*h+60))
	for i := range img.Pix {
		img.Pix[i] = 0x80 // grey page behind, to show the edges
	}
	for row, dark := range []bool{false, true} {
		px := osdRender(w, h, scale, 60, dark, accentColor())
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				i := (y*w + x) * 4
				b, g, r, a := px[i], px[i+1], px[i+2], px[i+3]
				bg := img.RGBAAt(x+20, y+20+row*(h+20))
				blend := func(c, under byte) byte { return byte(int(c) + int(under)*(255-int(a))/255) }
				img.SetRGBA(x+20, y+20+row*(h+20), color.RGBA{blend(r, bg.R), blend(g, bg.G), blend(b, bg.B), 255})
			}
		}
	}
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
