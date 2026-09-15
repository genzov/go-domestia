package config

import (
	"encoding/json"
	"fmt"

	"github.com/genzov/go-domestia/homeassistant"
)

type Light struct {
	Name     string `json:"name"`      // Light name
	Relay    uint8  `json:"relay"`     // Relay number
	Dimmable bool   `json:"dimmable"`  // Whether the light relay is dimmable
	AlwaysOn bool   `json:"always_on"` // Whether the light relay should always be on, hides the relay in home assistant
}

func (l *Light) HomeAssistant() *homeassistant.LightConfiguration {
	return homeassistant.NewLightConfiguration(l.Name, fmt.Sprintf("d_%v", l.Relay), l.Dimmable)
}

// HomeAssistantRegistrationJSON returns the discovery payload for this light,
// registering it as its own device. The device identifier is derived from the
// relay, like the entity's unique_id, so it stays stable across restarts.
func (l *Light) HomeAssistantRegistrationJSON(swVersion string) (string, error) {
	config := l.HomeAssistant()
	config.Device = homeassistant.NewDevice(fmt.Sprintf("domestia_relay_%v", l.Relay), l.Name, swVersion)

	if configMarshalled, err := json.Marshal(config); err != nil {
		return "", err
	} else {
		return string(configMarshalled), nil
	}
}
