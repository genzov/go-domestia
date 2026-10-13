package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/genzov/go-domestia/config"
	"github.com/genzov/go-domestia/domestia"
	"github.com/genzov/go-domestia/homeassistant"
)

type Bridge struct {
	configuration *config.Configuration
	domestia      *domestia.Client
	// Software version reported on each light's Home Assistant device
	version string

	// Channel to trigger a pull and publish of controller state
	updateChannel chan bool
	// Map to store current brightnesses of lights, used to publish only on changes to state
	relayToBrightness map[uint8]uint8

	// Number of consecutive failed controller polls
	controllerFailures int
	// Whether the controller is considered reachable; read from the MQTT
	// connect handler goroutine, so it is atomic.
	controllerUnavailable atomic.Bool
}

// maxControllerFailures is how many consecutive controller polls may fail
// before the lights are marked unavailable in Home Assistant.
const maxControllerFailures = 3

// discoveryConfigTopics matches the Home Assistant discovery configs of all lights.
const discoveryConfigTopics = "homeassistant/light/+/config"

// orphanCollectWindow is how long to collect retained discovery configs when
// looking for orphans; the broker sends them right after subscribing.
const orphanCollectWindow = 2 * time.Second

// lightTopicPrefix prefixes the command and state topics of the bridge's lights,
// used to recognise discovery configs published by the bridge.
const lightTopicPrefix = "domestia/light/"

// New creates a bridge. version is reported to Home Assistant as the device's
// software version.
func New(cfg *config.Configuration, version string) *Bridge {
	return &Bridge{
		configuration:     cfg,
		domestia:          domestia.NewClient(cfg.IpAddress, cfg.Lights),
		version:           version,
		relayToBrightness: make(map[uint8]uint8),
		// Buffered so the MQTT callback never blocks when the run loop is busy
		// publishing or has exited; a pending refresh is enough to coalesce.
		updateChannel: make(chan bool, 1),
	}
}

// Run runs the bridge, blocking. If this function returns an error it can be restarted.
// If it returns nil (because ctx was cancelled), it was cleanly shut down.
func (b *Bridge) Run(ctx context.Context) error {
	mqttClient, err := b.connectMQTT()
	if err != nil {
		return err
	}

	defer mqttClient.Disconnect(100)

	b.removeOrphanedConfigs(ctx, mqttClient)

	ticker := time.NewTicker(time.Duration(b.configuration.RefreshFrequency) * time.Millisecond)
	defer ticker.Stop()

	// Loop to poll controller and publish state updates
	for {
		select {
		case <-ctx.Done():
			// Clean shutdown: tell Home Assistant the lights are unavailable.
			// A graceful Disconnect suppresses the Last Will, so we publish it
			// ourselves before tearing the connection down.
			b.publishAvailability(mqttClient, false)
			return nil
		case <-ticker.C:
		case <-b.updateChannel:
		}

		// Controller errors are transient (every request uses a fresh
		// connection), so restarting the bridge would not help and would only
		// cause needless MQTT reconnects and re-registration. Retry on the next
		// tick instead.
		domestiaState, err := b.domestia.GetState()
		if err != nil {
			b.handleControllerFailure(mqttClient, err)
			continue
		}
		b.handleControllerSuccess(mqttClient)

		if err := b.publishLightState(mqttClient, domestiaState); err != nil {
			return err
		}
	}
}

// handleControllerFailure records a failed poll and marks the lights
// unavailable once the controller has failed repeatedly.
func (b *Bridge) handleControllerFailure(client mqtt.Client, err error) {
	b.controllerFailures++

	if b.controllerFailures < maxControllerFailures {
		slog.Debug("Failed to fetch controller state", "attempt", b.controllerFailures, "err", err)
	} else if !b.controllerUnavailable.Swap(true) {
		slog.Error("Failed to fetch controller state repeatedly, marking lights unavailable", "attempts", b.controllerFailures, "err", err)
		b.publishAvailability(client, false)
	}
}

// handleControllerSuccess resets the failure count and marks the lights
// available again if they had been marked unavailable.
func (b *Bridge) handleControllerSuccess(client mqtt.Client) {
	b.controllerFailures = 0

	if b.controllerUnavailable.Swap(false) {
		slog.Info("Controller reachable again, marking lights available")
		b.publishAvailability(client, true)
	}
}

// connectMQTT creates and connects MQTT client
func (b *Bridge) connectMQTT() (mqtt.Client, error) {
	cfg := b.configuration.MQTT
	opts := mqtt.NewClientOptions().
		AddBroker(fmt.Sprintf("tcp://%v:1883", cfg.IpAddress)).
		SetClientID("go-domestia").
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			slog.Warn("MQTT connection lost", "err", err)
		}).
		SetReconnectingHandler(func(mqtt.Client, *mqtt.ClientOptions) {
			slog.Info("MQTT reconnecting")
		}).
		// Last Will: if the bridge disconnects ungracefully, the broker publishes
		// "offline" so Home Assistant marks the lights unavailable.
		SetWill(homeassistant.AvailabilityTopic, homeassistant.PayloadNotAvailable, 0, true)
	// Configure MQTT subscriptions in the ConnectHandler to make sure they are set up after reconnect
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		// Log rather than exit: this runs on the MQTT client's goroutine, and
		// killing the process here would defeat both auto-reconnect and the
		// restart-on-error loop in main. The handler fires again on reconnect.
		if err := b.setupLights(client); err != nil {
			slog.Error("Failed to register with MQTT", "err", err)
			return
		}
		// Lights are registered; announce that the bridge is online, unless the
		// controller is currently unreachable.
		b.publishAvailability(client, !b.controllerUnavailable.Load())
	})

	mqttClient := mqtt.NewClient(opts)
	if t := mqttClient.Connect(); t.Wait() && t.Error() != nil {
		return nil, fmt.Errorf("MQTT connection error: %w", t.Error())
	}

	return mqttClient, nil
}

// removeOrphanedConfigs clears retained discovery configs left behind when a
// light was renamed (config topics derive from the name). Such an orphan shares
// the light's unique_id, so Home Assistant may keep the stale entity and ignore
// updates to the current one.
func (b *Bridge) removeOrphanedConfigs(ctx context.Context, client mqtt.Client) {
	var mutex sync.Mutex
	retained := make(map[string][]byte)

	if t := client.Subscribe(discoveryConfigTopics, 0, func(_ mqtt.Client, msg mqtt.Message) {
		mutex.Lock()
		defer mutex.Unlock()
		retained[msg.Topic()] = msg.Payload()
	}); t.Wait() && t.Error() != nil {
		slog.Warn("Failed to look for orphaned discovery configs", "err", t.Error())
		return
	}

	select {
	case <-ctx.Done():
	case <-time.After(orphanCollectWindow):
	}

	if t := client.Unsubscribe(discoveryConfigTopics); t.Wait() && t.Error() != nil {
		slog.Warn("Failed to unsubscribe from discovery configs", "err", t.Error())
	}

	mutex.Lock()
	defer mutex.Unlock()

	for _, topic := range orphanedTopics(retained, b.configuration.Lights) {
		slog.Info("Removing orphaned retained message", "topic", topic)
		if t := client.Publish(topic, 0, true, ""); t.Wait() && t.Error() != nil {
			slog.Warn("Failed to remove orphaned retained message", "topic", topic, "err", t.Error())
		}
	}
}

// orphanedTopics returns, sorted, the retained topics to clear: discovery configs
// published by the bridge that carry a configured light's unique_id under a topic
// other than that light's current config topic, plus the stale state topics they
// reference.
func orphanedTopics(retained map[string][]byte, lights []*config.Light) []string {
	currentByUniqueId := make(map[string]*homeassistant.LightConfiguration)
	currentStateTopics := make(map[string]bool)
	for _, light := range lights {
		current := light.HomeAssistant()
		currentByUniqueId[current.UniqueId] = current
		currentStateTopics[current.StateTopic] = true
	}

	var orphans []string
	for topic, payload := range retained {
		var discovered homeassistant.LightConfiguration
		if len(payload) == 0 || json.Unmarshal(payload, &discovered) != nil {
			continue
		}

		// Only touch configs the bridge published itself
		if !strings.HasPrefix(discovered.CommandTopic, lightTopicPrefix) {
			continue
		}

		current, ours := currentByUniqueId[discovered.UniqueId]
		if !ours || topic == current.ConfigTopic {
			continue
		}

		orphans = append(orphans, topic)
		if strings.HasPrefix(discovered.StateTopic, lightTopicPrefix) && !currentStateTopics[discovered.StateTopic] {
			orphans = append(orphans, discovered.StateTopic)
		}
	}

	slices.Sort(orphans)

	return orphans
}

// publishAvailability publishes the bridge's availability (retained) so Home
// Assistant knows whether the lights are reachable.
func (b *Bridge) publishAvailability(client mqtt.Client, available bool) {
	payload := homeassistant.PayloadNotAvailable
	if available {
		payload = homeassistant.PayloadAvailable
	}

	if t := client.Publish(homeassistant.AvailabilityTopic, 0, true, payload); t.Wait() && t.Error() != nil {
		slog.Error("Failed to publish availability", "payload", payload, "err", t.Error())
	}
}

// setupLights publishes Home Assistant configuration and subscribes to state updates
func (b *Bridge) setupLights(mqttClient mqtt.Client) error {
	for _, light := range b.configuration.Lights {
		// Always-on lights are not registered with Home Assistant
		if light.AlwaysOn {
			continue
		}

		// Register light with Home Assistant
		if err := b.registerLight(mqttClient, light); err != nil {
			return err
		}

		// Subscribe to command topic
		if t := mqttClient.Subscribe(light.HomeAssistant().CommandTopic, 0, b.lightSubscriptionCallback(light)); t.Wait() && t.Error() != nil {
			return fmt.Errorf("MQTT receive error: %v", t.Error())
		}
	}

	return nil
}

// lightSubscriptionCallback creates callback to handle messages on light command topic
func (b *Bridge) lightSubscriptionCallback(light *config.Light) func(mqttClient mqtt.Client, msg mqtt.Message) {
	return func(mqttClient mqtt.Client, msg mqtt.Message) {
		relay := light.Relay
		cmd := &homeassistant.LightState{}
		if err := json.Unmarshal(msg.Payload(), cmd); err != nil {
			slog.Error("MQTT deserialization failed", "err", err)
			return
		}

		if cmd.State == "ON" {
			slog.Info("Turning on", "light", light.Name)
			if err := b.domestia.TurnOn(relay); err != nil {
				slog.Error("Failed to turn on", "light", light.Name, "err", err)
			}

			if !light.Dimmable {
				if err := b.domestia.SetMaxBrightness(relay); err != nil {
					slog.Error("Failed to set max brightness", "light", light.Name, "err", err)
				}
			} else if cmd.Brightness != 0 {
				brightness := domestiaBrightness(cmd)
				// A non-zero Home Assistant brightness must not round down to 0,
				// which would switch the light off instead of dimming it to its
				// lowest level. Floor it to the controller's minimum on level.
				if brightness == 0 {
					brightness = 1
				}

				if err := b.domestia.SetBrightness(relay, brightness); err != nil {
					slog.Error("Failed to set brightness", "light", light.Name, "err", err)
				}
			}
		} else {
			slog.Info("Turning off", "light", light.Name)
			if err := b.domestia.TurnOff(relay); err != nil {
				slog.Error("Failed to turn off", "light", light.Name, "err", err)
			}
		}

		// Trigger pulling and publishing controller state. Non-blocking: if a
		// refresh is already pending the ticker will pick up this change too.
		select {
		case b.updateChannel <- true:
		default:
		}
	}
}

// registerLight registers a light with Home Assistant
func (b *Bridge) registerLight(mqttClient mqtt.Client, l *config.Light) error {
	configTopic := l.HomeAssistant().ConfigTopic
	if configJson, err := l.HomeAssistantRegistrationJSON(b.configuration.Model, b.version); err != nil {
		return fmt.Errorf("error marshalling light configuration: %v", err)
	} else if t := mqttClient.Publish(configTopic, 0, true, configJson); t.Wait() && t.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", t.Error())
	}

	slog.Info("Registered with Home Assistant", "light", l.Name)

	return nil
}

// Publishes updates of the given controller state to mqtt.
// Also makes sure always-on lights are in fact always on. Also makes sure
// that non-dimmable lights are not dimmed.
func (b *Bridge) publishLightState(mqttClient mqtt.Client, domestiaState []*domestia.Light) error {
	for _, light := range domestiaState {
		configuration := light.Configuration

		brightness, seen := b.relayToBrightness[configuration.Relay]
		shouldPublishUpdate := !seen || light.Brightness != brightness

		if configuration.AlwaysOn && !light.IsMaxBrightness() {
			// If the light is always-on, and the brightness is not 100%, set it to 100%
			slog.Info("Turning always-on light back on", "light", configuration.Name)

			if err := b.domestia.TurnOn(configuration.Relay); err != nil {
				slog.Error("Failed to turn on always-on light", "light", configuration.Name, "err", err)
			}
			if err := b.domestia.SetMaxBrightness(configuration.Relay); err != nil {
				slog.Error("Failed to set always-on light to max brightness", "light", configuration.Name, "err", err)
			}

			shouldPublishUpdate = false
		} else if !configuration.Dimmable && light.Brightness != 0 && !light.IsMaxBrightness() {
			// If the light is not dimmable and on it should always be set to 100%
			slog.Info("Non-dimmable light dimmed, resetting", "light", configuration.Name, "brightness", light.Brightness)

			if err := b.domestia.SetMaxBrightness(configuration.Relay); err != nil {
				slog.Error("Failed to reset non-dimmable light", "light", configuration.Name, "err", err)
			}

			shouldPublishUpdate = false
		} else {
			b.relayToBrightness[configuration.Relay] = light.Brightness
		}

		if shouldPublishUpdate {
			slog.Info("Light state changed", "light", configuration.Name, "state", describeLightState(light))

			stateTopic := configuration.HomeAssistant().StateTopic
			if stateJson, err := homeAssistantStateJSON(light); err != nil {
				return fmt.Errorf("[%v] Error marshalling light state: %v", stateTopic, err)
			} else if t := mqttClient.Publish(stateTopic, 0, true, stateJson); t.Wait() && t.Error() != nil {
				return fmt.Errorf("[%v] Publish error: %v", stateTopic, t.Error())
			}
		}
	}

	return nil
}
