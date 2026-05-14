package led

type Controller struct {
	ledMap map[string]int
}

func New(cartNumber int, profileName string) *Controller {
	return &Controller{ledMap: map[string]int{}}
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

func (c *Controller) HighlightLocations(locations []string, color [3]byte) {}

func (c *Controller) ClearLEDs() {}

func (c *Controller) Cleanup() {}
