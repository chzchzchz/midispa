package main

import (
	"errors"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
)

type trackingPorts struct {
	devices []alsa.SeqDevice
	reads   []alsa.SeqAddr
	writes  []alsa.SeqAddr
	err     error
}

func (ports *trackingPorts) DevicesFiltered(dir alsa.PortDir) ([]alsa.SeqDevice, error) {
	if dir != alsa.PortAny {
		return nil, errors.New("tracking must discover both directions")
	}
	return ports.devices, nil
}

func (ports *trackingPorts) OpenPortRead(addr alsa.SeqAddr) error {
	ports.reads = append(ports.reads, addr)
	return ports.err
}

func (ports *trackingPorts) OpenPortWrite(addr alsa.SeqAddr) error {
	ports.writes = append(ports.writes, addr)
	return ports.err
}

func TestTrackingDirection(t *testing.T) {
	addr := alsa.SeqAddr{Client: 128, Port: 1}
	source := alsa.PortCapRead | alsa.PortCapSubsRead
	destination := alsa.PortCapWrite | alsa.PortCapSubsWrite
	for _, test := range []struct {
		name  string
		caps  alsa.PortCaps
		read  bool
		valid bool
	}{
		{"source", source, true, true},
		{"destination", destination, false, true},
		{"duplex", source | destination, false, true},
		{"unsubscribable", alsa.PortCapRead | alsa.PortCapWrite, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ports := &trackingPorts{devices: []alsa.SeqDevice{{SeqAddr: addr, Caps: test.caps}}}
			err := openTrackingPort(ports, addr)
			if (err == nil) != test.valid {
				t.Fatalf("tracking error = %v", err)
			}
			if !test.valid {
				if len(ports.reads)+len(ports.writes) != 0 {
					t.Fatal("subscribed to unsupported port")
				}
				return
			}
			calls := ports.writes
			if test.read {
				calls = ports.reads
			}
			if len(calls) != 1 || calls[0] != addr || len(ports.reads)+len(ports.writes) != 1 {
				t.Fatalf("unexpected subscriptions: reads %v writes %v", ports.reads, ports.writes)
			}
		})
	}
	failure := errors.New("subscription failed")
	ports := &trackingPorts{devices: []alsa.SeqDevice{{SeqAddr: addr, Caps: source | destination}}, err: failure}
	if !errors.Is(openTrackingPort(ports, addr), failure) || len(ports.reads) != 0 {
		t.Fatal("subscription failure hidden by changing direction")
	}
	if openTrackingPort(&trackingPorts{}, addr) == nil {
		t.Fatal("missing port accepted")
	}
}

func TestTrackingDisconnect(t *testing.T) {
	local := alsa.SeqAddr{Client: 128, Port: 2}
	remote := alsa.SeqAddr{Client: 129, Port: 3}
	for _, data := range [][]byte{
		{alsa.EvPortUnsubscribed, 128, 2, 129, 3},
		{alsa.EvPortUnsubscribed, 129, 3, 128, 2},
	} {
		addr, ok := trackingDisconnect(local, data)
		if !ok || addr != remote {
			t.Fatalf("disconnect %v = %v, %v", data, addr, ok)
		}
	}
	for _, data := range [][]byte{
		nil,
		{alsa.EvPortUnsubscribed},
		{alsa.EvPortSubscribed, 128, 2, 129, 3},
		{alsa.EvPortUnsubscribed, 130, 2, 129, 3},
		{0x90, 60, 127},
	} {
		if _, ok := trackingDisconnect(local, data); ok {
			t.Fatalf("unrelated event accepted: %v", data)
		}
	}
}
