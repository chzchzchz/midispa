package main

import (
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

func TestChromaticVelocityControl(t *testing.T) {
	bank, voice := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	require.NoError(t, bank.controller.handlePatternGrid(nil, 1, 0, 100))
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the pad press placed no step")
	require.Equal(t, pressVelocity, event.Velocity)
	require.Equal(t, defaultStepVelocity, bank.chromaticVelocity,
		"the encoder should start at the default step velocity")
	// A detent moves from the encoder's own value, so the first one does not start from
	// the velocity the pad press gave the note.
	cc := alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(EncoderLeft)}}
	require.NoError(t, dispatch(bank, cc))
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, defaultStepVelocity-velocityStep, event.Velocity)
	cc.Data[2] = byte(EncoderRight)
	require.NoError(t, dispatch(bank, cc))
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, defaultStepVelocity, event.Velocity)
	require.LessOrEqual(t, event.Velocity, midiNoteMax)
}

// The encoder keeps its value across steps, so a step picked after the encoder was set
// takes that value on the next detent instead of its own.
func TestVelocityEncoderValueIsInheritedByTheNextStep(t *testing.T) {
	bank, voice := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))

	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 40)
	require.NoError(t, bank.ToggleNoteMode())
	// Put the encoder at 100 by turning it, then leave that step.
	for bank.chromaticVelocity > 100 {
		require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	}
	require.Equal(t, 100, bank.chromaticVelocity)
	// Selecting the second step must not drag the encoder back to that step's 40.
	require.NoError(t, bank.MoveStepCursor(1))
	require.Equal(t, 100, bank.chromaticVelocity, "the encoder value should be inherited")
	require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	event, _ := pattern.EventAtStep(1, voice)
	require.Equal(t, 99, event.Velocity, "the second step takes the encoder's detent")
	// The first step is untouched by the encoder moving on the second.
	event, _ = pattern.EventAtStep(0, voice)
	require.Equal(t, 100, event.Velocity, "the encoder reached back to the first step")
}

func encoderCC(direction int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(direction)}}
}

// A percussive step takes its dynamics from the pad that placed it, so the Volume knob
// trims that hit rather than dragging the step up to the encoder's carried value. That
// value belongs to the chromatic knob, and turning this one must leave it alone.
func TestPercussionVelocityKnobTrimsTheHit(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	controller := useController(t, fire, voiceBank)
	bank := controller.patbank
	require.NoError(t, bank.SelectTrackRow(1))
	voice := voiceBank.voices[0]
	note, _ := voice.PercussionNote()
	hit := 100
	require.NoError(t, bank.controller.handlePatternGrid(nil, 3, 0, hit))
	event, ok := bank.CurrentPattern().EventAtStep(3, voice)
	require.True(t, ok, "the pad press placed no step")
	require.Equal(t, hit, event.Velocity)
	// The press put the cursor on the step it placed, so the knob has a
	// step to trim without the grid buttons moving the edit first.
	require.Equal(t, 3, bank.StepCursor(), "the pad press did not move the cursor")
	// A detent counts from the hit, not from the encoder's own much higher value.
	require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	event, _ = bank.CurrentPattern().EventAtStep(3, voice)
	require.Equal(t, hit-velocityStep, event.Velocity)
	require.Equal(t, defaultStepVelocity, bank.chromaticVelocity,
		"the detent moved the chromatic register")
	// Turning back restores the hit, and the audition carries the value the step now has.
	preview := &captureMidiWriter{}
	require.NoError(t, bank.AdjustVelocity(preview, EncoderRight))
	event, _ = bank.CurrentPattern().EventAtStep(3, voice)
	require.Equal(t, hit, event.Velocity)
	require.Len(t, preview.events, 2, "the audition should write a note-on and a note-off")
	assertMidiData(t, preview.events[0], []byte{midi.MakeNoteOn(0), byte(note), byte(hit)})
}

// The knob cannot turn a percussive step off, because velocity zero is a note-off rather
// than a quiet note. The floor is enforced on the stored value and again on the wire, so
// the two can never disagree.
func TestPercussionVelocityKnobStopsAtOne(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	controller := useController(t, fire, voiceBank)
	bank := controller.patbank
	recorder := useScreenRecorder(t, &bank.screen)
	require.NoError(t, bank.SelectTrackRow(1))
	voice := voiceBank.voices[0]
	note, _ := voice.PercussionNote()
	require.NoError(t, bank.controller.handlePatternGrid(nil, 0, 0, 2))
	for range 8 {
		require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "turning the knob past the floor removed the step")
	require.Equal(t, minPercussionVelocity, event.Velocity)
	// The readout reports the floor, so the display never claims a velocity nothing plays.
	require.Equal(t, "S01 @001", recorder.row(readoutRow))
	assertMidiData(t, alsa.SeqEvent{Data: event.NoteOnMidi()}, []byte{midi.MakeNoteOn(0), byte(note), minPercussionVelocity})
}
