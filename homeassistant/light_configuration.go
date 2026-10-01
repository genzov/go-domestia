package homeassistant

import (
	"fmt"
	"strings"
)

// AvailabilityTopic is the single, bridge-wide topic Home Assistant watches to
// decide whether the lights are reachable. The bridge publishes PayloadAvailable
// once registered and PayloadNotAvailable on a clean shutdown; the same topic is
// also used as the MQTT Last Will so the broker marks the lights unavailable if
// the bridge dies ungracefully.
const AvailabilityTopic = "domestia/bridge/availability"

const (
	PayloadAvailable    = "online"
	PayloadNotAvailable = "offline"
)

// Device represents the Home Assistant device an entity belongs to. Each light
// is registered as its own device, identified by its relay.
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model,omitempty"`
	SwVersion    string   `json:"sw_version,omitempty"`
}

// LightState represents light state on Home Assistant. Either as current state in
// the state update topic, or requested state in command topic.
type LightState struct {
	State      string `json:"state"`
	Brightness int    `json:"brightness"`
}

// LightConfiguration represents a Home Assistant light, as used during light registration
type LightConfiguration struct {
	ConfigTopic string `json:"-"` // Topic the configuration is published on, not part of the payload

	Name              *string `json:"name"` // Always null, so Home Assistant names the entity after its device
	UniqueId          string  `json:"unique_id"`
	CommandTopic      string  `json:"command_topic"`
	StateTopic        string  `json:"state_topic"`
	AvailabilityTopic string  `json:"availability_topic"`
	Schema            string  `json:"schema"`
	Brightness        bool    `json:"brightness"`
	Device            *Device `json:"device,omitempty"`
}

func NewLightConfiguration(name string, uniqueId string, dimmable bool) *LightConfiguration {
	entityId := strings.ReplaceAll(strings.ToLower(name), " ", "_")

	return &LightConfiguration{
		ConfigTopic:       fmt.Sprintf("homeassistant/light/%v/config", entityId),
		UniqueId:          uniqueId,
		CommandTopic:      fmt.Sprintf("domestia/light/%v/set", entityId),
		StateTopic:        fmt.Sprintf("domestia/light/%v/state", entityId),
		AvailabilityTopic: AvailabilityTopic,
		Schema:            "json",
		Brightness:        dimmable,
	}
}
