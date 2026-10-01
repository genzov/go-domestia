package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/genzov/go-domestia/bridge"
	"github.com/genzov/go-domestia/config"
)

// version is set at build time via -ldflags "-X main.version=<version>" and
// reported to Home Assistant as the device's software version.
var version string

func main() {
	// Config path is overridable via CONFIG_PATH (e.g. /data/options.json when
	// running as a Home Assistant add-on), defaulting to the local file.
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "domestia.json"
	}

	cfg, err := config.LoadConfiguration(configPath)
	if err != nil {
		slog.Error("Error loading configuration", "err", err)
		os.Exit(1)
	}

	slog.Info("Connecting to controller", "ip_address", cfg.IpAddress, "relays", len(cfg.Lights))

	b := bridge.New(cfg, version)

	// Cancel the context on SIGINT/SIGTERM so the bridge can shut down cleanly
	// (marking lights unavailable) instead of being killed mid-flight.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	for {
		err := b.Run(ctx)
		if err == nil {
			// Run only returns nil when the context was cancelled.
			break
		}
		slog.Error("Bridge failed, restarting", "err", err)

		// Back off before restarting, but bail out immediately if we're shutting down.
		select {
		case <-ctx.Done():
		case <-time.After(time.Second):
		}
		if ctx.Err() != nil {
			break
		}
	}

	slog.Info("Shutting down")
}
