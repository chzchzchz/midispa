package main

// A pad press and a knob turn change one event and then normalise the whole pattern, so
// these are the numbers that say what editing costs. The fixture is the same full pattern
// the display benchmarks use, which is the worst case an edit can meet: every step already
// holds a note on every track, so the tie repair has the most events to walk past.

import "testing"

// Toggling the same step over and over alternates a removal and an insertion, so the
// pattern stays the size it started and neither half of the benchmark is measuring a
// pattern that is growing or shrinking.
func BenchmarkToggleEvent(b *testing.B) {
	bank := perfBank(b)
	pattern := bank.CurrentPattern()
	voice := bank.visibleTrackVoices()[0]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(i % maxPatternSteps), Velocity: 90})
	}
}

// A knob turn is the other edit a player makes, and it goes through the same normalisation.
func BenchmarkSetChromaticNote(b *testing.B) {
	bank := perfBank(b)
	pattern := bank.CurrentPattern()
	voice := bank.visibleTrackVoices()[0]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, ok := pattern.SetChromaticNote(i%maxPatternSteps, voice, 36+i%12, 90); !ok {
			b.Fatal("expected the fixture to hold a note on every step")
		}
	}
}
