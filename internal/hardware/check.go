package hardware

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

type Result struct {
	System               string
	Arch                 string
	RaspberryPi          bool
	Model                string
	BroadcomHardwareLine string
	RunningAsRoot        bool
	GPIOAccessible       bool
	GPIODevice           string
	WS281xLibraryFound   bool
	WS281xLibraryPath    string
	Passed               bool
	SimulationMode       bool
	Messages             []string
}

func Check() Result {
	result := Result{
		System: runtime.GOOS,
		Arch:   runtime.GOARCH,
	}

	result.RaspberryPi, result.Model, result.BroadcomHardwareLine = detectRaspberryPi()
	if result.RaspberryPi {
		if result.Model != "" {
			result.Messages = append(result.Messages, fmt.Sprintf("Detected Raspberry Pi: %s", result.Model))
		} else if result.BroadcomHardwareLine != "" {
			result.Messages = append(result.Messages, fmt.Sprintf("Detected Broadcom hardware: %s", result.BroadcomHardwareLine))
		}
	} else {
		result.Messages = append(result.Messages, "This does not appear to be a Raspberry Pi")
	}

	result.RunningAsRoot = os.Geteuid() == 0
	if result.RunningAsRoot {
		result.Messages = append(result.Messages, "Running with root privileges - good for hardware access")
	} else {
		result.Messages = append(result.Messages, "WARNING: Not running with root privileges")
	}

	result.GPIOAccessible, result.GPIODevice = checkGPIOAccess()
	if result.GPIOAccessible {
		result.Messages = append(result.Messages, fmt.Sprintf("GPIO access verified via %s", result.GPIODevice))
	} else {
		result.Messages = append(result.Messages, "GPIO access not verified")
	}

	result.WS281xLibraryFound, result.WS281xLibraryPath = checkWS281xLibrary()
	if result.WS281xLibraryFound {
		result.Messages = append(result.Messages, fmt.Sprintf("Found rpi_ws281x/libws2811 library: %s", result.WS281xLibraryPath))
	} else {
		result.Messages = append(result.Messages, "rpi_ws281x/libws2811 library not found")
	}

	result.Passed = result.RaspberryPi && result.GPIOAccessible && result.WS281xLibraryFound
	result.SimulationMode = !result.Passed
	return result
}

func detectRaspberryPi() (bool, string, string) {
	if modelBytes, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		model := strings.TrimSpace(strings.TrimRight(string(modelBytes), "\x00"))
		if strings.Contains(model, "Raspberry Pi") {
			return true, model, ""
		}
	}

	if cpuBytes, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(cpuBytes), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Hardware") && strings.Contains(line, "BCM") {
				return true, "", line
			}
		}
	}

	return false, "", ""
}

func checkGPIOAccess() (bool, string) {
	candidates := []string{"/dev/gpiomem", "/dev/gpiochip0", "/dev/mem"}
	for _, candidate := range candidates {
		file, err := os.OpenFile(candidate, os.O_RDWR, 0)
		if err == nil {
			file.Close()
			return true, candidate
		}
	}
	return false, ""
}

func checkWS281xLibrary() (bool, string) {
	candidates := []string{
		"/usr/lib/libws2811.so",
		"/usr/local/lib/libws2811.so",
		"/usr/lib/aarch64-linux-gnu/libws2811.so",
		"/usr/lib/arm-linux-gnueabihf/libws2811.so",
		"/usr/lib64/libws2811.so",
		"/usr/include/ws2811.h",
		"/usr/local/include/ws2811.h",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return true, candidate
		}
	}
	return false, ""
}
