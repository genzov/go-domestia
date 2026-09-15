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

// LightConfiguration represents a Home Assistant light, as used during light registration
type LightConfiguration struct {
	ConfigTopic string

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
	entityId := strings.Replace(strings.ToLower(name), " ", "_", -1)

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
