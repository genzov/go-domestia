package domestia

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/genzov/go-domestia/config"
)

// controllerPort is the TCP port the Domestia controller listens on.
const controllerPort = 52001

// requestTimeout bounds connecting to the controller and the full
// request/response exchange. The controller occasionally takes over a second to
// answer, so this is deliberately generous.
const requestTimeout = 3 * time.Second

// MaxBrightness is the highest brightness value the controller accepts and
// reports (the dim scale runs 0-64; a relay turned fully on reads back as 64).
const MaxBrightness uint8 = 64

type Client struct {
	mutex              sync.Mutex
	ipAddress          string
	lightConfiguration map[uint8]*config.Light
}

func NewClient(ipAddress string, lights []*config.Light) *Client {
	lightConfiguration := make(map[uint8]*config.Light)

	for _, light := range lights {
		lightConfiguration[light.Relay] = light
	}

	return &Client{
		ipAddress:          ipAddress,
		lightConfiguration: lightConfiguration,
	}
}

func (d *Client) connect() (net.Conn, error) {
	connectURL := fmt.Sprintf("%v:%v", d.ipAddress, controllerPort)

	conn, err := net.DialTimeout("tcp", connectURL, requestTimeout)
	if err != nil {
		return nil, err
	}
	if err := conn.SetDeadline(time.Now().Add(requestTimeout)); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return conn, nil
}

// GetState queries the controller and returns the current state of all lights.
func (d *Client) GetState() ([]*Light, error) {
	response, err := d.send([]byte{0x3c})
	if err != nil {
		return nil, err
	}

	var lights []*Light

	// Response layout: 0xff header, two status bytes, then one byte per relay.
	if len(response) < 4 || response[0] != 0xff {
		return lights, nil
	}

	for i, relayByte := range response[3:] {
		if cfg, ok := d.lightConfiguration[uint8(i+1)]; ok {
			lights = append(lights, NewLight(cfg, relayByte))
		}
	}

	return lights, nil
}

// TurnOff turns off relay with given index.
func (d *Client) TurnOff(relay uint8) error {
	return d.command(0x0f, relay)
}

// TurnOn turns on relay with given index.
func (d *Client) TurnOn(relay uint8) error {
	return d.command(0x0e, relay)
}

// SetBrightness sets brightness of relay to given brightness, between 0 and MaxBrightness.
func (d *Client) SetBrightness(relay uint8, brightness uint8) error {
	return d.command(0x10, relay, brightness)
}

// SetMaxBrightness sets given relay index to maximum brightness.
func (d *Client) SetMaxBrightness(relay uint8) error {
	return d.SetBrightness(relay, MaxBrightness)
}

// command sends a command the controller acknowledges with "OK".
func (d *Client) command(cmd ...byte) error {
	response, err := d.send(cmd)
	if err != nil {
		return err
	}
	if string(response) != "OK" {
		return fmt.Errorf("unexpected command response. Expected OK, received %v", string(response))
	}

	return nil
}

func (d *Client) send(command []byte) ([]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	conn, err := d.connect()
	if err != nil {
		return nil, err
	}
	// Throwaway per-request connection; a close error here is not actionable.
	defer func() { _ = conn.Close() }()

	response := make([]byte, 256)

	if _, err := conn.Write(packCommand(command)); err != nil {
		return nil, err
	} else if n, err := conn.Read(response); err != nil {
		return nil, err
	} else if n == 0 {
		return nil, errors.New("read 0 bytes")
	} else {
		// Return only the bytes actually read; the remaining buffer is zero
		// padding that would otherwise break response comparisons (e.g. "OK").
		return response[:n], nil
	}
}

// packCommand packs a command into a controller message: header, length,
// command, checksum.
func packCommand(cmd []byte) []byte {
	msg := append([]byte{0xff, 0x00, 0x00, byte(len(cmd))}, cmd...)

	var checksum uint8
	for _, c := range cmd {
		checksum += c
	}

	return append(msg, checksum)
}
