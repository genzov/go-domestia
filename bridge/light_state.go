package bridge

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/genzov/go-domestia/domestia"
	"github.com/genzov/go-domestia/homeassistant"
)

const (
	// maxDomestiaBrightness is the highest brightness the Domestia controller accepts and reports (0-64).
	maxDomestiaBrightness = 64
	// maxHomeAssistantBrightness is the highest brightness Home Assistant uses (0-255).
	maxHomeAssistantBrightness = 255
)

// describeLightState returns a human-readable description of a light's state
// for logging, e.g. "off", "on" (non-dimmable) or "on at 75% (brightness 47/63)".
func describeLightState(l *domestia.Light) string {
	if l.Brightness == 0 {
		return "off"
	}
	if !l.Configuration.Dimmable {
		return "on"
	}

	percent := int(math.Round(float64(l.Brightness) / maxDomestiaBrightness * 100))
	return fmt.Sprintf("on at %d%% (brightness %d/%d)", percent, l.Brightness, maxDomestiaBrightness)
}

func homeAssistantStateJSON(l *domestia.Light) (string, error) {
	state := &homeassistant.LightState{
		Brightness: homeAssistantBrightness(l),
	}

	if l.Brightness != 0 {
		state.State = "ON"
	} else {
		state.State = "OFF"
	}

	if stateMarshalled, err := json.Marshal(state); err != nil {
		return "", err
	} else {
		return string(stateMarshalled), nil
	}
}

// homeAssistantBrightness converts a Domestia brightness (0-64) to the 0-255 scale
// published over MQTT, rounding to the nearest value rather than truncating.
func homeAssistantBrightness(l *domestia.Light) int {
	scaled := math.Round(float64(l.Brightness) * (maxHomeAssistantBrightness / float64(maxDomestiaBrightness)))
	if scaled > maxHomeAssistantBrightness {
		scaled = maxHomeAssistantBrightness
	}

	return int(scaled)
}

// domestiaBrightness converts a Home Assistant brightness (0-255) to the Domestia
// controller's 0-64 scale, rounding to the nearest value rather than truncating.
// Truncation made low brightness values collapse to 0, switching the light
// off instead of dimming it. The result is clamped to the controller's valid range.
func domestiaBrightness(l *homeassistant.LightState) uint8 {
	scaled := math.Round(float64(l.Brightness) * (float64(maxDomestiaBrightness) / maxHomeAssistantBrightness))
	if scaled > maxDomestiaBrightness {
		scaled = maxDomestiaBrightness
	}
	if scaled < 0 {
		scaled = 0
	}

	return uint8(scaled)
}
