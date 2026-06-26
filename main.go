package main

import (
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/genzov/go-domestia/bridge"
	"github.com/genzov/go-domestia/config"
)

// TODO mark everything as unavailable on shutdown

func main() {
	// Config path is overridable via CONFIG_PATH (e.g. /data/options.json when
	// running as a Home Assistant add-on), defaulting to the local file.
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "domestia.json"
	}

	cfg, err := config.LoadConfiguration(configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	log.Printf("Connecting to %v, managing %v relays", cfg.IpAddress, len(cfg.Lights))

	b, err := bridge.New(cfg)
	if err != nil {
		log.Fatalf("Failed to set up bridge: %v", err)
	}

	for {
		if err := b.Run(); err != nil {
			log.Errorf("Bridge returned: %v, restarting", err)
		} else {
			log.Print("Shutting down")
			break
		}

		time.Sleep(time.Second)
	}
}
