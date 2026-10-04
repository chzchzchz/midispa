package main

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex/akai"
)

// The palette runs from A1 upwards, laid out so every row starts on A and spans the twelve
// semitones to the G# above it, which is what "A to G#" means once octave numbering turns
// over at A. It uses the first twelve columns of each sixteen-column pad row; the last four
// are left dark, because a row that wrapped past G# would put the next row's A somewhere
// unexpected. Rows therefore cover A1 to G#2, A2 to G#3, A3 to G#4 and A4 to G#5.
// A tie mark lifts a dark channel to tieMarkAmount, or lowers an inverted one from
// tieMarkFloor.
const (
	tieMarkAmount = 32
	tieMarkFloor  = 95
)

const (
	chromaticBaseNote       = 33
	chromaticPaletteRows    = 4
	chromaticPaletteColumns = 12
	// The columns past the palette are a strip of step indicators, one cell per step.
	chromaticStepColumns     = padColumns - chromaticPaletteColumns
	chromaticStepCells       = chromaticPaletteRows * chromaticStepColumns
	defaultChromaticVelocity = midi.DataMax
	chromaticVelocityStep    = 1
)

var chromaticNoteNames = [...]string{
	"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B",
}

func isPadRelease(status byte, velocity int) bool {
	return midi.IsNoteOff(status) || (midi.IsNoteOn(status) && velocity == 0)
}

// SelectedVoice returns the voice controlled by the selected mute row.
func (p *PatternBank) SelectedVoice() *Voice {
	if p == nil {
		return nil
	}
	return p.trackVoice(p.selTrackRow)
}

func (p *PatternBank) StepCursor() int {
	if p == nil {
		return 0
	}
	return p.stepCursor
}

func (p *PatternBank) NoteEditActive() bool {
	return p != nil && p.editingNote
}

// chromaticPaletteNote is the pitch at a palette position. A row or column outside the
// palette has no pitch, which is reported rather than clamped, so pressing an unused
// column cannot assign the first note of the first row by accident.
func chromaticPaletteNote(row, col int) (int, bool) {
	if row < 0 || row >= chromaticPaletteRows {
		return 0, false
	}
	if col < 0 || col >= chromaticPaletteColumns {
		return 0, false
	}
	return clampMidiDataValue(chromaticBaseNote + row*chromaticPaletteColumns + col), true
}

func midiNoteName(note int) string {
	note = clampMidiDataValue(note)
	return fmt.Sprintf("%s%d", chromaticNoteNames[note%len(chromaticNoteNames)], note/12-1)
}

func (p *PatternBank) clampStepCursor() {
	if p == nil {
		return
	}
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

// redrawPatternRows keeps all step rows synchronized after a cursor or note edit.
func (p *PatternBank) redrawPatternRows() error {
	for row := 1; row <= padRows; row++ {
		if err := p.redrawTrackPads(row); err != nil {
			return err
		}
	}
	return nil
}

// MoveStepCursor keeps the edit target inside the active pattern length.
func (p *PatternBank) MoveStepCursor(delta int) error {
	if p == nil || p.editingLength {
		return nil
	}
	previous := p.stepCursor
	p.stepCursor += delta
	p.clampStepCursor()
	if p.stepCursor != previous {
		// A guard belongs to the step that was left behind.
		p.noteChosen = false
	}
	logger.Debug("cursor", "step", p.stepCursor)
	if p.editingNote {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printChromaticStatus()
	}
	if err := p.redrawPatternRows(); err != nil {
		return err
	}
	return p.printChromaticStatus()
}

func (p *PatternBank) setStepCursor(step int) error {
	if p == nil {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil || step < 0 || step >= pattern.LengthSteps() {
		return nil
	}
	if step != p.stepCursor {
		// A guard belongs to the step that was left behind.
		p.noteChosen = false
	}
	p.stepCursor = step
	logger.Debug("cursor", "step", p.stepCursor)
	if p.editingNote {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printChromaticStatus()
	}
	if err := p.redrawPatternRows(); err != nil {
		return err
	}
	return p.printChromaticStatus()
}

// The pad masks model held hardware state, rather than a timing window for gestures.
func (p *PatternBank) clearPadState() {
	if p == nil {
		return
	}
	p.pressedPads = 0
	p.rowPadMasks = [padRows]uint16{}
}

func (p *PatternBank) pressPad(row, col int) bool {
	if p == nil || row < 0 || row >= len(p.rowPadMasks) || col < 0 || col >= chromaticPaletteColumns {
		return false
	}
	bit := uint64(1) << uint(row*chromaticPaletteColumns+col)
	if p.pressedPads&bit != 0 {
		return false
	}
	p.pressedPads |= bit
	p.rowPadMasks[row] |= uint16(1) << uint(col)
	return true
}

func (p *PatternBank) heldPadCount() int {
	if p == nil {
		return 0
	}
	return bits.OnesCount64(p.pressedPads)
}

func (p *PatternBank) releasePad(row, col int) {
	if p == nil || row < 0 || row >= len(p.rowPadMasks) || col < 0 || col >= chromaticPaletteColumns {
		return
	}
	bit := uint64(1) << uint(row*chromaticPaletteColumns+col)
	if p.pressedPads&bit == 0 {
		return
	}
	p.pressedPads &^= bit
	p.rowPadMasks[row] &^= uint16(1) << uint(col)
}

// AdjustChromaticVelocity moves the Volume encoder by one detent. The encoder carries its
// own value: it is not re-read from the selected step, so a step picked after the encoder
// was set takes the encoder's value on the next detent rather than its own. Each detent
// writes that value to the selected step.
func (p *PatternBank) AdjustChromaticVelocity(aseq midiWriter, encoderValue int) error {
	if p == nil || p.editingLength {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() {
		return nil
	}
	from := p.chromaticVelocity
	var value int
	switch encoderValue {
	case EncoderRight:
		value = from + chromaticVelocityStep
	case EncoderLeft:
		value = from - chromaticVelocityStep
	default:
		return nil
	}
	value = clampMidiDataValue(value)
	p.chromaticVelocity = value
	// The step is in the log so a velocity change can always be traced to one step rather
	// than guessed at from the sound.
	updated := false
	if pattern := p.CurrentPattern(); pattern != nil {
		if event, ok := pattern.SetChromaticVelocity(p.stepCursor, voice, value); ok {
			updated = true
			if err := p.auditionChromaticEvent(aseq, event); err != nil {
				return err
			}
		}
	}
	logger.Debug("volume knob", "step", p.stepCursor, "from", from, "to", value, "noteUpdated", updated)
	if p.editingNote {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	}
	return p.printChromaticStatus()
}

// Only a newly pressed pad can complete a two-pad tie gesture.
func (p *PatternBank) handleChromaticStepPress(row, col int) (bool, error) {
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() || p.editingNote {
		return false, nil
	}
	newPad := p.pressPad(row, col)
	if !newPad {
		return true, nil
	}
	pressedVoice := p.trackVoice(row + 1)
	if pressedVoice == nil {
		// The pad row holds no track, so there is nothing to edit or tie there.
		return true, nil
	}
	if row != p.selTrackRow-1 {
		if pressedVoice.IsChromatic() {
			return true, nil
		}
		return false, nil
	}
	rowPadCount := bits.OnesCount16(p.rowPadMasks[row])
	if rowPadCount == 1 {
		if err := p.setStepCursor(col); err != nil {
			return true, err
		}
	}
	if rowPadCount != 2 || p.heldPadCount() != 2 {
		return true, nil
	}
	steps := make([]int, 0, 2)
	for col := 0; col < chromaticPaletteColumns; col++ {
		if p.rowPadMasks[row]&(uint16(1)<<uint(col)) != 0 {
			steps = append(steps, col)
		}
	}
	if len(steps) != 2 {
		return true, nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return true, nil
	}
	if pattern.TieEventsAtSteps(steps[0], steps[1], voice) {
		if err := p.redrawPatternRows(); err != nil {
			return true, err
		}
	}
	return true, p.printChromaticStatus()
}

// stepCellPaintColor is how a step cell reads right now: white while the playhead is on
// it, otherwise the step's note colour, brightened while it is the step being edited.
// Sharing one rule means any redraw keeps the playhead visible.
func (p *PatternBank) stepCellPaintColor(pattern *Pattern, voice *Voice, step int) [3]int {
	if step == p.playheadStep {
		return oledWhite
	}
	return p.stepCellColor(pattern, voice, step)
}

// chromaticStepCell is the grid position of the strip cell standing for a step.
func chromaticStepCell(step int) (row, col int, ok bool) {
	if step < 0 || step >= chromaticStepCells {
		return 0, 0, false
	}
	return step / chromaticStepColumns, chromaticPaletteColumns + step%chromaticStepColumns, true
}

// drawStepCell paints one strip cell.
func (p *PatternBank) drawStepCell(step int) error {
	row, col, ok := chromaticStepCell(step)
	if !ok || p.f == nil {
		return nil
	}
	voice := p.SelectedVoice()
	pattern := p.CurrentPattern()
	if voice == nil || !voice.IsChromatic() || pattern == nil {
		return nil
	}
	color := p.stepCellPaintColor(pattern, voice, step)
	return p.f.LightPad(col, row, color[0], color[1], color[2])
}

// drawStepPlayhead moves the note-edit playhead along the step strip. The column playhead
// stays out of note-edit mode, where it would overwrite the pitch palette.
func (p *PatternBank) drawStepPlayhead(step int) error {
	previous := p.playheadStep
	if previous == step {
		return nil
	}
	p.playheadStep = step
	if err := p.drawStepCell(previous); err != nil {
		return err
	}
	return p.drawStepCell(step)
}

// clearStepPlayhead puts the strip back when playback stops, so no cell is left lit.
func (p *PatternBank) clearStepPlayhead() error {
	if p.playheadStep == noPlayheadStep {
		return nil
	}
	previous := p.playheadStep
	p.playheadStep = noPlayheadStep
	return p.drawStepCell(previous)
}

// guardsPad reports whether a palette pad is the one standing for the step being edited.
// On the selected track's own pad row, column n is step n in step mode, so that press
// means "this step" rather than a pitch. Only the first palette columns are involved: past
// them the right-hand block already treats a press as a step.
func (p *PatternBank) guardsPad(row, col int) bool {
	if p.noteChosen || p.selTrackRow < 1 || row != p.selTrackRow-1 {
		return false
	}
	return col == p.stepCursor && col < chromaticPaletteColumns
}

// In note-edit mode the left block chooses a pitch for the current step and the right-hand
// block chooses which step is being edited. Alt plus a pad removes the event, and so does
// A1, the palette's first pad, which stands for "no note here". How hard a pad was hit sets
// the note's velocity, so a new step lands with the dynamics that were played and the
// display reports that same value.
func (p *PatternBank) handleNoteEditPad(aseq midiWriter, row, col, pressed int) error {
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() || !p.editingNote {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	if step := chromaticStepAt(row, col); step >= 0 {
		// The right-hand block edits a step rather than assigning a pitch to one.
		return p.setStepCursor(step)
	}
	step := p.stepCursor
	note, onPalette := chromaticPaletteNote(row, col)
	if !onPalette {
		// Neither a pitch nor a step, so there is nothing here to edit.
		return nil
	}
	if p.guardsPad(row, col) && !altOn {
		// This pad stands for the step being edited, so treating it as a pitch pad here
		// would rewrite the note the user was trying to reach, and as A1 it would erase
		// it. The guard lifts once a note has been chosen at this step. Alt still clears,
		// because that is a request about the step rather than a pitch.
		logger.Debug("pitch refused", "step", step, "padRow", row, "padCol", col)
		return nil
	}
	if altOn || note == chromaticBaseNote {
		// A1 is where the palette starts, which makes it the natural key for "no note".
		reason := "palette"
		if altOn {
			reason = "alt"
		}
		pattern.RemoveEventAtStep(step, voice)
		logger.Debug("pitch removed", "step", step, "via", reason)
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printChromaticStatus()
	}
	// How hard the pad was pressed is the step's dynamics, whether the note is new or its
	// pitch is being changed. The Volume encoder still adjusts the value afterwards.
	velocity := clampMidiDataValue(pressed)
	event, ok := pattern.SetChromaticNote(step, voice, note, velocity)
	if !ok {
		return nil
	}
	// A note has been chosen here, so the step's own pad is a pitch pad again.
	p.noteChosen = true
	logger.Debug("pitch", "step", step, "note", note, "velocity", event.Velocity)
	if err := p.auditionChromaticEvent(aseq, event); err != nil {
		return err
	}
	if err := p.drawNotePalette(); err != nil {
		return err
	}
	return p.printChromaticStatus()
}

// Audition uses the edited velocity and always closes the preview with velocity zero.
func (p *PatternBank) auditionChromaticEvent(aseq midiWriter, event Event) error {
	if event.Voice == nil {
		return nil
	}
	destination := eventDestination(event)
	return writeMidiMsgs(aseq, destination, [][]byte{event.NoteOnMidi(), event.NoteOffMidi()})
}

// Mode is a state toggle only for the selected chromatic voice.
func (p *PatternBank) setNoteEdit(active bool) error {
	if p == nil {
		return nil
	}
	voice := p.SelectedVoice()
	if active && (voice == nil || !voice.IsChromatic()) {
		return nil
	}
	p.editingNote = active
	// The step rows are about to be redrawn, which puts the strip back on its own, so
	// the note-edit playhead stops lighting a cell here.
	p.playheadStep = noPlayheadStep
	p.clearPadState()
	if p.f == nil {
		return nil
	}
	if active {
		if err := p.f.SetLed(NoteMode, LEDGreen); err != nil {
			return err
		}
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printChromaticStatus()
	}
	if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
		return err
	}
	if err := p.redrawPatternRows(); err != nil {
		return err
	}
	return p.printChromaticStatus()
}

func (p *PatternBank) ToggleNoteMode() error {
	if p == nil || p.editingLength {
		return nil
	}
	return p.setNoteEdit(!p.editingNote)
}

// stepCellColor is how one cell of the step strip reads: the step's note colour, dark when
// the step holds no note, and brightened while it is the step being edited.
func (p *PatternBank) stepCellColor(pattern *Pattern, voice *Voice, step int) [3]int {
	if pattern == nil {
		return [3]int{}
	}
	event, ok := pattern.EventAtStep(step, voice)
	if !ok {
		return [3]int{}
	}
	color := chromaticPaletteColor(event.NoteNumber())
	if step == p.stepCursor {
		color = markCursorColor(color)
	}
	return color
}

// The palette uses a stable color per pitch and highlights the current event's pitch.
func (p *PatternBank) drawNotePalette() error {
	if p == nil || p.f == nil {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() {
		return nil
	}
	pads := make([]akai.Pad, 0, chromaticPaletteRows*padColumns)
	pattern := p.CurrentPattern()
	selectedNote := -1
	if pattern != nil {
		if event, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
			selectedNote = event.ChromaticNote
		}
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		for col := 0; col < padColumns; col++ {
			color := [3]int{}
			if step := chromaticStepAt(row, col); step >= 0 {
				color = p.stepCellPaintColor(pattern, voice, step)
			} else if note, onPalette := chromaticPaletteNote(row, col); onPalette {
				color = chromaticPaletteColor(note)
				if note == selectedNote {
					color = oledWhite
				}
			}
			pads = append(pads, makePad(col, row, color))
		}
	}
	return p.f.LightPadSlice(pads)
}

// chromaticStepAt maps a pad in the right-hand block to the step it stands for, or -1 when
// the pad belongs to the pitch palette instead. The block reads in the same order as the
// step grid, four steps per row.
func chromaticStepAt(row, col int) int {
	if row < 0 || row >= chromaticPaletteRows {
		return -1
	}
	if col < chromaticPaletteColumns || col >= padColumns {
		return -1
	}
	return row*chromaticStepColumns + (col - chromaticPaletteColumns)
}

func chromaticPaletteColor(note int) [3]int {
	index := note - chromaticBaseNote
	if index < 0 {
		index = 0
	}
	color := oledColorTable[index%len(oledColorTable)]
	return Dim(color, 4)
}

// chromaticStatusText shows the step, its note and its velocity. A step with no note shows
// no velocity at all, because the value the next note would inherit is not that step's
// velocity and reading it as one made two steps look equal.
func chromaticStatusText(step int, event *Event, velocity, tieStep int) string {
	if event == nil {
		return fmt.Sprintf("S%02d --", step+1)
	}
	text := fmt.Sprintf("S%02d %s@%03d", step+1, midiNoteName(event.ChromaticNote), velocity)
	if event.Tie && tieStep >= 0 {
		text = fmt.Sprintf("%s->%02d", text, tieStep+1)
	}
	return text
}

// The bottom OLED row shows the current step, pitch, velocity, and tie target.
func (p *PatternBank) printChromaticStatus() error {
	if p == nil || p.f == nil || p.editingLength {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() {
		return p.clearTextRows(lengthDisplayRow, 1)
	}
	if err := p.clearTextRows(lengthDisplayRow, 1); err != nil {
		return err
	}
	pattern := p.CurrentPattern()
	event, ok := Event{}, false
	if pattern != nil {
		event, ok = pattern.EventAtStep(p.stepCursor, voice)
	}
	velocity := p.chromaticVelocity
	tieStep := -1
	var eventPtr *Event
	if ok {
		velocity = event.Velocity
		eventPtr = &event
		if next, nextOK := pattern.NextTiedEvent(p.stepCursor, voice); nextOK {
			tieStep = eventStep(next)
		}
	}
	return p.printText(lengthDisplayRow, 0, fitOLEDText(chromaticStatusText(p.stepCursor, eventPtr, velocity, tieStep)), false)
}

func fitOLEDText(text string) string {
	const maxTextLength = 20
	if len(text) > maxTextLength {
		return strings.TrimSpace(text[:maxTextLength])
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

func chromaticEventColor(event Event) [3]int {
	if !event.IsChromatic() {
		return [3]int{0, 50, 0}
	}
	return chromaticPaletteColor(event.ChromaticNote)
}

// A tied step is marked by pushing its colour away from the playhead: a dark colour is
// lifted, and an inverted one is pushed down, because lifting an already bright colour
// would be invisible.
func markTieColor(color [3]int, invert bool) [3]int {
	for i := range color {
		if invert {
			if color[i] > tieMarkFloor {
				color[i] -= tieMarkAmount
			}
			continue
		}
		if color[i] < tieMarkAmount {
			color[i] = tieMarkAmount
		}
	}
	return color
}

func markCursorColor(color [3]int) [3]int {
	for i := range color {
		color[i] += 24
		if color[i] > midiNoteMax {
			color[i] = midiNoteMax
		}
	}
	return color
}
