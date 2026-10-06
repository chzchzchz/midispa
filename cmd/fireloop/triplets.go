package main

import "errors"

// Triplet groups, in the two sizes the grid can hold: an eighth-note group
// takes one beat, the four sixteenth-note cells fireloop calls a beat, and a
// sixteenth-note group takes one eighth, the two cells of a beat's first
// half. Both play three notes evenly across the cells they own, and both
// refuse to share those cells with anything else, of either size.

const (
	// tripletNotes is how many notes both sizes of group play.
	tripletNotes = 3
	// noGroup is the answer for a step no group covers.
	noGroup = -1
)

// tripletKind is the size of a triplet group, named by the cells it owns.
// It is a byte-wide integer because it rides on every event: an int8 packs
// into the padding beside Tie, so the flag leaves an event the size it was
// before the feature, and every event the bank copies or a session file
// writes is 8 bytes lighter for it.
type tripletKind int8

const (
	tripletNone      tripletKind = 0
	tripletSixteenth tripletKind = 2                   // one eighth: two cells, three notes
	tripletEighth    tripletKind = patternStepsPerBeat // one beat: four cells, three notes
)

// tripletBeatEpsilon absorbs the float error between a beat the pattern
// stored and the position the repair computes for it. It is a thousandth of
// a sixteenth, so it can only ever cover arithmetic that was meant to be
// exact, and it is what lets the repair recognise a group that came back
// from a session file.
const tripletBeatEpsilon = 1e-4

// tripletGroup is one group, resolved for a draw: its first cell and the
// cells it owns.
type tripletGroup struct {
	start int
	cells int // the kind's span: 2 for a sixteenth group, 4 for an eighth group
}

// tripletGroupStart is the first cell of the group a step falls in, for a
// group of the given width. Every cell a group owns answers the same group,
// which is what lets the gesture snap and lets a second press on any of
// them take the group away again.
func tripletGroupStart(step, cells int) int { return step - step%cells }

// tripletBeat is where the group's index-th note sounds. Only the notes
// after the first move: the first stays on the beat it was already on, and
// a sixteenth group's second and third each move back one cell from where
// they were pressed.
func tripletBeat(start, cells, index int) float32 {
	return stepBeat(start) + float32(index)*stepBeat(cells)/tripletNotes
}

// covers reports whether the group owns a step, its notes and, for an
// eighth group, its consumed cell.
func (g tripletGroup) covers(step int) bool { return step >= g.start && step < g.start+g.cells }

// consumed reports whether a step is the cell an eighth group owns and
// draws, the fourth cell of the beat. A sixteenth group consumes no cell,
// so this is false for every step it owns.
func (g tripletGroup) consumed(step int) bool {
	return g.cells == patternStepsPerBeat && step == g.start+g.cells-1
}

// tripletGroupCovering returns the group of a list that covers a step. A
// list holds at most one group per start cell and groups do not overlap, so
// the first cover is the only one.
func tripletGroupCovering(groups []tripletGroup, step int) (tripletGroup, bool) {
	for _, group := range groups {
		if group.covers(step) {
			return group, true
		}
	}
	return tripletGroup{}, false
}

// The four ways a gesture can be refused whole. They are named errors rather
// than text, so a caller matches them with errors.Is and the wording belongs
// to whoever draws them.
var (
	errTripletNoNotes = errors.New("the cells do not hold the three notes a group needs")
	errTripletBusy    = errors.New("the fourth cell of the beat holds a note")
	errTripletPastEnd = errors.New("the beat runs past the pattern length")
	errTripletOverlap = errors.New("another group covers a cell of the beat")
)

// The consumed cell is the fourth cell of a beat that carries an
// eighth-note group, and it is the only new colour rule this feature adds.
// It is the group's first note's colour at half brightness rather than a
// fixed level, because a fixed level either disappears against a palette
// colour dimmed to a quarter, or, lifted far enough to be a mark of its
// own, carries a chromatic step past white and takes the pitch with it.
// Halving keeps the hue and halves the brightness, so the cell reads as
// part of the same block.
const tripletShadeDivisor = 2

// tripletShadeColor is the consumed cell's colour: the group's first note's,
// dimmed and marked the way a tied step is, so the shade survives an
// inverted column as the tie mark does.
func tripletShadeColor(color [3]int, invert bool) [3]int {
	return markTieColor(Dim(color, tripletShadeDivisor), invert)
}

// tripletConsumedColor is the colour of the cell a group draws: the
// shade of the group's first note, read from the pattern, because the
// first note sits in an earlier cell than the one being painted.
func tripletConsumedColor(pattern *Pattern, group tripletGroup, v *Voice, invert bool) [3]int {
	first, ok := pattern.EventAtStep(group.start, v)
	if !ok {
		return [3]int{}
	}
	return tripletShadeColor(chromaticEventColor(first), invert)
}
