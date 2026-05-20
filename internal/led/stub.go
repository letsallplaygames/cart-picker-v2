//go:build !(linux && (arm || arm64) && cgo && ws2811)

package led

import (
	"log/slog"
	"maps"
)

type Controller struct {
	ledMap map[string]int
}

func New(cartNumber int, profileName string) *Controller {
	c := &Controller{ledMap: map[string]int{}}

	mappings, requiredCount, path, columnIndex, err := loadMappingsForCart(cartNumber, profileName)
	if err != nil {
		slog.Warn("failed to load LED mappings", "error", err, "cart", cartNumber, "profile", profileName)
		return c
	}

	c.LoadMappings(mappings)
	slog.Info("loaded LED mappings", "count", len(mappings), "required_led_count", requiredCount, "source", path, "column_index", columnIndex, "cart", cartNumber, "profile", profileName)
	return c
}

func (c *Controller) LoadMappings(m map[string]int) {
	if c == nil {
		return
	}
	c.ledMap = make(map[string]int, len(m))
	maps.Copy(c.ledMap, m)
}

func (c *Controller) HighlightLocations(locations []string, color [3]byte) {
	locationColors := make(map[string][3]byte, len(locations))
	for _, location := range locations {
		locationColors[location] = color
	}
	c.HighlightLocationColors(locationColors)
}

func (c *Controller) HighlightLocationColors(locationColors map[string][3]byte) {
	if c == nil || len(locationColors) == 0 {
		return
	}
	if len(c.ledMap) == 0 {
		slog.Debug("ignoring LED highlight request because no mappings are loaded", "location_colors", locationColors)
		return
	}
	mapped := 0
	for location := range locationColors {
		if _, ok := c.ledMap[location]; ok {
			mapped++
		}
	}
	slog.Debug("simulating LED highlights", "location_colors", locationColors, "mapped", mapped)
}

func (c *Controller) ClearLEDs() {}

func (c *Controller) Cleanup() {}
