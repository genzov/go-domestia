package bridge

import (
	"testing"

	"github.com/genzov/go-domestia/domestia"
	"github.com/genzov/go-domestia/homeassistant"
)

func TestDomestiaBrightness(t *testing.T) {
	tests := []struct {
		name string
		ha   int
		want uint8
	}{
		{"off", 0, 0},
		{"max", 255, 63},
		{"low value rounds up instead of collapsing to 0", 4, 1},
		{"smallest value that maps to a dim level", 3, 1},
		{"very low value still rounds to 0", 2, 0},
		{"midpoint rounds to nearest", 128, 32},
		{"out-of-range high value is clamped", 1000, 63},
		{"negative value is clamped to 0", -10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domestiaBrightness(&homeassistant.LightState{Brightness: tt.ha})
			if got != tt.want {
				t.Errorf("domestiaBrightness(%d) = %d, want %d", tt.ha, got, tt.want)
			}
		})
	}
}

func TestHomeAssistantBrightness(t *testing.T) {
	tests := []struct {
		name     string
		domestia uint8
		want     int
	}{
		{"off", 0, 0},
		{"max", 63, 255},
		{"low value", 1, 4},
		{"midpoint rounds to nearest", 32, 130},
		{"out-of-range value is clamped", 64, 255},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := homeAssistantBrightness(&domestia.Light{Brightness: tt.domestia})
			if got != tt.want {
				t.Errorf("homeAssistantBrightness(%d) = %d, want %d", tt.domestia, got, tt.want)
			}
		})
	}
}

// TestBrightnessRoundTrip verifies that converting a Domestia brightness to the
// Home Assistant scale and back yields the original value for every valid level.
func TestBrightnessRoundTrip(t *testing.T) {
	for b := uint8(0); b <= maxDomestiaBrightness; b++ {
		ha := homeAssistantBrightness(&domestia.Light{Brightness: b})
		got := domestiaBrightness(&homeassistant.LightState{Brightness: ha})
		if got != b {
			t.Errorf("round trip for brightness %d: ha=%d, back=%d", b, ha, got)
		}
	}
}
