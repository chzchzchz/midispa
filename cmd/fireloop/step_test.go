package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFitOLEDTextCountsGlyphsNotBytes(t *testing.T) {
	// A multi-byte rune sitting on the clip boundary is the case that matters: clipping by
	// raw bytes there would cut the name inside it and leave half a character on the row.
	require.Equal(t, "1234567890123456789?", fitOLEDText("1234567890123456789é"))
	require.Equal(t, "K?rg und nochmal bit", fitOLEDText("Körg und nochmal bitte"))
	// A name that already fits is only replaced, not clipped.
	require.Equal(t, "Snare [CHR]", fitOLEDText("Snare [CHR]"), "a short name is only replaced")
}

func TestStepStatusText(t *testing.T) {
	chromatic, _ := chromaticTestVoice(t, nil)
	drum := 36
	percussive, _ := chromaticTestVoice(t, &drum)
	event := &Event{ChromaticNote: 60, Velocity: 90, Tie: true}
	require.Equal(t, "S01 C4@090->02", stepStatusText(chromatic, 0, event, stepMarks{tieStep: 1}))
	// A step with no note must not show a velocity, or it reads as that step's own.
	require.Equal(t, "S02 --", stepStatusText(chromatic, 1, nil, stepMarks{tieStep: -1}))
	// A percussive step has no pitch to name, so it reports its dynamics alone.
	drumEvent := &Event{Velocity: 72}
	require.Equal(t, "S03 @072", stepStatusText(percussive, 2, drumEvent, stepMarks{tieStep: -1}))
	require.Equal(t, "S03 --", stepStatusText(percussive, 2, nil, stepMarks{tieStep: -1}))
}

// The Volume knob is the only way to change a percussive step's dynamics once it is
// placed, so the readout has to report what the knob is doing. A percussive step has no
// pitch to name, and an empty step shows no velocity at all.
func TestPercussionStatusFollowsTheSelectedStep(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	controller := useController(t, fire, voiceBank)
	bank := controller.patbank
	recorder := useScreenRecorder(t, &bank.screen)
	require.NoError(t, bank.SelectTrackRow(1))
	voice := voiceBank.voices[0]
	require.NoError(t, bank.controller.handlePatternGrid(nil, 2, 0, 84))
	require.Equal(t, "S03 @084", recorder.row(readoutRow))
	// An empty step shows nothing, the same as a chromatic one, so a velocity belonging to
	// no step cannot make two steps look equal.
	require.NoError(t, bank.MoveStepCursor(2))
	require.Equal(t, "S05 --", recorder.row(readoutRow))
	event, ok := bank.CurrentPattern().EventAtStep(2, voice)
	require.True(t, ok, "moving the cursor removed the step")
	require.Equal(t, 84, event.Velocity, "moving the cursor changed the step's dynamics")
}

// A percussive pad press on the selected row toggles its step and puts the
// cursor on it, which is what makes the hit just placed the one the Volume
// knob trims without the grid buttons. A press on another row still toggles
// without moving the cursor: the selection is what decides which voice the
// knob aims at, and two pads on the selected row are two steps, not a tie.
func TestPercussivePadPressMovesTheStepCursor(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	controller := useController(t, fire, voiceBank)
	bank := controller.patbank
	recorder := useScreenRecorder(t, &bank.screen)
	require.NoError(t, bank.SelectTrackRow(1))
	voice := voiceBank.voices[0]

	// Row 1 is the selected row, so its press both places the step and
	// moves the cursor onto it, and the readout names the pair.
	require.NoError(t, dispatch(bank, padMessage(54+5, pressVelocity)))
	require.Equal(t, 5, bank.StepCursor(), "the press did not move the cursor")
	event, ok := bank.CurrentPattern().EventAtStep(5, voice)
	require.True(t, ok, "the press did not place the step")
	require.Equal(t, pressVelocity, event.Velocity)
	require.Equal(t, "S06 @100", recorder.row(readoutRow))

	// A press on an unselected row toggles that row's own step and
	// leaves the cursor where it was.
	require.NoError(t, dispatch(bank, padMessage(54+16+3, pressVelocity)))
	require.Equal(t, 5, bank.StepCursor(), "a press off the selected row moved the cursor")
	other := voiceBank.voices[1]
	_, ok = bank.CurrentPattern().EventAtStep(3, other)
	require.True(t, ok, "the press did not toggle the unselected row's step")

	// Two pads on the selected row are two steps rather than a tie: each
	// press toggles its own step, and the cursor ends on the second.
	require.NoError(t, dispatch(bank, padMessage(54+7, pressVelocity)))
	require.Equal(t, 7, bank.StepCursor(), "the second press did not move the cursor")
	_, ok = bank.CurrentPattern().EventAtStep(7, voice)
	require.True(t, ok, "the second press did not place its step")
	_, first := bank.CurrentPattern().EventAtStep(5, voice)
	require.True(t, first, "the second press took the first step off")
}

func TestModeDoesNotStopPlayback(t *testing.T) {
	bank, _ := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	stopCalled := false
	bank.controller.playback = stubSession(&Playback{}, func() error {
		stopCalled = true
		return nil
	})
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	require.False(t, stopCalled, "Mode stopped active playback")
	require.NotNil(t, bank.controller.playback, "Mode cleared active playback")
}

func TestChromaticPadSetsEditingStep(t *testing.T) {
	bank, _ := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, dispatch(bank, padMessage(54+3, 100)))
	require.Equal(t, 3, bank.StepCursor(), "step cursor after pad")
	bank.releasePad(0, 3)
	require.NoError(t, dispatch(bank, padMessage(54+5, 100)))
	require.Equal(t, 5, bank.StepCursor(), "step cursor after second pad")
}
