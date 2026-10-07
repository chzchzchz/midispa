package main

import (
	"fmt"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/sysex/akai"
)

// The pitch palette and everything that draws it: which pitch each pad names, the octave the
// SELECT knob has moved the whole block to, the step strip beside it, and the colour rules
// that tell a pitch apart from a drum. It owns note-edit mode, because that mode is the
// palette editing the grid.

// The palette runs from A1 upwards, laid out so every row starts on A and spans the twelve
// semitones to the G# above it, which is what "A to G#" means once octave numbering turns
// over at A. It uses the first twelve columns of each sixteen-column pad row; the last four
// are left dark, because a row that wrapped past G# would put the next row's A somewhere
// unexpected. Rows therefore cover A1 to G#2, A2 to G#3, A3 to G#4 and A4 to G#5.
// The SELECT knob moves the whole palette by whole octaves, so those rows become whichever
// four octaves the user has turned the palette to. The shift changes nothing already written
// into a pattern; it only decides which pitches a pad press can reach.
// A tie mark lifts a dark channel to tieMarkAmount, or lowers an inverted one from
// tieMarkFloor. The step being edited is brightened by cursorMarkAmount instead, which
// has to stop at the highest value a pad can show.
const (
	tieMarkAmount = 32
	tieMarkFloor  = 95

	cursorMarkAmount = 24
)

const (
	chromaticBaseNote       = 33
	chromaticPaletteRows    = 4
	chromaticPaletteColumns = 12
	// The columns past the palette are a strip of step indicators, one cell per step.
	chromaticStepColumns = padColumns - chromaticPaletteColumns
	chromaticStepCells   = chromaticPaletteRows * chromaticStepColumns
	// chromaticOctaveShift is the distance one SELECT detent moves the palette. An octave is
	// the only shift that keeps every row running A to G#, so it is the only one allowed.
	chromaticOctaveShift = 12
	// chromaticPaletteSpan is the highest offset the palette reaches above its first pad, so
	// an octave shift can be clamped to a range where every pad still names a playable note.
	chromaticPaletteSpan = chromaticPaletteRows*chromaticPaletteColumns - 1
	// chromaticPaletteDim keeps a palette colour bright enough to read as a colour rather
	// than as the white selection highlight.
	chromaticPaletteDim = 4
)

// chromaticNoteNames are the twelve pitch classes, so a note number can be read as the pitch
// the palette pads name.
var chromaticNoteNames = [...]string{
	"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B",
}

// midiNoteName spells a MIDI note number as a pitch and an octave in scientific pitch notation,
// which is how the readout and the palette tests name a note.
func midiNoteName(note int) string {
	note = clampMidiDataValue(note)
	return fmt.Sprintf("%s%d", chromaticNoteNames[note%len(chromaticNoteNames)], note/12-1)
}

// chromaticPaletteNote is the pitch at a palette position, moved by whole octaves. A row or
// column outside the palette has no pitch, which is reported rather than clamped, so
// pressing an unused column cannot assign the first note of the first row by accident. The
// octave is the caller's to clamp with clampPaletteOctave; a shift beyond that still returns
// a playable note rather than a byte the sequencer cannot send.
func chromaticPaletteNote(row, col, octave int) (int, bool) {
	if row < 0 || row >= chromaticPaletteRows {
		return 0, false
	}
	if col < 0 || col >= chromaticPaletteColumns {
		return 0, false
	}
	base := chromaticBaseNote + chromaticOctaveShift*octave
	return clampMidiDataValue(base + row*chromaticPaletteColumns + col), true
}

// chromaticOctaveBounds are the palette shifts that keep every pad inside the MIDI range.
// Down, the first pad must stay at or above zero; Go truncates a division towards zero, so
// the bound is negated after the division to land on the shift that stays in range. Up, the
// last pad, chromaticPaletteSpan semitones above the first, must stay at or below the
// highest MIDI note.
func chromaticOctaveBounds() (min, max int) {
	min = -((chromaticBaseNote - 1) / chromaticOctaveShift)
	max = (midiNoteMax - chromaticPaletteSpan - chromaticBaseNote) / chromaticOctaveShift
	return min, max
}

func clampPaletteOctave(octave int) int {
	lowest, highest := chromaticOctaveBounds()
	return min(max(octave, lowest), highest)
}

// paletteBase is the pitch of the palette's first pad, which is also the pad that erases.
func (p *PatternBank) paletteBase() int {
	return chromaticBaseNote + chromaticOctaveShift*p.paletteOctave
}

// ShiftPaletteOctave moves the note palette by one detent's worth of detents, which is an
// octave each. Only the pitches the palette offers change: notes already in the pattern keep
// their pitch, and each pad keeps its colour rule, so the colours travel with the palette and
// the shift shows on the grid without a word on the display. It does nothing outside
// note-edit mode, where the knob moves the selected track's voice instead.
func (p *PatternBank) ShiftPaletteOctave(detents int) error {
	if p.readoutIsOwned() {
		return nil
	}
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() || !p.noteEditActive() {
		return nil
	}
	previous := p.paletteOctave
	p.paletteOctave = clampPaletteOctave(previous + detents)
	if p.paletteOctave == previous {
		// Already at the end of the MIDI range, so there is nothing to redraw.
		logger.Debug("palette octave held", "octave", p.paletteOctave, "detents", detents)
		return nil
	}
	logger.Debug("palette octave", "from", previous, "to", p.paletteOctave, "base", p.paletteBase())
	return p.repaintEditView()
}

// stepCellPaintColor is how a step cell reads right now: white while the playhead is on
// it, otherwise the step's note colour, brightened while it is the step being edited.
// Sharing one rule means any redraw keeps the playhead visible.
func (p *PatternBank) stepCellPaintColor(pattern *Pattern, voice *Voice, step int, groups []tripletGroup) [3]int {
	if step == p.playheadStep {
		return oledWhite
	}
	return p.stepCellColor(pattern, voice, step, groups)
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
	if !ok {
		return nil
	}
	voice := p.SelectedVoice()
	pattern := p.CurrentPattern()
	if voice == nil || !voice.IsChromatic() || pattern == nil {
		return nil
	}
	color := p.stepCellPaintColor(pattern, voice, step, pattern.tripletGroups(voice))
	return p.pads.LightPadColor(col, row, color)
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

// In note-edit mode the left block chooses a pitch for the current step and the right-hand
// block chooses which step is being edited. Two held cells tie those two steps. Alt plus a
// pad removes the event, and so does A1, the palette's first pad, which stands for "no note
// here". How hard a pad was hit sets the note's velocity, so a new step lands with the
// dynamics that were played and the display reports that same value.
func (p *PatternBank) handleNoteEditPad(aseq alsa.EventWriter, row, col, pressed int, alt, shift bool) error {
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() || !p.noteEditActive() {
		return nil
	}
	pattern := p.CurrentPattern()
	if pattern == nil {
		return nil
	}
	if step := chromaticStepAt(row, col); step >= 0 {
		// The right-hand block edits a step rather than assigning a pitch to one, and two
		// cells held together tie those two steps. Alt makes a press there a claim
		// about the step's place in time instead, which is the same gesture the
		// step grid answers, so it is intercepted before anything is recorded
		// as held for a tie.
		if alt && shift {
			return p.toggleTripletGroup(step, voice, tripletSixteenth)
		}
		if alt {
			return p.toggleTripletGroup(step, voice, tripletEighth)
		}
		return p.handleNoteEditStepPress(pattern, voice, step)
	}
	step := p.stepCursor
	note, onPalette := chromaticPaletteNote(row, col, p.paletteOctave)
	if !onPalette {
		// Neither a pitch nor a step, so there is nothing here to edit.
		return nil
	}
	if alt || note == p.paletteBase() {
		// The palette's first pad is where the palette starts, whichever octave that is,
		// which makes it the natural key for "no note".
		reason := "palette"
		if alt {
			reason = "alt"
		}
		return p.removeNoteAtStep(pattern, voice, step, reason)
	}
	return p.placeNoteOnStep(aseq, pattern, voice, step, note, pressed)
}

// removeNoteAtStep takes the note off a step and repaints, naming in the log which of the
// two ways of asking for that the pad press was.
func (p *PatternBank) removeNoteAtStep(pattern *Pattern, voice *Voice, step int, reason string) error {
	pattern.RemoveEventAtStep(step, voice)
	logger.Debug("pitch removed", "step", step, "via", reason)
	return p.repaintEditView()
}

// placeNoteOnStep writes a pitch onto a step and repaints. How hard the pad was pressed is
// the step's dynamics, whether the note is new or its pitch is being changed, and the step
// is auditioned so the user hears the pitch they just picked. The Volume encoder still
// adjusts the value afterwards.
func (p *PatternBank) placeNoteOnStep(aseq alsa.EventWriter, pattern *Pattern, voice *Voice, step, note, pressed int) error {
	event, ok := pattern.SetChromaticNote(step, voice, note, clampMidiDataValue(pressed))
	if !ok {
		return nil
	}
	logger.Debug("pitch", "step", step, "note", note, "velocity", event.Velocity)
	if err := p.auditionEvent(aseq, event); err != nil {
		return err
	}
	return p.repaintEditView()
}

// stepCellColor is how one cell of the step strip reads: the step's note colour,
// dark when the step holds no note, and brightened while it is the step being edited.
// A cell a group consumes holds no note at all, and its shade is what says the
// cell belongs to a group rather than to a step.
func (p *PatternBank) stepCellColor(pattern *Pattern, voice *Voice, step int, groups []tripletGroup) [3]int {
	if pattern == nil {
		return [3]int{}
	}
	event, ok := pattern.EventAtStep(step, voice)
	if !ok {
		if group, covered := tripletGroupCovering(groups, step); covered && group.consumed(step) {
			return tripletConsumedColor(pattern, group, voice, false)
		}
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
	voice := p.SelectedVoice()
	if voice == nil || !voice.IsChromatic() {
		return nil
	}
	pads := make([]akai.Pad, 0, chromaticPaletteRows*padColumns)
	pattern := p.CurrentPattern()
	selectedNote := -1
	var groups []tripletGroup
	if pattern != nil {
		if event, ok := pattern.EventAtStep(p.stepCursor, voice); ok {
			selectedNote = event.ChromaticNote
		}
		// The strip's cells are painted from one walk of the voice's
		// groups, so no cell asks the pattern for them on its own.
		groups = pattern.tripletGroups(voice)
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		for col := 0; col < padColumns; col++ {
			color := [3]int{}
			if step := chromaticStepAt(row, col); step >= 0 {
				color = p.stepCellPaintColor(pattern, voice, step, groups)
			} else if note, onPalette := chromaticPaletteNote(row, col, p.paletteOctave); onPalette {
				color = chromaticPaletteColor(note)
				if note == selectedNote {
					color = oledWhite
				}
			}
			pads = append(pads, makePad(col, row, color))
		}
	}
	return p.pads.LightPadSlice(pads)
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

// chromaticColorIndex counts semitones from the unshifted palette base rather than from the
// note's class, so turning the palette moves every colour with it and the shift is visible on
// the grid. The index is normalized because a palette below the base would otherwise go
// negative and collapse onto the first colour, leaving several pads indistinguishable.
func chromaticColorIndex(note int) int {
	index := (note - chromaticBaseNote) % len(oledColorTable)
	if index < 0 {
		index += len(oledColorTable)
	}
	return index
}

func chromaticPaletteColor(note int) [3]int {
	return Dim(oledColorTable[chromaticColorIndex(note)], chromaticPaletteDim)
}

// percussionStepColor is how a percussive step reads on the grid: one dark green. It has
// no pitch to colour by, so it is held apart from the chromatic colour rule rather than
// borrowing one, which is what tells the two kinds of step apart at a glance.
var percussionStepColor = [3]int{0, 50, 0}

// chromaticEventColor is how a step reads on the grid. A chromatic step carries its pitch,
// so it takes that pitch's palette colour; a percussive one has no pitch and is the drum
// green.
func chromaticEventColor(event Event) [3]int {
	if !event.IsChromatic() {
		return percussionStepColor
	}
	return chromaticPaletteColor(event.ChromaticNote)
}

// A tied step is marked by pushing its colour away from the playhead: a dark colour is
// lifted, and an inverted one is pushed down, because lifting an already bright colour
// would be invisible.
func markTieColor(color [3]int, invert bool) [3]int {
	return eachChannel(color, func(value int) int {
		if invert {
			if value > tieMarkFloor {
				return value - tieMarkAmount
			}
			return value
		}
		return max(value, tieMarkAmount)
	})
}

func markCursorColor(color [3]int) [3]int {
	return eachChannel(color, func(value int) int {
		return min(value+cursorMarkAmount, midiNoteMax)
	})
}
