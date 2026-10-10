package main

import (
	"errors"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// readErrFatal says whether a failed read ends the input. The two
// decode errors are per event, and the event is gone either way:
// active sensing and tune requests arrive unsolicited, so a loop that
// ended on them would stop its input the first time a device breathed
// at it. Everything else, a released client included, is fatal.
func readErrFatal(err error) bool {
	return !errors.Is(err, alsa.ErrUnsupportedEvent) && !errors.Is(err, alsa.ErrInvalidMessage)
}

// readInput forwards every control change the sequencer delivers to
// send, for the lifetime of the editor. Reading blocks inside the ALSA
// library and cannot be interrupted, not even by closing the
// sequencer, so the loop owns its goroutine the way fireloop's read
// does: it ends when the sequencer stops answering, and the process
// exit that follows the editor quitting reclaims the goroutine a quit
// otherwise leaves blocked in a read.
func readInput(seq alsa.EventReader, send func(alsa.SeqEvent)) {
	for {
		ev, err := seq.Read()
		if err != nil {
			if readErrFatal(err) {
				return
			}
			continue
		}
		// IsCC also rejects ALSA's own subscription
		// notifications, whose data begins with an event type
		// byte no MIDI status carries, and every other MIDI
		// message type. The channel filter is policy, and
		// policy lives in applyControlChange.
		if midi.IsCC(ev.Data[0]) {
			send(ev)
		}
	}
}
