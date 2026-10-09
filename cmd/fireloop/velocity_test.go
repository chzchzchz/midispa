package main

import (
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

// knobTestClock installs a clock the Volume knob's timing is measured
// against, and hands back the function that advances it. A test advances it
// by hand to land detents on either side of velocityFastWindow without
// sleeping.
func knobTestClock(t *testing.T) func(time.Duration) {
	t.Helper()
	now := time.Now()
	knobNow = func() time.Time { return now }
	t.Cleanup(func() { knobNow = time.Now })
	return func(d time.Duration) { now = now.Add(d) }
}

// percussionStepBank builds a bank with a percussive step placed at step
// with the velocity a pad press would give it, which is the state the
// Volume knob tests start from: the press leaves the cursor on the step, so
// the knob has a hit to trim. The note the step plays is handed back with
// the voice it belongs to.
func percussionStepBank(t *testing.T, step, hit int) (*PatternBank, *Voice, int) {
	t.Helper()
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := trackWindowKit(8, -1)
	controller := useController(t, fire, voiceBank)
	bank := controller.patbank
	require.NoError(t, bank.SelectTrackRow(1))
	voice := voiceBank.voices[0]
	note, _ := voice.PercussionNote()
	require.NoError(t, bank.controller.handlePatternGrid(nil, step, 0, hit))
	return bank, voice, note
}

func TestChromaticVelocityControl(t *testing.T) {
	bank, voice := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	require.NoError(t, bank.controller.handlePatternGrid(nil, 1, 0, 100))
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "the pad press placed no step")
	require.Equal(t, pressVelocity, event.Velocity)
	require.Equal(t, defaultStepVelocity, bank.chromaticVelocity,
		"the encoder should start at the default step velocity")
	// A detent moves from the encoder's own value, so the first one does not start from
	// the velocity the pad press gave the note. The second is spaced past the fast
	// window, so it counts as a careful turn of one step rather than the middle of a
	// fast sweep.
	cc := alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(EncoderLeft)}}
	require.NoError(t, dispatch(bank, cc))
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, defaultStepVelocity-velocityStep, event.Velocity)
	advance := knobTestClock(t)
	advance(velocityFastWindow + time.Millisecond)
	cc.Data[2] = byte(EncoderRight)
	require.NoError(t, dispatch(bank, cc))
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	require.Equal(t, defaultStepVelocity, event.Velocity)
	require.LessOrEqual(t, event.Velocity, midiNoteMax)
}

// The encoder keeps its value across steps, so a step picked after the encoder was set
// takes that value on the next detent instead of its own.
func TestVelocityEncoderValueIsInheritedByTheNextStep(t *testing.T) {
	bank, voice := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))

	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 40)
	require.NoError(t, bank.ToggleNoteMode())
	// Put the encoder at 100 by turning it, then leave that step. Every detent
	// is spaced past the fast window, so each one counts as a single step.
	advance := knobTestClock(t)
	for bank.chromaticVelocity > 100 {
		advance(velocityFastWindow + time.Millisecond)
		require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	}
	require.Equal(t, 100, bank.chromaticVelocity)
	// Selecting the second step must not drag the encoder back to that step's 40.
	require.NoError(t, bank.MoveStepCursor(1))
	require.Equal(t, 100, bank.chromaticVelocity, "the encoder value should be inherited")
	advance(velocityFastWindow + time.Millisecond)
	require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	event, _ := pattern.EventAtStep(1, voice)
	require.Equal(t, 99, event.Velocity, "the second step takes the encoder's detent")
	// The first step is untouched by the encoder moving on the second.
	event, _ = pattern.EventAtStep(0, voice)
	require.Equal(t, 100, event.Velocity, "the encoder reached back to the first step")
}

func encoderCC(direction int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(direction)}}
}

// A percussive step takes its dynamics from the pad that placed it, so the Volume knob
// trims that hit rather than dragging the step up to the encoder's carried value. That
// value belongs to the chromatic knob, and turning this one must leave it alone.
func TestPercussionVelocityKnobTrimsTheHit(t *testing.T) {
	const step, hit = 3, 100
	bank, voice, note := percussionStepBank(t, step, hit)
	event, ok := bank.CurrentPattern().EventAtStep(step, voice)
	require.True(t, ok, "the pad press placed no step")
	require.Equal(t, hit, event.Velocity)
	// The press put the cursor on the step it placed, so the knob has a
	// step to trim without the grid buttons moving the edit first.
	require.Equal(t, step, bank.StepCursor(), "the pad press did not move the cursor")
	// A detent counts from the hit, not from the encoder's own much higher value.
	advance := knobTestClock(t)
	require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	event, _ = bank.CurrentPattern().EventAtStep(step, voice)
	require.Equal(t, hit-velocityStep, event.Velocity)
	require.Equal(t, defaultStepVelocity, bank.chromaticVelocity,
		"the detent moved the chromatic register")
	// Turning back restores the hit, and the audition carries the value the step now has.
	preview := &captureMidiWriter{}
	advance(velocityFastWindow + time.Millisecond)
	require.NoError(t, bank.AdjustVelocity(preview, EncoderRight))
	event, _ = bank.CurrentPattern().EventAtStep(step, voice)
	require.Equal(t, hit, event.Velocity)
	// The audition is held back until the knob rests, so it is waited for
	// rather than read back the moment the detent returns.
	waitFor(t, "the audition", func() bool { return len(preview.snapshot()) >= 2 })
	audition := preview.snapshot()
	require.Len(t, audition, 2, "the audition should write a note-on and a note-off")
	assertMidiData(t, audition[0], []byte{midi.MakeNoteOn(0), byte(note), byte(hit)})
}

// The knob cannot turn a percussive step off, because velocity zero is a note-off rather
// than a quiet note. The floor is enforced on the stored value and again on the wire, so
// the two can never disagree.
func TestPercussionVelocityKnobStopsAtOne(t *testing.T) {
	bank, voice, note := percussionStepBank(t, 0, 2)
	recorder := useScreenRecorder(t, &bank.screen)
	for range 8 {
		require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	require.True(t, ok, "turning the knob past the floor removed the step")
	require.Equal(t, minPercussionVelocity, event.Velocity)
	// The readout reports the floor, so the display never claims a velocity nothing plays.
	require.Equal(t, "S01 @001", recorder.row(readoutRow))
	assertMidiData(t, alsa.SeqEvent{Data: event.NoteOnMidi()}, []byte{midi.MakeNoteOn(0), byte(note), minPercussionVelocity})
}

// A detent counts further the faster the knob turns: detents that arrive
// inside the fast window stack one more step each, a detent after a rest
// starts again at one, and the stack caps so a long sweep cannot throw a
// velocity across the whole range in one turn.
func TestVolumeKnobCountsFasterWhenTurnedFaster(t *testing.T) {
	bank, voice := chromaBank(t)
	require.NoError(t, bank.SelectTrackRow(1))
	require.NoError(t, dispatch(bank, padMessage(NoteMode, 100)))
	require.NoError(t, bank.controller.handlePatternGrid(nil, 1, 0, 100))
	advance := knobTestClock(t)
	stepVelocity := func() int {
		event, _ := bank.CurrentPattern().EventAtStep(0, voice)
		return event.Velocity
	}
	// Each detent's gap after the one before it, and how far that detent
	// counts: a detent inside the fast window counts one more than the
	// last, and a detent after a rest starts again at one. The first
	// detent has nothing before it, so its gap is unused.
	detents := []struct {
		gap       time.Duration
		increment int
	}{
		{0, 1},
		{10 * time.Millisecond, 2},
		{10 * time.Millisecond, 3},
		{velocityFastWindow + time.Millisecond, 1},
		{10 * time.Millisecond, 2},
		{10 * time.Millisecond, 3},
		{10 * time.Millisecond, 4},
		{10 * time.Millisecond, 5},
		{10 * time.Millisecond, 6},
		{10 * time.Millisecond, 7},
		{10 * time.Millisecond, 8},
		{10 * time.Millisecond, 8}, // the cap, and it holds
	}
	want := defaultStepVelocity
	for i, detent := range detents {
		advance(detent.gap)
		require.NoError(t, dispatch(bank, encoderCC(EncoderLeft)))
		want -= detent.increment * velocityStep
		require.Equal(t, want, stepVelocity(), "detent %d of the turn", i+1)
	}
}

// The audition is held back until the knob rests, so a fast turn is heard
// once, at the value the sweep landed on, and not at all while the knob is
// still moving.
func TestVolumeKnobAuditionsTheStepOnlyOnceTheKnobRests(t *testing.T) {
	const step, hit = 3, 100
	bank, voice, note := percussionStepBank(t, step, hit)
	// The window is widened so the detents below are inside it however
	// long a loaded test machine takes between them.
	previousQuiescence := velocityQuiescence
	t.Cleanup(func() { velocityQuiescence = previousQuiescence })
	velocityQuiescence = 100 * time.Millisecond
	advance := knobTestClock(t)
	preview := &captureMidiWriter{}
	// A fast sweep: three detents, each inside both the fast window and the
	// quiescence the audition waits for.
	for range 3 {
		advance(10 * time.Millisecond)
		require.NoError(t, bank.AdjustVelocity(preview, EncoderLeft))
	}
	// The sweep landed on the hit minus 1 - 2 - 3, and nothing has been heard yet.
	event, _ := bank.CurrentPattern().EventAtStep(step, voice)
	require.Equal(t, hit-1-2-3, event.Velocity)
	require.Empty(t, preview.snapshot(), "a fast turn should not audition per detent")
	// Once the knob rests, the step is heard once, at the value it landed on.
	waitFor(t, "the held-back audition", func() bool { return len(preview.snapshot()) >= 2 })
	audition := preview.snapshot()
	require.Len(t, audition, 2)
	assertMidiData(t, audition[0], []byte{midi.MakeNoteOn(0), byte(note), byte(hit - 1 - 2 - 3)})
	assertMidiData(t, audition[1], []byte{midi.MakeNoteOff(0), byte(note), byte(hit - 1 - 2 - 3)})
}

// A turn slow enough to leave the quiescence gap between its detents still
// hears every detent, which is what keeps a careful turn audible while a
// fast one is not machine-gunned.
func TestVolumeKnobAuditionsASlowTurnPerDetent(t *testing.T) {
	const step, hit = 3, 100
	bank, _, note := percussionStepBank(t, step, hit)
	preview := &captureMidiWriter{}
	// Two detents a careful turn apart: each is heard on its own, a step
	// apart and a quiescence after its own turn. The first is waited for
	// before the second is sent, so a slow machine cannot make the second
	// detent take the first's audition with it.
	require.NoError(t, bank.AdjustVelocity(preview, EncoderLeft))
	waitFor(t, "the first audition", func() bool { return len(preview.snapshot()) >= 2 })
	time.Sleep(velocityFastWindow + 10*time.Millisecond)
	require.NoError(t, bank.AdjustVelocity(preview, EncoderLeft))
	waitFor(t, "the second audition", func() bool { return len(preview.snapshot()) >= 4 })
	audition := preview.snapshot()
	require.Len(t, audition, 4)
	assertMidiData(t, audition[0], []byte{midi.MakeNoteOn(0), byte(note), byte(hit - velocityStep)})
	assertMidiData(t, audition[1], []byte{midi.MakeNoteOff(0), byte(note), byte(hit - velocityStep)})
	assertMidiData(t, audition[2], []byte{midi.MakeNoteOn(0), byte(note), byte(hit - 2*velocityStep)})
	assertMidiData(t, audition[3], []byte{midi.MakeNoteOff(0), byte(note), byte(hit - 2*velocityStep)})
}
