package main

import "fmt"

// readoutRow is the bottom OLED row, which one mode or another owns at any moment: the step
// being edited, the pattern's length, a number being typed, or a transient message. There is
// one name for it because there is one row, and three of them used to have three.
const readoutRow = 6

func (p *PatternBank) ToggleLengthMode() error {
	return p.setLengthMode(!p.lengthEditActive())
}

func (p *PatternBank) setLengthMode(active bool) error {
	p.clearPadState()
	if active && p.noteEditActive() {
		if err := p.setNoteEdit(false); err != nil {
			return err
		}
	}
	// Length entry and swing entry both want the readout row and the encoder. The enum
	// already drops the other mode, but only this call runs the leaving half — applying a
	// typed swing and clearing the Snap light — so it has to be made explicitly.
	if active && p.swingEditActive() {
		if err := p.setSwingEntry(false); err != nil {
			return err
		}
	}
	p.claimMode(active, lengthEdit)
	if active {
		if err := p.pads.SetLed(NoteOverview, LEDRed); err != nil {
			return err
		}
		return p.printLength()
	}
	if err := p.pads.SetLed(NoteOverview, LEDOff); err != nil {
		return err
	}
	return p.printStepStatus()
}

func (p *PatternBank) AdjustLength(delta int) error {
	if !p.lengthEditActive() {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	pattern.SetLengthSteps(pattern.LengthSteps() + delta)
	// Shortening leaves the cursor past the new end, and the rows are drawn from it, so it
	// is pulled back inside before anything reads it.
	p.clampStepCursor()
	return p.redraw()
}

func (p *PatternBank) printLength() error {
	pattern := p.CurrentPattern()
	if pattern == nil {
		// A bank that has not chosen a pattern yet has no length to report, and leaving
		// the row blank says that rather than reporting on no pattern at all.
		return p.clearTextRows(readoutRow, 1)
	}
	return p.printText(readoutRow, 0, fmt.Sprintf(
		"Length %02d steps",
		pattern.LengthSteps(),
	), false)
}
