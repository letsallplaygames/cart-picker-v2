#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
REPO_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
APP_BIN="${PICKCART_APP_BIN:-$REPO_DIR/bin/pickcart}"
SUDOERS_FILE="/etc/sudoers.d/pickcart"

if [ "$(id -u)" -eq 0 ] && [ -n "${SUDO_USER:-}" ] && [ "$SUDO_USER" != "root" ]; then
	TARGET_USER="$SUDO_USER"
	TARGET_HOME=$(getent passwd "$SUDO_USER" | cut -d: -f6)
else
	TARGET_USER=$(id -un)
	TARGET_HOME="$HOME"
fi

if [ -z "$TARGET_HOME" ] || [ "$TARGET_USER" = "root" ]; then
	echo "Run this installer as the desktop user, not as root." >&2
	exit 1
fi

HOME_BIN="$TARGET_HOME/bin"
DESKTOP_DIR="$TARGET_HOME/Desktop"
APPS_DIR="$TARGET_HOME/.local/share/applications"
run_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
	else
		sudo "$@"
	fi
}

mkdir -p "$HOME_BIN" "$APPS_DIR"

wrapper_tmp=$(mktemp)
desktop_tmp=$(mktemp)
sudoers_tmp=$(mktemp)
trap 'rm -f "$wrapper_tmp" "$desktop_tmp" "$sudoers_tmp"' EXIT
sed "s|/home/pi/cart-picker-v2|$REPO_DIR|g" "$SCRIPT_DIR/run-pickcart.sh" > "$wrapper_tmp"
install -m 0755 "$wrapper_tmp" "$HOME_BIN/run-pickcart.sh"
sed "s|/home/pi|$TARGET_HOME|g" "$SCRIPT_DIR/PickCart.desktop" > "$desktop_tmp"

if [ -d "$DESKTOP_DIR" ]; then
	install -m 0755 "$desktop_tmp" "$DESKTOP_DIR/PickCart.desktop"
	if command -v gio >/dev/null 2>&1; then
		gio set "$DESKTOP_DIR/PickCart.desktop" metadata::trusted true 2>/dev/null || true
	fi
fi
install -m 0755 "$desktop_tmp" "$APPS_DIR/PickCart.desktop"

cat > "$sudoers_tmp" <<EOF
Defaults:$TARGET_USER env_keep += "DISPLAY XAUTHORITY WAYLAND_DISPLAY XDG_RUNTIME_DIR"
$TARGET_USER ALL=(root) NOPASSWD:SETENV: $APP_BIN
EOF

VISUDO=$(command -v visudo || true)
if [ -z "$VISUDO" ] && [ -x /usr/sbin/visudo ]; then
	VISUDO=/usr/sbin/visudo
fi
if [ -z "$VISUDO" ] || ! "$VISUDO" -c -f "$sudoers_tmp" >/dev/null; then
	echo "Generated sudoers file is invalid; not installing GPIO passwordless access." >&2
	exit 1
fi

run_root install -o root -g root -m 0440 "$sudoers_tmp" "$SUDOERS_FILE"

if [ "$(id -u)" -eq 0 ]; then
	sudo_ok=$(sudo -u "$TARGET_USER" sudo -n -l -- "$APP_BIN" >/dev/null 2>&1 && echo yes || echo no)
else
	sudo_ok=$(sudo -n -l -- "$APP_BIN" >/dev/null 2>&1 && echo yes || echo no)
fi
if [ "$sudo_ok" != "yes" ]; then
	echo "Installed $SUDOERS_FILE, but passwordless sudo for $APP_BIN is not active yet." >&2
	echo "Confirm $TARGET_USER can use sudo and that $APP_BIN exists." >&2
	exit 1
fi

echo "Installed PickCart desktop shortcut, application menu entry, and passwordless GPIO sudo."
if [ -f "$DESKTOP_DIR/PickCart.desktop" ]; then
	echo "Desktop: $DESKTOP_DIR/PickCart.desktop"
fi
echo "Menu:    $APPS_DIR/PickCart.desktop"
echo "Sudoers: $SUDOERS_FILE"
if [ ! -x "$APP_BIN" ]; then
	echo "Build the app before launching: CGO_ENABLED=1 go build -tags ws2811 -o bin/pickcart ./cmd/pickcart"
fi
