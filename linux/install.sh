#!/usr/bin/env bash
# Builds and installs the Anticater Knob app for Linux (KDE Plasma) for the current user.
# Run from this folder as your normal user:   ./install.sh
# It asks for your password (sudo) once, for the udev rule; nothing else is changed system-wide.
set -euo pipefail
cd "$(dirname "$0")"

if [[ $EUID -eq 0 ]]; then
  echo "Run this as your normal user, not with sudo." >&2
  exit 1
fi
if ! command -v go >/dev/null; then
  echo "Go is not installed:  sudo dnf install golang" >&2
  exit 1
fi

BIN="$HOME/.local/bin/anticater-knob"
echo "1/5 Building..."
mkdir -p "$(dirname "$BIN")"
(cd .. && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$BIN" ./linux)

echo "2/5 udev rule (lets you read the knob and set its lights; needs sudo)..."
RULE=/etc/udev/rules.d/60-anticater-knob.rules
{
  echo "# Anticater knob app: the logged-in user may grab the knob's input nodes, write its lights"
  echo "# (hidraw) and create the app's virtual input device (uinput). Installed by linux/install.sh."
  for id in 05ac:022c 514c:8850 514c:4155; do
    v=${id%:*} p=${id#*:}
    echo "SUBSYSTEM==\"input\", KERNEL==\"event*\", ATTRS{id/vendor}==\"$v\", ATTRS{id/product}==\"$p\", TAG+=\"uaccess\""
    echo "SUBSYSTEM==\"hidraw\", KERNELS==\"*:${v^^}:${p^^}.*\", TAG+=\"uaccess\""
  done
  echo 'KERNEL=="uinput", SUBSYSTEM=="misc", TAG+="uaccess", OPTIONS+="static_node=uinput"'
} | sudo tee "$RULE" >/dev/null
# /dev/uinput only exists once the module is loaded; load it now and at every boot.
echo uinput | sudo tee /etc/modules-load.d/anticater-knob-uinput.conf >/dev/null
sudo modprobe uinput
sudo udevadm control --reload-rules
sudo udevadm trigger --subsystem-match=input --subsystem-match=hidraw --subsystem-match=misc
sudo udevadm settle

echo "3/5 Icon and menu entry..."
"$BIN" --install-icons   # the logo (same as the Windows app) into ~/.local/share/icons/hicolor
mkdir -p "$HOME/.local/share/applications"
cat >"$HOME/.local/share/applications/anticater-knob.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Anticater Knob
Comment=Per-app actions for the Anticater knob
Comment[vi]=Núm Anticater đổi chức năng theo ứng dụng
Exec="$BIN"
Icon=anticater-knob
Categories=Utility;Settings;
DESKTOP

echo "4/5 Start with Plasma..."
AUTOSTART="${XDG_CONFIG_HOME:-$HOME/.config}/autostart"
mkdir -p "$AUTOSTART"
cat >"$AUTOSTART/anticater-knob.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Anticater Knob
Exec="$BIN" --background
Icon=anticater-knob
X-GNOME-Autostart-enabled=true
DESKTOP

# The older xremap setup (xremap/) grabs the same knob; both cannot hold it.
if systemctl --user is-enabled anticater-knob.service >/dev/null 2>&1; then
  echo "   Turning off the old xremap service (anticater-knob.service)."
  systemctl --user disable --now anticater-knob.service || true
fi

echo "5/5 Starting..."
pkill -x anticater-knob 2>/dev/null && sleep 1 || true
setsid "$BIN" --background >/dev/null 2>&1 < /dev/null &
echo
echo "Installed: $BIN"
echo "Settings: open 'Anticater Knob' from the app menu, or click its icon in the system tray."
echo "Config:   ~/.config/anticater-knob/knob-config.json   Log: ~/.config/anticater-knob/knob.log"
