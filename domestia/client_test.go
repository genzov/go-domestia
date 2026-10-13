package domestia

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/genzov/go-domestia/config"
)

func TestParseState(t *testing.T) {
	client := NewClient("192.168.1.2", []*config.Light{
		{Name: "Garden", Relay: 2, Dimmable: true},
		{Name: "Hall", Relay: 3},
	})

	lights, err := client.parseState([]byte{0xff, 0x00, 0x00, 0, 32, 1, 64})
	if err != nil {
		t.Fatalf("parseState() error = %v", err)
	}
	if len(lights) != 2 {
		t.Fatalf("parseState() returned %d lights, want 2 (unconfigured relays skipped)", len(lights))
	}
	if lights[0].Configuration.Relay != 2 || lights[0].Brightness != 32 {
		t.Errorf("lights[0] = relay %d brightness %d, want relay 2 brightness 32", lights[0].Configuration.Relay, lights[0].Brightness)
	}
	if lights[1].Configuration.Relay != 3 || lights[1].Brightness != MaxBrightness {
		t.Errorf("lights[1] = relay %d brightness %d, want relay 3 brightness %d (non-dimmable 1 normalised)", lights[1].Configuration.Relay, lights[1].Brightness, MaxBrightness)
	}
}

func TestParseStateRejectsMalformedResponses(t *testing.T) {
	client := NewClient("192.168.1.2", nil)

	tests := map[string][]byte{
		"empty":            {},
		"OK":               []byte("OK"),
		"header only":      {0xff, 0x00, 0x00},
		"wrong first byte": {0x00, 0x00, 0x00, 0x01},
	}

	for name, response := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := client.parseState(response); err == nil {
				t.Errorf("parseState(% x) error = nil, want error", response)
			}
		})
	}
}

// serve writes the given chunks to the client side of a pipe, pausing between
// them, and returns the client side.
func serve(t *testing.T, chunks ...[]byte) net.Conn {
	t.Helper()

	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })

	go func() {
		defer func() { _ = server.Close() }()
		for _, chunk := range chunks {
			if _, err := server.Write(chunk); err != nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	return client
}

func TestReadResponse(t *testing.T) {
	tests := []struct {
		name      string
		chunks    [][]byte
		minLength int
		want      []byte
	}{
		{"complete reply", [][]byte{[]byte("OK")}, 2, []byte("OK")},
		{"reply split over segments", [][]byte{[]byte("O"), []byte("K")}, 2, []byte("OK")},
		{"state split over segments", [][]byte{{0xff, 0x00}, {0x00, 0, 64}, {64}}, 6, []byte{0xff, 0x00, 0x00, 0, 64, 64}},
		{"reply longer than the minimum", [][]byte{{0xff, 0x00, 0x00, 1, 2, 3, 4}}, 5, []byte{0xff, 0x00, 0x00, 1, 2, 3, 4}},
		{"controller with fewer relays than configured", [][]byte{{0xff, 0x00, 0x00, 1, 2}}, 15, []byte{0xff, 0x00, 0x00, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readResponse(serve(t, tt.chunks...), tt.minLength)
			if err != nil {
				t.Fatalf("readResponse() error = %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Errorf("readResponse() = % x, want % x", got, tt.want)
			}
		})
	}
}

func TestReadResponseEmpty(t *testing.T) {
	if _, err := readResponse(serve(t), 2); err == nil {
		t.Error("readResponse() error = nil, want error for a reply without bytes")
	}
}
