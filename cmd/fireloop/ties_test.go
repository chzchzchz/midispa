package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chzchzchz/midispa/midi"
)

func TestChromaticPadGesturesAndReleases(t *testing.T) {
	writeCount := 0
	bank, voice := chromaBankOn(t, func([]byte) error {
		writeCount++
		return nil
	})
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	if handled, err := bank.handleChromaticStepPress(0, 0); !handled || err != nil {
		t.Fatalf("first pad press = %v/%v", handled, err)
	}
	writeCount = 0
	if handled, err := bank.handleChromaticStepPress(0, 1); !handled || err != nil {
		t.Fatalf("second pad press = %v/%v", handled, err)
	}
	if writeCount != 6 {
		t.Fatalf("tie gesture wrote %d Fire messages, want row and status redraws", writeCount)
	}
	if event, _ := pattern.EventAtStep(0, voice); !event.Tie {
		t.Fatal("two held pads did not tie adjacent events")
	}
	bank.releasePad(0, 0)
	bank.releasePad(0, 1)
	if bank.pressedPads != 0 || bank.rowPadMasks[0] != 0 {
		t.Fatalf("pad state did not clear on release: held=%x row=%x", bank.pressedPads, bank.rowPadMasks[0])
	}

	bank.clearPadState()
	pattern.UntieEventsAtSteps(0, 1, voice)
	if handled, _ := bank.handleChromaticStepPress(0, 0); !handled {
		t.Fatal("selected chromatic row was not handled")
	}
	if handled, _ := bank.handleChromaticStepPress(1, 1); !handled {
		t.Fatal("cross-row gesture was not handled as a gesture")
	}
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("cross-row gesture created a tie")
	}
	pattern.TieEventsAtSteps(0, 1, voice)
	bank.clearPadState()
	if handled, _ := bank.handleChromaticStepPress(0, 2); !handled {
		t.Fatal("third selected-row pad was not handled")
	}
	if event, _ := pattern.EventAtStep(0, voice); !event.Tie {
		t.Fatal("third held pad removed the completed tie")
	}

	bank.clearPadState()
	empty := &Pattern{}
	bank.Patterns[bank.selPatIdx] = empty
	bank.handleChromaticStepPress(0, 0)
	bank.handleChromaticStepPress(0, 1)
	if len(empty.Events) != 0 {
		t.Fatal("empty chromatic steps created events")
	}

	bank.clearPadState()
	bank.handleChromaticStepPress(0, 0)
	previousPatbank := patbank
	patbank = bank
	t.Cleanup(func() { patbank = previousPatbank })
	if err := processPatternEvent(nil, releaseMessage(54)); err != nil {
		t.Fatal(err)
	}
	if bank.pressedPads != 0 || bank.rowPadMasks[0] != 0 {
		t.Fatal("NoteOff was not treated as a release")
	}
	bank.handleChromaticStepPress(0, 0)
	if err := processPatternEvent(nil, padMessage(54, 0)); err != nil {
		t.Fatal(err)
	}
	if bank.pressedPads != 0 || bank.rowPadMasks[0] != 0 {
		t.Fatal("NoteOn velocity zero was not treated as a release")
	}
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
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	// Hold the cell for step 1, then press the cell for step 4 without letting go.
	press := func(step, velocity int) {
		if err := processPatternEvent(nil, padMessage(stepCellPad(step), velocity)); err != nil {
			t.Fatal(err)
		}
	}
	press(0, 100)
	press(3, 100)
	first, _ := pattern.EventAtStep(0, voice)
	second, _ := pattern.EventAtStep(3, voice)
	if !first.Tie {
		t.Fatalf("two held cells did not tie step 1 to step 4: %+v", first)
	}
	if second.Tie {
		t.Fatal("the tie landed on the later event as well")
	}
	// The edit stays on the step it came from, which is what a tie is holding.
	if bank.StepCursor() != 0 {
		t.Fatalf("the tie moved the edit to step %d", bank.StepCursor()+1)
	}
	// Letting go of both ends the gesture.
	press(0, 0)
	press(3, 0)
	if bank.noteEditHeldStep != noHeldStep {
		t.Fatalf("held step = %d after releasing the cells", bank.noteEditHeldStep)
	}

	// The tie is what the pattern plays back: the first note sounds until the tied step.
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step < 5; step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		if _, err := playback.playBeat(writer, pattern); err != nil {
			t.Fatal(err)
		}
		switch step {
		case 0:
			// The tie starts the note and nothing releases it afterwards on its own.
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 100})
			if len(writer.events) != 1 {
				t.Fatalf("step 1 wrote %d messages, want only the note-on", len(writer.events))
			}
		case 1, 2:
			// The gap the tie covers: silence on the wire, not a note-off.
			if len(writer.events) != 0 {
				t.Fatalf("step %d wrote %d messages, want none while the note is held", step+1, len(writer.events))
			}
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
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	press := func(step, velocity int) {
		if err := processPatternEvent(nil, padMessage(stepCellPad(step), velocity)); err != nil {
			t.Fatal(err)
		}
	}

	// A cell on its own just moves the edit.
	press(0, 100)
	if bank.StepCursor() != 0 {
		t.Fatalf("the first cell press moved the edit to step %d", bank.StepCursor()+1)
	}
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("one cell press tied a step")
	}
	// Step 3 holds no note, so there is nothing for step 1 to hold on to.
	press(3, 100)
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("a step with no note was tied")
	}
	if bank.StepCursor() != 0 {
		t.Fatalf("a refused tie moved the edit to step %d", bank.StepCursor()+1)
	}
	// Step 5 would tie step 1 to step 7, but step 5's own note sits between them, and
	// tying across it would make that note unreachable. Refused, so the edit stays put.
	press(0, 0)
	press(6, 100)
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("a tie crossed an intervening note")
	}
	if bank.StepCursor() != 0 {
		t.Fatalf("a refused tie moved the edit to step %d", bank.StepCursor()+1)
	}
	if event, _ := pattern.EventAtStep(4, voice); event.Tie {
		t.Fatal("a refused tie landed on the wrong event")
	}
	// Releasing between presses ends the gesture, so two separate presses never tie.
	press(4, 0)
	press(0, 100)
	press(1, 100)
	press(1, 0)
	press(0, 0)
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("a released cell still took part in the gesture")
	}
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
	if !pattern.TieEventsAtSteps(0, 3, voice) {
		t.Fatal("tie 1-4 refused")
	}
	if !pattern.TieEventsAtSteps(3, 6, voice) {
		t.Fatal("tie 4-7 refused")
	}
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
		if _, err := playback.playBeat(writer, pattern); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, event := range writer.events {
			kind := "on"
			if midi.IsNoteOff(event.Data[0]) {
				kind = "off"
			}
			got = append(got, fmt.Sprintf("%s %d", kind, event.Data[1]))
		}
		if strings.Join(got, ", ") != strings.Join(want[step], ", ") {
			t.Fatalf("step %d wrote [%s], want [%s]", step+1,
				strings.Join(got, ", "), strings.Join(want[step], ", "))
		}
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
	if !pattern.TieEventsAtSteps(0, 3, voice) {
		t.Fatal("tie 1-4 refused")
	}
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step <= 4; step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		if _, err := playback.playBeat(writer, pattern); err != nil {
			t.Fatal(err)
		}
		switch step {
		case 0:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 40})
			if len(writer.events) != 1 {
				t.Fatalf("step 1 wrote %d messages, want only the note-on", len(writer.events))
			}
		case 3:
			// The tied step is silent on the wire, so its velocity cannot be applied.
			if len(writer.events) != 0 {
				t.Fatalf("step 4 wrote %d messages, want none from a same-pitch tie", len(writer.events))
			}
		case 4:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 60, 0})
		default:
			if len(writer.events) != 0 {
				t.Fatalf("step %d wrote %d messages, want none while the note is held", step+1, len(writer.events))
			}
		}
		writer.events = nil
	}
}
