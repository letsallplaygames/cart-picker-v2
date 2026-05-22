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

```
