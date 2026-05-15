package main

import (
	"fmt"
	"os"
	"runtime"

	"pickcart/internal/hardware"
)

func main() {
	result := hardware.Check()

	fmt.Println("Raspberry Pi Hardware Detection")
	fmt.Println("-" + stringsRepeat("-", 29))
	fmt.Printf("System: %s\n", runtime.GOOS)
	fmt.Printf("Architecture: %s\n", runtime.GOARCH)
	fmt.Println("-" + stringsRepeat("-", 29))
	for _, message := range result.Messages {
		fmt.Println(message)
	}
	fmt.Println("-" + stringsRepeat("-", 29))
	if result.Passed {
		fmt.Println("Hardware check PASSED: System is ready for WS2812B LED control")
		os.Exit(0)
	}

	fmt.Println("Hardware check FAILED: Some requirements are missing")
	fmt.Println("The system will run in simulation mode")
	os.Exit(1)
}

func stringsRepeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
