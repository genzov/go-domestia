package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/genzov/go-domestia/homeassistant"
)

// MinRefreshFrequency is the shortest allowed poll interval in milliseconds.
// Every poll opens a new TCP connection to the controller, so a tiny interval
// would hammer it.
const MinRefreshFrequency = 250

type Configuration struct {
	Lights           []*Light `json:"lights"`
	MQTT             *MQTT    `json:"mqtt"`
	IpAddress        string   `json:"ip_address"`
	Model            string   `json:"model"` // Controller model reported on each Home Assistant device, optional
	RefreshFrequency int      `json:"refresh_frequency"`
}

// Models lists the supported controller models. The model is only reported to
// Home Assistant; it does not change how the controller is driven.
var Models = []string{"DMC-012-003", "DMC-008-001"}

type MQTT struct {
	IpAddress string `json:"ip_address"`
	Username  string `json:"username"`
	Password  string `json:"password"`
}

func LoadConfiguration(filename string) (*Configuration, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	configuration := &Configuration{
		RefreshFrequency: 2000,
	}
	if err := json.Unmarshal(data, configuration); err != nil {
		return nil, err
	}

	// Validate configuration
	if configuration.IpAddress == "" {
		return nil, errors.New("ip_address is required")
	}
	if configuration.Model != "" && !slices.Contains(Models, configuration.Model) {
		return nil, fmt.Errorf("model must be one of %v", strings.Join(Models, ", "))
	}
	if configuration.MQTT == nil {
		return nil, errors.New("mqtt configuration is required")
	}
	if configuration.RefreshFrequency < MinRefreshFrequency {
		return nil, fmt.Errorf("refresh_frequency must be at least %d", MinRefreshFrequency)
	}
	if err := validateLights(configuration.Lights); err != nil {
		return nil, err
	}

	return configuration, nil
}

// validateLights checks that lights can be told apart on the controller and in
// MQTT. Light names become MQTT topics, so they must not contain topic syntax.
// Always-on lights are never registered with Home Assistant, so their name does
// not end up in a topic.
func validateLights(lights []*Light) error {
	relays := make(map[uint8]string)
	entityIds := make(map[string]string)

	for i, light := range lights {
		if light == nil {
			return fmt.Errorf("lights[%d] is empty", i)
		}
		if light.Relay == 0 {
			return fmt.Errorf("light %q: relay must be at least 1", light.Name)
		}
		if other, ok := relays[light.Relay]; ok {
			return fmt.Errorf("light %q: relay %d is already used by %q", light.Name, light.Relay, other)
		}
		relays[light.Relay] = light.Name

		if light.AlwaysOn {
			continue
		}
		if strings.TrimSpace(light.Name) == "" {
			return fmt.Errorf("light on relay %d: name is required", light.Relay)
		}
		if strings.ContainsAny(light.Name, "+#/\x00") {
			return fmt.Errorf("light %q: name must not contain +, # or /", light.Name)
		}
		entityId := homeassistant.EntityId(light.Name)
		if other, ok := entityIds[entityId]; ok {
			return fmt.Errorf("light %q: name conflicts with %q", light.Name, other)
		}
		entityIds[entityId] = light.Name
	}

	return nil
}
