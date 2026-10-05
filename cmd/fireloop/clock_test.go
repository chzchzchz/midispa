package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

// These tests run the real sequencer loop against a clock that is not the wall clock. The
// loop is the thing being described: what it waits for, in what order, and what it does when
// the set changes under it. Playing it for real would take minutes and would still not say
// which wait belonged to which step.

// virtualStepClock plays time forward by exactly what each wait asks for and records every
// request. The budget is how many waits it honours before reporting that playback should
// stop, which is how a test ends the loop without cancelling it from another goroutine.
// Advancing exactly, rather than by an arbitrary tick, is what keeps the drift the loop takes
// back at a pattern boundary visible: a boundary wait is the remainder of the pattern
// measured from the anchor, so it only comes out even if the clock is honest.
type virtualStepClock struct {
	now     time.Time
	waits   []time.Duration
	budget  int
	stopped bool
}

func (c *virtualStepClock) Now() time.Time {
	return c.now
}

func (c *virtualStepClock) Wait(ctx context.Context, d time.Duration) bool {
	if ctx.Err() != nil || len(c.waits) >= c.budget {
		return false
	}
	c.waits = append(c.waits, d)
	c.now = c.now.Add(d)
	return true
}

func (c *virtualStepClock) Stop() {
	c.stopped = true
}

// elapsed is the time the loop believes it spent, which is what a boundary wait is taken
// back from.
func (c *virtualStepClock) elapsed() time.Duration {
	return c.now.Sub(time.Time{})
}

func useTestBPM(t *testing.T, bpm int) {
	t.Helper()
	previous := currentBPM()
	t.Cleanup(func() { setBPM(previous) })
	setBPM(bpm)
}

// timingPattern is a pattern of the given length with a voice on step 0, which is the
// smallest thing that plays: the note marks where a measure begins and the length decides
// how many steps the loop takes to get back there.
func timingPattern(t *testing.T, steps int) (*Pattern, *Voice) {
	t.Helper()
	note := 36
	voice, _ := chromaticTestVoice(t, &note)
	pattern := &Pattern{}
	pattern.SetLengthSteps(steps)
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(0), Velocity: 100})
	return pattern, voice
}

// velocitiesIn reads back the hits a percussive event wrote, which is how a test counts the
// times a step sounded without counting the note-off that comes with each one.
func velocitiesIn(events []alsa.SeqEvent) []int {
	var velocities []int
	for _, event := range events {
		if midi.Message(event.Data[0]) == midi.NoteOn {
			velocities = append(velocities, int(event.Data[2]))
		}
	}
	return velocities
}

// A pattern of any length is walked one step at a time and handed back at its boundary, so
// a measure of N steps is N waits of one sixteenth each. The boundary wait is the interesting
// one: it is what is left of the pattern's duration once the steps inside it have been
// waited out, and it is the only place the loop takes back the drift it picked up.
func TestPlaybackWaitsOneStepPerPatternStep(t *testing.T) {
	useTestBPM(t, 120)
	sixteenth := beatDuration(120) / patternStepsPerBeat
	for _, steps := range []int{4, 8, maxPatternSteps} {
		t.Run(fmt.Sprintf("%d steps", steps), func(t *testing.T) {
			pattern, _ := timingPattern(t, steps)
			writer := &captureMidiWriter{}
			clock := &virtualStepClock{budget: 2 * steps}
			playback := &Playback{nextPattern: func(float32) *Pattern { return pattern }}
			require.NoError(t, playback.run(context.Background(), writer, clock))
			require.Lenf(t, clock.waits, 2*steps, "%d steps over two measures", steps)
			for i, wait := range clock.waits {
				require.Equalf(t, sixteenth, wait, "wait %d", i)
			}
			require.Equal(t, 2*patternDuration(pattern, 120), clock.elapsed(), "two measures")
			require.True(t, clock.stopped, "the loop left its clock running")
		})
	}
}

// Shortening a pattern while it plays moves the playhead past the new end, and the loop's
// answer is to rewind to the start of that pattern rather than to carry on past it or to
// stop. This is reached by shortening at a known step, because a worker cannot be raced to a
// moment reliably.
func TestPlaybackRewindsWhenAPatternIsShortenedLive(t *testing.T) {
	useTestBPM(t, 120)
	pattern, voice := timingPattern(t, maxPatternSteps)
	// A second hit halfway through, at its own velocity, is the step the test shortens on.
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(8), Velocity: 70})
	writer := &captureMidiWriter{}
	writer.onWrite = func(event alsa.SeqEvent) {
		if midi.Message(event.Data[0]) == midi.NoteOn && event.Data[2] == 70 {
			pattern.SetLengthSteps(4)
		}
	}
	// Long enough to reach step 8 and play the step the rewind returns to.
	clock := &virtualStepClock{budget: 10}
	playback := &Playback{nextPattern: func(float32) *Pattern { return pattern }}
	require.NoError(t, playback.run(context.Background(), writer, clock))
	// Step 0, step 8, then step 0 again: the rewind replayed the pattern's own start.
	want := []int{100, 70, 100}
	got := velocitiesIn(writer.snapshot())
	require.Len(t, got, len(want))
	for i, velocity := range want {
		require.Equalf(t, velocity, got[i], "hit %d", i)
	}
	// It carried on rather than sitting at the start. The position moves before the wait,
	// so the step after the one the budget ran out on still counts as taken: two steps
	// past the rewind, inside the shortened pattern rather than past its end.
	songBeat, patBeat, _ := playback.position()
	twoSteps := float32(2 * patternBeatsPerStep)
	require.Equal(t, twoSteps, songBeat, "song playhead")
	require.Equal(t, twoSteps, patBeat, "pattern playhead")
}

// A device that needs a dump loaded is not ready the moment its patch is sent, so the loop
// waits the settle out before it plays anything. It used to be a time.After of its own; on
// the injected clock it is now one more wait the loop asks for, in the right place, and
// that is the only thing that makes it checkable without a hardware clock.
func TestPlaybackSettlesBeforeItPlays(t *testing.T) {
	useTestBPM(t, 120)
	pattern, _ := timingPattern(t, 4)
	dir := t.TempDir()
	// A dump is the only patch that needs settling, and it needs a device behind it to
	// settle against.
	device := patchedDevice(dir, patchFile(t, dir, "dump.mid", patchDump), patchVoices(1)...)
	device.Settle = Settle(2 * time.Second)
	voiceBank := NewVoiceBank([]Device{device})
	require.NotNil(t, voiceBank.voices[0], "the kit has no voice")
	writer := &captureMidiWriter{}
	playback := &Playback{
		nextPattern: func(float32) *Pattern { return pattern },
		vb:          voiceBank,
		settle:      2 * time.Second,
	}
	clock := &virtualStepClock{budget: 3}
	require.NoError(t, playback.run(context.Background(), writer, clock))
	// The settle is waited first and on its own, then the steps follow at their own length:
	// had it been folded into a step, the first wait would not be the whole settle.
	want := []time.Duration{2 * time.Second, 125 * time.Millisecond, 125 * time.Millisecond}
	require.Len(t, clock.waits, len(want))
	for i, w := range want {
		require.Equalf(t, w, clock.waits[i], "wait %d", i)
	}
	require.NotEmpty(t, writer.events, "nothing played after the settle")
}

// A seek asked for while a pattern is playing waits for that pattern to end. Applying it
// where it was asked for would cut the pattern short mid-bar, which is the difference
// between moving the playhead and moving the music.
func TestJumpSongBeatTakesEffectAtTheNextPatternBoundary(t *testing.T) {
	useTestBPM(t, 120)
	pattern, _ := timingPattern(t, maxPatternSteps)
	var lookups []float32
	writer := &captureMidiWriter{}
	playback := &Playback{
		nextPattern: func(beat float32) *Pattern {
			lookups = append(lookups, beat)
			return pattern
		},
	}
	// Two beats into a four beat pattern, so there is a boundary left to wait for.
	playback.setPosition(2, 2)
	require.Equal(t, float32(2), playback.JumpSongBeat(8), "the beat the seek was made from")
	songBeat, patBeat, requested := playback.position()
	require.Equal(t, float32(2), songBeat, "the seek moved the song playhead")
	require.Equal(t, float32(2), patBeat, "the seek moved the pattern playhead")
	require.Equal(t, float32(8), requested, "the beat the seek was scheduled for")
	clock := &virtualStepClock{budget: 8}
	require.NoError(t, playback.run(context.Background(), writer, clock))
	// The loop asks its source for a pattern at the start, and then again for the beat the
	// seek asked for. Anything else means the seek was applied somewhere else.
	require.Equal(t, []float32{0, 8}, lookups, "pattern lookups")
	songBeat, _, _ = playback.position()
	require.Equal(t, float32(8+patternBeatsPerStep), songBeat, "playhead after the seek landed")
}
