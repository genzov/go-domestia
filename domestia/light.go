package domestia

import "github.com/genzov/go-domestia/config"

// Light represents a light as retrieved from Domestia controller.
type Light struct {
	Configuration *config.Light
	Brightness    uint8
}

// NewLight creates a new Light struct from given configuration and brightness.
func NewLight(cfg *config.Light, brightness uint8) *Light {
	// A non-dimmable relay reports brightness 1 when it is on; normalise that
	// to full brightness. Dimmable relays use 1 as a legitimate lowest dim
	// level (~2%), so their values must be left untouched.
	if brightness == 1 && !cfg.Dimmable {
		brightness = maxBrightness
	}

	return &Light{
		Configuration: cfg,
		Brightness:    brightness,
	}
}

func (l *Light) IsMaxBrightness() bool {
	return l.Brightness == maxBrightness
}

func (l *Light) IsMinBrightness() bool {
	return l.Brightness == 0
}
