package main

// The display is redrawn on every cursor move, every knob detent and every step of playback,
// so its cost is worth watching. The fixtures below fill a pattern as full as the unit
// allows — every step carrying a note on every track — which is the worst case a redraw can
// meet, and the one that makes an extra pass over the notes show up.

import "testing"

// perfKit is a four-voice kit: two pitched, two percussive, so a redraw has to ask for both
// kinds of step colour.
func perfKit() *VoiceBank {
	drum := 36
	return NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{
		{Name: "lead", Channel: 1},
		{Name: "snare", Channel: 1, Note: &drum},
		{Name: "hat", Channel: 1, Note: &drum},
		{Name: "bass", Channel: 1},
	}}})
}

// perfBank is the state a redraw has to paint: pattern 1 selected and full, with the cursor
// on its first step.
func perfBank(b *testing.B) *PatternBank {
	b.Helper()
	bank := NewPatternBank(NewFire(func([]byte) error { return nil }), perfKit())
	if err := bank.Jump(1); err != nil {
		b.Fatal(err)
	}
	pattern := bank.CurrentPattern()
	for step := 0; step < maxPatternSteps; step++ {
		for _, voice := range bank.visibleTrackVoices() {
			event := Event{Voice: voice, Beat: stepBeat(step), Velocity: 90}
			if voice.IsChromatic() {
				event.ChromaticNote = 36 + step
			}
			pattern.ToggleEvent(event)
		}
	}
	bank.stepCursor = 0
	return bank
}

// A cursor move repaints every track row and rewrites the readout. It is the most repeated
// thing a user does by hand.
func BenchmarkCursorMove(b *testing.B) {
	bank := perfBank(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := bank.MoveStepCursor(1); err != nil {
			b.Fatal(err)
		}
	}
}

// The note palette asks the pattern for one step at a time for every cell it paints, so this
// is the redraw that would notice a step lookup getting slower.
func BenchmarkPaletteRedraw(b *testing.B) {
	bank := perfBank(b)
	if err := bank.SelectTrackRow(1); err != nil {
		b.Fatal(err)
	}
	bank.editingNote = true
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := bank.drawNotePalette(); err != nil {
			b.Fatal(err)
		}
	}
}

// The playhead repaints one column per step, twice per step: once lit, once put back.
func BenchmarkPlayheadColumn(b *testing.B) {
	bank := perfBank(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := bank.drawPadColumn(i % maxPatternSteps); err != nil {
			b.Fatal(err)
		}
	}
}