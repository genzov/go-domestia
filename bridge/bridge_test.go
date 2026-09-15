package bridge

import (
	"slices"
	"testing"

	"github.com/genzov/go-domestia/config"
)

func TestOrphanedTopics(t *testing.T) {
	lights := []*config.Light{
		{Name: "Living salon", Relay: 13},
		{Name: "Keuken", Relay: 16},
	}

	retained := map[string][]byte{
		// Current config of a light: kept
		"homeassistant/light/living_salon/config": []byte(`{"unique_id":"d_13","command_topic":"domestia/light/living_salon/set","state_topic":"domestia/light/living_salon/state"}`),
		// Left behind by a rename: config and its state topic are cleared
		"homeassistant/light/woonkamer_salon/config": []byte(`{"unique_id":"d_13","command_topic":"domestia/light/woonkamer_salon/set","state_topic":"domestia/light/woonkamer_salon/state"}`),
		// Orphan pointing at a state topic still in use: only the config is cleared
		"homeassistant/light/keuken_oud/config": []byte(`{"unique_id":"d_16","command_topic":"domestia/light/keuken_oud/set","state_topic":"domestia/light/keuken/state"}`),
		// Another integration using the same unique_id: untouched
		"homeassistant/light/other/config": []byte(`{"unique_id":"d_13","command_topic":"zigbee2mqtt/other/set","state_topic":"zigbee2mqtt/other"}`),
		// Light no longer configured: untouched
		"homeassistant/light/removed/config": []byte(`{"unique_id":"d_99","command_topic":"domestia/light/removed/set","state_topic":"domestia/light/removed/state"}`),
		// Already cleared or unparseable: ignored
		"homeassistant/light/cleared/config": []byte(``),
		"homeassistant/light/garbage/config": []byte(`not json`),
	}

	got := orphanedTopics(retained, lights)
	want := []string{
		"domestia/light/woonkamer_salon/state",
		"homeassistant/light/keuken_oud/config",
		"homeassistant/light/woonkamer_salon/config",
	}

	if !slices.Equal(got, want) {
		t.Errorf("orphanedTopics() = %v, want %v", got, want)
	}
}
