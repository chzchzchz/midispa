package main

import "testing"

func TestFitOLEDTextCountsGlyphsNotBytes(t *testing.T) {
	// A multi-byte rune sitting on the clip boundary is the case that matters: clipping by
	// raw bytes there would cut the name inside it and leave half a character on the row.
	if got := fitOLEDText("1234567890123456789é"); got != "1234567890123456789?" {
		t.Fatalf("clipped name = %q, want %q", got, "1234567890123456789?")
	}
	if got := fitOLEDText("Körg und nochmal bitte"); got != "K?rg und nochmal bit" {
		t.Fatalf("clipped name = %q, want %q", got, "K?rg und nochmal bit")
	}
	// A name that already fits is only replaced, not clipped.
	if got := fitOLEDText("Snare [CHR]"); got != "Snare [CHR]" {
		t.Fatalf("short name = %q, want it unchanged", got)
	}
}

func TestStepStatusText(t *testing.T) {
	chromatic, _ := chromaticTestVoice(t, nil)
	drum := 36
	percussive, _ := chromaticTestVoice(t, &drum)
	event := &Event{ChromaticNote: 60, Velocity: 90, Tie: true}
	if got := stepStatusText(chromatic, 0, event, 1); got != "S01 C4@090->02" {
		t.Fatalf("tied status = %q", got)
	}
	// A step with no note must not show a velocity, or it reads as that step's own.
	if got := stepStatusText(chromatic, 1, nil, -1); got != "S02 --" {
		t.Fatalf("empty status = %q", got)
	}
	// A percussive step has no pitch to name, so it reports its dynamics alone.
	drumEvent := &Event{Velocity: 72}
	if got := stepStatusText(percussive, 2, drumEvent, -1); got != "S03 @072" {
		t.Fatalf("percussive status = %q", got)
	}
	if got := stepStatusText(percussive, 2, nil, -1); got != "S03 --" {
		t.Fatalf("empty percussive status = %q", got)
	}
}

// The Volume knob is the only way to change a percussive step's dynamics once it is
// placed, so the readout has to report what the knob is doing. A percussive step has no
// pitch to name, and an empty step shows no velocity at all.
func TestPercussionStatusFollowsTheSelectedStep(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	recorder := useScreenRecorder(t, &bank.screen)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	voice := voiceBank.voices[0]
	if err := handlePatternGrid(nil, 2, 0, 84); err != nil {
		t.Fatal(err)
	}
	if err := bank.MoveStepCursor(2); err != nil {
		t.Fatal(err)
	}
	if status := recorder.row(lengthDisplayRow); status != "S03 @084" {
		t.Fatalf("status on a percussive step = %q, want %q", status, "S03 @084")
	}
	// An empty step shows nothing, the same as a chromatic one, so a velocity belonging to
	// no step cannot make two steps look equal.
	if err := bank.MoveStepCursor(2); err != nil {
		t.Fatal(err)
	}
	if status := recorder.row(lengthDisplayRow); status != "S05 --" {
		t.Fatalf("status on an empty step = %q, want %q", status, "S05 --")
	}
	if event, ok := bank.CurrentPattern().EventAtStep(2, voice); !ok || event.Velocity != 84 {
		t.Fatalf("moving the cursor changed the step to %d/%v", event.Velocity, ok)
	}
}

func TestModeDoesNotStopPlayback(t *testing.T) {
	bank, _ := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	stopCalled := false
	playbackStop = func() error {
		stopCalled = true
		return nil
	}
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	if stopCalled || playbackStop == nil {
		t.Fatal("Mode stopped or cleared active playback")
	}
}

func TestChromaticPadSetsEditingStep(t *testing.T) {
	bank, _ := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(54+3, 100)); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 3 {
		t.Fatalf("step cursor after pad = %d, want 3", bank.StepCursor())
	}
	bank.releasePad(0, 3)
	if err := processPatternEvent(nil, padMessage(54+5, 100)); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 5 {
		t.Fatalf("step cursor after second pad = %d, want 5", bank.StepCursor())
	}
}
