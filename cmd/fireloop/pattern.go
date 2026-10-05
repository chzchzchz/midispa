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
	// Events is held in beat order. FindBeat binary searches it instead of sorting a copy
	// of it, so every path that assigns here has to normalise before the pattern is read
	// again: the setters here do it themselves, and the load and restore paths do it
	// through SetLengthSteps and normalizeLocked. TestEventsStayInBeatOrder is what holds
	// this to account.
	Events []Event
	// lengthSteps is measured in sixteenth notes; zero keeps the legacy four-beat default.
	lengthSteps int
	mu          sync.RWMutex
}

var emptyPattern Pattern

// eventStep converts the stored beat position to the integer grid position used by editing and ties.
func eventStep(event Event) int {
	return int(math.Round(float64(event.Beat * patternStepsPerBeat)))
}

func stepBeat(step int) float32 {
	return float32(step) * patternBeatsPerStep
}

func (p *Pattern) Copy() *Pattern {
	p.mu.RLock()
	evs := append([]Event(nil), p.Events...)
	lengthSteps := p.lengthSteps
	p.mu.RUnlock()
	copyPattern := &Pattern{Events: evs, lengthSteps: lengthSteps}
	copyPattern.normalizeLocked()
	return copyPattern
}

// snapshot returns the events and the resolved length under one read lock, so a save
// cannot pair the events of one edit with the length of another. The length is resolved
// because a stored zero means the legacy default rather than an empty pattern.
func (p *Pattern) snapshot() ([]Event, int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]Event(nil), p.Events...), p.lengthStepsLocked()
}

// ToggleEvent returns true if event is added, false if deleted.
func (p *Pattern) ToggleEvent(ev Event) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ev.Beat < 0 || eventStep(ev) >= p.lengthStepsLocked() {
		return false
	}
	for i, current := range p.Events {
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
	first := sort.Search(len(p.Events), func(i int) bool {
		return p.Events[i].Beat >= beat
	})
	return append([]Event(nil), p.Events[first:]...)
}

func (p *Pattern) ClearVoice(v *Voice) {
	p.mu.Lock()
	kept := p.Events[:0]
	for _, event := range p.Events {
		if event.Voice != v {
			kept = append(kept, event)
		}
	}
	p.Events = kept
	p.normalizeLocked()
	p.mu.Unlock()
}

// EventsForVoice returns a snapshot of one voice's events in beat order. The list it is
// filtered from is in beat order itself, so the filtered list is too and there is nothing
// to sort.
func (p *Pattern) EventsForVoice(v *Voice) []Event {
	p.mu.RLock()
	defer p.mu.RUnlock()
	events := make([]Event, 0, len(p.Events))
	for _, event := range p.Events {
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
	// the cost of the loop it replaced.
	for _, event := range p.Events {
		if eventStep(event) == step && event.Voice == v {
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
		p.Events[index].ChromaticNote = note
		p.Events[index].Velocity = velocity
		p.normalizeLocked()
		return p.Events[index], true
	}
	event := Event{
		Voice:         v,
		Beat:          stepBeat(step),
		ChromaticNote: note,
		Velocity:      velocity,
	}
	insertAt := p.insertEventLocked(event)
	return p.Events[insertAt], true
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
	p.Events[index].Velocity = clampStepVelocity(v, velocity)
	return p.Events[index], true
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
		if p.Events[i].Voice == v {
			return false
		}
	}
	p.Events[earlier].Tie = tie
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

// validStep reports whether a step is inside the pattern's length. It reads the length
// under the lock of its own, so it belongs before a caller takes the lock rather than
// inside one.
func (p *Pattern) validStep(step int) bool {
	return step >= 0 && step < p.LengthSteps()
}

func (p *Pattern) eventIndexAtStepLocked(step int, v *Voice) int {
	for i, event := range p.Events {
		if eventStep(event) == step && event.Voice == v {
			return i
		}
	}
	return -1
}

func (p *Pattern) insertEventLocked(event Event) int {
	insertAt := sort.Search(len(p.Events), func(i int) bool {
		return p.Events[i].Beat > event.Beat
	})
	p.clearTieBeforeLocked(insertAt, event.Voice)
	p.Events = append(p.Events, Event{})
	copy(p.Events[insertAt+1:], p.Events[insertAt:])
	p.Events[insertAt] = event
	p.normalizeLocked()
	return insertAt
}

func (p *Pattern) removeEventLocked(index int) Event {
	p.clearPreviousTieLocked(index)
	event := p.Events[index]
	p.Events = append(p.Events[:index], p.Events[index+1:]...)
	p.normalizeLocked()
	return event
}

// clearPreviousTieLocked ends the tie running into the event at index, which is what
// removing it has to do: the note after it would otherwise stay tied to a note that is no
// longer there.
func (p *Pattern) clearPreviousTieLocked(index int) {
	if index <= 0 || index >= len(p.Events) {
		return
	}
	p.clearTieBeforeLocked(index, p.Events[index].Voice)
}

// clearTieBeforeLocked ends the tie running into a position for one voice, which is also
// what inserting there has to do. The walk goes backwards because the events are held in
// beat order and only the nearest one before the position can be tied into it.
func (p *Pattern) clearTieBeforeLocked(index int, voice *Voice) {
	for i := index - 1; i >= 0; i-- {
		if p.Events[i].Voice == voice {
			p.Events[i].Tie = false
			return
		}
	}
}

func (p *Pattern) clearTieBeforeEventLocked(event Event) {
	previous := -1
	for i, current := range p.Events {
		if current.Voice != event.Voice || current.Beat >= event.Beat {
			continue
		}
		if previous < 0 || current.Beat > p.Events[previous].Beat {
			previous = i
		}
	}
	if previous >= 0 {
		p.Events[previous].Tie = false
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
	if !eventsInBeatOrder(p.Events) {
		sort.SliceStable(p.Events, func(i, j int) bool {
			return p.Events[i].Beat < p.Events[j].Beat
		})
	}
	// Nothing carries a tie, so there is nothing to point at anything: the repair below
	// would clear flags that are already clear and set none of them. Skipping it keeps an
	// ordinary note edit from walking every event twice and rounding every beat twice to
	// find that out.
	if !anyEventTied(p.Events) {
		return
	}
	tieFlags := make([]bool, len(p.Events))
	lengthSteps := p.lengthStepsLocked()
	for i := range p.Events {
		tieFlags[i] = p.Events[i].Tie && p.Events[i].IsChromatic() && eventStep(p.Events[i]) >= 0 && eventStep(p.Events[i]) < lengthSteps
		p.Events[i].Tie = false
	}
	for i := range p.Events {
		if !tieFlags[i] {
			continue
		}
		for j := i + 1; j < len(p.Events); j++ {
			if eventStep(p.Events[j]) < 0 || eventStep(p.Events[j]) >= lengthSteps {
				break
			}
			if p.Events[j].Voice == p.Events[i].Voice {
				p.Events[i].Tie = true
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
	for _, event := range p.Events {
		eventStepNumber := eventStep(event)
		if eventStepNumber < 0 || eventStepNumber >= steps {
			p.clearTieBeforeEventLocked(event)
		}
	}
	kept := p.Events[:0]
	for _, event := range p.Events {
		eventStepNumber := eventStep(event)
		if eventStepNumber >= 0 && eventStepNumber < steps {
			kept = append(kept, event)
		}
	}
	p.Events = kept
	p.normalizeLocked()
	p.mu.Unlock()
	return steps
}
