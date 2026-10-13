package domestia

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
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

// continuationTimeout bounds the wait for the rest of a reply after the first
// bytes arrived. The controller may split a reply over several TCP segments, but
// a reply that is complete when it first arrives never waits for this.
const continuationTimeout = 250 * time.Millisecond

// commandReplyLength is the length of the "OK" reply to a command.
const commandReplyLength = 2

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
	// The reply has a three byte header followed by one byte per relay, so
	// it is complete once the highest configured relay has been received.
	var highestRelay uint8
	for relay := range d.lightConfiguration {
		highestRelay = max(highestRelay, relay)
	}

	response, err := d.send([]byte{0x3c}, stateHeaderLength+int(highestRelay))
	if err != nil {
		return nil, err
	}

	return d.parseState(response)
}

// stateHeaderLength is the length of the get-state reply header: 0xff and two
// status bytes.
const stateHeaderLength = 3

// parseState turns a get-state reply into lights. Relays that are not
// configured are skipped. A reply that is not a get-state reply is an error, so
// the caller does not mistake it for a controller without lights.
func (d *Client) parseState(response []byte) ([]*Light, error) {
	if len(response) <= stateHeaderLength || response[0] != 0xff {
		return nil, fmt.Errorf("unexpected state response: % x", response)
	}

	var lights []*Light

	for i, relayByte := range response[stateHeaderLength:] {
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
	response, err := d.send(cmd, commandReplyLength)
	if err != nil {
		return err
	}
	if string(response) != "OK" {
		return fmt.Errorf("unexpected command response. Expected OK, received %v", string(response))
	}

	return nil
}

// send sends a command and returns the reply, which is expected to be at least
// minLength bytes long.
func (d *Client) send(command []byte, minLength int) ([]byte, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	conn, err := d.connect()
	if err != nil {
		return nil, err
	}
	// Throwaway per-request connection; a close error here is not actionable.
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write(packCommand(command)); err != nil {
		return nil, err
	}

	return readResponse(conn, minLength)
}

// readResponse reads a reply of at least minLength bytes. A single Read may
// return only part of a reply, so it keeps reading until the reply is complete.
// If the controller goes quiet (or closes the connection) after sending some
// bytes, those bytes are returned so the caller can judge them; a reply that
// is shorter than expected, for example from a controller with fewer relays than
// configured, then costs at most continuationTimeout.
func readResponse(conn net.Conn, minLength int) ([]byte, error) {
	// Sized to hold any reply to a get-state request (one byte per relay)
	response := make([]byte, 256)
	total := 0

	for total < max(minLength, 1) {
		n, err := conn.Read(response[total:])
		total += n

		if err != nil {
			if total > 0 && (errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, io.EOF)) {
				break
			}
			return nil, err
		}
		if total == len(response) {
			break
		}

		// Whatever arrives next is the rest of an incomplete reply; don't wait
		// the full request timeout for it.
		if err := conn.SetReadDeadline(time.Now().Add(continuationTimeout)); err != nil {
			return nil, err
		}
	}

	if total == 0 {
		return nil, errors.New("read 0 bytes")
	}

	// Return only the bytes actually read; the remaining buffer is zero
	// padding that would otherwise break response comparisons (e.g. "OK").
	return response[:total], nil
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
