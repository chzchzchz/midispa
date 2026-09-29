package main

import (
	"fmt"
	"math/bits"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex/akai"
)

// The palette is laid out as four consecutive sixteen-semitone bands.
const (
	chromaticBaseNote        = 21
	chromaticPaletteRows     = 4
	chromaticPaletteColumns  = 16
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

func chromaticPaletteNote(row, col int) int {
	if row < 0 || row >= chromaticPaletteRows {
		row = 0
	}
	if col < 0 || col >= chromaticPaletteColumns {
		col = 0
	}
	return clampMidiDataValue(chromaticBaseNote + row*chromaticPaletteColumns + col)
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

func (p *PatternBank) syncChromaticVelocity() {
	if p == nil {
		return
	}
	voice := p.SelectedVoice()
	pattern := p.CurrentPattern()
	if voice == nil || !voice.IsChromatic() || pattern == nil {
		return
	}
	if event, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
		p.chromaticVelocity = clampMidiDataValue(event.Velocity)
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
	p.stepCursor += delta
	p.clampStepCursor()
	p.syncChromaticVelocity()
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
	p.stepCursor = step
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

func (p *PatternBank) AdjustChromaticVelocity(aseq midiWriter, encoderValue int) error {
	if p == nil || p.editingLength {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() {
		return nil
	}
	p.syncChromaticVelocity()
	var value int
	switch encoderValue {
	case EncoderRight:
		value = p.chromaticVelocity + chromaticVelocityStep
	case EncoderLeft:
		value = p.chromaticVelocity - chromaticVelocityStep
	default:
		return nil
	}
	value = clampMidiDataValue(value)
	p.chromaticVelocity = value
	if pattern := p.CurrentPattern(); pattern != nil {
		if event, ok := pattern.SetChromaticVelocity(p.stepCursor, voice, value); ok {
			if err := p.auditionChromaticEvent(aseq, event); err != nil {
				return err
			}
		}
	}
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

// In note-edit mode the grid chooses a pitch, while Alt plus a pad removes the event.
func (p *PatternBank) handleNoteEditPad(aseq midiWriter, row, col int) error {
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() || !p.editingNote {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	step := p.stepCursor
	if altOn {
		pattern.RemoveEventAtStep(step, voice)
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printChromaticStatus()
	}
	event, ok := pattern.SetChromaticNote(step, voice, chromaticPaletteNote(row, col), p.chromaticVelocity)
	if !ok {
		return nil
	}
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
	p.clearPadState()
	if active {
		p.syncChromaticVelocity()
	}
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

// The palette uses a stable color per pitch and highlights the current event's pitch.
func (p *PatternBank) drawNotePalette() error {
	if p == nil || p.f == nil {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() {
		return nil
	}
	pads := make([]akai.Pad, 0, chromaticPaletteRows*chromaticPaletteColumns)
	selectedNote := -1
	if pattern := p.CurrentPattern(); pattern != nil {
		if event, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
			selectedNote = event.ChromaticNote
		}
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		for col := 0; col < chromaticPaletteColumns; col++ {
			note := chromaticPaletteNote(row, col)
			color := chromaticPaletteColor(note)
			if note == selectedNote {
				color = oledWhite
			}
			pads = append(pads, makePad(col, row, color))
		}
	}
	return p.f.LightPadSlice(pads)
}

func chromaticPaletteColor(note int) [3]int {
	index := note - chromaticBaseNote
	if index < 0 {
		index = 0
	}
	color := oledColorTable[index%len(oledColorTable)]
	return Dim(color, 4)
}

func chromaticStatusText(step int, event *Event, velocity, tieStep int) string {
	note := "--"
	if event != nil {
		note = midiNoteName(event.ChromaticNote)
	}
	text := fmt.Sprintf("S%02d %s@%03d", step+1, note, velocity)
	if event != nil && event.Tie && tieStep >= 0 {
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
		return p.f.ClearOLEDRows(lengthDisplayRow, 1)
	}
	if err := p.f.ClearOLEDRows(lengthDisplayRow, 1); err != nil {
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
	return p.f.Print(0, lengthDisplayRow, fitOLEDText(chromaticStatusText(p.stepCursor, eventPtr, velocity, tieStep)))
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

func markTieColor(color [3]int) [3]int {
	for i := range color {
		if color[i] < 32 {
			color[i] = 32
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
