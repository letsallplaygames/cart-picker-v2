```bash
sudo apt update
sudo apt install -y git build-essential pkg-config libgl1-mesa-dev xorg-dev scons cmake

cd ~/rpi_ws281x
rm -rf build
mkdir build
cd build
cmake -D BUILD_SHARED=ON -D BUILD_TEST=OFF ..
cmake --build .
sudo cmake --install .
sudo ldconfig

cd ~/cart-picker-v2
mkdir -p bin
go mod download
go mod tidy
CGO_ENABLED=1 go build -tags ws2811 -o bin/pickcart ./cmd/pickcart

# Desktop icon, start-menu entry, and passwordless GPIO sudo (once per Pi)
./deploy/pi/install-desktop-shortcut.sh

# Run PickCart from the command line (opens the cart/profile start screen)
sudo -E ./bin/pickcart --clear-cache

# Optional: pre-select cart number and profile
# Profiles: standard, medium_cart, small_cart, large_cart
sudo -E ./bin/pickcart --clear-cache --cart 1 --profile standard
```
