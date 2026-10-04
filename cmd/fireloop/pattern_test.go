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
		Voices:  []Voice{{Name: "voice", Note: testNote(60), Channel: 1}},
	}})
	pb := NewPatternBank(NewFire(func([]byte) error { return nil }), vb)
	if err := pb.Jump(1); err != nil {
		t.Fatal(err)
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

func TestChromaticPatternEditingAndTieInvariants(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	first, ok := pattern.SetChromaticNote(0, voice, 60, 90)
	if !ok || first.ChromaticNote != 60 || first.Velocity != 90 {
		t.Fatalf("first chromatic edit = %+v/%v", first, ok)
	}
	pattern.SetChromaticNote(1, voice, 62, 91)
	pattern.SetChromaticNote(2, voice, 64, 92)
	if pattern.TieEventsAtSteps(0, 2, voice) {
		t.Fatal("tie crossed an intervening event")
	}
	if !pattern.TieEventsAtSteps(1, 0, voice) {
		t.Fatal("reversed two-pad arrival did not create a tie")
	}
	firstEvent, _ := pattern.EventAtStep(0, voice)
	if !firstEvent.Tie {
		t.Fatal("tie was not stored on the earlier event")
	}
	pattern.SetChromaticNote(0, voice, 67, 90)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	if !firstEvent.Tie || firstEvent.ChromaticNote != 67 {
		t.Fatalf("pitch edit lost tie: %+v", firstEvent)
	}
	copyPattern := pattern.Copy()
	copyPattern.SetChromaticNote(0, voice, 69, 90)
	if firstEvent, _ := pattern.EventAtStep(0, voice); firstEvent.ChromaticNote != 67 || !firstEvent.Tie {
		t.Fatalf("copy shared chromatic state: %+v", firstEvent)
	}
	pattern.RemoveEventAtStep(1, voice)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	if firstEvent.Tie {
		t.Fatal("removing the target left a dangling tie")
	}
	pattern.SetChromaticNote(1, voice, 62, 91)
	pattern.TieEventsAtSteps(0, 1, voice)
	pattern.SetLengthSteps(1)
	if firstEvent, _ := pattern.EventAtStep(0, voice); firstEvent.Tie {
		t.Fatal("shortening left a tie crossing the new end")
	}
}
