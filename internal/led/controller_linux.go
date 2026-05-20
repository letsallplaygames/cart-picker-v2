//go:build linux && (arm || arm64) && cgo && ws2811

package led

/*
#cgo CFLAGS: -I/usr/local/include -I/usr/include
#cgo LDFLAGS: -L/usr/local/lib -L/usr/lib -lws2811
#include <stdint.h>
#include <string.h>
#include "ws2811.h"

static ws2811_t ledstring;

static ws2811_return_t led_init(int count, uint8_t brightness) {
    memset(&ledstring, 0, sizeof(ledstring));
    ledstring.freq = 800000;
    ledstring.dmanum = 10;
    ledstring.channel[0].gpionum = 18;
    ledstring.channel[0].invert = 0;
    ledstring.channel[0].count = count;
    ledstring.channel[0].strip_type = WS2811_STRIP_GRB;
    ledstring.channel[0].brightness = brightness;
    return ws2811_init(&ledstring);
}
static void led_set(int index, uint32_t color) {
    if (index >= 0 && index < ledstring.channel[0].count)
        ledstring.channel[0].leds[index] = color;
}
static void led_clear_all(void) {
    for (int i = 0; i < ledstring.channel[0].count; i++)
        ledstring.channel[0].leds[i] = 0;
}
static ws2811_return_t led_render(void) { return ws2811_render(&ledstring); }
static void led_fini(void) { ws2811_fini(&ledstring); }
*/
import "C"

import (
	"log/slog"

	"pickcart/internal/config"
)

type Controller struct {
	ledCount int
	ledMap   map[string]int
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

	ret := C.led_init(C.int(c.ledCount), C.uint8_t(config.LEDBrightness))
	if ret != C.WS2811_SUCCESS {
		slog.Warn("failed to initialize ws2811 LED controller", "error", C.GoString(C.ws2811_get_return_t_str(ret)), "count", c.ledCount, "brightness", config.LEDBrightness, "cart", cartNumber, "profile", profileName)
		return c
	}

	c.ok = true
	c.ClearLEDs()
	slog.Info("initialized ws2811 LED controller", "count", c.ledCount, "brightness", config.LEDBrightness, "cart", cartNumber, "profile", profileName)
	return c
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
	if c == nil || !c.ok || len(locationColors) == 0 {
		return
	}

	c.ClearLEDs()

	for location, color := range locationColors {
		index, ok := c.ledMap[location]
		if !ok {
			continue
		}
		if index < 0 || index >= c.ledCount {
			continue
		}
		C.led_set(C.int(index), C.uint32_t(packColor(color)))
	}

	if ret := C.led_render(); ret != C.WS2811_SUCCESS {
		slog.Warn("failed to render LED highlights", "error", C.GoString(C.ws2811_get_return_t_str(ret)))
	}
}

func (c *Controller) ClearLEDs() {
	if c == nil || !c.ok {
		return
	}

	C.led_clear_all()
	if ret := C.led_render(); ret != C.WS2811_SUCCESS {
		slog.Warn("failed to clear LEDs", "error", C.GoString(C.ws2811_get_return_t_str(ret)))
	}
}

func (c *Controller) Cleanup() {
	if c == nil || !c.ok {
		return
	}

	C.led_clear_all()
	if ret := C.led_render(); ret != C.WS2811_SUCCESS {
		slog.Warn("failed to render LED cleanup", "error", C.GoString(C.ws2811_get_return_t_str(ret)))
	}
	C.led_fini()
	c.ok = false
}

func packColor(color [3]byte) uint32 {
	// The deployed carts use GRB strips, matching the legacy Python behavior
	// that swapped red/green before sending colors to the controller.
	return uint32(color[1])<<16 | uint32(color[0])<<8 | uint32(color[2])
}
