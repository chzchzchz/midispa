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
	return p.applyStepCursor()
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
	return p.applyStepCursor()
}

// applyStepCursor redraws whatever an edit target change affects: the palette while note
// editing owns the grid, and the step rows otherwise.
func (p *PatternBank) applyStepCursor() error {
	logger.Debug("cursor", "step", p.stepCursor)
	if p.editingNote {
		if err := p.drawNotePalette(); err != nil {
			return err
		}
		return p.printStepStatus()
	}
	if err := p.redrawPatternRows(); err != nil {
		return err
	}
	return p.printStepStatus()
}

// The pad masks model held hardware state, rather than a timing window for gestures.
func (p *PatternBank) clearPadState() {
	if p == nil {
		return
	}
	p.pressedPads = 0
	p.rowPadMasks = [padRows]uint16{}
	p.noteEditHeldStep = noHeldStep
}

// Audition uses the edited velocity and always closes the preview with velocity zero.
func (p *PatternBank) auditionEvent(aseq midiWriter, event Event) error {
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
		return p.printStepStatus()
	}
	if err := p.f.SetLed(NoteMode, LEDOff); err != nil {
		return err
	}
	if err := p.redrawPatternRows(); err != nil {
		return err
	}
	return p.printStepStatus()
}

func (p *PatternBank) ToggleNoteMode() error {
	if p == nil || p.editingLength {
		return nil
	}
	return p.setNoteEdit(!p.editingNote)
}

// stepStatusText is the readout for the step under the cursor: its note, its velocity, and
// the step a tie runs into. A percussive step has no pitch to name, so it reports its
// dynamics alone. A step holding no note shows no velocity at all, because the value the
// next note would inherit is not that step's velocity and reading it as one made two steps
// look equal.
func stepStatusText(voice *Voice, step int, event *Event, tieStep int) string {
	if event == nil {
		return fmt.Sprintf("S%02d --", step+1)
	}
	pitch := ""
	if voice.IsChromatic() {
		pitch = midiNoteName(event.ChromaticNote)
	}
	text := fmt.Sprintf("S%02d %s@%03d", step+1, pitch, event.Velocity)
	if event.Tie && tieStep >= 0 {
		text = fmt.Sprintf("%s->%02d", text, tieStep+1)
	}
	return text
}

// The bottom OLED row shows the current step, its note and its velocity, and the tie target
// when there is one. A percussive track reports the step and its dynamics too, because the
// Volume knob sets them and a readout that went blank would leave the knob turning blind.
func (p *PatternBank) printStepStatus() error {
	if p == nil || p.f == nil || p.editingLength {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil {
		return p.clearTextRows(lengthDisplayRow, 1)
	}
	if err := p.clearTextRows(lengthDisplayRow, 1); err != nil {
		return err
	}
	pattern := p.CurrentPattern()
	tieStep := -1
	var event *Event
	if pattern != nil {
		if current, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
			event = &current
			if next, nextOK := pattern.NextTiedEvent(p.stepCursor, voice); nextOK {
				tieStep = eventStep(next)
			}
		}
	}
	return p.printText(lengthDisplayRow, 0, fitOLEDText(stepStatusText(voice, p.stepCursor, event, tieStep)), false)
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
