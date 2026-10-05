package main

import "math/bits"

// The pad masks below exist for the two-pad tie gesture: they track which pads the user
// is holding right now, so a second press can be recognised as the other half of a tie rather
// than as an unrelated press. They model held hardware state, not a timing window, so a tie
// needs both pads down at once and letting go of either ends the gesture.

// padBit is the mask bit a pad occupies, and whether it is a pad the gesture tracks. Only
// the pitch palette's columns are tracked; a step cell names a step rather than a pitch and
// drives the gesture through the held step instead.
func (p *PatternBank) padBit(row, col int) (uint64, bool) {
	if p == nil || row < 0 || row >= len(p.rowPadMasks) || col < 0 || col >= chromaticPaletteColumns {
		return 0, false
	}
	return uint64(1) << uint(row*chromaticPaletteColumns+col), true
}

func (p *PatternBank) pressPad(row, col int) bool {
	bit, tracked := p.padBit(row, col)
	if !tracked || p.pressedPads&bit != 0 {
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

// heldRowPadCount is how many pads of one row are held, which is what tells a first press
// from the second one that completes a tie.
func (p *PatternBank) heldRowPadCount(row int) int {
	if p == nil || row < 0 || row >= len(p.rowPadMasks) {
		return 0
	}
	return bits.OnesCount16(p.rowPadMasks[row])
}

// heldStepsOnRow are the columns of one row the user is holding, in order. Two of them are
// the pair of steps a tie gesture names.
func (p *PatternBank) heldStepsOnRow(row int) []int {
	if p == nil || row < 0 || row >= len(p.rowPadMasks) {
		return nil
	}
	steps := make([]int, 0, 2)
	for col := 0; col < chromaticPaletteColumns; col++ {
		if p.rowPadMasks[row]&(uint16(1)<<uint(col)) != 0 {
			steps = append(steps, col)
		}
	}
	return steps
}

func (p *PatternBank) releasePad(row, col int) {
	if p == nil {
		return
	}
	// A step cell names a step rather than a pitch, so it is not one of the pads the mask
	// bookkeeping below knows about. It is the held step that drives the tie gesture, so
	// letting go of that cell is what ends the gesture.
	if p.noteEditHeldStep != noHeldStep {
		if step := chromaticStepAt(row, col); step >= 0 && step == p.noteEditHeldStep {
			p.noteEditHeldStep = noHeldStep
		}
	}
	bit, tracked := p.padBit(row, col)
	if !tracked || p.pressedPads&bit == 0 {
		return
	}
	p.pressedPads &^= bit
	p.rowPadMasks[row] &^= uint16(1) << uint(col)
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
	rowPads := p.heldRowPadCount(row)
	if rowPads == 1 {
		if err := p.setStepCursor(col); err != nil {
			return true, err
		}
	}
	if rowPads != 2 || p.heldPadCount() != 2 {
		return true, nil
	}
	steps := p.heldStepsOnRow(row)
	if len(steps) != 2 {
		return true, nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return true, nil
	}
	if !pattern.TieEventsAtSteps(steps[0], steps[1], voice) {
		// Nothing changed, so only the readout is worth rewriting.
		return true, p.printStepStatus()
	}
	return true, p.repaintEditView()
}

// handleNoteEditStepPress moves the edit to a step cell, or ties the two steps when another
// cell is already held. That is the gesture step mode uses on the step grid, and the strip
// is where a step lives while the palette owns the rest of the grid.
//
// A tie needs an event on both steps, so a gesture that cannot tie is refused whole: the edit
// stays where it was rather than jumping to a step whose note has nothing to hold.
func (p *PatternBank) handleNoteEditStepPress(pattern *Pattern, voice *Voice, step int) error {
	previous := p.noteEditHeldStep
	p.noteEditHeldStep = step
	if previous < 0 || previous == step {
		return p.setStepCursor(step)
	}
	if !pattern.TieEventsAtSteps(previous, step, voice) {
		logger.Debug("tie refused", "from", previous, "to", step, "voice", voiceLabel(voice))
		return p.printStepStatus()
	}
	logger.Debug("tie", "from", previous, "to", step, "voice", voiceLabel(voice))
	return p.repaintEditView()
}
