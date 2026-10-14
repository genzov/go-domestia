package bridge

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/genzov/go-domestia/domestia"
	"github.com/genzov/go-domestia/homeassistant"
)

// maxHomeAssistantBrightness is the highest brightness Home Assistant uses (0-255).
const maxHomeAssistantBrightness = 255

// describeLightState returns a human-readable description of a light's state
// for logging, e.g. "off", "on" (non-dimmable) or "on at 75% (brightness 48/64)".
func describeLightState(l *domestia.Light) string {
	if l.Brightness == 0 {
		return "off"
	}
	if !l.Configuration.Dimmable {
		return "on"
	}

	percent := int(math.Round(float64(l.Brightness) / float64(domestia.MaxBrightness) * 100))
	return fmt.Sprintf("on at %d%% (brightness %d/%d)", percent, l.Brightness, domestia.MaxBrightness)
}

func homeAssistantStateJSON(l *domestia.Light) (string, error) {
	state := &homeassistant.LightState{
		State:      "OFF",
		Brightness: homeAssistantBrightness(l),
	}
	if l.Brightness != 0 {
		state.State = "ON"
	}

	stateMarshalled, err := json.Marshal(state)
	return string(stateMarshalled), err
}

// homeAssistantBrightness converts a Domestia brightness (0-64) to the 0-255 scale
// published over MQTT, rounding to the nearest value rather than truncating.
func homeAssistantBrightness(l *domestia.Light) int {
	scaled := math.Round(float64(l.Brightness) * (maxHomeAssistantBrightness / float64(domestia.MaxBrightness)))
	return int(min(scaled, maxHomeAssistantBrightness))
}

// domestiaBrightness converts a Home Assistant brightness (0-255) to the Domestia
// controller's 0-64 scale, rounding to the nearest value rather than truncating.
// Truncation made low brightness values collapse to 0, switching the light
// off instead of dimming it. The result is clamped to the controller's valid range.
func domestiaBrightness(l *homeassistant.LightState) uint8 {
	scaled := math.Round(float64(l.Brightness) * (float64(domestia.MaxBrightness) / maxHomeAssistantBrightness))
	return uint8(min(max(scaled, 0), float64(domestia.MaxBrightness)))
}
