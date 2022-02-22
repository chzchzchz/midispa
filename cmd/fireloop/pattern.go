package main

import (
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
	l, r := 0, len(p.Events)-1
	for l < r {
		mid := (l + r) / 2
		midv := p.Events[mid].Beat
		if midv < beat {
			l = mid + 1
		} else if midv > beat {
			r = mid
		} else if p.Events[l].Beat < beat {
			// ev[l] <= beat; ev[r] >= beat
			l = l + 1
		} else {
			r = mid - 1
		}
	}
	if len(p.Events) > 0 && p.Events[l].Beat >= beat {
		ret = p.Events[l:]
	}
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
