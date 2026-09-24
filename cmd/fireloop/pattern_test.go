package main

import (
	"testing"
)

func TestFindBeat(t *testing.T) {
	evs := []Event{
		{Beat: 1},
		{Beat: 2},
		{Beat: 3},
		{Beat: 4},
	}
	p := Pattern{Events: evs}
	for i, tt := range []struct {
		beat float32
		evs  int
	}{
		{0, 4}, {1, 4}, {2, 3}, {3, 2}, {4, 1}, {4.1, 0},
	} {
		if v := len(p.FindBeat(tt.beat)); v != tt.evs {
			t.Errorf("test#%d: expected %d, got %d", i, tt.evs, v)
		}
	}
}

func TestFindBeatReturnsSnapshot(t *testing.T) {
	p := Pattern{Events: []Event{{Beat: 1, Velocity: 10}}}
	found := p.FindBeat(0)
	if len(found) != 1 {
		t.Fatalf("expected one event, got %d", len(found))
	}
	found[0].Beat = 2
	found[0].Velocity = 20
	if p.Events[0].Beat != 1 || p.Events[0].Velocity != 10 {
		t.Fatalf("editing result changed pattern: %+v", p.Events[0])
	}
	p.ToggleEvent(Event{Beat: 1})
	if len(found) != 1 {
		t.Fatalf("editing pattern changed result length: %d", len(found))
	}
}

func TestSetPatternCopiesEvents(t *testing.T) {
	vb := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "voice", Note: 60, Channel: 1}},
	}})
	pb := &PatternBank{
		Patterns:  map[int]*Pattern{1: {}},
		selPatIdx: 1,
		f:         NewFire(func([]byte) error { return nil }),
		vb:        vb,
	}
	source := &Pattern{Events: []Event{{Beat: 1}}}
	if err := pb.SetPattern(source); err != nil {
		t.Fatal(err)
	}
	source.Events[0].Beat = 2
	if got := pb.Patterns[1].Events[0].Beat; got != 1 {
		t.Fatalf("pasted pattern changed with source: %v", got)
	}
}
