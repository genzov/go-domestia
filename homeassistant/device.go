package homeassistant

// Device represents the Home Assistant device an entity belongs to. Each light
// is registered as its own device, identified by its relay.
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model,omitempty"`
	SwVersion    string   `json:"sw_version,omitempty"`
}

func NewDevice(id string, name string, model string, swVersion string) *Device {
	return &Device{
		Identifiers:  []string{id},
		Name:         name,
		Manufacturer: "Domestia",
		Model:        model,
		SwVersion:    swVersion,
	}
}
