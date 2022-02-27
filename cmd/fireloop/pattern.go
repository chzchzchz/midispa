package main

import (
	"sort"
	"sync"
)

type Pattern struct {
	Events []Event
	mu     sync.RWMutex
}

var emptyPattern Pattern

func (p *Pattern) Copy() *Pattern {
	p.mu.RLock()
	defer p.mu.RUnlock()
	evs := make([]Event, len(p.Events))
	copy(evs, p.Events)
	return &Pattern{Events: evs}
}

// ToggleEvent returns true if event is added, false if deleted.
func (p *Pattern) ToggleEvent(ev Event) bool {
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
	ret = p.Events[l:]
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

func (p *Pattern) Beats() float32 { return 4.0 }
