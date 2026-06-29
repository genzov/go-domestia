package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/genzov/go-domestia/config"
	"github.com/genzov/go-domestia/domestia"
	"github.com/genzov/go-domestia/homeassistant"
)

type Bridge struct {
	configuration *config.Configuration
	domestia      *domestia.Client
	mqtt          mqtt.Client

	// Channel to trigger a pull and publish of controller state
	updateChannel chan bool
	// Map to store current brightnesses of lights, used to publish only on changes to state
	relayToBrightness map[uint8]uint8
}

func New(cfg *config.Configuration) (*Bridge, error) {
	domestiaClient, err := domestia.NewClient(cfg.IpAddress, cfg.Lights)
	if err != nil {
		return nil, err
	}

	return &Bridge{
		configuration:     cfg,
		domestia:          domestiaClient,
		relayToBrightness: make(map[uint8]uint8),
		// Buffered so the MQTT callback never blocks when the run loop is busy
		// publishing or has exited; a pending refresh is enough to coalesce.
		updateChannel: make(chan bool, 1),
	}, nil
}

// Run runs the bridge, blocking. If this function returns an error it can be restarted.
// If it returns nil (because ctx was cancelled), it was cleanly shut down.
func (b *Bridge) Run(ctx context.Context) error {
	mqttClient, err := b.connectMQTT()
	if err != nil {
		return err
	}

	defer func() {
		mqttClient.Disconnect(100)
	}()

	b.mqtt = mqttClient

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

		if err := b.publishLightState(); err != nil {
			return err
		}
	}
}

// connectMQTT creates and connects MQTT client
func (b *Bridge) connectMQTT() (mqtt.Client, error) {
	opts := b.configuration.MQTT.ClientOptions()
	// Last Will: if the bridge disconnects ungracefully, the broker publishes
	// "offline" so Home Assistant marks the lights unavailable.
	opts.SetWill(homeassistant.AvailabilityTopic, homeassistant.PayloadNotAvailable, 0, true)
	// Configure MQTT subscriptions in the ConnectHandler to make sure they are set up after reconnect
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		// Log rather than exit: this runs on the MQTT client's goroutine, and
		// killing the process here would defeat both auto-reconnect and the
		// restart-on-error loop in main. The handler fires again on reconnect.
		if err := b.setupLights(client); err != nil {
			log.Errorf("Failed to register with MQTT: %v", err)
			return
		}
		// Lights are registered; announce that the bridge is online.
		b.publishAvailability(client, true)
	})

	mqttClient := mqtt.NewClient(opts)
	if t := mqttClient.Connect(); t.Wait() && t.Error() != nil {
		return nil, fmt.Errorf("MQTT connection error: %w", t.Error())
	}

	return mqttClient, nil
}

// publishAvailability publishes the bridge's availability (retained) so Home
// Assistant knows whether the lights are reachable.
func (b *Bridge) publishAvailability(client mqtt.Client, available bool) {
	payload := homeassistant.PayloadNotAvailable
	if available {
		payload = homeassistant.PayloadAvailable
	}

	if t := client.Publish(homeassistant.AvailabilityTopic, 0, true, payload); t.Wait() && t.Error() != nil {
		log.Errorf("Failed to publish availability %q: %v", payload, t.Error())
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
			log.Errorf("MQTT deserialization failed: %v", err)
			return
		}

		if cmd.State == "ON" {
			log.Printf("Turning on %v", light.Name)
			if err := b.domestia.TurnOn(relay); err != nil {
				log.Errorf("Failed to turn on %v: %v", light.Name, err)
			}

			if !light.Dimmable {
				if err := b.domestia.SetMaxBrightness(relay); err != nil {
					log.Errorf("Failed to set %v to max brightness: %v", light.Name, err)
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
					log.Errorf("Failed to set brightness of %v: %v", light.Name, err)
				}
			}
		} else {
			log.Printf("Turning off %v", light.Name)
			if err := b.domestia.TurnOff(relay); err != nil {
				log.Errorf("Failed to turn off %v: %v", light.Name, err)
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
	if configJson, err := l.HomeAssistantRegistrationJSON(); err != nil {
		return fmt.Errorf("error marshalling light configuration: %v", err)
	} else if t := mqttClient.Publish(configTopic, 0, true, configJson); t.Wait() && t.Error() != nil {
		return fmt.Errorf("MQTT publish failed: %v", t.Error())
	}

	log.Printf("Registered %v with Home Assistant", l.Name)

	return nil
}

// Fetches current state of the controller and publishes updates to mqtt.
// Also makes sure always-on lights are in fact always on. Also makes sure
// that non-dimmable lights are not dimmed.
func (b *Bridge) publishLightState() error {
	domestiaState, err := b.domestia.GetState()

	if err != nil {
		return err
	}

	for _, light := range domestiaState {
		configuration := light.Configuration

		var shouldPublishUpdate bool
		if brightness, present := b.relayToBrightness[configuration.Relay]; !present {
			shouldPublishUpdate = true
		} else {
			shouldPublishUpdate = light.Brightness != brightness
		}

		if configuration.AlwaysOn && !light.IsMaxBrightness() {
			// If the light is always-on, and the brightness is not 100%, set it to 100%
			log.Printf("Turning always-on light %v back on", configuration.Name)

			if err := b.domestia.TurnOn(configuration.Relay); err != nil {
				log.Errorf("Failed to turn on always-on light %v: %v", configuration.Name, err)
			}
			if err := b.domestia.SetMaxBrightness(configuration.Relay); err != nil {
				log.Errorf("Failed to set always-on light %v to max brightness: %v", configuration.Name, err)
			}

			shouldPublishUpdate = false
		} else if !configuration.Dimmable && !light.IsMinBrightness() && !light.IsMaxBrightness() {
			// If the light is not dimmable and on it should always be set to 100%
			log.Printf("Non-dimmable light %v at brightness %v, resetting", configuration.Name, light.Brightness)

			if err := b.domestia.SetMaxBrightness(configuration.Relay); err != nil {
				log.Errorf("Failed to reset non-dimmable light %v: %v", configuration.Name, err)
			}

			shouldPublishUpdate = false
		} else {
			b.relayToBrightness[configuration.Relay] = light.Brightness
		}

		if shouldPublishUpdate {
			log.Printf("%v is now %v", configuration.Name, describeLightState(light))

			stateTopic := configuration.HomeAssistant().StateTopic
			if stateJson, err := homeAssistantStateJSON(light); err != nil {
				return fmt.Errorf("[%v] Error marshalling light state: %v", stateTopic, err)
			} else if t := b.mqtt.Publish(stateTopic, 0, true, stateJson); t.Wait() && t.Error() != nil {
				return fmt.Errorf("[%v] Publish error: %v", stateTopic, t.Error())
			}
		}
	}

	return nil
}
