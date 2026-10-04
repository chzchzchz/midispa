package main

import (
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func TestChromaticVelocityControl(t *testing.T) {
	bank, voice := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	if err := handlePatternGrid(nil, 1, 0, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	if !ok || event.Velocity != pressVelocity {
		t.Fatalf("initial velocity = %d/%v, want the pad press %d", event.Velocity, ok, pressVelocity)
	}
	if bank.chromaticVelocity != defaultStepVelocity {
		t.Fatalf("encoder value = %d, want it to start at %d", bank.chromaticVelocity, defaultStepVelocity)
	}
	// A detent moves from the encoder's own value, so the first one does not start from
	// the velocity the pad press gave the note.
	cc := alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(EncoderLeft)}}
	if err := processPatternEvent(nil, cc); err != nil {
		t.Fatal(err)
	}
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	if event.Velocity != defaultStepVelocity-velocityStep {
		t.Fatalf("downward velocity = %d, want %d", event.Velocity, defaultStepVelocity-velocityStep)
	}
	cc.Data[2] = byte(EncoderRight)
	if err := processPatternEvent(nil, cc); err != nil {
		t.Fatal(err)
	}
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	if event.Velocity != defaultStepVelocity {
		t.Fatalf("upward velocity = %d, want %d", event.Velocity, defaultStepVelocity)
	}
	if event.Velocity > midiNoteMax {
		t.Fatalf("velocity above the maximum = %d", event.Velocity)
	}
}

// The encoder keeps its value across steps, so a step picked after the encoder was set
// takes that value on the next detent instead of its own.
func TestVelocityEncoderValueIsInheritedByTheNextStep(t *testing.T) {
	bank, voice := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}

	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 40)
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	// Put the encoder at 100 by turning it, then leave that step.
	for bank.chromaticVelocity > 100 {
		if err := processPatternEvent(nil, encoderCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	if bank.chromaticVelocity != 100 {
		t.Fatalf("encoder value = %d, want 100", bank.chromaticVelocity)
	}
	// Selecting the second step must not drag the encoder back to that step's 40.
	if err := bank.MoveStepCursor(1); err != nil {
		t.Fatal(err)
	}
	if bank.chromaticVelocity != 100 {
		t.Fatalf("encoder value after selecting = %d, want the inherited 100", bank.chromaticVelocity)
	}
	if err := processPatternEvent(nil, encoderCC(EncoderLeft)); err != nil {
		t.Fatal(err)
	}
	if event, _ := pattern.EventAtStep(1, voice); event.Velocity != 99 {
		t.Fatalf("second step velocity = %d, want the encoder's 99", event.Velocity)
	}
	// The first step is untouched by the encoder moving on the second.
	if event, _ := pattern.EventAtStep(0, voice); event.Velocity != 100 {
		t.Fatalf("first step velocity = %d, want it left at 100", event.Velocity)
	}
}

func encoderCC(direction int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(direction)}}
}

// A percussive step takes its dynamics from the pad that placed it, so the Volume knob
// trims that hit rather than dragging the step up to the encoder's carried value. That
// value belongs to the chromatic knob, and turning this one must leave it alone.
func TestPercussionVelocityKnobTrimsTheHit(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	voice := voiceBank.voices[0]
	note, _ := voice.PercussionNote()
	hit := 100
	if err := handlePatternGrid(nil, 3, 0, hit); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(3, voice)
	if !ok || event.Velocity != hit {
		t.Fatalf("placed step velocity = %d/%v, want the hit %d", event.Velocity, ok, hit)
	}
	// A percussive pad press toggles a step without selecting it, so the edit still has to
	// be moved onto it before the knob has anything to trim.
	if err := bank.MoveStepCursor(3); err != nil {
		t.Fatal(err)
	}
	// A detent counts from the hit, not from the encoder's own much higher value.
	if err := processPatternEvent(nil, encoderCC(EncoderLeft)); err != nil {
		t.Fatal(err)
	}
	event, _ = bank.CurrentPattern().EventAtStep(3, voice)
	if event.Velocity != hit-velocityStep {
		t.Fatalf("velocity after one detent = %d, want %d trimmed from the hit", event.Velocity, hit-velocityStep)
	}
	if bank.chromaticVelocity != defaultStepVelocity {
		t.Fatalf("the detent moved the chromatic register to %d", bank.chromaticVelocity)
	}
	// Turning back restores the hit, and the audition carries the value the step now has.
	preview := &captureMidiWriter{}
	if err := bank.AdjustVelocity(preview, EncoderRight); err != nil {
		t.Fatal(err)
	}
	event, _ = bank.CurrentPattern().EventAtStep(3, voice)
	if event.Velocity != hit {
		t.Fatalf("velocity after turning back = %d, want the hit %d", event.Velocity, hit)
	}
	if len(preview.events) != 2 {
		t.Fatalf("the audition wrote %d messages, want a note-on and a note-off", len(preview.events))
	}
	assertMidiData(t, preview.events[0], []byte{midi.MakeNoteOn(0), byte(note), byte(hit)})
}

// The knob cannot turn a percussive step off, because velocity zero is a note-off rather
// than a quiet note. The floor is enforced on the stored value and again on the wire, so
// the two can never disagree.
func TestPercussionVelocityKnobStopsAtOne(t *testing.T) {
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
	note, _ := voice.PercussionNote()
	if err := handlePatternGrid(nil, 0, 0, 2); err != nil {
		t.Fatal(err)
	}
	for range 8 {
		if err := processPatternEvent(nil, encoderCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	if !ok {
		t.Fatal("turning the knob past the floor removed the step")
	}
	if event.Velocity != minPercussionVelocity {
		t.Fatalf("velocity past the floor = %d, want it held at %d", event.Velocity, minPercussionVelocity)
	}
	// The readout reports the floor, so the display never claims a velocity nothing plays.
	if status := recorder.row(lengthDisplayRow); status != "S01 @001" {
		t.Fatalf("status at the floor = %q, want %q", status, "S01 @001")
	}
	assertMidiData(t, alsa.SeqEvent{Data: event.NoteOnMidi()}, []byte{midi.MakeNoteOn(0), byte(note), minPercussionVelocity})
}
