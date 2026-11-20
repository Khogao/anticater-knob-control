# macOS probes

Small standalone tools used while debugging the macOS app. Each runs with `swift <file>`
(or compile with `swiftc -O`). The terminal needs Input Monitoring for the event probes.

| File | What it shows |
|---|---|
| `list-event-taps.swift` | Every installed event tap: owning pid, location, whether it is enabled, its event mask. Confirms the app's key blocker is installed. |
| `event-probe.swift [seconds]` | Key and media-key events, logged both before (`PRE`) and after (`POST`) other apps' taps, with source pid and tag. An event seen only as `PRE` was dropped by a tap in between. |
| `timestamp-probe.swift` | For each media key macOS generates, the distance to the nearest HID report from the knob (`05AC:022C`). On the tested unit it is 0 ms. |
| `list-app-windows.swift` | Windows owned by Knob Console and whether they are on screen. |
