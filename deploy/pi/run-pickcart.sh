#!/bin/sh
set -eu

APP_DIR="${PICKCART_APP_DIR:-/home/pi/cart-picker-v2}"
APP_BIN="${PICKCART_APP_BIN:-$APP_DIR/bin/pickcart}"
CART_NUMBER="${PICKCART_CART:-1}"
PROFILE_NAME="${PICKCART_PROFILE:-standard}"

if [ ! -x "$APP_BIN" ]; then
	echo "PickCart binary not found or not executable: $APP_BIN" >&2
	echo "Build it with: CGO_ENABLED=1 go build -tags ws2811 -o bin/pickcart ./cmd/pickcart" >&2
	exit 1
fi

cd "$APP_DIR"

if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
	exec sudo -E "$APP_BIN" --cart "$CART_NUMBER" --profile "$PROFILE_NAME"
fi

exec "$APP_BIN" --cart "$CART_NUMBER" --profile "$PROFILE_NAME"
