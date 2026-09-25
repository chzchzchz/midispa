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

func TestPatternLength(t *testing.T) {
	pattern := Pattern{Events: []Event{
		{Beat: 0},
		{Beat: 0.75},
		{Beat: 1},
		{Beat: 3.75},
		{Beat: 4},
	}}
	if got := pattern.LengthSteps(); got != defaultPatternSteps {
		t.Fatalf("default length = %d, want %d", got, defaultPatternSteps)
	}
	if got := pattern.SetLengthSteps(4); got != 4 {
		t.Fatalf("set length = %d, want 4", got)
	}
	if got := pattern.LengthSteps(); got != 4 {
		t.Fatalf("stored length = %d, want 4", got)
	}
	if got := pattern.Beats(); got != 1 {
		t.Fatalf("length in beats = %v, want 1", got)
	}
	if got := len(pattern.Events); got != 2 {
		t.Fatalf("events after shortening = %d, want 2", got)
	}
	if pattern.ToggleEvent(Event{Beat: 1}) {
		t.Fatal("event at the pattern boundary was accepted")
	}
	if got := pattern.SetLengthSteps(0); got != 1 {
		t.Fatalf("minimum length = %d, want 1", got)
	}
	if got := pattern.SetLengthSteps(17); got != maxPatternSteps {
		t.Fatalf("maximum length = %d, want %d", got, maxPatternSteps)
	}
}

func TestSongUsesVariablePatternLengths(t *testing.T) {
	first := &Pattern{}
	first.SetLengthSteps(4)
	second := &Pattern{}
	second.SetLengthSteps(8)
	song := &Song{}
	song.SetPattern(first, 0)
	song.SetPattern(second, 1)
	if got := song.IndexToBeat(1); got != 1 {
		t.Fatalf("second pattern starts at %v beats, want 1", got)
	}
	if got, index := song.BeatToPattern(1.25); got != second || index != 1 {
		t.Fatalf("beat 1.25 resolved to %p/%d, want second/1", got, index)
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
	source.SetLengthSteps(8)
	if err := pb.SetPattern(source); err != nil {
		t.Fatal(err)
	}
	source.Events[0].Beat = 2
	source.SetLengthSteps(4)
	if got := pb.Patterns[1].Events[0].Beat; got != 1 {
		t.Fatalf("pasted pattern changed with source: %v", got)
	}
	if got := pb.Patterns[1].LengthSteps(); got != 8 {
		t.Fatalf("pasted pattern length = %d, want 8", got)
	}
}
