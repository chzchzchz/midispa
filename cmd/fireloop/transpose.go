package main

import (
	"github.com/chzchzchz/midispa/alsa"
)

// The pan knob, which transposes the selected chromatic track. It lives apart from the
// palette because the two do different jobs on the notes: the palette pitches one step,
// and this moves every note the track holds.

// TransposeSelected turns the pan knob: it shifts the selected track's chromatic notes by
// the detents the turn counts and repaints, so the grid shows the new pitches. Only a
// selected chromatic track has notes to transpose: a percussive track's notes come from
// its voice, and no selection means no voice to aim at.
func (p *PatternBank) TransposeSelected(aseq alsa.EventWriter, encoderValue int) error {
	if p.readoutIsOwned() {
		return nil
	}
	voice := p.selectedChromaticVoice()
	if voice == nil {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	direction, turning := encoderDirection(encoderValue)
	if !turning {
		return nil
	}
	// The detent count is the Volume knob's, shared: what it keeps is how fast a hand is
	// turning, and one hand turns one knob at a time. The guards above run before the
	// count, so a turn they turn away spends none of it; a turn that reaches the count
	// spends it whatever it finds, the way the Volume knob's detent over an empty step
	// does.
	semitones := p.detentIncrement(direction)
	moved := pattern.Transpose(voice, semitones)
	logger.Debug("transpose",
		"voice", voiceLabel(voice), "semitones", semitones, "moved", moved)
	if moved == 0 {
		// Nothing changed, so no repaint or audition owes the turn anything: the same
		// gate the Volume knob holds over an empty step.
		return nil
	}
	// The readout is about to name the cursor step's new pitch, so that step is the note
	// the audition confirms; a cursor step with no note on it has nothing to hear. The
	// event is copied at arm time, the way every preview is, so the timer's goroutine
	// reads nothing the event loop owns.
	if event, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
		p.armAudition(aseq, event)
	}
	if p.noteEditActive() {
		// The palette owns every pad in note-edit mode, so it replaces the row redraw,
		// which would paint step-grid colours over the palette; its strip cells carry
		// the new note colours instead.
		if err := p.drawNotePalette(); err != nil {
			return err
		}
	} else if err := p.redrawRowsWithVoice(voice); err != nil {
		return err
	}
	return p.printStepStatus()
}
