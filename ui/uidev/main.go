// uidev serves the settings page with a stand-in backend, so the page can be
// worked on in an ordinary browser on any OS, without the knob or the app.
//
//	go run ./ui/uidev [config.json]
//
// It prints the address to open. Saved configs stay in memory.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"anticater-knob/ui"
)

type fake struct {
	mu      sync.Mutex
	config  []byte
	rev     int
	paused  bool
	auto    bool
	lang    string
	started time.Time
}

func (f *fake) State() any {
	f.mu.Lock()
	defer f.mu.Unlock()
	ticks := int(time.Since(f.started).Seconds())
	return map[string]any{
		"platform": "windows", "language": f.lang, "paused": f.paused, "hookInstalled": true,
		"devices": []string{"05AC:022C"}, "knobAgoMs": 1200,
		"frontApp": "chrome.exe", "frontTitle": "Example video - YouTube - Google Chrome", "effective": "chrome.exe|YouTube",
		"raw": ticks * 2, "recognized": ticks, "swallowed": ticks, "resent": 0,
		"gestures":  map[string]int{"rotate_right": ticks, "rotate_left": 3, "press": 1},
		"lastEvent": "rotate_right in chrome.exe -> scroll [chrome.exe|YouTube]", "lastRaw": "12:00:00.000  05 e9 00",
		"autostart": f.auto, "configPath": `C:\Knob\knob-config.json`, "configRev": f.rev,
	}
}

func (f *fake) Config() ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.config, nil
}

func (f *fake) SaveConfig(data []byte) error {
	if !json.Valid(data) {
		return fmt.Errorf("not valid JSON")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.config = data
	f.rev++
	return nil
}

func (f *fake) Apps() any {
	return []map[string]string{
		{"name": "chrome.exe", "title": "Example video - YouTube - Google Chrome"},
		{"name": "Code.exe", "title": "main.go - Visual Studio Code"},
		{"name": "notepad.exe", "title": "Untitled - Notepad"},
		{"name": "WINWORD.EXE", "title": "Document1 - Word"},
	}
}

func (f *fake) Command(name, arg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch name {
	case "pause":
		f.paused = !f.paused
	case "autostart":
		f.auto = !f.auto
	case "language":
		f.lang = arg
		f.rev++
	case "clearCounters":
		f.started = time.Now()
	}
	fmt.Println("command:", name, arg)
	return nil
}

func (f *fake) Icon() []byte { return nil }

func main() {
	path := "windows/default-config.json"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv, err := ui.Start(&fake{config: data, lang: "en", started: time.Now()}, 47816)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(srv.URL())
	select {}
}
