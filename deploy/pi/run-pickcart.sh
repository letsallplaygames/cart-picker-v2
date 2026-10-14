#!/bin/sh
set -eu

APP_DIR="${PICKCART_APP_DIR:-/home/pi/cart-picker-v2}"
APP_BIN="${PICKCART_APP_BIN:-$APP_DIR/bin/pickcart}"

if [ ! -x "$APP_BIN" ]; then
	echo "PickCart binary not found or not executable: $APP_BIN" >&2
	echo "Build it with: CGO_ENABLED=1 go build -tags ws2811 -o bin/pickcart ./cmd/pickcart" >&2
	exit 1
fi

cd "$APP_DIR"

show_root_error() {
	msg="PickCart needs permission to use the cart lights without a password. Ask IT to run: $APP_DIR/deploy/pi/install-desktop-shortcut.sh"
	echo "$msg" >&2
	if command -v zenity >/dev/null 2>&1; then
		zenity --error --no-wrap --text="$msg" || true
	elif command -v notify-send >/dev/null 2>&1; then
		notify-send -u critical PickCart "$msg" || true
	fi
}

if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
	if sudo -n -l -- "$APP_BIN" >/dev/null 2>&1; then
		exec sudo -n -E "$APP_BIN" --clear-cache "$@"
	fi
	show_root_error
	exit 1
fi

exec "$APP_BIN" --clear-cache "$@"
