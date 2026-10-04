package main

import (
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// A percussive step sounds with the dynamics it was hit at, so the velocity stored on the
// event is what reaches the wire. Velocity zero never gets there, because it is a note-off
// rather than a silent note: a step that stored zero would stop sounding altogether.
//
// The expected bytes are written out rather than derived from the clamp, because a clamp
// that was wrong in the same direction as the code would otherwise agree with itself.
func TestPercussionNoteOnUsesTheEventVelocity(t *testing.T) {
	note := 36
	voice, _ := chromaticTestVoice(t, &note)
	tests := []struct {
		name   string
		stored int
		want   byte
	}{
		{"loudest", midi.DataMax, midi.DataMax},
		{"middle", 72, 72},
		{"quietest", minPercussionVelocity, minPercussionVelocity},
		{"zero would stop the note rather than quiet it", 0, minPercussionVelocity},
		{"negative would stop the note rather than quiet it", -5, minPercussionVelocity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := Event{Voice: voice, Velocity: tc.stored}
			assertMidiData(t, alsa.SeqEvent{Data: event.NoteOnMidi()},
				[]byte{midi.MakeNoteOn(0), byte(note), tc.want})
		})
	}
}
