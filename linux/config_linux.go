package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

//go:embed default-config.json
var defaultConfigJSON []byte

type Action struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Keys        []string `json:"keys,omitempty"`
	Dx          int      `json:"dx,omitempty"`
	Dy          int      `json:"dy,omitempty"`
	Button      string   `json:"button,omitempty"`
	Command     string   `json:"command,omitempty"`
	Every       int      `json:"every,omitempty"`
}

type Config struct {
	Help    []string `json:"_help,omitempty"`
	Devices []string `json:"devices"`
	Log     bool     `json:"log"`
	// Language is "vi", "en", or empty to follow the system language ($LANG).
	Language string                       `json:"language,omitempty"`
	Global   map[string]Action            `json:"global"`
	Apps     map[string]map[string]Action `json:"apps"`
	// LED is the colour of the knob's lights per app (led_linux.go).
	LED *LEDConfig `json:"led,omitempty"`
	// BrightnessScreens is used by the Windows app only; kept so a config
	// copied between the two keeps it.
	BrightnessScreens string `json:"brightness_screens,omitempty"`
}

var gestureNames = []string{"rotate_left", "rotate_right", "press", "press_rotate_left", "press_rotate_right"}

// configDir is ~/.config/anticater-knob (or $XDG_CONFIG_HOME/anticater-knob).
func configDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "anticater-knob")
}

func configPath() string { return filepath.Join(configDir(), "knob-config.json") }
func logPath() string    { return filepath.Join(configDir(), "knob.log") }

// loadConfig reads the config, writing the default one on first run.
func loadConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		os.MkdirAll(configDir(), 0o755)
		if werr := os.WriteFile(path, defaultConfigJSON, 0o644); werr != nil {
			return nil, fmt.Errorf("cannot write %s: %v", path, werr)
		}
		data = defaultConfigJSON
	} else if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %v", filepath.Base(path), err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("%s: %v", filepath.Base(path), err)
	}
	return &c, nil
}

func (c *Config) validate() error {
	if len(c.Devices) == 0 {
		c.Devices = []string{"05AC:022C"}
	}
	if c.BrightnessScreens != "" && c.BrightnessScreens != "cursor" && c.BrightnessScreens != "all" {
		return fmt.Errorf("brightness_screens must be \"cursor\" or \"all\", not %q", c.BrightnessScreens)
	}
	if c.LED != nil {
		if err := c.LED.validate(); err != nil {
			return err
		}
	}
	if c.Language != "" && c.Language != "vi" && c.Language != "en" {
		return fmt.Errorf("language must be \"vi\" or \"en\", not %q", c.Language)
	}
	for _, d := range c.Devices {
		if _, _, ok := parseDeviceID(d); !ok {
			return fmt.Errorf("bad device id %q (want VID:PID in hex, e.g. 05AC:022C)", d)
		}
	}
	check := func(where string, m map[string]Action) error {
		for g, a := range m {
			if !isGesture(g) {
				return fmt.Errorf("%s: unknown gesture %q", where, g)
			}
			if err := a.validate(); err != nil {
				return fmt.Errorf("%s.%s: %v", where, g, err)
			}
		}
		return nil
	}
	if err := check("global", c.Global); err != nil {
		return err
	}
	for app, m := range c.Apps {
		if err := check("apps["+app+"]", m); err != nil {
			return err
		}
	}
	return nil
}

func isGesture(s string) bool {
	for _, g := range gestureNames {
		if g == s {
			return true
		}
	}
	return false
}

func (a Action) validate() error {
	switch a.Type {
	case "passthrough", "noop", "":
	case "shortcut", "key":
		if len(a.Keys) == 0 {
			return fmt.Errorf("needs keys")
		}
		for _, k := range a.Keys {
			if _, ok := lookupKey(k); !ok {
				return fmt.Errorf("unknown key %q", k)
			}
		}
	case "scroll":
		if a.Dx == 0 && a.Dy == 0 {
			return fmt.Errorf("scroll needs dx or dy")
		}
		for _, k := range a.Keys {
			if _, ok := lookupKey(k); !ok {
				return fmt.Errorf("unknown key %q", k)
			}
		}
	case "mouse":
		switch strings.ToLower(a.Button) {
		case "", "left", "right", "middle", "center":
		default:
			return fmt.Errorf("unknown mouse button %q", a.Button)
		}
	case "shell":
		if a.Command == "" {
			return fmt.Errorf("shell needs command")
		}
	default:
		return fmt.Errorf("unknown action type %q", a.Type)
	}
	return nil
}

func parseDeviceID(s string) (vid, pid string, ok bool) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(s)), ":")
	if len(parts) != 2 || len(parts[0]) != 4 || len(parts[1]) != 4 {
		return "", "", false
	}
	for _, p := range parts {
		for _, ch := range p {
			if !strings.ContainsRune("0123456789ABCDEF", ch) {
				return "", "", false
			}
		}
	}
	return parts[0], parts[1], true
}

// matchesNode reports whether an input node is a configured knob. Over
// Bluetooth LE, BlueZ creates the knob's input nodes with vendor and product
// 0000:0000 (seen on Fedora 44 with a VK-01: "ANTICATER Keyboard" and
// "ANTICATER Mouse"), so such nodes are recognized by their name instead.
func (c *Config) matchesNode(n evdevNode) bool {
	if c.matchesDevice(n.Vendor, n.Product) {
		return true
	}
	return n.Vendor == 0 && n.Product == 0 && strings.HasPrefix(strings.ToUpper(n.Name), "ANTICATER")
}

// matchesDevice reports whether a vendor/product pair is a configured knob.
func (c *Config) matchesDevice(vendor, product uint16) bool {
	want := fmt.Sprintf("%04X:%04X", vendor, product)
	for _, d := range c.Devices {
		vid, pid, _ := parseDeviceID(d)
		if vid+":"+pid == want {
			return true
		}
	}
	return false
}

// resolve picks the action for a gesture in the given foreground app. On
// Linux "exe" is the window class KWin reports (google-chrome, firefox, code).
// "exe|title text" entries win over plain "exe" entries; anything an app entry
// leaves out falls back to global. source names where the action came from.
func (c *Config) resolve(gesture, exe, title string) (act Action, source string) {
	lexe, ltitle := strings.ToLower(exe), strings.ToLower(title)
	best, bestLen := "", -1
	for key, m := range c.Apps {
		if _, has := m[gesture]; !has {
			continue
		}
		name, want, hasTitle := strings.Cut(key, "|")
		if strings.ToLower(strings.TrimSpace(name)) != lexe || lexe == "" {
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
	if best != "" {
		return c.Apps[best][gesture], best
	}
	if a, ok := c.Global[gesture]; ok {
		return a, "global"
	}
	return Action{Type: "passthrough"}, "default"
}

// profileFor names the apps entry that best matches a window, or "" when only
// global applies. resolve can still fall back per gesture; this is the entry
// shown as "in force" on the settings page.
func (c *Config) profileFor(exe, title string) string {
	lexe, ltitle := strings.ToLower(exe), strings.ToLower(title)
	best, bestLen := "", -1
	for key := range c.Apps {
		name, want, hasTitle := strings.Cut(key, "|")
		if strings.ToLower(strings.TrimSpace(name)) != lexe || lexe == "" {
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
	return best
}

func (a Action) isPassthrough() bool { return a.Type == "passthrough" || a.Type == "" }
