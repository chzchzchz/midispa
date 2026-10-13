package main

import (
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
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
		{"quietest", minStepVelocity, minStepVelocity},
		{"zero would stop the note rather than quiet it", 0, minStepVelocity},
		{"negative would stop the note rather than quiet it", -5, minStepVelocity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event := Event{Voice: voice, Velocity: tc.stored}
			assertMidiData(t, alsa.SeqEvent{Data: event.NoteOnMidi()},
				[]byte{midi.MakeNoteOn(0), byte(note), tc.want})
		})
	}
}

// A declared range bounds the clamp for either kind of voice: a
// value outside it is folded to the nearest end, for a drum as for
// a synth.
func TestClampStepVelocityHonorsTheVoiceRange(t *testing.T) {
	drum := 36
	percussive, _ := chromaticTestVoice(t, &drum)
	percussive.VelocityMin, percussive.VelocityMax = 32, 96
	chromatic, _ := chromaticTestVoice(t, nil)
	chromatic.VelocityMin, chromatic.VelocityMax = 32, 96
	voices := []struct {
		name  string
		voice *Voice
	}{
		{name: "percussive", voice: percussive},
		{name: "chromatic", voice: chromatic},
	}
	for _, v := range voices {
		t.Run(v.name, func(t *testing.T) {
			for _, tt := range []struct {
				stored int
				want   int
			}{
				{stored: 200, want: 96},
				{stored: 96, want: 96},
				{stored: 64, want: 64},
				{stored: 32, want: 32},
				{stored: 10, want: 32},
				{stored: -5, want: 32},
			} {
				require.Equalf(t, tt.want, clampStepVelocity(v.voice, tt.stored),
					"a stored velocity of %d", tt.stored)
			}
		})
	}
}

// A chromatic voice is kept off velocity zero as well: its releases
// are sent as explicit note-offs, so a stored zero would be a silent
// note-on rather than a quiet note.
func TestClampStepVelocityKeepsAChromaticVoiceOffZero(t *testing.T) {
	chromatic, _ := chromaticTestVoice(t, nil)
	for _, tt := range []struct {
		stored int
		want   int
	}{
		{stored: 0, want: minStepVelocity},
		{stored: -5, want: minStepVelocity},
		{stored: 100, want: 100},
		{stored: 200, want: midiNoteMax},
	} {
		require.Equalf(t, tt.want, clampStepVelocity(chromatic, tt.stored),
			"a stored velocity of %d", tt.stored)
	}
}
