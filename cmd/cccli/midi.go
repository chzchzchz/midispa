package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const alsaClientName = "cccli"

// openMIDI opens the ALSA client the editor sends control changes
// from, returning the sequencer to close when the editor quits and
// the writer to send through. The client's own port carries read,
// write and subscription capabilities, so the same sequencer serves
// the input subscriptions openMIDIInput makes on it.
func openMIDI(portName string) (*alsa.Seq, io.Writer, error) {
	if strings.TrimSpace(portName) == "" {
		return nil, nil, fmt.Errorf("MIDI port is empty")
	}
	sequence, err := alsa.OpenSeq(alsaClientName)
	if err != nil {
		return nil, nil, fmt.Errorf("open ALSA sequencer: %w", err)
	}
	address, err := sequence.PortAddress(portName)
	if err != nil {
		sequence.Close()
		return nil, nil, fmt.Errorf("find MIDI port %q: %w", portName, err)
	}
	if err := sequence.OpenPortWrite(address); err != nil {
		sequence.Close()
		return nil, nil, fmt.Errorf("open MIDI port %q: %w", portName, err)
	}
	return sequence, sequence.NewWriter(address), nil
}

// openMIDIInput subscribes the editor's port to incoming control
// changes. A named port is the one the user asked for, so a port that
// refuses the subscription is fatal. With no name every readable port
// is subscribed on a best-effort basis, and a port that refuses, one
// that vanished between the listing and the subscription included, is
// skipped. The editor's own port is skipped in the every-port mode: a
// self-subscription would hand every control change the editor sends
// straight back to it. Zero readable ports is not an error: the editor
// was asked to listen, and silence is not a failure.
func openMIDIInput(seq *alsa.Seq, portName string) error {
	if strings.TrimSpace(portName) == "" {
		devices, err := seq.Devices()
		if err != nil {
			return fmt.Errorf("list MIDI ports: %w", err)
		}
		for _, device := range devices {
			if device.Client == seq.Client {
				continue
			}
			// The subscription is best-effort here: the one
			// realistic refusal is a port that went away since
			// the listing, and one missing port is not worth
			// ending an editor that still has the rest.
			_ = seq.OpenPortRead(device.SeqAddr)
		}
		return nil
	}
	address, err := seq.PortAddress(portName)
	if err != nil {
		return fmt.Errorf("find MIDI input port %q: %w", portName, err)
	}
	if err := seq.OpenPortRead(address); err != nil {
		return fmt.Errorf("open MIDI input port %q: %w", portName, err)
	}
	return nil
}

// sendCC writes one control change. The writer goes through
// snd_seq_event_output_direct, which sends the event at once
// rather than queueing it, so a send from inside Update is
// safe: the value is on the wire before the next keypress is
// read.
func sendCC(out io.Writer, channelIndex, controller, value int) error {
	message := []byte{midi.MakeCC(channelIndex), byte(controller), byte(value)}
	written, err := out.Write(message)
	if err != nil {
		return err
	}
	if written != len(message) {
		return io.ErrShortWrite
	}
	return nil
}
