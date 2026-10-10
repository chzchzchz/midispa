package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

const alsaClientName = "cccli"

// openMIDIOutput opens the ALSA port the editor sends control
// changes on, returning the writer to send through and the
// sequencer to close when the editor quits.
func openMIDIOutput(portName string) (io.Writer, io.Closer, error) {
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
	return sequence.NewWriter(address), sequence, nil
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
