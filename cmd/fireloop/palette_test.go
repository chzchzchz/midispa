package main

import (
	"fmt"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func TestChromaticPaletteAndModeEditing(t *testing.T) {
	writeCount := 0
	bank, voice := chromaBankOn(t, func([]byte) error {
		writeCount++
		return nil
	})
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	if !bank.NoteEditActive() {
		t.Fatal("Mode did not enter note-edit mode")
	}
	if err := handlePatternGrid(nil, 1, 0, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	firstNote, _ := chromaticPaletteNote(0, 1, 0)
	if !ok || event.ChromaticNote != firstNote {
		t.Fatalf("palette assignment = %+v/%v", event, ok)
	}
	preview := &captureMidiWriter{}
	if err := bank.auditionEvent(preview, event); err != nil {
		t.Fatal(err)
	}
	if len(preview.events) != 2 {
		t.Fatalf("audition wrote %d messages, want 2", len(preview.events))
	}
	// The palette pad was pressed at pressVelocity, so that is the note's velocity.
	assertMidiData(t, preview.events[0], []byte{midi.MakeNoteOn(0), byte(firstNote), pressVelocity})
	assertMidiData(t, preview.events[1], []byte{midi.MakeNoteOff(0), byte(firstNote), 0})
	if err := processPatternEvent(nil, padMessage(NoteGridRight, 100)); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 1 {
		t.Fatalf("grid right moved cursor to %d, want 1", bank.StepCursor())
	}
	altOn = true
	writeCount = 0
	if err := handlePatternGrid(nil, 1, 0, 100); err != nil {
		t.Fatal(err)
	}
	if writeCount != 3 {
		t.Fatalf("Alt clear wrote %d Fire messages, want palette and status writes", writeCount)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(1, voice); ok {
		t.Fatal("Alt plus palette pad did not clear the current-step event")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); !ok {
		t.Fatal("Alt plus palette pad cleared a different step")
	}
}

// The pitch palette is four octaves from A1, twelve semitones to the row, so every row
// starts on A and the four columns past G# carry no pitch.
func TestChromaticPaletteLayout(t *testing.T) {
	if got, _ := chromaticPaletteNote(0, 0, 0); got != 33 {
		t.Fatalf("first palette note = %d (%s), want A1 at 33", got, midiNoteName(got))
	}
	if name := midiNoteName(33); name != "A1" {
		t.Fatalf("MIDI 33 names as %q, want A1", name)
	}
	if chromaticPaletteColumns != 12 || chromaticStepColumns != 4 {
		t.Fatalf("palette is %d columns with %d for steps, want 12 and 4", chromaticPaletteColumns, chromaticStepColumns)
	}
	if chromaticPaletteColumns+chromaticStepColumns != padColumns {
		t.Fatalf("palette %d plus steps %d does not fill a %d column row",
			chromaticPaletteColumns, chromaticStepColumns, padColumns)
	}
	if chromaticStepCells != maxPatternSteps {
		t.Fatalf("step block has %d cells, want one per step", chromaticStepCells)
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		first, ok := chromaticPaletteNote(row, 0, 0)
		if !ok {
			t.Fatalf("row %d has no first note", row)
		}
		if name := midiNoteName(first); name != fmt.Sprintf("A%d", row+1) {
			t.Fatalf("row %d starts on %s, want A%d", row, name, row+1)
		}
		last, ok := chromaticPaletteNote(row, chromaticPaletteColumns-1, 0)
		if !ok {
			t.Fatalf("row %d has no last note", row)
		}
		// Twelve semitones from A is a major seventh, so the row ends on the G# above
		// its starting A: row 0 runs A1 up to G#2.
		if name := midiNoteName(last); name != fmt.Sprintf("G#%d", row+2) {
			t.Fatalf("row %d ends on %s, want G#%d", row, name, row+2)
		}
		if last-first != 11 {
			t.Fatalf("row %d spans %d semitones, want the 11 from A to G#", row, last-first)
		}
		// The columns past the palette have no pitch at all.
		for _, col := range []int{12, 13, 14, 15} {
			if note, ok := chromaticPaletteNote(row, col, 0); ok {
				t.Fatalf("row %d column %d has pitch %d, want none", row, col, note)
			}
		}
	}
	if _, ok := chromaticPaletteNote(-1, 0, 0); ok {
		t.Fatal("a row above the palette reported a pitch")
	}
	if _, ok := chromaticPaletteNote(0, -1, 0); ok {
		t.Fatal("a column left of the palette reported a pitch")
	}
}

// paletteTestBank is a bank sitting in note-edit mode on a chromatic track, which is the
// only state the palette octave is used from.
func paletteTestBank(t *testing.T) (*PatternBank, *Voice, *fireSim) {
	t.Helper()
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
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

	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, selectKnobCC(64)); err != nil {
		t.Fatal(err)
	}
	if bank.paletteOctave != 0 {
		t.Fatalf("palette octave = %d after a value that is not a turn, want 0", bank.paletteOctave)
	}
	if bank.trackVoice(1) != voiceBefore {
		t.Fatal("a value that is not a turn moved the track's voice")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); !ok {
		t.Fatal("the note placed before the stray value is gone")
	}
}

// The SELECT knob is an octave transpose for the palette: it moves which pitches the pads
// choose, and nothing that is already written into the pattern.
func TestSelectKnobShiftsPaletteByOctave(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	voiceBefore := bank.trackVoice(1)

	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	first, _ := chromaticPaletteNote(1, 3, 0)
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != first {
		t.Fatalf("palette note = %d, want %d", event.ChromaticNote, first)
	}

	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	if bank.paletteOctave != 1 {
		t.Fatalf("palette octave = %d after one detent, want 1", bank.paletteOctave)
	}
	if bank.trackVoice(1) != voiceBefore {
		t.Fatal("the knob moved the track's voice instead of the palette")
	}
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != first {
		t.Fatalf("stored note = %d after the shift, want it left at %d", event.ChromaticNote, first)
	}

	// The same pad now reaches the pitch an octave up.
	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	up, _ := chromaticPaletteNote(1, 3, bank.paletteOctave)
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != up {
		t.Fatalf("note after shifting up = %d, want %d", event.ChromaticNote, up)
	}
	if up != first+chromaticOctaveShift {
		t.Fatalf("shifted palette note = %d, want an octave above %d", up, first)
	}

	// And an octave down takes it back past where it started.
	for range 2 {
		if err := processPatternEvent(nil, selectKnobCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	down, _ := chromaticPaletteNote(1, 3, bank.paletteOctave)
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != down {
		t.Fatalf("note after shifting down = %d, want %d", event.ChromaticNote, down)
	}
	if down != first-chromaticOctaveShift {
		t.Fatalf("lowered palette note = %d, want an octave below %d", down, first)
	}
}

// The palette has to say which octave it is offering, so every pad's colour travels with it
// while the steps keep the colours of the notes they hold.
func TestPaletteShiftMovesColours(t *testing.T) {
	bank, voice, sim := paletteTestBank(t)
	bank.CurrentPattern().SetChromaticNote(3, voice, 40, pressVelocity)
	if err := bank.setStepCursor(3); err != nil {
		t.Fatal(err)
	}
	if err := bank.drawNotePalette(); err != nil {
		t.Fatal(err)
	}
	stepColour := stripCell(sim, 3)
	before := paletteRegion(sim)

	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	after := paletteRegion(sim)
	changed := 0
	for i := range before {
		row, col := i/chromaticPaletteColumns, i%chromaticPaletteColumns
		index := row*padColumns + col
		if before[i] != after[i] {
			changed++
		}
		note, _ := chromaticPaletteNote(row, col, bank.paletteOctave)
		if want := chromaticPaletteColor(note); sim.pads[index] != want {
			t.Fatalf("palette pad row %d col %d = %v, want the shifted pitch colour %v",
				row, col, sim.pads[index], want)
		}
	}
	if changed != len(before) {
		t.Fatalf("%d of %d palette pads changed colour, want the shift visible on all of them",
			changed, len(before))
	}
	// A note already in the pattern keeps its own colour; only the palette moved.
	if got := stripCell(sim, 3); got != stepColour {
		t.Fatalf("step 3 cell = %v after the shift, want the note colour %v", got, stepColour)
	}
}

// A palette below the unshifted base must keep its colours apart instead of collapsing them
// onto the first entry of the table.
func TestPaletteBelowBaseKeepsDistinctColours(t *testing.T) {
	bank, _, sim := paletteTestBank(t)
	for range 2 {
		if err := processPatternEvent(nil, selectKnobCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	if bank.paletteOctave >= 0 {
		t.Fatalf("palette octave = %d, want the palette below its base", bank.paletteOctave)
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		seen := make(map[[3]int]bool, chromaticPaletteColumns)
		for col := 0; col < chromaticPaletteColumns; col++ {
			seen[sim.pads[row*padColumns+col]] = true
		}
		if len(seen) != chromaticPaletteColumns {
			t.Fatalf("row %d has %d colours across %d pads, want one each",
				row, len(seen), chromaticPaletteColumns)
		}
	}
}

// The palette may only turn as far as the MIDI range allows, or a pad would name a note
// that cannot be sent.
func TestPaletteOctaveStaysInsideMIDIRange(t *testing.T) {
	bank, _, _ := paletteTestBank(t)
	lowest, highest := chromaticOctaveBounds()
	if lowest != -2 || highest != 3 {
		t.Fatalf("octave bounds = %d to %d, want -2 to 3 for a palette from A1", lowest, highest)
	}

	for _, direction := range []int{EncoderRight, EncoderLeft} {
		for range 8 {
			if err := processPatternEvent(nil, selectKnobCC(direction)); err != nil {
				t.Fatal(err)
			}
		}
		octave := bank.paletteOctave
		// Turning further must not move it, or the palette would leave the MIDI range.
		if err := processPatternEvent(nil, selectKnobCC(direction)); err != nil {
			t.Fatal(err)
		}
		if bank.paletteOctave != octave {
			t.Fatalf("palette octave = %d after turning past the end, want it held at %d",
				bank.paletteOctave, octave)
		}
		base := bank.paletteBase()
		last, _ := chromaticPaletteNote(chromaticPaletteRows-1, chromaticPaletteColumns-1, octave)
		if base < 0 || last > midiNoteMax {
			t.Fatalf("palette spans %d to %d, outside the MIDI range", base, last)
		}
		if direction == EncoderRight && octave != highest {
			t.Fatalf("palette stopped at octave %d, want the top %d", octave, highest)
		}
		if direction == EncoderLeft && octave != lowest {
			t.Fatalf("palette stopped at octave %d, want the bottom %d", octave, lowest)
		}
	}
}

// The erase key is wherever the palette starts, so it moves with the palette.
func TestPaletteFirstPadErasesAfterShift(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	base := bank.paletteBase()
	if base == chromaticBaseNote {
		t.Fatalf("palette base = %d, want it shifted off A1", base)
	}
	// A pad an octave along places the shifted pitch.
	if err := bank.handleNoteEditPad(nil, 1, 0, pressVelocity); err != nil {
		t.Fatal(err)
	}
	placed, _ := chromaticPaletteNote(1, 0, bank.paletteOctave)
	if placed == base {
		t.Fatalf("the test needs a pad that is not the palette's first one: %d", placed)
	}
	if event, ok := bank.CurrentPattern().EventAtStep(0, voice); !ok || event.ChromaticNote != placed {
		t.Fatalf("note = %d/%v, want the shifted palette pitch %d", event.ChromaticNote, ok, placed)
	}
	// The palette's first pad is the erase key wherever the palette has moved to. Placing
	// the note above also lifted the guard on that pad, which doubles as the step-one pad.
	if err := bank.handleNoteEditPad(nil, 0, 0, pressVelocity); err != nil {
		t.Fatal(err)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); ok {
		t.Fatalf("the palette's first pad at %d did not erase", base)
	}
}

// Outside note-edit mode the knob is still what chooses the track's voice, and the palette
// is left where the user put it.
func TestSelectKnobJogsVoiceOutsideNoteEdit(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	before := bank.trackVoice(1)
	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	if bank.trackVoice(1) == before {
		t.Fatal("the knob did not jog the track's voice")
	}
	if bank.paletteOctave != 0 {
		t.Fatalf("palette octave = %d outside note-edit mode, want it untouched", bank.paletteOctave)
	}
	if bank.NoteEditActive() {
		t.Fatal("the knob entered note-edit mode")
	}
}

// The right-hand block stands for the steps themselves: it reads in the same order as the
// step grid, four per row, and covers every step exactly once.
func TestChromaticStepBlockCoversEveryStep(t *testing.T) {
	seen := make(map[int]bool, chromaticStepCells)
	for row := 0; row < chromaticPaletteRows; row++ {
		for offset := 0; offset < chromaticStepColumns; offset++ {
			col := chromaticPaletteColumns + offset
			step := chromaticStepAt(row, col)
			if step != row*chromaticStepColumns+offset {
				t.Fatalf("cell row %d offset %d = step %d", row, offset, step)
			}
			if step < 0 || step >= maxPatternSteps {
				t.Fatalf("cell row %d offset %d maps outside the pattern: %d", row, offset, step)
			}
			if seen[step] {
				t.Fatalf("step %d is shown by more than one cell", step)
			}
			seen[step] = true
		}
	}
	if len(seen) != maxPatternSteps {
		t.Fatalf("the block covers %d steps, want %d", len(seen), maxPatternSteps)
	}
	// The palette side of the grid is not a step.
	for col := 0; col < chromaticPaletteColumns; col++ {
		if step := chromaticStepAt(0, col); step >= 0 {
			t.Fatalf("palette column %d reports step %d", col, step)
		}
	}
	for _, pad := range [][2]int{{-1, 12}, {0, -1}, {chromaticPaletteRows, 12}, {0, padColumns}} {
		if step := chromaticStepAt(pad[0], pad[1]); step >= 0 {
			t.Fatalf("pad %v reports step %d", pad, step)
		}
	}
}

// Pressing a cell in the step block moves the edit there, and A1 clears the step's note.
func TestChromaticStepBlockSelectsAndA1Removes(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{{Name: "lead", Channel: 1}}}})
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	voice := voiceBank.voices[0]

	// Put a note on step 5, then select step 5 from the block rather than the grid.
	if err := bank.setStepCursor(5); err != nil {
		t.Fatal(err)
	}
	if err := bank.handleNoteEditPad(nil, 2, 3, 100); err != nil {
		t.Fatal(err)
	}
	if err := bank.setStepCursor(0); err != nil {
		t.Fatal(err)
	}
	row, col := 1, chromaticPaletteColumns+1 // step 5
	if err := bank.handleNoteEditPad(nil, row, col, 100); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 5 {
		t.Fatalf("cursor = %d after pressing the step block, want 5", bank.StepCursor())
	}
	if _, ok := bank.CurrentPattern().EventAtStep(5, voice); !ok {
		t.Fatal("the note on step 5 disappeared")
	}
	// Column 5 on the selected row is step 6 in step mode, so that pad is guarded until a
	// note has been chosen at this step.
	guarded, _ := bank.CurrentPattern().EventAtStep(5, voice)
	if err := bank.handleNoteEditPad(nil, 0, 5, 100); err != nil {
		t.Fatal(err)
	}
	if event, _ := bank.CurrentPattern().EventAtStep(5, voice); event.ChromaticNote != guarded.ChromaticNote {
		t.Fatalf("the step's own pad changed the pitch to %d", event.ChromaticNote)
	}
	// Choosing a note elsewhere lifts the guard, and a palette pad then edits step 5.
	if err := bank.handleNoteEditPad(nil, 1, 3, 100); err != nil {
		t.Fatal(err)
	}
	chosen, _ := chromaticPaletteNote(1, 3, 0)
	if event, _ := bank.CurrentPattern().EventAtStep(5, voice); event.ChromaticNote != chosen {
		t.Fatalf("step 5 pitch = %d, want %d", event.ChromaticNote, chosen)
	}
	if err := bank.handleNoteEditPad(nil, 0, 5, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(5, voice)
	if !ok {
		t.Fatal("a palette pad stopped editing the selected step")
	}
	want, _ := chromaticPaletteNote(0, 5, 0)
	if event.ChromaticNote != want {
		t.Fatalf("step 5 pitch = %d, want the lifted guard to allow %d", event.ChromaticNote, want)
	}

	// A1 is the palette's first pad and means no note.
	if err := bank.handleNoteEditPad(nil, 0, 0, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(5, voice); ok {
		t.Fatal("A1 did not remove the note")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); ok {
		t.Fatal("A1 removed a note from the wrong step")
	}
}

// A step cell shows that step's note colour, dark when the step is empty.
func TestChromaticStepCellsShowNoteColours(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	note, _ := chromaticPaletteNote(1, 4, 0)
	bank.CurrentPattern().SetChromaticNote(6, voice, note, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.setStepCursor(0); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := bank.drawNotePalette(); err != nil {
		t.Fatal(err)
	}
	row, col := 1, chromaticPaletteColumns+2 // step 6
	index := row*padColumns + col
	if sim.pads[index] != chromaticPaletteColor(note) {
		t.Fatalf("step 6 cell = %v, want the note colour %v", sim.pads[index], chromaticPaletteColor(note))
	}
	// An empty step is dark.
	emptyRow, emptyCol := 0, chromaticPaletteColumns // step 0
	if sim.pads[emptyRow*padColumns+emptyCol] != [3]int{} {
		t.Fatalf("empty step cell = %v, want dark", sim.pads[emptyRow*padColumns+emptyCol])
	}
}

// A grid pad is a step selector in step mode and a pitch pad in note-edit mode. Pressing
// the same pad again in note-edit mode must not rewrite the note the user navigated to,
// until a note has been chosen at that step.
func TestPalettePadThatSelectedTheStepIsRefused(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	original := 40 // E2, which the palette pad below would not choose
	pattern.SetChromaticNote(6, voice, original, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	press := func(row, col, vel int) {
		if err := processPatternEvent(nil, padMessage(54+row*16+col, vel)); err != nil {
			t.Fatal(err)
		}
	}
	row, col := 0, 6 // the pad that means "step 7" in step mode
	paletteNote, _ := chromaticPaletteNote(row, col, 0)
	if paletteNote == original {
		t.Fatalf("the test needs a palette pad whose pitch differs from %d", original)
	}

	// Select the step with that pad in step mode, then release it.
	press(row, col, 100)
	press(row, col, 0)
	if bank.StepCursor() != 6 || bank.NoteEditActive() {
		t.Fatalf("after the step press: cursor=%d noteEdit=%v", bank.StepCursor(), bank.NoteEditActive())
	}
	// Enter note selection. The step's note must survive the same pad being pressed.
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	press(row, col, 100)
	press(row, col, 0)
	event, ok := pattern.EventAtStep(6, voice)
	if !ok || event.ChromaticNote != original {
		t.Fatalf("note = %d/%v, want the guarded pad to leave it at %d", event.ChromaticNote, ok, original)
	}

	// Choosing a note lifts the guard, so the same pad is a pitch pad again.
	press(1, 3, 100)
	press(1, 3, 0)
	chosen, _ := pattern.EventAtStep(6, voice)
	if chosen.ChromaticNote == original {
		t.Fatal("choosing a note on another pad did nothing")
	}
	press(row, col, 100)
	event, _ = pattern.EventAtStep(6, voice)
	if event.ChromaticNote != paletteNote {
		t.Fatalf("note = %d, want the lifted guard to allow the palette pitch %d", event.ChromaticNote, paletteNote)
	}
}

// The guard belongs to the step it was armed on: moving the cursor lifts it.
func TestPaletteGuardLiftsWhenTheCursorMoves(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(6, voice, 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	press := func(row, col, vel int) {
		if err := processPatternEvent(nil, padMessage(54+row*16+col, vel)); err != nil {
			t.Fatal(err)
		}
	}
	row, col := 0, 6
	paletteNote, _ := chromaticPaletteNote(row, col, 0)

	press(row, col, 100)
	press(row, col, 0)
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	// The guard holds while the cursor stays on step 7.
	press(row, col, 100)
	press(row, col, 0)
	if event, _ := pattern.EventAtStep(6, voice); event.ChromaticNote != 40 {
		t.Fatalf("note = %d while the guard should hold, want 40", event.ChromaticNote)
	}
	// Moving to another step lifts it, so the pad is a pitch pad again.
	if err := bank.MoveStepCursor(2); err != nil {
		t.Fatal(err)
	}
	press(row, col, 100)
	if event, _ := pattern.EventAtStep(8, voice); event.ChromaticNote != paletteNote {
		t.Fatalf("note on the newly selected step = %d, want %d", event.ChromaticNote, paletteNote)
	}
	if event, _ := pattern.EventAtStep(6, voice); event.ChromaticNote != 40 {
		t.Fatalf("the note left behind changed to %d", event.ChromaticNote)
	}
}

// The pad standing for step 1 is also A1, the erase key, so the guard has to run before
// the removal or it deletes the note it is meant to protect. Alt still clears.
func TestStepOnePadDoesNotEraseTheNote(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 43, 100) // G2 on step 1
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	// The step pad for step 1 is palette (0,0), which is A1.
	if err := bank.handleNoteEditPad(nil, 0, 0, 100); err != nil {
		t.Fatal(err)
	}
	if event, ok := pattern.EventAtStep(0, voice); !ok || event.ChromaticNote != 43 {
		t.Fatalf("note = %d/%v, want the step's own pad to leave it at 43", event.ChromaticNote, ok)
	}
	// Alt on the same pad is a request about the step, so it clears.
	altOn = true
	if err := bank.handleNoteEditPad(nil, 0, 0, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := pattern.EventAtStep(0, voice); ok {
		t.Fatal("Alt on the step pad did not clear the note")
	}
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
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NotePlay, 100)); err != nil {
		t.Fatal(err)
	}
	before := paletteRegion(sim)
	for step := 0; step < maxPatternSteps; step++ {
		if err := patbank.playback.updatePads(stepBeat(step)); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range before {
		row, col := i/chromaticPaletteColumns, i%chromaticPaletteColumns
		if got := sim.pads[row*padColumns+col]; got != want {
			t.Fatalf("palette pad row %d col %d changed from %v to %v during playback", row, col, want, got)
		}
	}
}

// In note-edit mode the playhead moves along the step strip, one cell per step, and the
// cell it leaves goes back to that step's note colour.
func TestNoteEditPlayheadLightsTheStepStrip(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().SetChromaticNote(5, voice, 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NotePlay, 100)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got == oledWhite {
		t.Fatalf("step 5 cell = %v before the playhead arrived, want its note colour", got)
	}
	if err := patbank.playback.updatePads(stepBeat(5)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got != oledWhite {
		t.Fatalf("step 5 cell = %v while the playhead is there, want white", got)
	}
	// The step the note is on keeps its colour apart from the playhead.
	noteColor := chromaticPaletteColor(40)
	if err := patbank.playback.updatePads(stepBeat(6)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 6); got != oledWhite {
		t.Fatalf("step 6 cell = %v while the playhead is there, want white", got)
	}
	if got := stripCell(sim, 5); got == oledWhite {
		t.Fatal("the cell the playhead left is still lit")
	}
	if got := stripCell(sim, 5); got != noteColor {
		t.Fatalf("step 5 cell = %v after the playhead left, want the note colour %v", got, noteColor)
	}
}

// Stopping must put the strip back, or a lit cell outlives the playback that put it there.
func TestNoteEditPlayheadClearsOnStop(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().SetChromaticNote(5, voice, 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NotePlay, 100)); err != nil {
		t.Fatal(err)
	}
	if err := patbank.playback.updatePads(stepBeat(5)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got != oledWhite {
		t.Fatalf("step 5 cell = %v, want white while playing", got)
	}
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got == oledWhite {
		t.Fatal("stopping left a strip cell lit")
	}
	if got := stripCell(sim, 5); got != chromaticPaletteColor(40) {
		t.Fatalf("step 5 cell = %v after stopping, want the note colour", got)
	}
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

	if err := bank.drawPadColumnInvert(3); err != nil {
		t.Fatal(err)
	}
	if got, want := sim.pads[3], invertColor(pitch); got != want {
		t.Fatalf("inverted chromatic step = %v, want the inverted pitch colour %v", got, want)
	}
	if got, want := sim.pads[3+padColumns], invertColor(drum); got != want {
		t.Fatalf("inverted percussive step = %v, want %v", got, want)
	}

	// The column the playhead leaves goes back to the real colours.
	if err := bank.drawPadColumn(3); err != nil {
		t.Fatal(err)
	}
	if got := sim.pads[3]; got != pitch {
		t.Fatalf("restored chromatic step = %v, want the pitch colour %v", got, pitch)
	}
	if got := sim.pads[3+padColumns]; got != drum {
		t.Fatalf("restored percussive step = %v, want %v", got, drum)
	}
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

	if err := bank.drawPadColumn(3); err != nil {
		t.Fatal(err)
	}
	lifted := markTieColor(chromaticPaletteColor(40), false)
	if got := sim.pads[3]; got != lifted {
		t.Fatalf("tied step behind the playhead = %v, want the lifted colour %v", got, lifted)
	}
	if err := bank.drawPadColumnInvert(3); err != nil {
		t.Fatal(err)
	}
	lowered := markTieColor(invertColor(chromaticPaletteColor(40)), true)
	if got := sim.pads[3]; got != lowered {
		t.Fatalf("tied step under the playhead = %v, want the lowered colour %v", got, lowered)
	}
	// The mark has to differ from the unmarked colour in both directions.
	if lowered == invertColor(chromaticPaletteColor(40)) {
		t.Fatal("a tie under the playhead is not distinguishable")
	}
}
