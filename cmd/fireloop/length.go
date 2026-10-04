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
	return p.Jump(0)
}

func (p *PatternBank) printLength() error {
	return p.printText(lengthDisplayRow, 0, fmt.Sprintf(
		"Length %02d steps",
		p.CurrentPattern().LengthSteps(),
	), false)
}
