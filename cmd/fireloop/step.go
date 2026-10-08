package main

import (
	"fmt"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// Step editing that is not specific to one kind of voice: which step the cursor is on, moving
// it, leaving the modes that change what a pad means, and the readout that reports it. The
// controls that do depend on a kind of voice live with it: the pitch palette in palette.go,
// the tie gesture in ties.go, and the Volume knob in velocity.go.

func isPadRelease(status byte, velocity int) bool {
	return midi.IsNoteOff(status) || (midi.IsNoteOn(status) && velocity == 0)
}

// SelectedVoice returns the voice controlled by the selected mute row.
func (p *PatternBank) SelectedVoice() *Voice {
	return p.trackVoice(p.selTrackRow)
}

func (p *PatternBank) StepCursor() int {
	return p.stepCursor
}

// noteEditActive and lengthEditActive are the two ways of asking what mode the grid is in.
// They exist so a reader states which mode it means rather than reading a bare flag.
func (p *PatternBank) noteEditActive() bool {
	return p.mode == noteEdit
}

func (p *PatternBank) lengthEditActive() bool {
	return p.mode == lengthEdit
}

// swingEditActive is the third way of asking what mode the grid is in. It joins the enum
// rather than sitting beside it as a flag: swing entry takes the pads as a keypad and the
// readout row, exactly what length entry takes, so the two are the same kind of exclusive
// claim and a bank holding both flags at once would let two readouts fight over one row.
func (p *PatternBank) swingEditActive() bool {
	return p.mode == swingEdit
}

// claimMode records the mode the grid is in, or steps back out of it when active is false.
// It leaves every other mode alone: choosing a pattern or scrolling the track window is not
// a request to give up the length, and the modes are exclusive, so stepping out has anything
// to clear only when the mode named is the one that was on.
//
// The way back to stepEdit is written once here rather than in each of the three toggles,
// because three copies of it is three chances for one of them to forget and leave a bank in
// a mode nothing can leave.
func (p *PatternBank) claimMode(active bool, mode editMode) {
	if active {
		p.mode = mode
	} else if p.mode == mode {
		p.mode = stepEdit
	}
}

func (p *PatternBank) clampStepCursor() {
	pattern := p.CurrentPattern()
	if pattern == nil {
		p.stepCursor = 0
		return
	}
	length := pattern.LengthSteps()
	if p.stepCursor < 0 {
		p.stepCursor = 0
	}
	if p.stepCursor >= length {
		p.stepCursor = length - 1
	}
}

// redrawStepRows repaints every row's steps together, which is what a cursor or a note
// edit needs: the rows are one grid on the unit, so a row left showing where it was reads
// as a note that is not there.
func (p *PatternBank) redrawStepRows() error {
	for row := 1; row <= padRows; row++ {
		if err := p.redrawTrackPads(row); err != nil {
			return err
		}
	}
	return nil
}

// readoutIsOwned reports whether an entry mode owns the readout row.
// Every editor that would write a line holds this guard, because a
// line written over a number being typed is the failure the entry
// modes exist to prevent.
func (p *PatternBank) readoutIsOwned() bool {
	return p.lengthEditActive() || p.swingEditActive()
}

// MoveStepCursor keeps the edit target inside the active pattern length.
func (p *PatternBank) MoveStepCursor(delta int) error {
	if p.readoutIsOwned() {
		return nil
	}
	p.stepCursor += delta
	p.clampStepCursor()
	logger.Debug("cursor", "step", p.stepCursor)
	return p.repaintEditView()
}

func (p *PatternBank) setStepCursor(step int) error {
	pattern := p.CurrentPattern()
	if pattern == nil || step < 0 || step >= pattern.LengthSteps() {
		return nil
	}
	p.stepCursor = step
	logger.Debug("cursor", "step", p.stepCursor)
	return p.repaintEditView()
}

// followStepCursor moves the step cursor onto the step a pad press touched, when
// the press landed on the selected row. The cursor is what the Volume knob and
// the status row aim at, so a percussive hit can be trimmed the moment it is
// placed instead of after a walk along the grid buttons. A press on any other row
// leaves it where it is: those rows edit their own tracks, and the selection is
// what decides which voice the knob trims.
func (p *PatternBank) followStepCursor(row, col int) error {
	if row+1 != p.selTrackRow {
		return nil
	}
	return p.setStepCursor(col)
}

// repaintEditView puts the display back after an edit: the pitch palette while note
// editing owns the grid, the step rows otherwise, and then the readout the edit is
// reported on. Which of those two the redraw is depends on the mode rather than on the
// edit, so it is written once here rather than repeated at every place an edit ends.
func (p *PatternBank) repaintEditView() error {
	if p.noteEditActive() {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printStepStatus()
	}
	if err := p.redrawStepRows(); err != nil {
		return err
	}
	return p.printStepStatus()
}

// The pad masks model held hardware state, rather than a timing window for gestures.
func (p *PatternBank) clearPadState() {
	p.pressedPads = 0
	p.rowPadMasks = [padRows]uint16{}
	p.noteEditHeldStep = noHeldStep
}

// Audition uses the edited velocity and always closes the preview with velocity zero.
func (p *PatternBank) auditionEvent(aseq alsa.EventWriter, event Event) error {
	if event.Voice == nil {
		return nil
	}
	destination := eventDestination(event)
	return writeMidiMsgs(aseq, destination, [][]byte{event.NoteOnMidi(), event.NoteOffMidi()})
}

// Mode is a state toggle only for the selected chromatic voice.
func (p *PatternBank) setNoteEdit(active bool) error {
	voice := p.SelectedVoice()
	if active && (voice == nil || !voice.IsChromatic()) {
		return nil
	}
	p.claimMode(active, noteEdit)
	// The step rows are about to be redrawn, which puts the strip back on its own, so
	// the note-edit playhead stops lighting a cell here.
	p.playheadStep = noPlayheadStep
	p.clearPadState()
	if active {
		if err := p.pads.SetLed(NoteMode, LEDGreen); err != nil {
			return err
		}
	} else if err := p.pads.SetLed(NoteMode, LEDOff); err != nil {
		return err
	}
	return p.repaintEditView()
}

func (p *PatternBank) ToggleNoteMode() error {
	if p.readoutIsOwned() {
		return nil
	}
	return p.setNoteEdit(p.noteEditActive() == false)
}

// stepMarks is where a step reaches: the step a tie runs into, the first
// step of the triplet group covering it, and how many cells that group
// owns. All are answers about the step rather than about the event on it,
// and the readout shows them, so they are named once and passed once.
// groupCells tells the readout an eighth group's consumed cell, the
// 1-3 clause, apart from a cell that holds notes, because the x3 mark is
// the same for both kinds. It is also what says a group is there at all:
// zero cells is no group, which keeps the zero value of this struct
// meaning "nothing reached this step".
type stepMarks struct {
	tieStep    int
	groupStart int
	groupCells int
}

// stepStatusText is the readout for the step under the cursor: its note, its velocity, and
// the step a tie runs into. A percussive step has no pitch to name, so it reports its
// dynamics alone. A step holding no note shows no velocity at all, because the value the
// next note would inherit is not that step's velocity and reading it as one made two steps
// look equal. A step a triplet group covers carries the group's x3 mark, and the cell a
// group consumes shows the notes it belongs to instead of a velocity, because a dark cell
// under the cursor otherwise reads as a step with nothing on it.
func stepStatusText(voice *Voice, step int, event *Event, marks stepMarks) string {
	if event == nil {
		group := tripletGroup{start: marks.groupStart, cells: marks.groupCells}
		if group.consumed(step) {
			return fmt.Sprintf("S%02d x3 %02d-%02d", step+1, marks.groupStart+1, marks.groupStart+tripletNotes)
		}
		return fmt.Sprintf("S%02d --", step+1)
	}
	pitch := ""
	if voice.IsChromatic() {
		pitch = midiNoteName(event.ChromaticNote)
	}
	text := fmt.Sprintf("S%02d %s@%03d", step+1, pitch, event.Velocity)
	if marks.groupCells != 0 {
		text = fmt.Sprintf("%s x3", text)
	}
	if event.Tie && marks.tieStep >= 0 {
		text = fmt.Sprintf("%s->%02d", text, marks.tieStep+1)
	}
	return text
}

// The bottom OLED row shows the current step, its note and its velocity, and the tie target
// when there is one. A percussive track reports the step and its dynamics too, because the
// Volume knob sets them and a readout that went blank would leave the knob turning blind.
func (p *PatternBank) printStepStatus() error {
	// The guard belongs here rather than in the twelve callers: every one of them ends at
	// this line, and a guard in one of them covers neither the other eleven nor the next one
	// anybody writes.
	if p.readoutIsOwned() {
		return nil
	}
	if err := p.clearTextRows(readoutRow, 1); err != nil {
		return err
	}
	voice := p.SelectedVoice()
	if voice == nil {
		return nil
	}
	pattern := p.CurrentPattern()
	// The tie target and the group a step belongs to are both
	// answers about the step rather than about the event on it,
	// so they are resolved together and passed together.
	marks := stepMarks{tieStep: -1, groupStart: noGroup}
	var event *Event
	if pattern != nil {
		if current, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
			event = &current
			if next, nextOK := pattern.NextTiedEvent(p.stepCursor, voice); nextOK {
				marks.tieStep = eventStep(next)
			}
		}
		if group, covered := pattern.TripletGroupAt(p.stepCursor, voice); covered {
			marks.groupStart = group.start
			marks.groupCells = group.cells
		}
	}
	return p.printText(readoutRow, 0, fitOLEDText(stepStatusText(voice, p.stepCursor, event, marks)), false)
}

func fitOLEDText(text string) string {
	// Replacing what the font cannot draw comes first so that the clip counts glyphs: every
	// rune is one byte afterwards, so a cut can no longer land inside one.
	text = asciiFontText(text)
	if len(text) > oledTextWidth {
		return strings.TrimSpace(text[:oledTextWidth])
	}
	return text
}

// A missing device is valid in unit tests, so use the zero address as a safe fallback.
func eventDestination(event Event) alsa.SeqAddr {
	if event.Voice != nil && event.Voice.device != nil {
		return event.Voice.device.SeqAddr
	}
	return alsa.SeqAddr{}
}

func voiceDisplayName(v *Voice) string {
	if v == nil {
		return ""
	}
	mode := "DRM"
	if v.IsChromatic() {
		mode = "CHR"
	}
	return fitOLEDText(fmt.Sprintf("%s [%s]", v.Name, mode))
}
