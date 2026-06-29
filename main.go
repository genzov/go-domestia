package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/genzov/go-domestia/bridge"
	"github.com/genzov/go-domestia/config"
)

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

	// Cancel the context on SIGINT/SIGTERM so the bridge can shut down cleanly
	// (marking lights unavailable) instead of being killed mid-flight.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		if err := b.Run(ctx); err == nil {
			// Run only returns nil when the context was cancelled.
			break
		} else {
			log.Errorf("Bridge returned: %v, restarting", err)
		}

		// Back off before restarting, but bail out immediately if we're shutting down.
		select {
		case <-ctx.Done():
			break
		case <-time.After(time.Second):
		}
		if ctx.Err() != nil {
			break
		}
	}

	log.Print("Shutting down")
}
