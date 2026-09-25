package main

import (
	"fmt"
	"strings"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex/akai"
)

// The palette is laid out as four consecutive sixteen-semitone bands.
const (
	chromaticBaseNote       = 21
	chromaticPaletteRows    = 4
	chromaticPaletteColumns = 16
)

var chromaticNoteNames = [...]string{
	"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B",
}

func isPadRelease(status byte, velocity int) bool {
	return midi.IsNoteOff(status) || (midi.IsNoteOn(status) && velocity == 0)
}

// SelectedVoice returns the voice controlled by the selected mute row.
func (p *PatternBank) SelectedVoice() *Voice {
	if p == nil || p.selTrackRow < 1 || p.selTrackRow > len(p.trackVoices) || p.vb == nil {
		return nil
	}
	index := p.trackVoices[p.selTrackRow-1]
	if index < 0 || index >= len(p.vb.voices) {
		return nil
	}
	return p.vb.voices[index]
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

// redrawPatternRows keeps all step rows synchronized after a cursor or note edit.
func (p *PatternBank) redrawPatternRows() error {
	for row := 1; row <= 4; row++ {
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
	p.rowPadMasks = [4]uint16{}
	p.rowPadCounts = [4]int{}
}

func (p *PatternBank) pressPad(row, col int) (bool, int) {
	if p == nil || row < 0 || row >= len(p.rowPadMasks) || col < 0 || col >= chromaticPaletteColumns {
		return false, 0
	}
	bit := uint64(1) << uint(row*chromaticPaletteColumns+col)
	if p.pressedPads&bit != 0 {
		return false, p.rowPadCounts[row]
	}
	p.pressedPads |= bit
	p.rowPadMasks[row] |= uint16(1) << uint(col)
	p.rowPadCounts[row]++
	return true, p.rowPadCounts[row]
}

func (p *PatternBank) heldPadCount() int {
	if p == nil {
		return 0
	}
	count := 0
	for mask := p.pressedPads; mask != 0; mask &= mask - 1 {
		count++
	}
	return count
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
	p.rowPadCounts[row]--
	if p.rowPadCounts[row] < 0 {
		p.rowPadCounts[row] = 0
	}
}

// Only a newly pressed pad can complete a two-pad tie gesture.
func (p *PatternBank) handleChromaticStepPress(row, col int) (bool, error) {
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() || p.editingNote {
		return false, nil
	}
	newPad, count := p.pressPad(row, col)
	if !newPad {
		return true, nil
	}
	if row < 0 || row >= len(p.trackVoices) {
		return true, nil
	}
	if row != p.selTrackRow-1 {
		if p.vb == nil {
			return true, nil
		}
		voiceIndex := p.trackVoices[row]
		if voiceIndex < 0 || voiceIndex >= len(p.vb.voices) {
			return true, nil
		}
		pressedVoice := p.vb.voices[voiceIndex]
		if pressedVoice != nil && pressedVoice.IsChromatic() {
			return true, nil
		}
		return false, nil
	}
	if count != 2 || p.heldPadCount() != 2 {
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
	pattern.TieEventsAtSteps(steps[0], steps[1], voice)
	return true, p.printChromaticStatus()
}

// In note-edit mode the grid chooses a pitch, while Alt plus a pad removes the event.
func (p *PatternBank) handleNoteEditPad(aseq midiWriter, row, col, velocity int) error {
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
		if err := p.redrawPatternRows(); err != nil {
			return err
		}
		return p.printChromaticStatus()
	}
	event, ok := pattern.SetChromaticNote(step, voice, chromaticPaletteNote(row, col), velocity)
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
	return writeMidiMsgs(aseq, eventDestination(event), [][]byte{event.NoteOnMidi(), event.NoteOffMidi()})
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

// The bottom OLED row shows the current step, pitch, and tie target.
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
	if !ok {
		return p.f.Print(0, lengthDisplayRow, fitOLEDText(fmt.Sprintf("Step %02d Note --", p.stepCursor+1)))
	}
	text := fmt.Sprintf("Step %02d Note %s", p.stepCursor+1, midiNoteName(event.ChromaticNote))
	if event.Tie {
		if next, nextOK := pattern.NextTiedEvent(p.stepCursor, voice); nextOK {
			text = fmt.Sprintf("Tie %02d->%02d %s", p.stepCursor+1, eventStep(next)+1, midiNoteName(event.ChromaticNote))
		}
	}
	return p.f.Print(0, lengthDisplayRow, fitOLEDText(text))
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
