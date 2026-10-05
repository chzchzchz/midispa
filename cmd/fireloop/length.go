package main

import "fmt"

// The bottom OLED row is reserved for the active pattern-length edit state.
const lengthDisplayRow = 6

func (p *PatternBank) ToggleLengthMode() error {
	return p.setLengthMode(!p.editingLength)
}

func (p *PatternBank) setLengthMode(active bool) error {
	p.clearPadState()
	if active && p.editingNote {
		if err := p.setNoteEdit(false); err != nil {
			return err
		}
	}
	p.editingLength = active
	if active {
		if err := p.f.SetLed(NoteOverview, LEDRed); err != nil {
			return err
		}
		return p.printLength()
	}
	if err := p.f.SetLed(NoteOverview, LEDOff); err != nil {
		return err
	}
	return p.printStepStatus()
}

func (p *PatternBank) AdjustLength(delta int) error {
	if !p.editingLength {
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
		return p.clearTextRows(lengthDisplayRow, 1)
	}
	return p.printText(lengthDisplayRow, 0, fmt.Sprintf(
		"Length %02d steps",
		pattern.LengthSteps(),
	), false)
}
