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

// FindBeat returns a slice of all events >= a given beat.
func (p *Pattern) FindBeat(beat float32) (ret []Event) {
	p.mu.RLock()
	evs := append([]Event(nil), p.Events...)
	p.mu.RUnlock()
	sort.SliceStable(evs, func(i, j int) bool {
		return evs[i].Beat < evs[j].Beat
	})
	l := sort.Search(len(evs), func(i int) bool {
		return evs[i].Beat >= beat
	})
	ret = evs[l:]
	return ret
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

func (p *Pattern) EventsForVoice(v *Voice) []Event {
	p.mu.RLock()
	events := make([]Event, 0)
	for _, event := range p.Events {
		if event.Voice == v {
			events = append(events, event)
		}
	}
	p.mu.RUnlock()
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Beat < events[j].Beat
	})
	return events
}

func (p *Pattern) EventAtStep(step int, v *Voice) (Event, bool) {
	if step < 0 || step >= p.LengthSteps() {
		return Event{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, event := range p.Events {
		if eventStep(event) == step && event.Voice == v {
			return event, true
		}
	}
	return Event{}, false
}

func (p *Pattern) RemoveEventAtStep(step int, v *Voice) (Event, bool) {
	if step < 0 || step >= p.LengthSteps() {
		return Event{}, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, event := range p.Events {
		if eventStep(event) == step && event.Voice == v {
			return p.removeEventLocked(i), true
		}
	}
	return Event{}, false
}

// SetChromaticNote creates or updates a pitched event while preserving its tie state.
func (p *Pattern) SetChromaticNote(step int, v *Voice, note, velocity int) (Event, bool) {
	if v == nil || !v.IsChromatic() || step < 0 || step >= p.LengthSteps() {
		return Event{}, false
	}
	if note < 0 || note > midiNoteMax {
		return Event{}, false
	}
	if velocity < 0 {
		velocity = 0
	}
	if velocity > midiNoteMax {
		velocity = midiNoteMax
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, event := range p.Events {
		if eventStep(event) == step && event.Voice == v {
			p.Events[i].ChromaticNote = note
			p.Events[i].Velocity = velocity
			p.normalizeLocked()
			return p.Events[i], true
		}
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

func (p *Pattern) SetChromaticVelocity(step int, v *Voice, velocity int) (Event, bool) {
	if v == nil || !v.IsChromatic() || step < 0 || step >= p.LengthSteps() {
		return Event{}, false
	}
	velocity = clampMidiDataValue(velocity)
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, event := range p.Events {
		if eventStep(event) == step && event.Voice == v {
			p.Events[i].Velocity = velocity
			return p.Events[i], true
		}
	}
	return Event{}, false
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
	p.clearInsertionTieLocked(insertAt, event.Voice)
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

func (p *Pattern) clearPreviousTieLocked(index int) {
	if index <= 0 || index >= len(p.Events) {
		return
	}
	voice := p.Events[index].Voice
	for i := index - 1; i >= 0; i-- {
		if p.Events[i].Voice == voice {
			p.Events[i].Tie = false
			return
		}
	}
}

func (p *Pattern) clearInsertionTieLocked(index int, voice *Voice) {
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
	if p.lengthSteps == 0 {
		return defaultPatternSteps
	}
	return p.lengthSteps
}

// normalizeLocked keeps event order stable and makes every stored tie point to a valid successor.
func (p *Pattern) normalizeLocked() {
	sort.SliceStable(p.Events, func(i, j int) bool {
		return p.Events[i].Beat < p.Events[j].Beat
	})
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

// Beats returns the configured pattern duration, defaulting to four beats.
func (p *Pattern) Beats() float32 {
	return float32(p.LengthSteps()) * patternBeatsPerStep
}

func (p *Pattern) LengthSteps() int {
	p.mu.RLock()
	steps := p.lengthSteps
	p.mu.RUnlock()
	if steps == 0 {
		return defaultPatternSteps
	}
	return steps
}

func (p *Pattern) SetLengthSteps(steps int) int {
	if steps < 1 {
		steps = 1
	}
	if steps > maxPatternSteps {
		steps = maxPatternSteps
	}
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
