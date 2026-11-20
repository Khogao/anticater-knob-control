package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Prints the light colour each app gets from a real config:
//
//	KNOB_LED_CONFIG=path\to\knob-config.json go test ./windows -run LEDTable -v
func TestLEDTable(t *testing.T) {
	path := os.Getenv("KNOB_LED_CONFIG")
	if path == "" {
		t.Skip("set KNOB_LED_CONFIG to a config file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{"desktop", "explorer.exe", "msedge.exe", "chrome.exe", "CalculatorApp.exe", "claude.exe",
		"EXCEL.EXE", "WINWORD.EXE", "POWERPNT.EXE", "Zalo.exe", "acad.exe", "WhatsApp.exe", "Photoshop.exe", "SystemSettings.exe", "unknownapp.exe"} {
		t.Logf("%-20s %s", exe, c.LED.colour(exe, ""))
	}
}
