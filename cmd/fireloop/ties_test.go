package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

func TestChromaticPadGesturesAndReleases(t *testing.T) {
	writeCount := 0
	bank, voice := chromaBankOn(t, func([]byte) error {
		writeCount++
		return nil
	})
	require.NoError(t, bank.SelectTrackRow(1))
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	handled, err := bank.handleChromaticStepPress(0, 0)
	require.NoErrorf(t, err, "the first pad press failed (handled=%v)", handled)
	require.True(t, handled, "the first pad press was not handled")
	writeCount = 0
	handled, err = bank.handleChromaticStepPress(0, 1)
	require.NoErrorf(t, err, "the second pad press failed (handled=%v)", handled)
	require.True(t, handled, "the second pad press was not handled")
	require.Equal(t, 6, writeCount, "a tie gesture should write the row and status redraws")
	event, _ := pattern.EventAtStep(0, voice)
	require.True(t, event.Tie, "two held pads did not tie adjacent events")
	bank.releasePad(0, 0)
	bank.releasePad(0, 1)
	require.Zero(t, bank.pressedPads, "a release should clear the held pads")
	require.Zero(t, bank.rowPadMasks[0], "a release should clear the row mask")

	bank.clearPadState()
	pattern.UntieEventsAtSteps(0, 1, voice)
	handled, _ = bank.handleChromaticStepPress(0, 0)
	require.True(t, handled, "the selected chromatic row was not handled")
	handled, _ = bank.handleChromaticStepPress(1, 1)
	require.True(t, handled, "a cross-row gesture was not handled as a gesture")
	event, _ = pattern.EventAtStep(0, voice)
	require.False(t, event.Tie, "a cross-row gesture created a tie")
	pattern.TieEventsAtSteps(0, 1, voice)
	bank.clearPadState()
	handled, _ = bank.handleChromaticStepPress(0, 2)
	require.True(t, handled, "the third selected-row pad was not handled")
	event, _ = pattern.EventAtStep(0, voice)
	require.True(t, event.Tie, "a third held pad removed the completed tie")

	bank.clearPadState()
	empty := &Pattern{}
	bank.Patterns[bank.selPatIdx] = empty
	bank.handleChromaticStepPress(0, 0)
	bank.handleChromaticStepPress(0, 1)
	created, _ := empty.snapshot()
	require.Empty(t, created, "empty chromatic steps created events")

	bank.clearPadState()
	bank.handleChromaticStepPress(0, 0)
	require.NoError(t, dispatch(bank, releaseMessage(54)))
	require.Zero(t, bank.pressedPads, "NoteOff was not treated as a release")
	require.Zero(t, bank.rowPadMasks[0], "NoteOff was not treated as a release")
	bank.handleChromaticStepPress(0, 0)
	require.NoError(t, dispatch(bank, padMessage(54, 0)))
	require.Zero(t, bank.pressedPads, "a NoteOn of velocity zero was not treated as a release")
	require.Zero(t, bank.rowPadMasks[0], "a NoteOn of velocity zero was not treated as a release")
}

// stepCellPad is the grid pad note of the strip cell standing for a step, which is where the
// tie gesture is made while the palette owns the grid.
func stepCellPad(step int) int {
	row, col, ok := chromaticStepCell(step)
	if !ok {
		return 0
	}
	return 54 + row*padColumns + col
}

// A tie is what holds a note past its step, so the gesture that makes one matters as much as
// playback honouring it: two step cells held together tie the two steps, the same gesture
// step mode uses on the step grid. This drives the pads through the real handler.
func TestNoteEditStepCellsTieSteps(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(3, voice, 64, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	// Hold the cell for step 1, then press the cell for step 4 without letting go.
	press := func(step, velocity int) {
		require.NoError(t, dispatch(bank, padMessage(stepCellPad(step), velocity)))
	}
	press(0, 100)
	press(3, 100)
	first, _ := pattern.EventAtStep(0, voice)
	second, _ := pattern.EventAtStep(3, voice)
	require.Truef(t, first.Tie, "two held cells did not tie step 1 to step 4: %+v", first)
	require.False(t, second.Tie, "the tie landed on the later event as well")
	// The edit stays on the step it came from, which is what a tie is holding.
	require.Equal(t, 0, bank.StepCursor(), "the tie moved the edit")
	// Letting go of both ends the gesture.
	press(0, 0)
	press(3, 0)
	require.Equalf(t, noHeldStep, bank.noteEditHeldStep, "held step after releasing the cells")

	// The tie is what the pattern plays back: the first note sounds until the tied step.
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step < 5; step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		_, err := playback.playBeat(writer, pattern)
		require.NoError(t, err)
		switch step {
		case 0:
			// The tie starts the note and nothing releases it afterwards on its own.
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 100})
			require.Lenf(t, writer.events, 1, "step 1 should write only the note-on")
		case 1, 2:
			// The gap the tie covers: silence on the wire, not a note-off.
			require.Lenf(t, writer.events, 0, "step %d should write nothing while the note is held", step+1)
		case 3:
			// Legato into the tied step: the next note starts before the first stops.
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 64, 100})
			assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOff(0), 60, 0})
		default:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 64, 0})
		}
		writer.events = nil
	}
}

// A tie needs a note on both steps and nothing in between. A gesture that cannot tie is
// refused whole: the edit does not jump to a step whose note has nothing to hold, and the
// pattern is left as it was.
func TestNoteEditTieGestureRefusesWhenItCannotTie(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(4, voice, 64, 100)
	pattern.SetChromaticNote(6, voice, 65, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	press := func(step, velocity int) {
		require.NoError(t, dispatch(bank, padMessage(stepCellPad(step), velocity)))
	}

	// A cell on its own just moves the edit.
	press(0, 100)
	require.Equal(t, 0, bank.StepCursor(), "the first cell press moved the edit")
	event, _ := pattern.EventAtStep(0, voice)
	require.False(t, event.Tie, "one cell press tied a step")
	// Step 3 holds no note, so there is nothing for step 1 to hold on to.
	press(3, 100)
	event, _ = pattern.EventAtStep(0, voice)
	require.False(t, event.Tie, "a step with no note was tied")
	require.Equal(t, 0, bank.StepCursor(), "a refused tie moved the edit")
	// Step 5 would tie step 1 to step 7, but step 5's own note sits between them, and
	// tying across it would make that note unreachable. Refused, so the edit stays put.
	press(0, 0)
	press(6, 100)
	event, _ = pattern.EventAtStep(0, voice)
	require.False(t, event.Tie, "a tie crossed an intervening note")
	require.Equal(t, 0, bank.StepCursor(), "a refused tie moved the edit")
	event, _ = pattern.EventAtStep(4, voice)
	require.False(t, event.Tie, "a refused tie landed on the wrong event")
	// Releasing between presses ends the gesture, so two separate presses never tie.
	press(4, 0)
	press(0, 100)
	press(1, 100)
	press(1, 0)
	press(0, 0)
	event, _ = pattern.EventAtStep(0, voice)
	require.False(t, event.Tie, "a released cell still took part in the gesture")
}

// A tie between two notes of the same pitch reuses the sounding note, so the tied step sends
// nothing at all: one note-on for the whole chain, however long it runs. Releasing and
// retriggering instead would sound as two notes with a gap in them, which is what the step
// expiry would do if it did not know about the tie.
func TestSamePitchTieSendsOneNoteOnAcrossTheChain(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	for _, step := range []int{0, 3, 6} {
		pattern.SetChromaticNote(step, voice, 60, 100)
	}
	require.True(t, pattern.TieEventsAtSteps(0, 3, voice), "tie 1-4 refused")
	require.True(t, pattern.TieEventsAtSteps(3, 6, voice), "tie 4-7 refused")
	// Each tied step extends the note by one more step, so the chain covers steps 1 to 7 and
	// the note stops at step 8.
	want := map[int][]string{
		0: {"on 60"},
		1: nil,
		2: nil,
		3: nil,
		4: nil,
		5: nil,
		6: nil,
		7: {"off 60"},
	}
	assertChromaticSteps(t, voice, pattern, want)
}

// The same pitch without a tie is a fresh note, so the old one stops before the new one
// starts. The contrast is what makes the tied case above meaningful: both look alike on the
// grid until the tie is there or not.
func TestSamePitchWithoutATieRetriggers(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(3, voice, 60, 100)
	want := map[int][]string{
		0: {"on 60"},
		1: {"off 60"},
		2: nil,
		3: {"on 60"},
		4: {"off 60"},
	}
	assertChromaticSteps(t, voice, pattern, want)
}

// assertChromaticSteps plays a pattern one step at a time and compares the MIDI each step
// wrote against what it was given, spelled as "on 60" or "off 60".
func assertChromaticSteps(t *testing.T, voice *Voice, pattern *Pattern, want map[int][]string) {
	t.Helper()
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step < len(want); step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		_, err := playback.playBeat(writer, pattern)
		require.NoError(t, err)
		var got []string
		for _, event := range writer.events {
			kind := "on"
			if midi.IsNoteOff(event.Data[0]) {
				kind = "off"
			}
			got = append(got, fmt.Sprintf("%s %d", kind, event.Data[1]))
		}
		require.Equalf(t, strings.Join(want[step], ", "), strings.Join(got, ", "),
			"step %d wrote the wrong messages", step+1)
		writer.events = nil
	}
}

// The velocity of a tied step is never heard, because a same-pitch tie writes nothing at all:
// no new note starts, so no new dynamics can. The note sounds with the velocity it was given
// at its own step, all the way to where it stops.
func TestSamePitchTieKeepsTheFirstVelocity(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 40)
	pattern.SetChromaticNote(3, voice, 60, midi.DataMax)
	require.True(t, pattern.TieEventsAtSteps(0, 3, voice), "tie 1-4 refused")
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step <= 4; step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		_, err := playback.playBeat(writer, pattern)
		require.NoError(t, err)
		switch step {
		case 0:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 40})
			require.Len(t, writer.events, 1, "step 1 should write only the note-on")
		case 3:
			// The tied step is silent on the wire, so its velocity cannot be applied.
			require.Len(t, writer.events, 0, "step 4 should write nothing from a same-pitch tie")
		case 4:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 60, 0})
		default:
			require.Lenf(t, writer.events, 0, "step %d should write nothing while the note is held", step+1)
		}
		writer.events = nil
	}
}
