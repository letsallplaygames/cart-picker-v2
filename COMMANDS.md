```bash
sudo apt update
sudo apt install -y git build-essential pkg-config libgl1-mesa-dev xorg-dev scons

cd ~
git clone https://github.com/jgarff/rpi_ws281x.git
cd rpi_ws281x
scons
sudo scons install
sudo ldconfig

cd ~/cart-picker-v2
go mod download
go run ./cmd/hardwarecheck
go run -tags ws2811 ./cmd/testleds --cart 1 --locations A1 --hold
mkdir -p bin
go build -tags ws2811 -o bin/pickcart ./cmd/pickcart
./bin/pickcart --cart 1 --profile standard

```
