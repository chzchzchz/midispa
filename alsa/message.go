package alsa

import (
	"fmt"

	"github.com/chzchzchz/midispa/midi"
)

const messageDumpLimit = 32

type InvalidMessageError struct {
	Status byte
	Reason string
}

func (err *InvalidMessageError) Error() string {
	return fmt.Sprintf("invalid MIDI %s (0x%02x): %s", midi.MessageName(err.Status), err.Status, err.Reason)
}

type UnsupportedMessageError struct {
	Status byte
}

func (err *UnsupportedMessageError) Error() string {
	return fmt.Sprintf("unsupported MIDI %s (0x%02x)", midi.MessageName(err.Status), err.Status)
}

func validateMessage(data []byte) (err error) {
	defer func() {
		if err != nil {
			if len(data) > messageDumpLimit {
				err = fmt.Errorf("%w; bytes: [% X ...] (%d bytes total)", err, data[:messageDumpLimit], len(data))
			} else {
				err = fmt.Errorf("%w; bytes: [% X]", err, data)
			}
		}
	}()
	if len(data) == 0 {
		return nil
	}
	status := data[0]
	if !midi.IsMessage(status) {
		return &InvalidMessageError{status, "missing status byte"}
	}
	length := 0
	payload := data[1:]
	switch midi.Message(status) {
	case midi.NoteOff, midi.NoteOn, midi.KeyAftertouch, midi.CC, midi.SongPosition, midi.Pitch:
		length = 3
	case midi.Pgm, midi.SongSelect, midi.ChannelAftertouch:
		length = 2
	case midi.Clock, midi.Start, midi.Continue, midi.Stop:
		length = 1
	case midi.SysEx:
		if len(data) < 2 || data[len(data)-1] != midi.EndSysEx {
			return &InvalidMessageError{status, "SysEx must be framed by F0 and F7"}
		}
		payload = data[1 : len(data)-1]
	default:
		return &UnsupportedMessageError{status}
	}
	if length != 0 && len(data) != length {
		return &InvalidMessageError{status, fmt.Sprintf("expected %d bytes, got %d", length, len(data))}
	}
	for index, value := range payload {
		if midi.IsMessage(value) {
			return &InvalidMessageError{status, fmt.Sprintf("data byte %d is not seven-bit", index+1)}
		}
	}
	return nil
}
