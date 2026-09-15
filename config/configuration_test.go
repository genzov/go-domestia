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
