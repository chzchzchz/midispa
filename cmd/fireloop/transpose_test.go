package main

import (
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

// panBank is the state a pan-knob test starts from: the chromatic track selected and one
// note placed, the way percussionStepBank is the state a Volume-knob test starts from.
// Tests that need more notes, or none, place them on top.
func panBank(t *testing.T, step, note int) (*PatternBank, *Voice) {
	t.Helper()
	bank, voice := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	bank.CurrentPattern().SetChromaticNote(step, voice, note, 100)
	return bank, voice
}

// assertHeardAudition checks the pair an audition of one chromatic note writes: a note-on
// at the pitch and the dynamics, and the note-off that closes it at velocity zero, which
// is how a chromatic release is always sent.
func assertHeardAudition(t *testing.T, on, off alsa.SeqEvent, note, velocity int) {
	t.Helper()
	assertMidiData(t, on, []byte{midi.MakeNoteOn(0), byte(note), byte(velocity)})
	assertMidiData(t, off, []byte{midi.MakeNoteOff(0), byte(note), byte(0)})
}

// palettePadFor is the pad the pitch palette puts a note on, found by asking the palette
// rather than by dividing its layout out again, so the test moves with the palette's own
// mapping the way padTyping moves with the keypad's.
func palettePadFor(note int) (row, col int) {
	for row := range chromaticPaletteRows {
		for col := range chromaticPaletteColumns {
			if padNote, _ := chromaticPaletteNote(row, col, 0); padNote == note {
				return row, col
			}
		}
	}
	panic("no palette pad holds " + midiNoteName(note))
}

// The pan knob transposes every note of the selected chromatic track, and the row it
// shows repaints, because a chromatic step's colour is its pitch's. The row redraw marks
// the cursor step, so the plain palette colour is asserted on a step the cursor is not on.
func TestPanKnobTransposesAndRepaintsTheSelectedTrack(t *testing.T) {
	const cursorNote, otherNote = 60, 64
	bank, voice := panBank(t, 0, cursorNote)
	bank.CurrentPattern().SetChromaticNote(5, voice, otherNote, 90)
	recorder := usePadRecorder(t, &bank.pads)
	screen := useScreenRecorder(t, &bank.screen)

	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))

	for _, tt := range []struct{ step, want int }{
		{0, cursorNote + 1}, {5, otherNote + 1},
	} {
		event, ok := bank.CurrentPattern().EventAtStep(tt.step, voice)
		require.Truef(t, ok, "the transpose lost the note on step %d", tt.step)
		require.Equalf(t, tt.want, event.ChromaticNote, "the note on step %d", tt.step)
	}
	require.Equal(t, chromaticPaletteColor(otherNote+1), recorder.pad(5, 0),
		"the step away from the cursor should carry its new note's plain colour")
	require.Equal(t, markCursorColor(chromaticPaletteColor(cursorNote+1)), recorder.pad(0, 0),
		"the cursor step's pad should carry its new note's colour, marked as the cursor")
	require.Equal(t, "S01 C#4@100", screen.row(readoutRow),
		"the readout should name the cursor step's new note")
}

// In note-edit mode the palette owns every pad, so the knob repaints it rather than the
// step rows: the strip cell for the cursor step carries the new note's colour, and the
// palette highlights the pad for the note the cursor step now holds.
func TestPanKnobRepaintsThePaletteInNoteEditMode(t *testing.T) {
	const note = 60
	bank, _ := panBank(t, 0, note)
	require.NoError(t, bank.ToggleNoteMode())
	require.True(t, bank.noteEditActive(), "the setup needs note editing on")
	recorder := usePadRecorder(t, &bank.pads)

	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))

	row, col, ok := chromaticStepCell(0)
	require.True(t, ok, "the cursor step has no strip cell")
	require.Equal(t, markCursorColor(chromaticPaletteColor(note+1)), recorder.pad(col, row),
		"the cursor step's strip cell should carry its new note's colour")
	paletteRow, paletteCol := palettePadFor(note + 1)
	require.Equal(t, oledWhite, recorder.pad(paletteCol, paletteRow),
		"the palette should highlight the transposed note")
}

// A detent counts further the faster the pan knob is turned, on the Volume knob's own
// counter, and the cap holds: turning a pitch across a range is the same long-sweep
// problem turning a velocity is.
func TestPanKnobCountsFasterWhenTurnedFaster(t *testing.T) {
	const note = 30
	bank, voice := panBank(t, 0, note)
	pattern := bank.CurrentPattern()
	advance := knobTestClock(t)
	stepNote := func() int {
		event, _ := pattern.EventAtStep(0, voice)
		return event.ChromaticNote
	}
	// The counter's unit is velocityStep, and for this knob that unit is a semitone.
	want := note
	for i, detent := range fastTurnDetents {
		advance(detent.gap)
		require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
		want += detent.increment * velocityStep
		require.Equalf(t, want, stepNote(), "detent %d of the turn", i+1)
	}
}

// The audition is held back until the knob rests, so a fast transpose is heard once, at
// the pitch the sweep landed on, and not at all while the knob is still moving.
func TestPanKnobAuditionsTheCursorStepOnceTheKnobRests(t *testing.T) {
	const note, velocity = 60, 100
	bank, voice := panBank(t, 0, note)
	// The window is widened so the detents below are inside it however long a loaded
	// test machine takes between them.
	previousQuiescence := velocityQuiescence
	t.Cleanup(func() { velocityQuiescence = previousQuiescence })
	velocityQuiescence = 100 * time.Millisecond
	advance := knobTestClock(t)
	preview := &captureMidiWriter{}
	// A fast sweep: three detents, each inside both the fast window and the quiescence
	// the audition waits for.
	for range 3 {
		advance(10 * time.Millisecond)
		require.NoError(t, bank.TransposeSelected(preview, EncoderRight))
	}
	// The sweep landed a sixth above, and nothing has been heard yet.
	event, _ := bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, note+1+2+3, event.ChromaticNote)
	require.Empty(t, preview.snapshot(), "a fast turn should not audition per detent")
	// Once the knob rests, the cursor step's note is heard once, at the pitch it landed on.
	waitFor(t, "the held-back audition", func() bool { return len(preview.snapshot()) >= 2 })
	audition := preview.snapshot()
	require.Len(t, audition, 2, "the audition should write a note-on and a note-off")
	assertHeardAudition(t, audition[0], audition[1], note+1+2+3, velocity)
}

// A turn slow enough to leave the quiescence gap between its detents still hears every
// detent, which is what keeps a careful transpose audible while a fast one is not
// machine-gunned.
func TestPanKnobAuditionsASlowTurnPerDetent(t *testing.T) {
	const note, velocity = 60, 100
	bank, _ := panBank(t, 0, note)
	preview := &captureMidiWriter{}
	// Two detents a careful turn apart: each is heard on its own, a step apart and a
	// quiescence after its own turn. The first is waited for before the second is sent,
	// so a slow machine cannot make the second detent take the first's audition with it.
	require.NoError(t, bank.TransposeSelected(preview, EncoderRight))
	waitFor(t, "the first audition", func() bool { return len(preview.snapshot()) >= 2 })
	time.Sleep(velocityFastWindow + 10*time.Millisecond)
	require.NoError(t, bank.TransposeSelected(preview, EncoderRight))
	waitFor(t, "the second audition", func() bool { return len(preview.snapshot()) >= 4 })
	audition := preview.snapshot()
	require.Len(t, audition, 4)
	assertHeardAudition(t, audition[0], audition[1], note+1, velocity)
	assertHeardAudition(t, audition[2], audition[3], note+2, velocity)
}

// The guards, as one table: every way a turn can arrive with nothing to transpose, each
// leaving the pattern exactly as it was. The whole-voice snapshot is the assertion, which
// is stronger than any one field, because a no-op has to change nothing at all.
func TestPanKnobNoOpsLeaveThePatternAsItWas(t *testing.T) {
	tests := []struct {
		why   string
		value int
		setup func(t *testing.T) (*PatternBank, *Voice)
	}{
		{
			why:   "a swing entry owning the readout",
			value: EncoderRight,
			setup: func(t *testing.T) (*PatternBank, *Voice) {
				bank, voice := panBank(t, 0, 60)
				require.NoError(t, bank.ToggleSwingEntry())
				require.True(t, bank.swingEditActive(), "the setup needs swing entry on")
				return bank, voice
			},
		},
		{
			why:   "no row selected, so no voice to aim at",
			value: EncoderRight,
			setup: func(t *testing.T) (*PatternBank, *Voice) {
				bank, voice := chromaBank(t)
				bank.CurrentPattern().SetChromaticNote(0, voice, 60, 100)
				return bank, voice
			},
		},
		{
			why:   "a percussive track, whose notes come from its kit voice",
			value: EncoderRight,
			setup: func(t *testing.T) (*PatternBank, *Voice) {
				kit := trackWindowKit(8, -1)
				bank := useController(t, NewFire(func([]byte) error { return nil }), kit).patbank
				voice := kit.voices[0]
				require.NoError(t, bank.SelectTrackRow(1))
				require.NoError(t, bank.controller.handlePatternGrid(nil, 3, 0, 100))
				return bank, voice
			},
		},
		{
			why:   "an empty pattern, with no notes to move",
			value: EncoderRight,
			setup: func(t *testing.T) (*PatternBank, *Voice) {
				bank, voice := chromaBank(t)
				require.NoError(t, bank.SelectTrackRow(1))
				return bank, voice
			},
		},
		{
			why:   "a value that is neither way of turning",
			value: 64,
			setup: func(t *testing.T) (*PatternBank, *Voice) {
				return panBank(t, 0, 60)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.why, func(t *testing.T) {
			bank, voice := tt.setup(t)
			before := bank.CurrentPattern().EventsForVoice(voice)
			require.NoError(t, sendCC(bank, deviceChannel, CCPan, tt.value))
			require.Equal(t, before, bank.CurrentPattern().EventsForVoice(voice),
				"the turn changed the pattern")
		})
	}
}

// A bank that has never been on a pattern has no pattern to transpose, and the turn
// says so quietly: the knob can be turned at any moment, including before any pattern
// was ever chosen, and reaching for a pattern anyway would be a crash.
func TestPanKnobDoesNothingOnABankWithNoPattern(t *testing.T) {
	kit := trackWindowKit(1, 0)
	bank := useEmptyController(t, NewFire(func([]byte) error { return nil }), kit).patbank
	require.NoError(t, bank.SelectTrackRow(1))
	require.Nil(t, bank.CurrentPattern(), "the setup needs a bank that never chose a pattern")

	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
}

// A transpose changes what the set plays, but it is safe to make while the set plays it:
// the notes move under the same lock playback reads through, and a note already sounding
// is released from the pitch it was attacked at, so nothing hangs. Clearing a row stops
// playback because it removes events outright; this moves their pitches and nothing else.
func TestTransposeDoesNotStopPlayback(t *testing.T) {
	bank, voice := panBank(t, 0, 60)
	stubRunning(bank.controller, bank.newPlayback(), nil)

	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))

	require.NotNil(t, bank.controller.running, "the pan knob stopped the set")
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the turn lost the note")
	require.Equal(t, 61, event.ChromaticNote, "the track did not transpose")
}

// A turn that moves nothing owes the wire nothing: the moved count gates the audition the
// same way it gates the repaint.
func TestPanKnobHoldsAtTheEndsOfTheMIDIRange(t *testing.T) {
	bank, voice := panBank(t, 0, 127)
	preview := &captureMidiWriter{}
	require.NoError(t, bank.TransposeSelected(preview, EncoderRight))
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the turn lost the note")
	require.Equal(t, 127, event.ChromaticNote, "a note at the ceiling should hold")
	time.Sleep(3 * velocityQuiescence)
	require.Empty(t, preview.snapshot(), "a turn that moved nothing has nothing to audition")
}

// The audition is the cursor step's own note, so a turn whose cursor step holds nothing
// has nothing to play: the notes that moved are elsewhere, and the readout names an
// empty step rather than a pitch.
func TestPanKnobDoesNotAuditionWhenTheCursorStepHoldsNoNote(t *testing.T) {
	const note = 60
	bank, voice := panBank(t, 5, note)
	preview := &captureMidiWriter{}
	require.NoError(t, bank.TransposeSelected(preview, EncoderRight))
	event, ok := bank.CurrentPattern().EventAtStep(5, voice)
	require.True(t, ok, "the turn lost the note")
	require.Equal(t, note+1, event.ChromaticNote, "the track did not transpose")
	time.Sleep(3 * velocityQuiescence)
	require.Empty(t, preview.snapshot(), "an empty cursor step has nothing to audition")
}

// The detent count is the hand's speed, so a turn the guards turn away — over a
// percussive row — spends none of it: the next chromatic turn counts from where the
// hand left off, not from the cap a no-op sweep would have built.
func TestPanKnobNoOpsDoNotSpendTheDetentCount(t *testing.T) {
	kit := trackWindowKit(2, 1)
	bank := useController(t, NewFire(func([]byte) error { return nil }), kit).patbank
	voice := kit.voices[1]
	bank.CurrentPattern().SetChromaticNote(0, voice, 60, 100)
	require.NoError(t, bank.SelectTrackRow(2), "the setup needs the chromatic row selected")
	advance := knobTestClock(t)
	// Two real detents build the count to two.
	advance(10 * time.Millisecond)
	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
	advance(10 * time.Millisecond)
	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
	// A sweep over the percussive row: nothing to transpose, so nothing spent.
	require.NoError(t, bank.SelectTrackRow(1))
	advance(10 * time.Millisecond)
	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
	advance(10 * time.Millisecond)
	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
	// Back on the chromatic row, inside the fast window of the last real detent: the
	// count carries the two the hand had built, so this detent counts three.
	require.NoError(t, bank.SelectTrackRow(2))
	advance(10 * time.Millisecond)
	require.NoError(t, sendCC(bank, deviceChannel, CCPan, EncoderRight))
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the turn lost the note")
	require.Equal(t, 66, event.ChromaticNote, "the detent after the no-op turns")
}
