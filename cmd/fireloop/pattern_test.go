package main

import (
	"sort"
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

// FindBeat binary searches Events rather than sorting a copy of it, so keeping Events in
// beat order is an invariant every path that changes the notes has to keep. This walks the
// paths that can break it: the editing setters, a copy, a file listing its events out of
// order, and a restore into the bank.
func TestEventsStayInBeatOrder(t *testing.T) {
	drum := 36
	kit := NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{
		{Name: "lead", Channel: 1},
		{Name: "snare", Channel: 1, Note: &drum},
	}}})
	lead := kit.voices[0]
	snare := kit.voices[1]

	pattern := &Pattern{}
	// Edited in an order no player would type: a late step first, then earlier ones, one
	// event toggled off again, and a length that drops the tail.
	for _, step := range []int{12, 3, 7, 3, 9, 1} {
		pattern.ToggleEvent(Event{Voice: lead, Beat: stepBeat(step), Velocity: 90})
	}
	pattern.ToggleEvent(Event{Voice: snare, Beat: stepBeat(5), Velocity: 90})
	if _, ok := pattern.SetChromaticNote(7, lead, 60, 90); !ok {
		t.Fatal("expected step 7 to take the note it just got")
	}
	pattern.SetVelocity(7, lead, 100)
	pattern.RemoveEventAtStep(3, lead)
	pattern.SetLengthSteps(8)
	assertEventsInBeatOrder(t, "after editing", pattern)
	assertEventsInBeatOrder(t, "after clearing a voice", pattern.Copy())

	pattern.ClearVoice(snare)
	assertEventsInBeatOrder(t, "after clearing a voice", pattern)

	// A file is not obliged to list its events in step order, and the two paths that take a
	// list from outside the pattern are the ones a sorted hand-off would hide a lapse in.
	fromFile, dropped := patternFromState(statePattern{Index: 1, LengthSteps: 8, Events: []stateEvent{
		{Voice: 0, Step: 12, Velocity: 90},
		{Voice: 1, Step: 3, Velocity: 90},
		{Voice: 0, Step: 7, Note: 60, Velocity: 90},
		{Voice: 1, Step: 0, Velocity: 90},
	}}, kit)
	if dropped != 0 {
		t.Fatalf("loading dropped %d events, want 0", dropped)
	}
	assertEventsInBeatOrder(t, "loaded from a file", fromFile)

	bank, _ := quietBank(t, kit)
	scrambled := &Pattern{Events: []Event{
		{Voice: lead, Beat: stepBeat(9), Velocity: 90},
		{Voice: snare, Beat: stepBeat(2), Velocity: 90},
		{Voice: lead, Beat: stepBeat(4), Velocity: 90},
	}}
	if err := bank.SetPattern(scrambled); err != nil {
		t.Fatal(err)
	}
	assertEventsInBeatOrder(t, "restored into the bank", bank.CurrentPattern())
}

// assertEventsInBeatOrder fails when a pattern's events are out of beat order, and then
// checks FindBeat against what the same lookup would return from a copy that was sorted the
// slow way, so a pattern that kept its order still cannot hand back the wrong window.
func assertEventsInBeatOrder(t *testing.T, what string, p *Pattern) {
	t.Helper()
	events, _ := p.snapshot()
	for i := 1; i < len(events); i++ {
		if events[i].Beat < events[i-1].Beat {
			t.Fatalf("%s: event %d is at beat %v, after the beat %v of the event before it", what, i, events[i].Beat, events[i-1].Beat)
		}
	}
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Beat < sorted[j].Beat })
	for _, beat := range []float32{0, 0.5, 1, 2, 3} {
		var want []Event
		for _, event := range sorted {
			if event.Beat >= beat {
				want = append(want, event)
			}
		}
		got := p.FindBeat(beat)
		if len(got) != len(want) {
			t.Fatalf("%s: FindBeat(%v) returned %d events, want %d", what, beat, len(got), len(want))
		}
		for i := range got {
			if got[i].Beat != want[i].Beat || got[i].Voice != want[i].Voice {
				t.Fatalf("%s: FindBeat(%v) event %d is beat %v voice %v, want beat %v voice %v",
					what, beat, i, got[i].Beat, got[i].Voice, want[i].Beat, want[i].Voice)
			}
		}
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
