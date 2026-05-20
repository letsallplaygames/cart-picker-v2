//go:build linux && (arm || arm64) && cgo && ws2811

package led

import (
	"log/slog"

	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go"

	"pickcart/internal/config"
)

type Controller struct {
	ledCount int
	ledMap   map[string]int
	device   *ws2811.WS2811
	ok       bool
}

func New(cartNumber int, profileName string) *Controller {
	c := &Controller{
		ledCount: config.LEDCount,
		ledMap:   map[string]int{},
	}

	mappings, requiredCount, path, columnIndex, err := loadMappingsForCart(cartNumber, profileName)
	if err != nil {
		slog.Warn("failed to load LED mappings", "error", err, "cart", cartNumber, "profile", profileName)
	} else {
		c.LoadMappings(mappings)
		if requiredCount > c.ledCount {
			c.ledCount = requiredCount
		}
		slog.Info("loaded LED mappings", "count", len(mappings), "required_led_count", requiredCount, "source", path, "column_index", columnIndex, "cart", cartNumber, "profile", profileName)
	}

	options := ws2811.Option{
		Frequency: config.LEDFreqHz,
		DmaNum:    config.LEDDMA,
		Channels:  makeChannelOptions(c.ledCount),
	}

	device, err := ws2811.MakeWS2811(&options)
	if err != nil {
		slog.Warn("failed to create ws2811 LED controller", "error", err, "count", c.ledCount, "brightness", config.LEDBrightness, "cart", cartNumber, "profile", profileName)
		return c
	}
	if err := device.Init(); err != nil {
		slog.Warn("failed to initialize ws2811 LED controller", "error", err, "count", c.ledCount, "brightness", config.LEDBrightness, "cart", cartNumber, "profile", profileName)
		return c
	}

	c.device = device
	c.ok = true
	c.ClearLEDs()
	slog.Info("initialized ws2811 LED controller", "count", c.ledCount, "brightness", config.LEDBrightness, "gpio_pin", config.LEDPin, "dma", config.LEDDMA, "channel", config.LEDChannel, "cart", cartNumber, "profile", profileName)
	return c
}

func makeChannelOptions(ledCount int) []ws2811.ChannelOption {
	channelCount := config.LEDChannel + 1
	if channelCount < 1 {
		channelCount = 1
	}

	channels := make([]ws2811.ChannelOption, channelCount)
	channels[config.LEDChannel] = ws2811.ChannelOption{
		GpioPin:    config.LEDPin,
		Invert:     config.LEDInvert,
		LedCount:   ledCount,
		StripeType: ws2811.WS2811StripGRB,
		Brightness: config.LEDBrightness,
	}
	return channels
}

func (c *Controller) LoadMappings(m map[string]int) {
	if c == nil {
		return
	}
	c.ledMap = make(map[string]int, len(m))
	for key, value := range m {
		c.ledMap[key] = value
	}
}

func (c *Controller) HighlightLocations(locations []string, color [3]byte) {
	locationColors := make(map[string][3]byte, len(locations))
	for _, location := range locations {
		locationColors[location] = color
	}
	c.HighlightLocationColors(locationColors)
}

func (c *Controller) HighlightLocationColors(locationColors map[string][3]byte) {
	if c == nil || !c.ok || c.device == nil || len(locationColors) == 0 {
		return
	}

	leds, ok := c.frameBuffer("highlight LEDs")
	if !ok {
		return
	}
	clearFrameBuffer(leds)

	for location, color := range locationColors {
		index, ok := c.ledMap[location]
		if !ok {
			continue
		}
		if index < 0 || index >= len(leds) {
			continue
		}
		leds[index] = packColor(color)
	}

	if err := c.device.Render(); err != nil {
		slog.Warn("failed to render LED highlights", "error", err)
	}
}

func (c *Controller) ClearLEDs() {
	if c == nil || !c.ok || c.device == nil {
		return
	}

	leds, ok := c.frameBuffer("clear LEDs")
	if !ok {
		return
	}
	clearFrameBuffer(leds)

	if err := c.device.Render(); err != nil {
		slog.Warn("failed to clear LEDs", "error", err)
	}
}

func (c *Controller) Cleanup() {
	if c == nil || !c.ok || c.device == nil {
		return
	}

	if leds, ok := c.frameBuffer("cleanup LEDs"); ok {
		clearFrameBuffer(leds)
		if err := c.device.Render(); err != nil {
			slog.Warn("failed to render LED cleanup", "error", err)
		}
	}

	c.device.Fini()
	c.device = nil
	c.ok = false
}

func (c *Controller) frameBuffer(action string) ([]uint32, bool) {
	if c == nil || c.device == nil {
		return nil, false
	}
	if err := c.device.Wait(); err != nil {
		slog.Warn("failed waiting for LED controller", "error", err, "action", action)
		return nil, false
	}
	leds := c.device.Leds(config.LEDChannel)
	if len(leds) == 0 {
		slog.Warn("LED controller returned an empty channel buffer", "channel", config.LEDChannel, "action", action)
		return nil, false
	}
	return leds, true
}

func clearFrameBuffer(leds []uint32) {
	for i := range leds {
		leds[i] = 0
	}
}

func packColor(color [3]byte) uint32 {
	// The deployed carts expect the legacy GRB-packed value ordering.
	// Even with StripeType configured, the hardware output only matches the
	// on-screen quantity colors when we swap red/green here before render.
	return uint32(color[1])<<16 | uint32(color[0])<<8 | uint32(color[2])
}
