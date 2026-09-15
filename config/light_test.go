package config

import (
	"encoding/json"
	"testing"
)

func TestHomeAssistantRegistrationJSONRegistersDevice(t *testing.T) {
	light := &Light{Name: "Living room", Relay: 13, Dimmable: true}

	payload, err := light.HomeAssistantRegistrationJSON("DMC-012-003", "1.2.0")
	if err != nil {
		t.Fatalf("HomeAssistantRegistrationJSON() error = %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}

	// The unique_id must not change, or Home Assistant creates duplicate entities.
	if got["unique_id"] != "d_13" {
		t.Errorf("unique_id = %v, want d_13", got["unique_id"])
	}

	// The entity name must be an explicit null so the entity takes the device's name.
	if name, present := got["name"]; !present || name != nil {
		t.Errorf("name = %v (present: %v), want explicit null", name, present)
	}

	gotDevice, ok := got["device"].(map[string]any)
	if !ok {
		t.Fatalf("device missing from payload: %s", payload)
	}
	if ids, _ := gotDevice["identifiers"].([]any); len(ids) != 1 || ids[0] != "domestia_relay_13" {
		t.Errorf("device.identifiers = %v, want [domestia_relay_13]", gotDevice["identifiers"])
	}
	if gotDevice["name"] != "Living room" {
		t.Errorf("device.name = %v, want Living room", gotDevice["name"])
	}
	if gotDevice["model"] != "DMC-012-003" {
		t.Errorf("device.model = %v, want DMC-012-003", gotDevice["model"])
	}
	if gotDevice["sw_version"] != "1.2.0" {
		t.Errorf("device.sw_version = %v, want 1.2.0", gotDevice["sw_version"])
	}
}

func TestHomeAssistantRegistrationJSONOmitsEmptyModelAndVersion(t *testing.T) {
	payload, err := (&Light{Name: "Garden", Relay: 2}).HomeAssistantRegistrationJSON("", "")
	if err != nil {
		t.Fatalf("HomeAssistantRegistrationJSON() error = %v", err)
	}

	var got struct {
		Device map[string]any `json:"device"`
	}
	if err := json.Unmarshal([]byte(payload), &got); err != nil {
		t.Fatalf("payload is not valid JSON: %v", err)
	}
	for _, key := range []string{"model", "sw_version"} {
		if _, present := got.Device[key]; present {
			t.Errorf("%v should be omitted when empty, got %v", key, got.Device[key])
		}
	}
}
