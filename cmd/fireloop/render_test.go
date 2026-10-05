package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pads and the button lights are readable without an event going anywhere. Toggling a
// mode lights a light and repaints the display; the light is the part that used to need a
// whole Fire to stand in front of.
func TestAModeToggleIsReadableWithoutAnEvent(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, -1))
	lights := usePadRecorder(t, &bank.pads)

	require.NoError(t, bank.ToggleLengthMode())
	require.True(t, bank.lengthEditActive(), "the mode did not open")
	require.Equal(t, LEDRed, lights.led(NoteOverview), "opening the length mode should light Overview")

	require.NoError(t, bank.ToggleLengthMode())
	require.False(t, bank.lengthEditActive(), "the mode did not close")
	require.Equal(t, LEDOff, lights.led(NoteOverview), "closing the length mode should put Overview out")
}

// A pad the bank paints is readable the same way, which is what makes a repaint assertable
// on its own rather than only as a side effect of an event.
func TestAPadRepaintIsReadableWithoutAnEvent(t *testing.T) {
	// The colour comes from the pitch, so the track has to hold a chromatic voice: a
	// percussive step has no pitch to paint and would read as dark either way.
	bank, kit := quietBank(t, trackWindowKit(4, 0))
	pads := usePadRecorder(t, &bank.pads)
	require.NoError(t, bank.SelectTrackRow(1))
	voice := bank.trackVoice(1)
	require.NotNil(t, voice, "the first pad row holds no voice")
	require.Same(t, kit.voices[0], voice, "the test writes to a different voice than it reads")

	// Row one of the pattern is pad row zero, and a step is the column of the same number.
	require.NoError(t, bank.redrawTrackPads(1))
	dark := pads.pad(1, 0)
	require.Equal(t, [3]int{}, dark, "an empty step should be dark")

	bank.CurrentPattern().SetChromaticNote(1, voice, 60, 100)
	require.NoError(t, bank.redrawTrackPads(1))
	require.NotEqual(t, dark, pads.pad(1, 0), "a step holding a note should not be dark")
	require.Equal(t, [3]int{}, pads.pad(2, 0), "the step next to it should still be dark")
}

// The unit has more buttons than fireloop binds, and a press on one of those is said out
// loud rather than swallowed. This matters because NoteSnap, Play and Record and several
// others share a value, so a constant that stops being routed looks identical to a button
// that is merely not ours.
func TestAnUnboundControlIsReportedRatherThanSwallowed(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(4, -1))
	capture := useCaptureLog(t)

	require.NoError(t, dispatch(bank, padMessage(NoteSnap, 100)), "an unbound control should not fail a press")
	assert.Contains(t, capture.messages(), "unbound control", "a press on an unbound control was swallowed")
}
