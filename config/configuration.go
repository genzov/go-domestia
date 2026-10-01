package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

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
	if configuration.RefreshFrequency <= 0 {
		return nil, errors.New("refresh_frequency must be greater than 0")
	}

	return configuration, nil
}
