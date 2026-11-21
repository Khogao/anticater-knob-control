package main

// Moving light effects. The knob has one colour for all six lights and no
// light modes of its own (checked 21/11/2026: the vendor app sends only colour,
// brightness and on/off, and the knob's stored light record holds just those
// five bytes; the old VK-01 mode command FE B0 does nothing). So the effects
// are made here, by sending the colour again and again:
//
//	"led": { "effect": "breathe" }   // brightness rises and falls
//	"led": { "effect": "rainbow" }   // the colour goes round the colour wheel
//	"led": { "effect": "pulse" }     // a short flash each time the knob turns
//
// "effect_seconds" sets the length of one breath or one round of the wheel
// (defaults 4 and 8). Breathe and rainbow send about 12 colours a second;
// pulse sends a few per turn only.

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const ledFrameMs = 80

var ledEffects = []string{"", "breathe", "rainbow", "pulse"}

// What the effect loop needs, copied from the main thread under ledMu.
var (
	ledBase    string    // the app's line ("RRGGBB nn"), before any effect
	ledEffect  string    // "", "breathe", "rainbow", "pulse"
	ledPeriod  float64   // seconds
	ledKickAt  time.Time // last knob turn, for "pulse"
	ledPending int       // lines written to the script and not answered yet
	ledLoopOn  bool
)

func (l *LEDConfig) validateEffect() error {
	l.Effect = strings.ToLower(strings.TrimSpace(l.Effect))
	ok := false
	for _, e := range ledEffects {
		ok = ok || e == l.Effect
	}
	if !ok {
		return fmt.Errorf("led.effect must be one of breathe, rainbow, pulse, not %q", l.Effect)
	}
	if l.EffectSeconds < 0 || l.EffectSeconds > 120 {
		return fmt.Errorf("led.effect_seconds must be 0..120, not %v", l.EffectSeconds)
	}
	return nil
}

// ledSetBase hands the app's colour line to the effect loop. It returns true
// when the loop takes care of sending it (an effect that keeps sending).
func ledSetBase(line string) bool {
	ledMu.Lock()
	defer ledMu.Unlock()
	ledBase = line
	ledEffect, ledPeriod = "", 0
	if cfg != nil && cfg.LED != nil && line != "off" && !paused {
		ledEffect, ledPeriod = cfg.LED.Effect, cfg.LED.EffectSeconds
	}
	if ledEffect != "" && !ledLoopOn {
		ledLoopOn = true
		go ledLoop()
	}
	return ledEffect == "breathe" || ledEffect == "rainbow"
}

// ledKick starts a pulse; called on each knob turn.
func ledKick() {
	ledMu.Lock()
	ledKickAt = time.Now()
	ledMu.Unlock()
}

// ledLoop sends the effect frames until no effect is set.
func ledLoop() {
	start := time.Now()
	lastPulse := false
	for range time.Tick(ledFrameMs * time.Millisecond) {
		ledMu.Lock()
		base, effect, period, kick, pending := ledBase, ledEffect, ledPeriod, ledKickAt, ledPending
		var l *LEDConfig
		if cfg != nil {
			l = cfg.LED
		}
		if effect == "" || l == nil {
			ledLoopOn = false
			ledMu.Unlock()
			return
		}
		ledMu.Unlock()
		var colour string
		var bright int
		if _, err := fmt.Sscanf(base, "%6s %d", &colour, &bright); err != nil {
			continue
		}
		if pending > 2 {
			continue // Bluetooth is behind: drop this frame instead of queueing
		}
		t := time.Since(start).Seconds()
		line := ""
		switch effect {
		case "breathe":
			if period <= 0 {
				period = 4
			}
			// From a fifth of the brightness up to all of it, smooth at both ends.
			x := 0.5 - 0.5*cosTurn(t/period)
			line = fmt.Sprintf("%s %d", colour, max(3, int(float64(bright)*(0.2+0.8*x)+0.5)))
		case "rainbow":
			if period <= 0 {
				period = 8
			}
			r, g, b := hsvToRGB(mod(t/period*360, 360), 1, 1)
			line = l.lineAt(fmt.Sprintf("%02X%02X%02X", r, g, b), float64(bright)/float64(max(1, l.Brightness)))
		case "pulse":
			age := time.Since(kick).Seconds()
			if age > 0.6 {
				if lastPulse {
					line, lastPulse = base, false // back to the plain colour once
				}
				break
			}
			// Full brightness at the turn, fading back over 0.6 s.
			k := 1 - age/0.6
			line, lastPulse = fmt.Sprintf("%s %d", colour, int(float64(bright)+float64(100-bright)*k+0.5)), true
		}
		if line != "" {
			ledSendFrame(line)
		}
	}
}

// ledSendFrame writes one frame; frames are not logged, only errors are.
func ledSendFrame(line string) {
	ledMu.Lock()
	defer ledMu.Unlock()
	if ledIn == nil {
		if err := ledLaunch(); err != nil {
			return
		}
	}
	if _, err := ledIn.Write([]byte(line + "\n")); err != nil {
		ledIn.Close()
		ledIn = nil
		return
	}
	ledPending++
}

func cosTurn(x float64) float64 { return math.Cos(2 * math.Pi * x) }
