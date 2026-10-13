package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigurationModel(t *testing.T) {
	tests := []struct {
		model   string
		wantErr bool
	}{
		{model: "", wantErr: false},
		{model: "DMC-012-003", wantErr: false},
		{model: "DMC-008-001", wantErr: false},
		{model: "DMC-999", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "domestia.json")
			content := `{"ip_address":"192.168.1.2","model":"` + tt.model + `","mqtt":{"ip_address":"core-mosquitto"}}`
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatalf("writing config: %v", err)
			}

			cfg, err := LoadConfiguration(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && cfg.Model != tt.model {
				t.Errorf("Model = %q, want %q", cfg.Model, tt.model)
			}
		})
	}
}

func loadConfigJSON(t *testing.T, content string) (*Configuration, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "domestia.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing config: %v", err)
	}

	return LoadConfiguration(path)
}

func TestLoadConfigurationRefreshFrequency(t *testing.T) {
	tests := []struct {
		name    string
		extra   string
		wantErr bool
	}{
		{"default", ``, false},
		{"minimum", `,"refresh_frequency":250`, false},
		{"too low", `,"refresh_frequency":249`, true},
		{"zero", `,"refresh_frequency":0`, true},
		{"negative", `,"refresh_frequency":-1`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfigJSON(t, `{"ip_address":"192.168.1.2","mqtt":{"ip_address":"core-mosquitto"}`+tt.extra+`}`)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigurationLights(t *testing.T) {
	tests := []struct {
		name    string
		lights  string
		wantErr bool
	}{
		{"valid", `[{"name":"Garden","relay":2},{"name":"Hall","relay":3,"always_on":true}]`, false},
		{"relay 0", `[{"name":"Garden","relay":0}]`, true},
		{"relay missing", `[{"name":"Garden"}]`, true},
		{"duplicate relay", `[{"name":"Garden","relay":2},{"name":"Hall","relay":2}]`, true},
		{"empty name", `[{"name":" ","relay":2}]`, true},
		{"wildcard name", `[{"name":"+","relay":2}]`, true},
		{"multi-level wildcard name", `[{"name":"Garden #","relay":2}]`, true},
		{"slash in name", `[{"name":"Garden/Front","relay":2}]`, true},
		{"names with the same entity id", `[{"name":"Living room","relay":2},{"name":"living_room","relay":3}]`, true},
		{"null light", `[null]`, true},
		{"always-on lights may share a name", `[{"name":"Hall","relay":2,"always_on":true},{"name":"Hall","relay":3,"always_on":true}]`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfigJSON(t, `{"ip_address":"192.168.1.2","mqtt":{"ip_address":"core-mosquitto"},"lights":`+tt.lights+`}`)
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadConfiguration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
