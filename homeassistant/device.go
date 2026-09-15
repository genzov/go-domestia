package homeassistant

// Device represents the Home Assistant device an entity belongs to. Each light
// is registered as its own device, identified by its relay.
type Device struct {
	Identifiers  []string  `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	SwVersion    string   `json:"sw_version,omitempty"`
}

func NewDevice(id string, name string, swVersion string) *Device {
	return &Device{
		Identifiers:  []string{id},
		Name:         name,
		Manufacturer: "Domestia",
		SwVersion:    swVersion,
	}
}
