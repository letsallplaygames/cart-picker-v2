package main

import (
	"bufio"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"pickcart/internal/config"
	"pickcart/internal/led"
)

func main() {
	cartNumber := flag.Int("cart", 1, "cart number")
	profileName := flag.String("profile", "standard", fmt.Sprintf("cart profile (%s)", strings.Join(config.AvailableProfiles(), ", ")))
	locationsFlag := flag.String("locations", "A1", "comma-separated shelf locations to highlight")
	colorFlag := flag.String("color", "255,255,255", "RGB color as r,g,b")
	durationFlag := flag.Duration("duration", 2*time.Second, "how long to keep LEDs lit before clearing; use 0s with --hold to wait for Enter")
	pauseFlag := flag.Duration("pause", 500*time.Millisecond, "pause between locations when using --cycle")
	cycleFlag := flag.Bool("cycle", false, "highlight each location one at a time instead of all at once")
	holdFlag := flag.Bool("hold", false, "wait for Enter before clearing LEDs")
	clearOnlyFlag := flag.Bool("clear", false, "clear LEDs and exit")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	profile, err := config.GetCartProfile(*profileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve cart profile: %v\n", err)
		os.Exit(1)
	}

	controller := led.New(*cartNumber, profile.Name)
	defer controller.Cleanup()

	if *clearOnlyFlag {
		fmt.Println("Clearing LEDs...")
		controller.ClearLEDs()
		return
	}

	locations := parseLocations(*locationsFlag)
	if len(locations) == 0 {
		fmt.Fprintln(os.Stderr, "at least one location is required")
		os.Exit(1)
	}

	color, err := parseColor(*colorFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse color: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Cart %d • profile %s\n", *cartNumber, profile.Name)
	fmt.Printf("Color %d,%d,%d\n", color[0], color[1], color[2])

	if *cycleFlag {
		cycleLocations(controller, locations, color, *durationFlag, *pauseFlag, *holdFlag)
		return
	}

	fmt.Printf("Highlighting locations: %s\n", strings.Join(locations, ", "))
	controller.HighlightLocations(locations, color)
	waitOrSleep(*durationFlag, *holdFlag)
	fmt.Println("Clearing LEDs...")
	controller.ClearLEDs()
}

func cycleLocations(controller *led.Controller, locations []string, color [3]byte, duration time.Duration, pause time.Duration, hold bool) {
	for idx, location := range locations {
		fmt.Printf("[%d/%d] Highlighting %s\n", idx+1, len(locations), location)
		controller.HighlightLocations([]string{location}, color)
		waitOrSleep(duration, hold)
		fmt.Printf("[%d/%d] Clearing %s\n", idx+1, len(locations), location)
		controller.ClearLEDs()
		if idx < len(locations)-1 && !hold && pause > 0 {
			time.Sleep(pause)
		}
	}
}

func waitOrSleep(duration time.Duration, hold bool) {
	if hold || duration <= 0 {
		fmt.Print("Press Enter to continue...")
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
		return
	}
	time.Sleep(duration)
}

func parseLocations(raw string) []string {
	parts := strings.Split(raw, ",")
	locations := make([]string, 0, len(parts))
	seen := map[string]struct{}{}
	for _, part := range parts {
		location := strings.ToUpper(strings.TrimSpace(part))
		if location == "" {
			continue
		}
		if _, ok := seen[location]; ok {
			continue
		}
		seen[location] = struct{}{}
		locations = append(locations, location)
	}
	return locations
}

func parseColor(raw string) ([3]byte, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return [3]byte{}, fmt.Errorf("expected r,g,b")
	}

	var color [3]byte
	for i, part := range parts {
		part = strings.TrimSpace(part)
		value, err := parseByte(part)
		if err != nil {
			return [3]byte{}, fmt.Errorf("component %d: %w", i+1, err)
		}
		color[i] = value
	}
	return color, nil
}

func parseByte(raw string) (byte, error) {
	var value int
	_, err := fmt.Sscanf(raw, "%d", &value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", raw)
	}
	if value < 0 || value > 255 {
		return 0, fmt.Errorf("value %d out of range 0-255", value)
	}
	return byte(value), nil
}
