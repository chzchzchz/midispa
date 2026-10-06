package main

import (
	"fmt"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

func TestChromaticPaletteAndModeEditing(t *testing.T) {
	writeCount := 0
	bank, voice := chromaBankOn(t, func([]byte) error {
		writeCount++
		return nil
	})
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	require.True(t, bank.noteEditActive(), "Mode did not enter note-edit mode")
	require.NoError(t, bank.controller.handlePatternGrid(nil, 1, 0, 100))

	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the palette pad placed no note")
	firstNote, _ := chromaticPaletteNote(0, 1, 0)
	require.Equal(t, firstNote, event.ChromaticNote)

	preview := &captureMidiWriter{}
	require.NoError(t, bank.auditionEvent(preview, event))
	require.Len(t, preview.events, 2)
	// The palette pad was pressed at pressVelocity, so that is the note's velocity.
	assertMidiData(t, preview.events[0], []byte{midi.MakeNoteOn(0), byte(firstNote), pressVelocity})
	assertMidiData(t, preview.events[1], []byte{midi.MakeNoteOff(0), byte(firstNote), 0})

	require.NoError(t, dispatch(bank, padMessage(NoteGridRight, 100)))
	require.Equal(t, 1, bank.StepCursor(), "grid right moved the cursor")

	bank.controller.alt = true
	writeCount = 0
	require.NoError(t, bank.controller.handlePatternGrid(nil, 1, 0, 100))
	require.Equal(t, 3, writeCount, "Alt clear should write the palette and the status row")

	_, cleared := bank.CurrentPattern().EventAtStep(1, voice)
	require.False(t, cleared, "Alt plus palette pad did not clear the current-step event")
	_, kept := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, kept, "Alt plus palette pad cleared a different step")
}

// The pitch palette is four octaves from A1, twelve semitones to the row, so every row
// starts on A and the four columns past G# carry no pitch.
func TestChromaticPaletteLayout(t *testing.T) {
	got, _ := chromaticPaletteNote(0, 0, 0)
	require.Equal(t, 33, got, "the palette should start on A1 at 33")
	require.Equal(t, "A1", midiNoteName(33), "MIDI 33 should name as A1")
	require.Equal(t, 12, chromaticPaletteColumns)
	require.Equal(t, 4, chromaticStepColumns)
	require.Equal(t, padColumns, chromaticPaletteColumns+chromaticStepColumns,
		"the palette and the step block together fill a pad row")
	require.Equal(t, maxPatternSteps, chromaticStepCells, "one step cell per step")

	for row := 0; row < chromaticPaletteRows; row++ {
		first, ok := chromaticPaletteNote(row, 0, 0)
		require.Truef(t, ok, "row %d has no first note", row)
		require.Equalf(t, fmt.Sprintf("A%d", row+1), midiNoteName(first),
			"row %d starts on", row)
		last, ok := chromaticPaletteNote(row, chromaticPaletteColumns-1, 0)
		require.Truef(t, ok, "row %d has no last note", row)
		// Twelve semitones from A is a major seventh, so the row ends on the G# above
		// its starting A: row 0 runs A1 up to G#2.
		require.Equalf(t, fmt.Sprintf("G#%d", row+2), midiNoteName(last),
			"row %d ends on the G# above its A", row)
		require.Equalf(t, 11, last-first, "row %d spans A to G# in semitones", row)
		// The columns past the palette have no pitch at all.
		for _, col := range []int{12, 13, 14, 15} {
			note, ok := chromaticPaletteNote(row, col, 0)
			require.Falsef(t, ok, "row %d column %d carries pitch %d", row, col, note)
		}
	}
	_, above := chromaticPaletteNote(-1, 0, 0)
	require.False(t, above, "a row above the palette reported a pitch")
	_, left := chromaticPaletteNote(0, -1, 0)
	require.False(t, left, "a column left of the palette reported a pitch")
}

// paletteTestBank is a bank sitting in note-edit mode on a chromatic track, which is the
// only state the palette octave is used from.
func paletteTestBank(t *testing.T) (*PatternBank, *Voice, *fireSim) {
	t.Helper()
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	return bank, kit.voices[0], sim
}

func selectKnobCC(direction int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCSelect), byte(direction)}}
}

// A knob reporting neither direction has not been turned. Reading such a value as a turn
// moved whatever the knob drives, which for this knob means the palette shifting or the
// track's voice changing without anyone touching it.
func TestSelectKnobIgnoresAValueThatIsNotATurn(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	voiceBefore := bank.trackVoice(1)

	require.NoError(t, bank.handleNoteEditPad(nil, 1, 3, pressVelocity, false, false))
	require.NoError(t, dispatch(bank, selectKnobCC(64)))
	require.Equal(t, 0, bank.paletteOctave, "a value that is not a turn shifted the palette")
	require.Equal(t, voiceBefore, bank.trackVoice(1), "a value that is not a turn moved the voice")
	_, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the note placed before the stray value is gone")
}

// The SELECT knob is an octave transpose for the palette: it moves which pitches the pads
// choose, and nothing that is already written into the pattern.
func TestSelectKnobShiftsPaletteByOctave(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	voiceBefore := bank.trackVoice(1)

	require.NoError(t, bank.handleNoteEditPad(nil, 1, 3, pressVelocity, false, false))
	first, _ := chromaticPaletteNote(1, 3, 0)
	event, _ := bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, first, event.ChromaticNote)

	require.NoError(t, dispatch(bank, selectKnobCC(EncoderRight)))
	require.Equal(t, 1, bank.paletteOctave, "one detent up")
	require.Equal(t, voiceBefore, bank.trackVoice(1), "the knob moved the voice instead of the palette")
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, first, event.ChromaticNote, "the stored note is left where it was")

	// The same pad now reaches the pitch an octave up.
	require.NoError(t, bank.handleNoteEditPad(nil, 1, 3, pressVelocity, false, false))
	up, _ := chromaticPaletteNote(1, 3, bank.paletteOctave)
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, up, event.ChromaticNote, "the pad reached the shifted pitch")
	require.Equal(t, first+chromaticOctaveShift, up, "the shift is an octave")

	// And an octave down takes it back past where it started.
	for range 2 {
		require.NoError(t, dispatch(bank, selectKnobCC(EncoderLeft)))
	}
	require.NoError(t, bank.handleNoteEditPad(nil, 1, 3, pressVelocity, false, false))
	down, _ := chromaticPaletteNote(1, 3, bank.paletteOctave)
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, down, event.ChromaticNote, "the pad reached the lowered pitch")
	require.Equal(t, first-chromaticOctaveShift, down, "the shift down is an octave")
}

// The palette has to say which octave it is offering, so every pad's colour travels with it
// while the steps keep the colours of the notes they hold.
func TestPaletteShiftMovesColours(t *testing.T) {
	bank, voice, sim := paletteTestBank(t)
	bank.CurrentPattern().SetChromaticNote(3, voice, 40, pressVelocity)
	require.NoError(t, bank.setStepCursor(3))
	require.NoError(t, bank.drawNotePalette())
	stepColour := stripCell(sim, 3)
	before := paletteRegion(sim)

	require.NoError(t, dispatch(bank, selectKnobCC(EncoderRight)))
	after := paletteRegion(sim)
	changed := 0
	for i := range before {
		row, col := i/chromaticPaletteColumns, i%chromaticPaletteColumns
		index := row*padColumns + col
		if before[i] != after[i] {
			changed++
		}
		note, _ := chromaticPaletteNote(row, col, bank.paletteOctave)
		require.Equalf(t, chromaticPaletteColor(note), sim.pads[index],
			"palette pad row %d col %d did not take the shifted pitch colour", row, col)
	}
	require.Equalf(t, len(before), changed, "the shift should be visible on every palette pad")
	// A note already in the pattern keeps its own colour; only the palette moved.
	require.Equal(t, stepColour, stripCell(sim, 3), "a note in the pattern keeps its own colour")
}

// A palette below the unshifted base must keep its colours apart instead of collapsing them
// onto the first entry of the table.
func TestPaletteBelowBaseKeepsDistinctColours(t *testing.T) {
	bank, _, sim := paletteTestBank(t)
	for range 2 {
		require.NoError(t, dispatch(bank, selectKnobCC(EncoderLeft)))
	}
	require.Less(t, bank.paletteOctave, 0, "the palette should sit below its base")
	for row := 0; row < chromaticPaletteRows; row++ {
		seen := make(map[[3]int]bool, chromaticPaletteColumns)
		for col := 0; col < chromaticPaletteColumns; col++ {
			seen[sim.pads[row*padColumns+col]] = true
		}
		require.Lenf(t, seen, chromaticPaletteColumns, "row %d collapsed two pads onto one colour", row)
	}
}

// The palette may only turn as far as the MIDI range allows, or a pad would name a note
// that cannot be sent.
func TestPaletteOctaveStaysInsideMIDIRange(t *testing.T) {
	bank, _, _ := paletteTestBank(t)
	lowest, highest := chromaticOctaveBounds()
	require.Equal(t, -2, lowest, "a palette from A1 goes two octaves down")
	require.Equal(t, 3, highest, "a palette from A1 goes three octaves up")

	for _, direction := range []int{EncoderRight, EncoderLeft} {
		for range 8 {
			require.NoError(t, dispatch(bank, selectKnobCC(direction)))
		}
		octave := bank.paletteOctave
		// Turning further must not move it, or the palette would leave the MIDI range.
		require.NoError(t, dispatch(bank, selectKnobCC(direction)))
		require.Equalf(t, octave, bank.paletteOctave, "turning past the end moved the octave")
		base := bank.paletteBase()
		last, _ := chromaticPaletteNote(chromaticPaletteRows-1, chromaticPaletteColumns-1, octave)
		require.Falsef(t, base < 0 || last > midiNoteMax,
			"the palette spans %d to %d, outside the MIDI range", base, last)
		if direction == EncoderRight {
			require.Equalf(t, highest, octave, "the palette stopped short of the top")
		}
		if direction == EncoderLeft {
			require.Equalf(t, lowest, octave, "the palette stopped short of the bottom")
		}
	}
}

// The erase key is wherever the palette starts, so it moves with the palette.
func TestPaletteFirstPadErasesAfterShift(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	require.NoError(t, dispatch(bank, selectKnobCC(EncoderRight)))
	base := bank.paletteBase()
	require.NotEqual(t, chromaticBaseNote, base, "the test needs the palette shifted off A1")
	// A pad an octave along places the shifted pitch.
	require.NoError(t, bank.handleNoteEditPad(nil, 1, 0, pressVelocity, false, false))
	placed, _ := chromaticPaletteNote(1, 0, bank.paletteOctave)
	require.NotEqual(t, base, placed, "the test needs a pad that is not the palette's first one")
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the shifted pitch was not placed")
	require.Equal(t, placed, event.ChromaticNote)
	// The palette's first pad is the erase key wherever the palette has moved to. Placing
	// the note above also lifted the guard on that pad, which doubles as the step-one pad.
	require.NoError(t, bank.handleNoteEditPad(nil, 0, 0, pressVelocity, false, false))
	_, still := bank.CurrentPattern().EventAtStep(0, voice)
	require.Falsef(t, still, "the palette's first pad at %d did not erase", base)
}

// Outside note-edit mode the knob is still what chooses the track's voice, and the palette
// is left where the user put it.
func TestSelectKnobJogsVoiceOutsideNoteEdit(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	require.NoError(t, bank.SelectTrackRow(1))
	before := bank.trackVoice(1)
	require.NoError(t, dispatch(bank, selectKnobCC(EncoderRight)))
	require.NotEqual(t, before, bank.trackVoice(1), "the knob did not jog the track's voice")
	require.Equal(t, 0, bank.paletteOctave, "the palette is untouched outside note-edit mode")
	require.False(t, bank.noteEditActive(), "the knob entered note-edit mode")
}

// The right-hand block stands for the steps themselves: it reads in the same order as the
// step grid, four per row, and covers every step exactly once.
func TestChromaticStepBlockCoversEveryStep(t *testing.T) {
	seen := make(map[int]bool, chromaticStepCells)
	for row := 0; row < chromaticPaletteRows; row++ {
		for offset := 0; offset < chromaticStepColumns; offset++ {
			col := chromaticPaletteColumns + offset
			step := chromaticStepAt(row, col)
			require.Equalf(t, row*chromaticStepColumns+offset, step, "cell row %d offset %d", row, offset)
			require.GreaterOrEqualf(t, step, 0, "cell row %d offset %d maps outside the pattern", row, offset)
			require.Lessf(t, step, maxPatternSteps, "cell row %d offset %d maps outside the pattern", row, offset)
			require.Falsef(t, seen[step], "step %d is shown by more than one cell", step)
			seen[step] = true
		}
	}
	require.Lenf(t, seen, maxPatternSteps, "the block should cover every step exactly once")
	// The palette side of the grid is not a step.
	for col := 0; col < chromaticPaletteColumns; col++ {
		require.Negativef(t, chromaticStepAt(0, col), "palette column %d reports a step", col)
	}
	for _, pad := range [][2]int{{-1, 12}, {0, -1}, {chromaticPaletteRows, 12}, {0, padColumns}} {
		require.Negativef(t, chromaticStepAt(pad[0], pad[1]), "pad %v reports a step", pad)
	}
}

// Pressing a cell in the step block moves the edit there, and A1 clears the step's note.
func TestChromaticStepBlockSelectsAndA1Removes(t *testing.T) {
	voiceBank := NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{{Name: "lead", Channel: 1}}}})
	bank := useController(t, NewFire(func([]byte) error { return nil }), voiceBank).patbank
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	voice := voiceBank.voices[0]

	// Put a note on step 5, then select step 5 from the block rather than the grid.
	require.NoError(t, bank.setStepCursor(5))
	require.NoError(t, bank.handleNoteEditPad(nil, 2, 3, 100, false, false))
	require.NoError(t, bank.setStepCursor(0))
	row, col := 1, chromaticPaletteColumns+1 // step 5
	require.NoError(t, bank.handleNoteEditPad(nil, row, col, 100, false, false))
	require.Equal(t, 5, bank.StepCursor(), "pressing the step block moved the edit")
	_, ok := bank.CurrentPattern().EventAtStep(5, voice)
	require.True(t, ok, "the note on step 5 disappeared")

	// Column 5 on the selected row is step 6 in step mode, so that pad is guarded until a
	// note has been chosen at this step.
	guarded, _ := bank.CurrentPattern().EventAtStep(5, voice)
	require.NoError(t, bank.handleNoteEditPad(nil, 0, 5, 100, false, false))
	event, _ := bank.CurrentPattern().EventAtStep(5, voice)
	require.Equal(t, guarded.ChromaticNote, event.ChromaticNote, "the step's own pad rewrote the pitch")

	// Choosing a note elsewhere lifts the guard, and a palette pad then edits step 5.
	require.NoError(t, bank.handleNoteEditPad(nil, 1, 3, 100, false, false))
	chosen, _ := chromaticPaletteNote(1, 3, 0)
	event, _ = bank.CurrentPattern().EventAtStep(5, voice)
	require.Equal(t, chosen, event.ChromaticNote, "step 5 pitch after choosing a note")

	require.NoError(t, bank.handleNoteEditPad(nil, 0, 5, 100, false, false))
	event, ok = bank.CurrentPattern().EventAtStep(5, voice)
	require.True(t, ok, "a palette pad stopped editing the selected step")
	want, _ := chromaticPaletteNote(0, 5, 0)
	require.Equal(t, want, event.ChromaticNote, "the lifted guard should let the pad through")

	// A1 is the palette's first pad and means no note.
	require.NoError(t, bank.handleNoteEditPad(nil, 0, 0, 100, false, false))
	_, stillThere := bank.CurrentPattern().EventAtStep(5, voice)
	require.False(t, stillThere, "A1 did not remove the note")
	_, elsewhere := bank.CurrentPattern().EventAtStep(0, voice)
	require.False(t, elsewhere, "A1 removed a note from the wrong step")
}

// A step cell shows that step's note colour, dark when the step is empty.
func TestChromaticStepCellsShowNoteColours(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	note, _ := chromaticPaletteNote(1, 4, 0)
	bank.CurrentPattern().SetChromaticNote(6, voice, note, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.setStepCursor(0))
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, bank.drawNotePalette())

	row, col := 1, chromaticPaletteColumns+2 // step 6
	require.Equal(t, chromaticPaletteColor(note), sim.pads[row*padColumns+col], "the step cell")
	// An empty step is dark.
	emptyRow, emptyCol := 0, chromaticPaletteColumns // step 0
	require.Equal(t, [3]int{}, sim.pads[emptyRow*padColumns+emptyCol], "an empty step cell is dark")
}

// A grid pad is a step selector in step mode and a pitch pad in note-edit mode. Pressing
// the same pad again in note-edit mode must not rewrite the note the user navigated to,
// until a note has been chosen at that step.
func TestPalettePadThatSelectedTheStepIsRefused(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	original := 40 // E2, which the palette pad below would not choose
	pattern.SetChromaticNote(6, voice, original, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	press := func(row, col, vel int) {
		require.NoError(t, dispatch(bank, padMessage(54+row*16+col, vel)))
	}
	row, col := 0, 6 // the pad that means "step 7" in step mode
	paletteNote, _ := chromaticPaletteNote(row, col, 0)
	require.NotEqual(t, original, paletteNote, "the test needs a pad whose pitch differs")

	// Select the step with that pad in step mode, then release it.
	press(row, col, 100)
	press(row, col, 0)
	require.Equal(t, 6, bank.StepCursor(), "the step press moved the cursor")
	require.False(t, bank.noteEditActive(), "the step press entered note-edit mode")

	// Enter note selection. The step's note must survive the same pad being pressed.
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	press(row, col, 100)
	press(row, col, 0)
	event, ok := pattern.EventAtStep(6, voice)
	require.True(t, ok, "the guarded pad removed the note")
	require.Equal(t, original, event.ChromaticNote, "the guarded pad rewrote the pitch")

	// Choosing a note lifts the guard, so the same pad is a pitch pad again.
	press(1, 3, 100)
	press(1, 3, 0)
	chosen, _ := pattern.EventAtStep(6, voice)
	require.NotEqual(t, original, chosen.ChromaticNote, "choosing a note on another pad did nothing")
	press(row, col, 100)
	event, _ = pattern.EventAtStep(6, voice)
	require.Equal(t, paletteNote, event.ChromaticNote, "the lifted guard should let the palette pitch through")
}

// The guard belongs to the step it was armed on: moving the cursor lifts it.
func TestPaletteGuardLiftsWhenTheCursorMoves(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(6, voice, 40, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	press := func(row, col, vel int) {
		require.NoError(t, dispatch(bank, padMessage(54+row*16+col, vel)))
	}
	row, col := 0, 6
	paletteNote, _ := chromaticPaletteNote(row, col, 0)

	press(row, col, 100)
	press(row, col, 0)
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	// The guard holds while the cursor stays on step 7.
	press(row, col, 100)
	press(row, col, 0)
	event, _ := pattern.EventAtStep(6, voice)
	require.Equal(t, 40, event.ChromaticNote, "the guard did not hold while the cursor stayed")

	// Moving to another step lifts it, so the pad is a pitch pad again.
	require.NoError(t, bank.MoveStepCursor(2))
	press(row, col, 100)
	event, _ = pattern.EventAtStep(8, voice)
	require.Equal(t, paletteNote, event.ChromaticNote, "the newly selected step took the palette pitch")
	event, _ = pattern.EventAtStep(6, voice)
	require.Equal(t, 40, event.ChromaticNote, "the note left behind changed")
}

// The pad standing for step 1 is also A1, the erase key, so the guard has to run before
// the removal or it deletes the note it is meant to protect. Alt still clears.
func TestStepOnePadDoesNotEraseTheNote(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 43, 100) // G2 on step 1
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	// The step pad for step 1 is palette (0,0), which is A1.
	require.NoError(t, bank.handleNoteEditPad(nil, 0, 0, 100, false, false))
	event, ok := pattern.EventAtStep(0, voice)
	require.True(t, ok, "the step's own pad removed the note")
	require.Equal(t, 43, event.ChromaticNote, "the step's own pad rewrote the note")

	// Alt on the same pad is a request about the step, so it clears.
	require.NoError(t, bank.handleNoteEditPad(nil, 0, 0, 100, true, false))
	_, cleared := pattern.EventAtStep(0, voice)
	require.False(t, cleared, "Alt on the step pad did not clear the note")
}

// paletteRegion is the pitch palette as the unit holds it, excluding the step strip.
func paletteRegion(sim *fireSim) [][3]int {
	var out [][3]int
	for row := 0; row < chromaticPaletteRows; row++ {
		for col := 0; col < chromaticPaletteColumns; col++ {
			out = append(out, sim.pads[row*padColumns+col])
		}
	}
	return out
}

// stripCell is one step cell as the unit holds it.
func stripCell(sim *fireSim, step int) [3]int {
	row, col, ok := chromaticStepCell(step)
	if !ok {
		return [3]int{}
	}
	return sim.pads[row*padColumns+col]
}

// Playing a pattern while choosing notes must not disturb the palette. The column
// playhead repaints every pad row, which in note-edit mode is the palette.
func TestNoteEditPlayheadLeavesThePaletteAlone(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	for _, step := range []int{1, 3, 7} {
		bank.CurrentPattern().SetChromaticNote(step, voice, 36+step, 100)
	}
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, dispatch(bank, padMessage(NotePlay, 100)))

	before := paletteRegion(sim)
	for step := 0; step < maxPatternSteps; step++ {
		require.NoError(t, installedPlayback(t, bank.controller).updatePads(stepBeat(step)))
	}
	for i, want := range before {
		row, col := i/chromaticPaletteColumns, i%chromaticPaletteColumns
		require.Equalf(t, want, sim.pads[row*padColumns+col],
			"palette pad row %d col %d changed during playback", row, col)
	}
}

// In note-edit mode the playhead moves along the step strip, one cell per step, and the
// cell it leaves goes back to that step's note colour.
func TestNoteEditPlayheadLightsTheStepStrip(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().SetChromaticNote(5, voice, 40, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, dispatch(bank, padMessage(NotePlay, 100)))

	require.NotEqual(t, oledWhite, stripCell(sim, 5), "step 5 was lit before the playhead arrived")
	require.NoError(t, installedPlayback(t, bank.controller).updatePads(stepBeat(5)))
	require.Equal(t, oledWhite, stripCell(sim, 5), "step 5 while the playhead is there")

	// The step the note is on keeps its colour apart from the playhead.
	noteColor := chromaticPaletteColor(40)
	require.NoError(t, installedPlayback(t, bank.controller).updatePads(stepBeat(6)))
	require.Equal(t, oledWhite, stripCell(sim, 6), "step 6 while the playhead is there")
	require.NotEqual(t, oledWhite, stripCell(sim, 5), "the cell the playhead left is still lit")
	require.Equal(t, noteColor, stripCell(sim, 5), "step 5 after the playhead left")
}

// Stopping must put the strip back, or a lit cell outlives the playback that put it there.
func TestNoteEditPlayheadClearsOnStop(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().SetChromaticNote(5, voice, 40, 100)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, bank.ToggleNoteMode())
	require.NoError(t, dispatch(bank, padMessage(NotePlay, 100)))
	require.NoError(t, installedPlayback(t, bank.controller).updatePads(stepBeat(5)))
	require.Equal(t, oledWhite, stripCell(sim, 5), "step 5 while playing")

	require.NoError(t, bank.controller.stopPlayback())
	require.NotEqual(t, oledWhite, stripCell(sim, 5), "stopping left a strip cell lit")
	require.Equal(t, chromaticPaletteColor(40), stripCell(sim, 5), "step 5 after stopping")
}

// playheadTestKit is one chromatic voice followed by one percussive voice, so the two
// colour rules can be compared side by side.
func playheadTestKit() *VoiceBank {
	kick := 36
	return NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}, {Name: "kick", Note: &kick, Channel: 1}},
	}})
}

// The playhead must not flatten a chromatic step to green. It keeps the pitch colour, and
// the column behind it restores that exact colour rather than a flat one.
func TestPlayheadKeepsChromaticPitchColour(t *testing.T) {
	bank, kit, sim := recordedBank(t, playheadTestKit())
	chromatic, percussive := kit.voices[0], kit.voices[1]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(3, chromatic, 40, 100) // E2, untied
	pattern.ToggleEvent(Event{Voice: percussive, Beat: stepBeat(3), Velocity: 100})

	pitch := chromaticPaletteColor(40)
	drum := chromaticEventColor(Event{Voice: percussive})

	require.NoError(t, bank.drawPadColumnInvert(3))
	require.Equal(t, invertColor(pitch), sim.pads[3], "the inverted chromatic step")
	require.Equal(t, invertColor(drum), sim.pads[3+padColumns], "the inverted percussive step")

	// The column the playhead leaves goes back to the real colours.
	require.NoError(t, bank.drawPadColumn(3))
	require.Equal(t, pitch, sim.pads[3], "the restored chromatic step")
	require.Equal(t, drum, sim.pads[3+padColumns], "the restored percussive step")
}

// A tie is marked by pushing the colour away from the playhead, which means lifting it
// normally and lowering it when inverted, where lifting would be invisible.
func TestPlayheadMarksTiesBothWays(t *testing.T) {
	bank, kit, sim := recordedBank(t, playheadTestKit())
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(3, voice, 40, 100)
	pattern.SetChromaticNote(5, voice, 43, 100)
	pattern.TieEventsAtSteps(3, 5, voice)

	require.NoError(t, bank.drawPadColumn(3))
	require.Equal(t, markTieColor(chromaticPaletteColor(40), false), sim.pads[3],
		"a tied step behind the playhead")

	require.NoError(t, bank.drawPadColumnInvert(3))
	lowered := markTieColor(invertColor(chromaticPaletteColor(40)), true)
	require.Equal(t, lowered, sim.pads[3], "a tied step under the playhead")
	// The mark has to differ from the unmarked colour in both directions.
	require.NotEqual(t, invertColor(chromaticPaletteColor(40)), lowered,
		"a tie under the playhead is not distinguishable")
}
