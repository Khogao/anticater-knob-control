# Knob Console

Turns an Anticater VK-01 knob into a per-app shortcut controller on macOS: volume
everywhere, tab switching in the browser, brush size in Photoshop, and so on.

This is a fork of [dreamug/anticater-knob-control](https://github.com/dreamug/anticater-knob-control).
The original Chinese README is kept as [README.zh-CN.md](README.zh-CN.md); it describes
upstream behaviour, not this fork.

## What this fork changes

- **English UI.** Every string in the window, menu bar menu, overlays and diagnostic
  report is translated. Chinese key names in existing config files are still accepted.
- **Automatic per-app switching.** An app's mapping applies as soon as that app is
  frontmost. Upstream needs a triple press to lock onto one app. The old behaviour is
  still available: turn off *Auto-switch by frontmost app* in *Listening tools* or the
  menu bar menu.
- **Works with knobs that enumerate as a keyboard.** See below.
- **Show console reopens the window.** Upstream only activated the app, so a closed
  window could not be brought back.

## Knobs that enumerate as a keyboard

Upstream was written for a VK-01 whose Bluetooth interface reports itself as a mouse,
which a normal app can open exclusively. The unit this fork was tested with reports
`usagePage=0x0001 usage=0x0006` (keyboard) over USB, the 2.4G dongle and Bluetooth
alike, and macOS only lets root seize keyboard-class devices: the seize fails with
`0xe00002c1`.

So when the seize fails, this fork:

1. Opens the knob again with a fresh HID manager, without a seize. Reusing the manager
   whose seize failed delivered no input at all.
2. Installs a session event tap that drops the volume, mute and brightness keys macOS
   generates for the knob, by matching them in time against the knob's HID reports.
   Only the mapped action runs.

Volume keys on a real keyboard still work; they are held for about 60 ms while the app
checks whether the knob produced them.

**Tested:** with a VK-01 (`05AC:022C`, Bluetooth LE, macOS 15) the app receives the
knob's input and runs the frontmost app's mapping. **Not yet confirmed:** that the
system volume stays put while a remapped gesture runs.

## Requirements

- macOS 14 or later
- Xcode Command Line Tools (`xcode-select --install`)
- An Anticater VK-01 with vendor/product ID `05AC:022C`

## Build and run

```bash
./run-gui.sh
```

or

```bash
./build-gui.sh
open build/ANTICATERKnobControl.app
```

## Permissions

The app needs two permissions in System Settings → Privacy & Security:

- **Input Monitoring** to read the knob.
- **Accessibility** to send shortcuts, scroll, and block the knob's native keys.

Click *Start listening*, allow Input Monitoring, use *Permissions → Request
Accessibility*, then quit and reopen the app.

The build is ad-hoc signed unless you have an Apple Development certificate, so every
rebuild is a new app as far as macOS is concerned. The switches in System Settings
still look enabled but belong to the old build. After a rebuild, clear them and grant
again:

```bash
tccutil reset ListenEvent cc.codex.anticater-knob-control
tccutil reset Accessibility cc.codex.anticater-knob-control
```

## Using it

- The sidebar lists *Global default* and each configured app. *Discovered apps* lets
  you add more; *Add current app* adds whatever is frontmost.
- Each profile maps five gestures: rotate left, rotate right, press, hold + rotate
  left, hold + rotate right.
- Action types: shortcut, key (including media keys), mouse click, scroll, shell
  command, no action.
- *Quick templates* fill a profile for browsers, code editors, media, design and video
  timelines.
- *Save config* writes `~/Library/Application Support/ANTICATER Knob Control/app-mapping.json`.
- Quit the vendor's ANTICATER app while using this one.

## Diagnostics

*Listening tools → Copy diagnostic report* copies the permission state, the result of
the seize attempt, event counters and the most recent raw HID events. If *Raw events*
stays at 0 while you turn the knob, the app is not receiving input.

## Known issues

- After the app sends a shortcut with modifiers (for example Ctrl+Shift+Tab), the next
  gesture can be read as if those modifiers were held. Mappings that do not use the
  *Modifier* edit mode are unaffected.
- There is no debounce for rotation.

## License

Upstream does not include a license file. This fork adds none; treat the code
accordingly.
