#!/usr/bin/env bash
# Sets up xremap for the Anticater VK-01 knob on Fedora KDE (Plasma, Wayland).
# Run as your normal user, from this folder, with the knob connected:  ./install.sh
# It asks for sudo for two things: adding you to the "input" group and a udev rule
# that lets that group use /dev/uinput. Nothing else is changed system-wide.
set -euo pipefail
cd "$(dirname "$0")"

if [[ $EUID -eq 0 ]]; then
  echo "Run this as your normal user, not with sudo." >&2
  exit 1
fi
if ! command -v xremap >/dev/null; then
  echo "xremap is not installed or not on PATH. See README.md, step 1." >&2
  exit 1
fi

# The knob shows up as several input nodes that share one name prefix; find it by
# USB ids so the service grabs only the knob and no other keyboard.
VENDOR="${KNOB_VENDOR:-05ac}"
PRODUCT="${KNOB_PRODUCT:-022c}"
NAME="$(awk -v v="$VENDOR" -v p="$PRODUCT" '
  /^I:/ { hit = (tolower($0) ~ "vendor=" v " product=" p) }
  /^N:/ && hit { sub(/^N: Name="/, ""); sub(/"$/, ""); print; exit }
' /proc/bus/input/devices)"
if [[ -z "$NAME" ]]; then
  echo "No input device with ids $VENDOR:$PRODUCT found. Is the knob connected?" >&2
  echo "Other hardware revisions: KNOB_VENDOR=514c KNOB_PRODUCT=8850 ./install.sh" >&2
  exit 1
fi
# Drop a trailing " Keyboard" / " Consumer Control" / " Mouse" so every node matches.
BASE="$(sed -E 's/ (Keyboard|Consumer Control|System Control|Mouse)$//' <<<"$NAME")"
echo "Knob found: \"$NAME\"  (matching on \"$BASE\")"

echo "Adding $USER to the input group and allowing it to use /dev/uinput (sudo)..."
sudo gpasswd -a "$USER" input
echo 'KERNEL=="uinput", GROUP="input", TAG+="uaccess", MODE:="0660", OPTIONS+="static_node=uinput"' |
  sudo tee /etc/udev/rules.d/99-input.rules >/dev/null
echo uinput | sudo tee /etc/modules-load.d/uinput.conf >/dev/null
sudo modprobe uinput
sudo udevadm control --reload-rules
sudo udevadm trigger

CONFIG_DIR="$HOME/.config/anticater-knob"
mkdir -p "$CONFIG_DIR" "$HOME/.config/systemd/user"
if [[ -f "$CONFIG_DIR/config.yml" ]]; then
  echo "Keeping your existing $CONFIG_DIR/config.yml"
else
  cp config.yml "$CONFIG_DIR/config.yml"
fi
if [[ "$VENDOR:$PRODUCT" != "05ac:022c" ]]; then
  sed -i "s/ids:0x5ac:0x22c/ids:0x${VENDOR#0}:0x${PRODUCT#0}/g" "$CONFIG_DIR/config.yml"
fi
xremap --validate-config "$CONFIG_DIR/config.yml" 2>/dev/null || true

cat >"$HOME/.config/systemd/user/anticater-knob.service" <<UNIT
[Unit]
Description=xremap for the Anticater VK-01 knob
After=graphical-session.target
PartOf=graphical-session.target

[Service]
ExecStart=$(command -v xremap) --watch=config,device --device "$BASE" %h/.config/anticater-knob/config.yml
Restart=on-failure
RestartSec=3

[Install]
WantedBy=graphical-session.target
UNIT

systemctl --user daemon-reload
systemctl --user enable anticater-knob.service
echo
echo "Installed. Log out and back in (or reboot) so the new group membership applies;"
echo "the service then starts with your Plasma session."
echo "  status: systemctl --user status anticater-knob"
echo "  log:    journalctl --user -u anticater-knob -f"
echo "  config: $CONFIG_DIR/config.yml (reloaded automatically when saved)"
