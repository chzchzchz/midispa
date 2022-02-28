package main

import (
	"sort"
	"time"
)

type Track struct {
	events [][]Event // indexed by second offset
}

type Event struct {
	clock time.Duration // time since start of song
	data  []byte
}

func (t *Track) Events() (ret []Event) {
	for _, evs := range t.events {
		for _, ev := range evs {
			ret = append(ret, ev)
		}
	}
	return ret
}

func (t *Track) Add(ev Event) {
	idx := int(ev.clock.Seconds())
	if needed := idx - (len(t.events) - 1); needed > 0 {
		t.events = append(t.events, make([][]Event, needed)...)
	}
	evs := t.events[idx]
	cmp := func(v int) bool { return evs[v].clock >= ev.clock }
	i := sort.Search(len(evs), cmp)
	evs = append(evs[:i], append([]Event{ev}, evs[i:]...)...)
	t.events[idx] = evs
}

func (t *Track) Erase(start time.Duration, end time.Duration) {
	if int(start.Seconds()) >= len(t.events) {
		return
	}
	if int(end.Seconds()) >= len(t.events) {
		end = time.Second * time.Duration(len(t.events)-1)
	}
	ssecs, esecs := start.Seconds(), end.Seconds()
	// Partial beginning second.
	startEvs := t.events[int(ssecs)]
	cmpStartLo := func(v int) bool { return startEvs[v].clock >= start }
	cmpStartHi := func(v int) bool { return startEvs[v].clock >= end }
	startLo := sort.Search(len(startEvs), cmpStartLo)
	startHi := sort.Search(len(startEvs), cmpStartHi)
	t.events[int(ssecs)] = startEvs[startLo:startHi]
	// Full seconds that can be dropped completely.
	for i := int(ssecs) + 1; i < int(esecs); i++ {
		t.events[i] = nil
	}
	// Partial end second.
	endEvs := t.events[int(esecs)]
	cmpEndHi := func(v int) bool { return endEvs[v].clock >= end }
	endHi := sort.Search(len(endEvs), cmpEndHi)
	t.events[int(esecs)] = endEvs[:endHi]
}
