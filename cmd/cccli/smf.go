package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	gomidi "gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/midimessage/channel"
	"gitlab.com/gomidi/midi/midimessage/meta"
	"gitlab.com/gomidi/midi/midireader"
	"gitlab.com/gomidi/midi/smf"
	"gitlab.com/gomidi/midi/smf/smfreader"
	"gitlab.com/gomidi/midi/smf/smfwriter"

	"github.com/chzchzchz/midispa/cc"
)

const (
	ccBPM             = 120
	ccTicksPerQuarter = 96
)

// readCCSMF reads the control changes out of a Standard MIDI
// File. Meta events describe the file rather than the MIDI in
// it, so only channel control changes are kept.
func readCCSMF(path string) ([][]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := smfreader.New(file)
	if err := reader.ReadHeader(); err != nil {
		return nil, err
	}
	var messages [][]byte
	for {
		message, err := reader.Read()
		if err == smf.ErrFinished {
			return messages, nil
		}
		if err != nil {
			return nil, err
		}
		messages = appendControlChange(messages, message)
	}
}

// readCCRaw reads the control changes out of a raw MIDI
// stream: the bytes as they would arrive off a cable, with no
// file framing around them. A message that arrives together
// with io.EOF is a sysex the stream ended inside, which is
// refused because half of a dump is worse than none of one.
func readCCRaw(path string) ([][]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := midireader.New(file, nil, midireader.NoteOffVelocity())
	var messages [][]byte
	for {
		message, err := reader.Read()
		if err == io.EOF {
			if message != nil {
				return nil, errors.New("stream ends inside a sysex message")
			}
			if len(messages) == 0 {
				return nil, errors.New("no MIDI messages in the stream")
			}
			return messages, nil
		}
		if err != nil {
			return nil, err
		}
		messages = appendControlChange(messages, message)
	}
}

// appendControlChange keeps one channel control change,
// copied, in the message list and drops every other
// message type, so both readers share the one filter.
func appendControlChange(messages [][]byte, message gomidi.Message) [][]byte {
	if control, ok := message.(channel.ControlChange); ok {
		messages = append(messages, append([]byte(nil), control.Raw()...))
	}
	return messages
}

// readSeedMessages reads a seed file, trying SMF framing first
// and a raw MIDI stream second, so a dump saved either way
// loads. When neither parses, both errors are kept, because
// which one explains the file depends on what the user meant
// it to be.
func readSeedMessages(path string) ([][]byte, error) {
	messages, err := readCCSMF(path)
	if err == nil {
		return messages, nil
	}
	raw, rawErr := readCCRaw(path)
	if rawErr == nil {
		return raw, nil
	}
	return nil, errors.Join(err, rawErr)
}

// writeCCSMF writes every field in declaration order as one
// control change per field, all at delta 0 so the instrument
// receives the whole model at once. Channel numbers stay
// one-based here and are converted where the MIDI library
// requires a zero-based value.
func writeCCSMF(path string, fields []cc.ControlField, modelName string, channelNumber int) error {
	if channelNumber < 1 || channelNumber > 16 {
		return fmt.Errorf("MIDI channel %d is outside 1-16", channelNumber)
	}
	channelIndex := channelNumber - 1
	for _, field := range fields {
		if field.Value == nil || *field.Value < 0 || *field.Value > maxMIDIValue {
			return fmt.Errorf("field %q has value outside 0-%d", field.Name, maxMIDIValue)
		}
	}

	var writeErr error
	err := smfwriter.WriteFile(path, func(midiWriter smf.Writer) {
		if writeErr = midiWriter.Write(meta.Instrument(modelName)); writeErr != nil {
			return
		}
		timeSignature := meta.TimeSig{
			Numerator:                4,
			Denominator:              4,
			ClocksPerClick:           24,
			DemiSemiQuaverPerQuarter: 8,
		}
		if writeErr = midiWriter.Write(timeSignature); writeErr != nil {
			return
		}
		if writeErr = midiWriter.Write(meta.BPM(ccBPM)); writeErr != nil {
			return
		}
		midiWriter.SetDelta(0)
		for _, field := range fields {
			control := channel.Channel(channelIndex).ControlChange(
				byte(field.Controller), byte(*field.Value))
			if writeErr = midiWriter.Write(control); writeErr != nil {
				return
			}
		}
	}, smfwriter.NumTracks(1), smfwriter.TimeFormat(smf.MetricTicks(ccTicksPerQuarter)))
	if writeErr != nil {
		return writeErr
	}
	if err == smf.ErrFinished {
		return nil
	}
	return err
}
