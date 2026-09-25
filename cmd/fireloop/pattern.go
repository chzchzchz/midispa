package main

import (
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

func (p *Pattern) Copy() *Pattern {
	p.mu.RLock()
	defer p.mu.RUnlock()
	evs := make([]Event, len(p.Events))
	copy(evs, p.Events)
	return &Pattern{Events: evs, lengthSteps: p.lengthSteps}
}

// ToggleEvent returns true if event is added, false if deleted.
func (p *Pattern) ToggleEvent(ev Event) bool {
	if ev.Beat < 0 || ev.Beat >= p.lengthBeats() {
		return false
	}
	isAdd := true
	i := 0
	p.mu.Lock()
	for i < len(p.Events) {
		pev := p.Events[i]
		if pev.Beat == ev.Beat && pev.Voice == ev.Voice {
			p.Events = append(p.Events[:i], p.Events[i+1:]...)
			isAdd = false
			break
		} else if pev.Beat > ev.Beat {
			break
		}
		i++
	}
	if isAdd {
		p.Events = append(p.Events[:i], append([]Event{ev}, p.Events[i:]...)...)
	}
	p.mu.Unlock()
	return isAdd
}

// FindBeat returns a slice of all events >= a given beat.
func (p *Pattern) FindBeat(beat float32) (ret []Event) {
	p.mu.RLock()
	cmp := func(i int) bool { return p.Events[i].Beat >= beat }
	l := sort.Search(len(p.Events), cmp)
	// Copy the suffix before releasing the lock so playback can iterate while editing continues.
	ret = append([]Event(nil), p.Events[l:]...)
	p.mu.RUnlock()
	return ret
}

func (p *Pattern) ClearVoice(v *Voice) {
	p.mu.Lock()
	j := 0
	for i := 0; i < len(p.Events); i++ {
		if p.Events[i].Voice != v {
			p.Events[j] = p.Events[i]
			j++
		}
	}
	p.Events = p.Events[:j]
	p.mu.Unlock()
}

// Beats returns the configured pattern duration, defaulting to four beats.
func (p *Pattern) Beats() float32 { return p.lengthBeats() }

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
	end := float32(steps) * patternBeatsPerStep
	kept := p.Events[:0]
	for _, event := range p.Events {
		if event.Beat < end {
			kept = append(kept, event)
		}
	}
	p.Events = kept
	p.mu.Unlock()
	return steps
}

func (p *Pattern) lengthBeats() float32 {
	return float32(p.LengthSteps()) * patternBeatsPerStep
}
