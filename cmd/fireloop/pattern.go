package main

import (
	"math"
	"sort"
	"sync"
)

const (
	maxPatternSteps     = 16
	defaultPatternSteps = maxPatternSteps
	patternStepsPerBeat = 4
	patternBeatsPerStep = 1.0 / patternStepsPerBeat
)

type Pattern struct {
	// events is held in beat order. FindBeat binary searches it instead of sorting a copy
	// of it, so the collection is private and every way in either goes through a setter or
	// through newPattern, both of which normalise before the pattern is read again.
	// TestEventsStayInBeatOrder is what holds this to account.
	events []Event
	// lengthSteps is measured in sixteenth notes; zero keeps the legacy four-beat default.
	lengthSteps int
	mu          sync.RWMutex
}

var emptyPattern Pattern

// eventStep converts the stored beat position to the integer grid position used by editing, ties and
// playback. It floors rather than rounds because the question every caller asks is which cell a note
// starts in: a note belongs to the cell its beat falls in, not to the nearest one. Every beat the
// setters write is an exact multiple of a sixteenth, where floor and round agree, so only the off-grid
// beats a triplet group packs behave differently, and those are the notes that need the floor, because
// a sixteenth group's middle note shares its first note's cell. The floor runs as a truncation,
// which is one instruction, for the beats that can occur, all at or after the pattern's start; only
// a hand-edited session file can name a beat before it, and that one keeps the true floor so its
// step stays negative and the repairs that skip negative steps keep skipping it.
func eventStep(event Event) int {
	steps := event.Beat * patternStepsPerBeat
	if steps < 0 {
		return int(math.Floor(float64(steps)))
	}
	return int(steps)
}

func stepBeat(step int) float32 {
	return float32(step) * patternBeatsPerStep
}

// newPattern builds a pattern from events that no setter has touched, which is what a
// session file and a test hand over. The events go in beat order and the ties are repaired
// here rather than left to whichever setter a caller happened to call next, because a load
// that relied on that ordering was one reordering away from playing the wrong notes.
//
// The length is left at the default; a caller that has one sets it with SetLengthSteps,
// which also drops whatever falls past the end it sets.
func newPattern(events []Event) *Pattern {
	pattern := &Pattern{events: events}
	pattern.normalizeLocked()
	return pattern
}

// install replaces the events and the length in place, which is how a paste keeps the
// pattern pointer a song may already be sharing. It is install rather than a new pattern
// because the songs hold the old pointer and must keep hearing from it.
func (p *Pattern) install(events []Event, lengthSteps int) {
	p.mu.Lock()
	p.events = events
	p.lengthSteps = lengthSteps
	p.normalizeLocked()
	p.mu.Unlock()
}

func (p *Pattern) Copy() *Pattern {
	p.mu.RLock()
	evs := append([]Event(nil), p.events...)
	lengthSteps := p.lengthSteps
	p.mu.RUnlock()
	copyPattern := newPattern(evs)
	copyPattern.lengthSteps = lengthSteps
	copyPattern.normalizeLocked()
	return copyPattern
}

// snapshot returns the events and the resolved length under one read lock, so a save
// cannot pair the events of one edit with the length of another. The length is resolved
// because a stored zero means the legacy default rather than an empty pattern.
func (p *Pattern) snapshot() ([]Event, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]Event(nil), p.events...), p.lengthStepsLocked()
}

// ToggleEvent returns true if event is added, false if deleted.
func (p *Pattern) ToggleEvent(ev Event) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ev.Beat < 0 || eventStep(ev) >= p.lengthStepsLocked() {
		return false
	}
	for i, current := range p.events {
		if eventStep(current) == eventStep(ev) && current.Voice == ev.Voice {
			p.removeEventLocked(i)
			return false
		}
	}
	p.insertEventLocked(ev)
	return true
}

// FindBeat returns a snapshot of every event at or after a given beat, in beat order.
//
// Only the tail the caller is handed is copied. The search reads the live list under the
// same lock, so there is nothing to copy it for, and it relies on Events being in beat
// order, which is what normalizeLocked is for. The copy itself is not optional: a caller
// keeps what it gets, and the pattern is free to change underneath it afterwards.
func (p *Pattern) FindBeat(beat float32) []Event {
	p.mu.RLock()
	defer p.mu.RUnlock()
	first := sort.Search(len(p.events), func(i int) bool {
		return p.events[i].Beat >= beat
	})
	return append([]Event(nil), p.events[first:]...)
}

func (p *Pattern) ClearVoice(v *Voice) {
	p.mu.Lock()
	kept := p.events[:0]
	for _, event := range p.events {
		if event.Voice != v {
			kept = append(kept, event)
		}
	}
	p.events = kept
	p.normalizeLocked()
	p.mu.Unlock()
}

// EventsForVoice returns a snapshot of one voice's events in beat order. The list it is
// filtered from is in beat order itself, so the filtered list is too and there is nothing
// to sort.
func (p *Pattern) EventsForVoice(v *Voice) []Event {
	p.mu.RLock()
	defer p.mu.RUnlock()
	events := make([]Event, 0, len(p.events))
	for _, event := range p.events {
		if event.Voice == v {
			events = append(events, event)
		}
	}
	return events
}

func (p *Pattern) EventAtStep(step int, v *Voice) (Event, bool) {
	if !p.validStep(step) {
		return Event{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	// This scan is written out here rather than shared with the setters. The palette redraw
	// asks for one step at a time for every cell it paints, so it runs this more often
	// than anything else that looks a step up, and behind a call it measured about twice
	// the cost of the loop it replaced. The cell a beat sits in is the half-open range
	// [stepBeat(step), stepBeat(step+1)), which is what eventStep's floor asks, so the
	// scan compares beats directly: a conversion to int per event costs more than the
	// two compares, and this walk runs once per event per painted cell.
	lo, hi := stepBeat(step), stepBeat(step+1)
	for _, event := range p.events {
		if event.Beat >= lo && event.Beat < hi && event.Voice == v {
			return event, true
		}
	}
	return Event{}, false
}

// RemoveEventAtStep deletes the event a step holds for a voice. Like every other change to
// the notes, removing one ends the tie that ran into it, so the note after it stops
// sounding on its own.
func (p *Pattern) RemoveEventAtStep(step int, v *Voice) (Event, bool) {
	if !p.validStep(step) {
		return Event{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if index := p.eventIndexAtStepLocked(step, v); index >= 0 {
		return p.removeEventLocked(index), true
	}
	return Event{}, false
}

// SetChromaticNote creates or updates a pitched event while preserving its tie state.
func (p *Pattern) SetChromaticNote(step int, v *Voice, note, velocity int) (Event, bool) {
	if v == nil || !v.IsChromatic() || !p.validStep(step) {
		return Event{}, false
	}
	if note < 0 || note > midiNoteMax {
		return Event{}, false
	}
	velocity = clampStepVelocity(v, velocity)
	p.mu.Lock()
	defer p.mu.Unlock()
	if index := p.eventIndexAtStepLocked(step, v); index >= 0 {
		p.events[index].ChromaticNote = note
		p.events[index].Velocity = velocity
		p.normalizeLocked()
		return p.events[index], true
	}
	event := Event{
		Voice:         v,
		Beat:          stepBeat(step),
		ChromaticNote: note,
		Velocity:      velocity,
	}
	insertAt := p.insertEventLocked(event)
	return p.events[insertAt], true
}

// SetVelocity writes the dynamics of an existing step, whichever kind of voice it stands
// on. A velocity change is not a new note, so the tie the step may carry is left alone.
func (p *Pattern) SetVelocity(step int, v *Voice, velocity int) (Event, bool) {
	if v == nil || !p.validStep(step) {
		return Event{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// Only an existing step is written: a velocity is the dynamics of a note that is
	// already there, and turning the knob over an empty step must not place one.
	index := p.eventIndexAtStepLocked(step, v)
	if index < 0 {
		return Event{}, false
	}
	p.events[index].Velocity = clampStepVelocity(v, velocity)
	return p.events[index], true
}

// TieEventsAtSteps links two existing events only when the earlier event's next event is the later one.
func (p *Pattern) TieEventsAtSteps(firstStep, secondStep int, v *Voice) bool {
	return p.setTieBetweenSteps(firstStep, secondStep, v, true)
}

func (p *Pattern) UntieEventsAtSteps(firstStep, secondStep int, v *Voice) bool {
	return p.setTieBetweenSteps(firstStep, secondStep, v, false)
}

func (p *Pattern) setTieBetweenSteps(firstStep, secondStep int, v *Voice, tie bool) bool {
	if v == nil || !v.IsChromatic() || firstStep == secondStep {
		return false
	}
	first, firstOK := p.EventAtStep(firstStep, v)
	second, secondOK := p.EventAtStep(secondStep, v)
	if !firstOK || !secondOK || first.Beat == second.Beat {
		return false
	}
	earlierStep, laterStep := firstStep, secondStep
	if first.Beat > second.Beat {
		earlierStep, laterStep = secondStep, firstStep
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	earlier := p.eventIndexAtStepLocked(earlierStep, v)
	later := p.eventIndexAtStepLocked(laterStep, v)
	if earlier < 0 || later < 0 || earlier >= later {
		return false
	}
	for i := earlier + 1; i < later; i++ {
		if p.events[i].Voice == v {
			return false
		}
	}
	p.events[earlier].Tie = tie
	p.normalizeLocked()
	return true
}

func (p *Pattern) NextTiedEvent(step int, v *Voice) (Event, bool) {
	current, ok := p.EventAtStep(step, v)
	if !ok || !current.Tie {
		return Event{}, false
	}
	for _, event := range p.EventsForVoice(v) {
		if event.Beat > current.Beat {
			return event, true
		}
	}
	return Event{}, false
}

// TripletGroupAt returns the triplet group that owns a step, if one does. The
// flag rides on a group's first note, so the groups of a voice are found by
// walking its events; the first one whose span covers the step is the only
// one, because groups of one voice do not overlap.
func (p *Pattern) TripletGroupAt(step int, v *Voice) (tripletGroup, bool) {
	if v == nil || !p.validStep(step) {
		return tripletGroup{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tripletGroupAtStepLocked(step, v)
}

// tripletGroups returns every group of one voice in beat order, which is the
// order the draw paths walk their cells in.
func (p *Pattern) tripletGroups(v *Voice) []tripletGroup {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tripletGroupsLocked(v)
}

// tripletGroupsConsuming resolves, for each row's voice, the one group
// that draws the cell a step sits in, which is what a column repaint asks
// for. Only an eighth group draws a cell, its fourth, so the group's first
// note is the one flagged event of the cell three back, and one voice's
// groups never overlap, so at most one group per voice can answer. The
// events are held in beat order, so the cell's notes are reached by
// halving the pattern down to the cell rather than by walking it, which
// is what a column repaint pays for on every step of playback. A row with
// no answering group keeps the zero group, whose consumed reports false
// for every step.
func (p *Pattern) tripletGroupsConsuming(step int, voices [padRows]*Voice) [padRows]tripletGroup {
	var groups [padRows]tripletGroup
	if step < patternStepsPerBeat-1 {
		return groups
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	// A note sits in the cell three back when its beat is in the
	// cell's half-open range, which is the range eventStep's floor
	// asks for, so the search compares beats and the walk that
	// follows it needs no conversion at all.
	start := step - (patternStepsPerBeat - 1)
	cellBeat, nextBeat := stepBeat(start), stepBeat(start+1)
	lo, hi := 0, len(p.events)
	for lo < hi {
		mid := (lo + hi) / 2
		if p.events[mid].Beat < cellBeat {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	for i := lo; i < len(p.events) && p.events[i].Beat < nextBeat; i++ {
		event := p.events[i]
		if event.Triplet != tripletEighth {
			continue
		}
		for row, voice := range voices {
			if event.Voice == voice {
				groups[row] = tripletGroup{start: start, cells: patternStepsPerBeat}
			}
		}
	}
	return groups
}

// tripletGroupsLocked collects the groups of one voice from the events
// that carry their flags. The flag that names a group rides on its
// first note, so this one walk is what every group lookup shares.
func (p *Pattern) tripletGroupsLocked(v *Voice) []tripletGroup {
	groups := make([]tripletGroup, 0, 2)
	for _, event := range p.events {
		if event.Voice != v || event.Triplet == tripletNone {
			continue
		}
		groups = append(groups, tripletGroup{start: eventStep(event), cells: int(event.Triplet)})
	}
	return groups
}

func (p *Pattern) tripletGroupAtStepLocked(step int, v *Voice) (tripletGroup, bool) {
	return tripletGroupCovering(p.tripletGroupsLocked(v), step)
}

// SetTripletGroup packs the three notes a triplet group plays into the cells it
// owns: an eighth group re-times the notes of a beat's first three cells into
// the thirds of the beat, and a sixteenth group packs the notes of three cells
// into two, leaving the third cell free. The flag rides on the first note, the
// one note of the three that stays where it was.
//
// The setters refuse everything the repair would otherwise have to undo:
// another group's span, a group that would lose its first note to the packing,
// a cell that does not hold exactly the one note a group needs, and for an
// eighth group a fourth note on the cell it draws. SetTripletGroup answers
// whether it set; ToggleTripletGroup also names the refusal.
func (p *Pattern) SetTripletGroup(start int, v *Voice, kind tripletKind) bool {
	if v == nil || kind == tripletNone || start < 0 || start%int(kind) != 0 || start+int(kind) > p.LengthSteps() {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.groupFitsLocked(start, int(kind), v); err != nil {
		return false
	}
	p.setTripletGroupLocked(start, int(kind), v)
	return true
}

// ToggleTripletGroup is the gesture's entry point: it sets the group the
// pressed step belongs to, or takes that group away again when one already
// covers the step. The bool answers whether a group is now set, and an error
// names the refusal, so the readout can say which cell is busy or which beat
// runs past the end of the pattern.
func (p *Pattern) ToggleTripletGroup(step int, v *Voice, kind tripletKind) (bool, error) {
	if v == nil || kind == tripletNone || !p.validStep(step) {
		return false, errTripletNoNotes
	}
	cells := int(kind)
	start := tripletGroupStart(step, cells)
	p.mu.Lock()
	defer p.mu.Unlock()
	if group, covered := p.tripletGroupAtStepLocked(step, v); covered {
		if group.cells != cells {
			// The step belongs to a group of the other size, and the two
			// sizes refuse to share cells.
			return false, errTripletOverlap
		}
		p.clearTripletGroupLocked(group.start, v)
		return false, nil
	}
	if start+cells > p.lengthStepsLocked() {
		return false, errTripletPastEnd
	}
	if err := p.groupFitsLocked(start, cells, v); err != nil {
		return false, err
	}
	p.setTripletGroupLocked(start, cells, v)
	return true, nil
}

// groupFitsLocked answers whether a group of the given width can start
// at the cell, naming the refusal when it cannot. Both setters hold
// the same rules, so a beat one refuses the other refuses too,
// whichever entry point the gesture took.
func (p *Pattern) groupFitsLocked(start, cells int, v *Voice) error {
	if !p.groupCellsFreeLocked(start, cells, v) {
		return errTripletOverlap
	}
	if cells == int(tripletSixteenth) && p.hasFlaggedEventAtStepLocked(start+cells, v) {
		// Packing the third cell's note into the group would move the first
		// note of a group that starts there.
		return errTripletOverlap
	}
	if cells == int(tripletEighth) && p.countEventsAtStepLocked(start+cells-1, v) != 0 {
		return errTripletBusy
	}
	for i := 0; i < tripletNotes; i++ {
		if p.countEventsAtStepLocked(start+i, v) != 1 {
			return errTripletNoNotes
		}
	}
	return nil
}

// clearTripletGroupLocked takes a group away: the flag comes off its first
// note, and the repair the normalise runs puts the notes back on the grid,
// because nothing else marks them as a group's any more.
func (p *Pattern) clearTripletGroupLocked(start int, v *Voice) {
	for i := range p.events {
		if p.events[i].Voice == v && p.events[i].Triplet != tripletNone && eventStep(p.events[i]) == start {
			p.events[i].Triplet = tripletNone
			break
		}
	}
	p.normalizeLocked()
}

// groupCellsFreeLocked reports whether no group of a voice covers any cell of a
// span, which is the overlap rule both setters hold to.
func (p *Pattern) groupCellsFreeLocked(start, cells int, v *Voice) bool {
	for _, group := range p.tripletGroupsLocked(v) {
		for cell := start; cell < start+cells; cell++ {
			if group.covers(cell) {
				return false
			}
		}
	}
	return true
}

// hasFlaggedEventAtStepLocked reports whether a group of a voice starts on a
// cell, which is how the sixteenth setters learn that packing the next cell's
// note would steal the first note of that group.
func (p *Pattern) hasFlaggedEventAtStepLocked(step int, v *Voice) bool {
	for _, group := range p.tripletGroupsLocked(v) {
		if group.start == step {
			return true
		}
	}
	return false
}

// setTripletGroupLocked writes a group whose cells the caller has already
// checked: the notes of the group's first three cells are re-timed to its
// beats and the first of them carries the flag from then on. Both setters go
// through here so they write the same group.
func (p *Pattern) setTripletGroupLocked(start, cells int, v *Voice) {
	for i := 0; i < tripletNotes; i++ {
		index := p.eventIndexAtStepLocked(start+i, v)
		p.events[index].Beat = tripletBeat(start, cells, i)
		if i == 0 {
			p.events[index].Triplet = tripletKind(cells)
		}
	}
	p.normalizeLocked()
}

// validStep reports whether a step is inside the pattern's length. It reads the length
// under the lock of its own, so it belongs before a caller takes the lock rather than
// inside one.
func (p *Pattern) validStep(step int) bool {
	return step >= 0 && step < p.LengthSteps()
}

func (p *Pattern) eventIndexAtStepLocked(step int, v *Voice) int {
	for i, event := range p.events {
		if eventStep(event) == step && event.Voice == v {
			return i
		}
	}
	return -1
}

// countEventsAtStepLocked counts the events of one voice on one cell, which is
// how the group setters ask whether a cell holds the one note a group needs
// and whether the cell an eighth group draws is free.
func (p *Pattern) countEventsAtStepLocked(step int, v *Voice) int {
	count := 0
	for _, event := range p.events {
		if eventStep(event) == step && event.Voice == v {
			count++
		}
	}
	return count
}

func (p *Pattern) insertEventLocked(event Event) int {
	insertAt := sort.Search(len(p.events), func(i int) bool {
		return p.events[i].Beat > event.Beat
	})
	p.clearTieBeforeLocked(insertAt, event.Voice)
	p.events = append(p.events, Event{})
	copy(p.events[insertAt+1:], p.events[insertAt:])
	p.events[insertAt] = event
	p.normalizeLocked()
	return insertAt
}

func (p *Pattern) removeEventLocked(index int) Event {
	p.clearPreviousTieLocked(index)
	event := p.events[index]
	p.events = append(p.events[:index], p.events[index+1:]...)
	p.normalizeLocked()
	return event
}

// clearPreviousTieLocked ends the tie running into the event at index, which is what
// removing it has to do: the note after it would otherwise stay tied to a note that is no
// longer there.
func (p *Pattern) clearPreviousTieLocked(index int) {
	if index <= 0 || index >= len(p.events) {
		return
	}
	p.clearTieBeforeLocked(index, p.events[index].Voice)
}

// clearTieBeforeLocked ends the tie running into a position for one voice, which is also
// what inserting there has to do. The walk goes backwards because the events are held in
// beat order and only the nearest one before the position can be tied into it.
func (p *Pattern) clearTieBeforeLocked(index int, voice *Voice) {
	for i := index - 1; i >= 0; i-- {
		if p.events[i].Voice == voice {
			p.events[i].Tie = false
			return
		}
	}
}

func (p *Pattern) clearTieBeforeEventLocked(event Event) {
	previous := -1
	for i, current := range p.events {
		if current.Voice != event.Voice || current.Beat >= event.Beat {
			continue
		}
		if previous < 0 || current.Beat > p.events[previous].Beat {
			previous = i
		}
	}
	if previous >= 0 {
		p.events[previous].Tie = false
	}
}

func (p *Pattern) lengthStepsLocked() int {
	return storedLengthSteps(p.lengthSteps)
}

// storedLengthSteps resolves a length that has not been through SetLengthSteps. A stored
// zero is the legacy four-beat default rather than an empty pattern, and that is true of a
// live pattern and of a length read back from a session file alike, so both resolve it
// here: a rule written out twice would let the file and the pattern in memory disagree
// about what a zero means.
func storedLengthSteps(stored int) int {
	if stored == 0 {
		return defaultPatternSteps
	}
	return stored
}

// normalizeLocked keeps event order stable and makes every stored tie point to a valid successor.
func (p *Pattern) normalizeLocked() {
	if !eventsInBeatOrder(p.events) {
		sort.SliceStable(p.events, func(i, j int) bool {
			return p.events[i].Beat < p.events[j].Beat
		})
	}
	// Neither repair has anything to do when no note carries a tie and no note
	// sits off the grid, and skipping them keeps an ordinary note edit from
	// walking every event to find that out.
	tripletRepair := anyTripletRepair(p.events)
	if !tripletRepair && !anyEventTied(p.events) {
		return
	}
	if tripletRepair {
		p.retimeTripletsLocked()
	}
	if !anyEventTied(p.events) {
		return
	}
	tieFlags := make([]bool, len(p.events))
	lengthSteps := p.lengthStepsLocked()
	for i := range p.events {
		tieFlags[i] = p.events[i].Tie && p.events[i].IsChromatic() && eventStep(p.events[i]) >= 0 && eventStep(p.events[i]) < lengthSteps
		p.events[i].Tie = false
	}
	for i := range p.events {
		if !tieFlags[i] {
			continue
		}
		for j := i + 1; j < len(p.events); j++ {
			if eventStep(p.events[j]) < 0 || eventStep(p.events[j]) >= lengthSteps {
				break
			}
			if p.events[j].Voice == p.events[i].Voice {
				p.events[i].Tie = true
				break
			}
		}
	}
}

// eventsInBeatOrder reports whether a list is already sorted by beat. Every path that
// changes the notes ends in normalizeLocked, and by then it is: an insert lands by binary
// search and a removal shifts a hole closed. Checking is one pass of plain comparisons,
// where sorting an ordered list is a pass with a closure call per pair.
func eventsInBeatOrder(events []Event) bool {
	for i := 1; i < len(events); i++ {
		if events[i].Beat < events[i-1].Beat {
			return false
		}
	}
	return true
}

// anyEventTied reports whether any event carries a tie, which is what the repair in
// normalizeLocked has to have something to do about.
func anyEventTied(events []Event) bool {
	for i := range events {
		if events[i].Tie {
			return true
		}
	}
	return false
}

// retimeTripletsLocked is the repair that keeps the grid's one note per cell
// per voice while groups come and go. Every setter ends in normalizeLocked, so
// it runs after any change at all, and it holds three invariants:
//
//   - A group whose notes are still where the setter packed them stays a group:
//     its notes are re-timed to its beats, and the cells it owns are claimed,
//     so no other note of that voice lands on them, not even the cell an
//     eighth group only draws.
//   - A group the edits broke (a note moved, removed, or crowded out) loses its
//     flag, and its notes become survivors.
//   - Every survivor, and any note that arrived off the grid some other way,
//     walks from the cell it sits in to the first cell at or after it that
//     holds no note of its voice and belongs to no span of its voice.
//
// The placement walk is in beat order and per voice, so a voice's survivors
// land on non-decreasing cells; only the order between voices can break, and
// one sort puts it back.
func (p *Pattern) retimeTripletsLocked() {
	lengthSteps := p.lengthStepsLocked()
	cellsByVoice := map[*Voice]map[int][]int{}
	for i, event := range p.events {
		cell := eventStep(event)
		if cellsByVoice[event.Voice] == nil {
			cellsByVoice[event.Voice] = map[int][]int{}
		}
		cellsByVoice[event.Voice][cell] = append(cellsByVoice[event.Voice][cell], i)
	}

	claimed := map[*Voice]map[int]bool{}
	occupied := map[*Voice]map[int]bool{}
	whole := make([]bool, len(p.events))

	// The first pass keeps the groups that are still intact and clears the
	// rest. A group is intact when its cells are unclaimed and hold the notes
	// the setter would have written: an eighth group is recognised by its cells
	// alone, because its notes can only have moved within their cells, while a
	// sixteenth group's notes share a cell, so its beats are what tell them
	// apart.
	for i, event := range p.events {
		if event.Triplet == tripletNone {
			continue
		}
		cells := int(event.Triplet)
		start := eventStep(event)
		byCell := cellsByVoice[event.Voice]
		if !p.cellsUnclaimedLocked(claimed, event.Voice, start, cells) || !p.tripletWholeLocked(start, cells, event, byCell) {
			event.Triplet = tripletNone
			p.events[i] = event
			continue
		}
		var first, middle, third int
		if cells == patternStepsPerBeat {
			first, middle, third = byCell[start][0], byCell[start+1][0], byCell[start+2][0]
		} else {
			first, middle, third = byCell[start][0], byCell[start][1], byCell[start+1][0]
		}
		p.events[first].Beat = tripletBeat(start, cells, 0)
		p.events[middle].Beat = tripletBeat(start, cells, 1)
		p.events[third].Beat = tripletBeat(start, cells, 2)
		whole[first], whole[middle], whole[third] = true, true, true
		for cell := start; cell < start+cells; cell++ {
			markCell(claimed, event.Voice, cell)
		}
	}

	// Re-timing a group can move one of its notes past another voice's note (an
	// eighth group's third note steps from a beat's last cell into the middle
	// of it), so the list is sorted again before the placement pass walks it in
	// beat order. The walk is short and nearly ordered, so it is an insertion
	// sort, and the whole marks travel with the events they belong to.
	for i := 1; i < len(p.events); i++ {
		for j := i; j > 0 && p.events[j].Beat < p.events[j-1].Beat; j-- {
			p.events[j], p.events[j-1] = p.events[j-1], p.events[j]
			whole[j], whole[j-1] = whole[j-1], whole[j]
		}
	}

	// The cells a whole group's notes sit on, and every on-grid note's cell,
	// are taken: a survivor may not land on any of them.
	for i, event := range p.events {
		if whole[i] || event.Beat == stepBeat(eventStep(event)) {
			markCell(occupied, event.Voice, eventStep(event))
		}
	}

	// The second pass places every note the repair did not account for. Each
	// survivor walks from the cell it sits in to the first cell at or after it
	// that is free of its voice's notes and of its voice's group spans.
	for i, event := range p.events {
		if whole[i] || event.Beat == stepBeat(eventStep(event)) {
			continue
		}
		cell := eventStep(event)
		for cell < lengthSteps && (occupied[event.Voice][cell] || claimed[event.Voice][cell]) {
			cell++
		}
		if cell >= lengthSteps {
			// The tail is full, which is the one corner where the repair leaves
			// two notes of one voice on one beat: there is nowhere else for the
			// later note to go.
			cell = lengthSteps - 1
		}
		event.Beat = stepBeat(cell)
		p.events[i] = event
		markCell(occupied, event.Voice, cell)
	}

	// A survivor can land past another voice's note, so the order between
	// voices is sorted back. Within a voice the order already holds: each
	// survivor walks forward from the cell it sits in and takes the first free
	// cell, so a voice's notes land on non-decreasing cells.
	if !eventsInBeatOrder(p.events) {
		sort.SliceStable(p.events, func(i, j int) bool {
			return p.events[i].Beat < p.events[j].Beat
		})
	}
}

// tripletWholeLocked reports whether the group a flag names is still the group
// the setter wrote. Its cells must be aligned to the kind and hold the notes
// the setter packed, and a sixteenth group's notes must sit at the beats the
// packing gives them, because its three notes share two cells and only the
// beats tell them apart. An eighth group's notes each have a cell of their own,
// so its cells alone say whether the group survived.
func (p *Pattern) tripletWholeLocked(start, cells int, flagged Event, byCell map[int][]int) bool {
	if start%cells != 0 {
		return false
	}
	if start+cells > p.lengthStepsLocked() {
		// A group owns its whole span, so a pattern shortened past
		// any cell of it has broken the group, however intact its
		// notes look.
		return false
	}
	if cells == patternStepsPerBeat {
		for i := 0; i < tripletNotes; i++ {
			if len(byCell[start+i]) != 1 {
				return false
			}
		}
		return len(byCell[start+cells-1]) == 0
	}
	if len(byCell[start]) != 2 || len(byCell[start+1]) != 1 {
		return false
	}
	if !tripletBeatsMatch(flagged.Beat, stepBeat(start)) {
		return false
	}
	if !tripletBeatsMatch(p.events[byCell[start][1]].Beat, tripletBeat(start, cells, 1)) {
		return false
	}
	return tripletBeatsMatch(p.events[byCell[start+1][0]].Beat, tripletBeat(start, cells, 2))
}

// cellsUnclaimedLocked reports whether no group of a voice has claimed any cell
// of a span, which is the overlap rule the repair restores after a session file
// or an edit leaves two groups sharing cells.
func (p *Pattern) cellsUnclaimedLocked(claimed map[*Voice]map[int]bool, v *Voice, start, cells int) bool {
	for cell := start; cell < start+cells; cell++ {
		if claimed[v][cell] {
			return false
		}
	}
	return true
}

// anyTripletRepair reports whether the triplet repair has anything to do: a
// group flag to keep, or a note sitting off the grid that the repair has to
// place. The off-grid half is what finds the survivors of a group that was
// taken away, because clearing a group leaves nothing behind but the beats its
// notes were packed to.
func anyTripletRepair(events []Event) bool {
	for i := range events {
		if events[i].Triplet != tripletNone {
			return true
		}
		if events[i].Beat != stepBeat(eventStep(events[i])) {
			return true
		}
	}
	return false
}

// tripletBeatsMatch compares a stored beat with the beat the repair computes,
// absorbing the float error of the trip between them.
func tripletBeatsMatch(stored, want float32) bool {
	return math.Abs(float64(stored-want)) <= tripletBeatEpsilon
}

// markCell records a cell as taken for a voice, in the per-voice cell maps the
// repair works with.
func markCell(cells map[*Voice]map[int]bool, v *Voice, cell int) {
	if cells[v] == nil {
		cells[v] = map[int]bool{}
	}
	cells[v][cell] = true
}

// Beats returns the configured pattern duration, defaulting to four beats.
func (p *Pattern) Beats() float32 {
	return float32(p.LengthSteps()) * patternBeatsPerStep
}

func (p *Pattern) LengthSteps() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lengthStepsLocked()
}

func (p *Pattern) SetLengthSteps(steps int) int {
	steps = min(max(steps, 1), maxPatternSteps)
	p.mu.Lock()
	p.lengthSteps = steps
	for _, event := range p.events {
		eventStepNumber := eventStep(event)
		if eventStepNumber < 0 || eventStepNumber >= steps {
			p.clearTieBeforeEventLocked(event)
		}
	}
	kept := p.events[:0]
	for _, event := range p.events {
		eventStepNumber := eventStep(event)
		if eventStepNumber >= 0 && eventStepNumber < steps {
			kept = append(kept, event)
		}
	}
	p.events = kept
	p.normalizeLocked()
	p.mu.Unlock()
	return steps
}
