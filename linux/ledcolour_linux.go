package main

// The colour of the knob's lights follows the app in front, as set in the
// "led" section of the config (same format as the Windows app):
//
//	"led": { "brightness": 45, "auto": true,
//	         "apps": { "desktop": "FFFFFF", "google-chrome": "FF0000" } }
//
// Keys in "apps" match like profile keys: a window class, or "class|text".
// "desktop" is the Plasma desktop; "settings" this app's settings window.
//
// The colour logic below is copied from windows/led_windows.go; how the
// command reaches the knob is in led_linux.go (ledPacket, ledSend).

import (
	"fmt"
	"regexp"
	"strings"
)

type LEDConfig struct {
	Brightness int  `json:"brightness"`
	Auto       bool `json:"auto,omitempty"`
	// Green scales the green part of strong colours (0.3..1, default 0.45, set on the test knob);
	// white and pale colours get only a fifth of the cut.
	// The knob's board is green and tints the light green: orange looked
	// yellow-green on the test knob.
	Green  float64           `json:"green,omitempty"`
	Global string            `json:"global,omitempty"`
	Apps   map[string]string `json:"apps,omitempty"`
}

var ledColorRe = regexp.MustCompile(`^(?i:[0-9a-f]{6}|off)$`)

func (l *LEDConfig) validate() error {
	if l.Brightness < 0 || l.Brightness > 100 {
		return fmt.Errorf("led.brightness must be 0..100, not %d", l.Brightness)
	}
	if l.Brightness == 0 {
		l.Brightness = 45
	}
	if l.Green == 0 {
		l.Green = 0.45
	}
	if l.Green < 0.3 || l.Green > 1 {
		return fmt.Errorf("led.green must be 0.3..1, not %v", l.Green)
	}
	check := func(where, c string) error {
		if c != "" && !ledColorRe.MatchString(strings.TrimPrefix(c, "#")) {
			return fmt.Errorf("%s: colour must be RRGGBB or \"off\", not %q", where, c)
		}
		return nil
	}
	if err := check("led.global", l.Global); err != nil {
		return err
	}
	for k, c := range l.Apps {
		if err := check("led.apps."+k, c); err != nil {
			return err
		}
	}
	return nil
}

// colour returns the colour line ("RRGGBB brightness" or "off") for this app, or "" to leave the
// lights as they are.
func (l *LEDConfig) colour(exe, title string) string {
	c, factor := l.pick(exe, title)
	return l.lineAt(c, factor)
}

// pick is the colour an app gets, before the correction for the knob's green
// board, and the brightness factor; "" leaves the lights alone. The settings
// page shows it as the app's current colour.
func (l *LEDConfig) pick(exe, title string) (string, float64) {
	lexe, ltitle := strings.ToLower(exe), strings.ToLower(title)
	if lexe == "desktop" {
		if c, ok := l.Apps["desktop"]; ok {
			return c, 1
		}
		return l.Global, 1
	}
	if lexe == "settings" {
		if c, ok := l.Apps["settings"]; ok {
			return c, 1
		}
		return "FF9124", 1 // the app's logo
	}
	best, bestLen := "", -1
	for key := range l.Apps {
		name, want, hasTitle := strings.Cut(key, "|")
		if lexe == "" || strings.ToLower(strings.TrimSpace(name)) != lexe {
			continue
		}
		score := 0
		if hasTitle {
			want = strings.ToLower(strings.TrimSpace(want))
			if want == "" || !strings.Contains(ltitle, want) {
				continue
			}
			score = 1 + len(want)
		}
		if score > bestLen || (score == bestLen && key < best) {
			best, bestLen = key, score
		}
	}
	switch {
	case best != "":
		return l.Apps[best], 1
	case l.Auto:
		return autoColour(lexe)
	}
	return l.Global, 1
}

// lineAt is the colour line that ledPacket turns into the command, with the
// brightness scaled by factor.
func (l *LEDConfig) lineAt(c string, factor float64) string {
	c = strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(c), "#"))
	switch c {
	case "":
		return ""
	case "OFF":
		return "off"
	}
	var r, g, b int
	if _, err := fmt.Sscanf(c, "%02X%02X%02X", &r, &g, &b); err == nil && l.Green > 0 {
		// Full green cut for strong colours, less for white and pale ones, which
		// also need some blue taken out: on the test knob white sent as FFBFFF
		// looked purple and as FFE3FF turquoise (its red light is the weakest).
		sat := 0.0
		if mx := max(r, g, b); mx > 0 {
			sat = float64(mx-min(r, g, b)) / float64(mx)
		}
		kg := l.Green + (1-l.Green)*(1-sat)*0.65
		kb := 1 - 0.3*(1-sat)
		c = fmt.Sprintf("%02X%02X%02X", r, int(float64(g)*kg+0.5), int(float64(b)*kb+0.5))
	}
	return fmt.Sprintf("%s %d", c, max(5, int(float64(l.Brightness)*factor+0.5)))
}

// brandColours are the colours people know these apps by.
var brandColours = map[string]string{
	// Microsoft Office and Windows
	"winword.exe": "2B579A", "excel.exe": "217346", "powerpnt.exe": "D24726", "outlook.exe": "0078D4",
	"onenote.exe": "7719AA", "msaccess.exe": "A4373A", "visio.exe": "3955A3", "winproj.exe": "31752F",
	"mspub.exe": "077568", "ms-teams.exe": "6264A7", "teams.exe": "6264A7", "notepad.exe": "FFFFFF",
	"windowsterminal.exe": "FFFFFF", "mspaint.exe": "0078D4", "taskmgr.exe": "0078D4",
	// Office alternatives and PDF
	"wps.exe": "C9282D", "et.exe": "2E8B57", "wpp.exe": "D24726", "acrobat.exe": "EC1C24", "acrord32.exe": "EC1C24",
	"foxitpdfreader.exe": "F04D23", "foxitpdfeditor.exe": "F04D23",
	// Chat
	"zalo.exe": "0068FF", "whatsapp.exe": "25D366", "whatsapp.root.exe": "25D366", "telegram.exe": "26A5E4",
	"messenger.exe": "0084FF", "skype.exe": "00AFF0", "viber.exe": "7360F2", "line.exe": "06C755",
	"wechat.exe": "07C160", "weixin.exe": "07C160", "slack.exe": "4A154B", "discord.exe": "5865F2", "zoom.exe": "2D8CFF",
	// Browsers other than Edge and Chrome
	"firefox.exe": "FF7139", "brave.exe": "FB542B", "opera.exe": "FF1B2D", "coccoc.exe": "1DB954", "vivaldi.exe": "EF3939",
	// CAD and 3D
	"acad.exe": "E51937", "revit.exe": "0696D7", "3dsmax.exe": "37A5CC", "sketchup.exe": "005F9E",
	"rhino.exe": "8A2BE2", "blender.exe": "F5792A", "inventor.exe": "F2A900", "civil3d.exe": "E51937",
	// Adobe and design
	"photoshop.exe": "31A8FF", "illustrator.exe": "FF9A00", "indesign.exe": "FF3366", "adobe premiere pro.exe": "9999FF",
	"afterfx.exe": "9999FF", "lightroom.exe": "31A8FF", "figma.exe": "A259FF", "canva.exe": "00C4CC",
	"resolve.exe": "FF5A36", "capcut.exe": "FFFFFF",
	// Code
	"code.exe": "007ACC", "cursor.exe": "FFFFFF", "devenv.exe": "5C2D91", "idea64.exe": "FE315D", "pycharm64.exe": "21D789",
	"notepad++.exe": "90E59A", "sublime_text.exe": "FF9800", "githubdesktop.exe": "6E40C9", "postman.exe": "FF6C37",
	"claude.exe": "D97757", "chatgpt.exe": "10A37F",
	// Linux window classes (KWin resourceClass, lower case)
	"google-chrome": "FF0000", "chromium": "4285F4", "microsoft-edge": "00A7E1", "firefox": "FF7139",
	"org.mozilla.firefox": "FF7139", "brave-browser": "FB542B", "vivaldi-stable": "EF3939", "code": "007ACC",
	"codium": "2F80ED", "cursor": "FFFFFF", "org.kde.dolphin": "FFC800", "org.kde.konsole": "FFFFFF",
	"org.kde.okular": "3DAEE9", "org.kde.gwenview": "3DAEE9", "org.kde.kate": "3DAEE9", "org.kde.systemsettings": "3DAEE9",
	"libreoffice-writer": "2B579A", "libreoffice-calc": "217346", "libreoffice-impress": "D24726",
	"soffice": "18A303", "org.telegram.desktop": "26A5E4", "telegramdesktop": "26A5E4", "slack": "4A154B",
	"discord": "5865F2", "zoom": "2D8CFF", "spotify": "1DB954", "obsidian": "7C3AED", "blender": "F5792A",
	"gimp": "8C7A64", "org.inkscape.inkscape": "FFFFFF", "krita": "3BABFF", "vlc": "FF8800", "mpv": "8E44AD",
	"org.kde.haruna": "3DAEE9", "steam": "1A9FFF", "thunderbird": "0A84FF", "claude": "D97757",
	// Notes, media, other
	"obsidian.exe": "7C3AED", "notion.exe": "FFFFFF", "evernote.exe": "00A82D", "spotify.exe": "1DB954",
	"vlc.exe": "FF8800", "potplayermini64.exe": "FFCC00", "steam.exe": "1A9FFF", "tradingview.exe": "2962FF",
}

// autoColour picks a fixed colour for an app, and a factor for the light's
// brightness. Apps of the same hue (Word, Zalo, Photoshop are all blue) still
// differ: the colour keeps how deep the brand colour is and the brightness
// follows how light it is, so a dark navy brand gives a dimmer, deeper light
// than a bright sky-blue one. Unknown apps get a hue, depth and brightness
// made from a hash of their exe name, the same every time.
func autoColour(lexe string) (string, float64) {
	if c, ok := brandColours[lexe]; ok {
		return vivid(c)
	}
	h := uint32(2166136261)
	for i := 0; i < len(lexe); i++ {
		h = (h ^ uint32(lexe[i])) * 16777619
	}
	sat := 0.7 + 0.3*float64(h>>9%7)/6
	r, g, b := hsvToRGB(float64(h%360), sat, 1)
	return fmt.Sprintf("%02X%02X%02X", r, g, b), 0.4 + 0.6*float64(h>>17%7)/6
}

// vivid keeps a brand colour's hue and returns a colour the knob can show,
// with a brightness factor. The lights wash out muted colours (Claude's
// #D97757 showed nearly white), so the depth is kept between 65 % and full
// instead of copied, and the colour is drawn at full value; how dark the
// brand colour is goes into the brightness factor (35 % to full) instead.
// Grey and white (no hue) stay as they are, at full brightness.
func vivid(hex string) (string, float64) {
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b); err != nil {
		return hex, 1
	}
	mx, mn := max(r, g, b), min(r, g, b)
	if mx-mn < 24 {
		return hex, 1
	}
	var h float64
	d := float64(mx - mn)
	switch mx {
	case r:
		h = mod(float64(g-b)/d+6, 6) * 60
	case g:
		h = (float64(b-r)/d + 2) * 60
	default:
		h = (float64(r-g)/d + 4) * 60
	}
	// The knob's blue light is strong: a green with any blue in it (Excel's
	// #217346 sits at hue 146) shows as turquoise. Brand greens become pure green.
	if h >= 90 && h <= 165 {
		h = 120
	}
	s := d / float64(mx)
	v := float64(mx) / 255
	R, G, B := hsvToRGB(h, 0.65+0.35*s, 1)
	return fmt.Sprintf("%02X%02X%02X", R, G, B), 0.35 + 0.65*v
}
func hsvToRGB(h, s, v float64) (byte, byte, byte) {
	c := v * s
	x := c * (1 - abs(mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g = c, x
	case h < 120:
		r, g = x, c
	case h < 180:
		g, b = c, x
	case h < 240:
		g, b = x, c
	case h < 300:
		r, b = x, c
	default:
		r, b = c, x
	}
	return byte((r + m) * 255), byte((g + m) * 255), byte((b + m) * 255)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func mod(a, b float64) float64 { return a - b*float64(int(a/b)) }
