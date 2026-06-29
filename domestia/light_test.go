package domestia

import (
	"testing"

	"github.com/genzov/go-domestia/config"
)

func TestNewLightBrightness(t *testing.T) {
	tests := []struct {
		name       string
		dimmable   bool
		brightness uint8
		want       uint8
	}{
		{"non-dimmable on normalises 1 to max", false, 1, maxBrightness},
		{"non-dimmable off stays 0", false, 0, 0},
		{"dimmable lowest level stays 1", true, 1, 1},
		{"dimmable off stays 0", true, 0, 0},
		{"dimmable mid value is untouched", true, 32, 32},
		{"dimmable max is untouched", true, maxBrightness, maxBrightness},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Light{Dimmable: tt.dimmable}
			got := NewLight(cfg, tt.brightness).Brightness
			if got != tt.want {
				t.Errorf("NewLight(dimmable=%v, %d).Brightness = %d, want %d", tt.dimmable, tt.brightness, got, tt.want)
			}
		})
	}
}
