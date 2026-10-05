package main

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
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
		require.Lenf(t, p.FindBeat(tt.beat), tt.evs, "test#%d", i)
	}
}

func TestFindBeatReturnsSnapshot(t *testing.T) {
	p := Pattern{Events: []Event{{Beat: 1, Velocity: 10}}}
	found := p.FindBeat(0)
	require.Len(t, found, 1)
	found[0].Beat = 2
	found[0].Velocity = 20
	require.EqualValues(t, 1, p.Events[0].Beat, "editing the result changed the pattern")
	require.Equal(t, 10, p.Events[0].Velocity, "editing the result changed the pattern")
	p.ToggleEvent(Event{Beat: 1})
	require.Len(t, found, 1, "editing the pattern changed the length of the earlier result")
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
	_, took := pattern.SetChromaticNote(7, lead, 60, 90)
	require.True(t, took, "expected step 7 to take the note it just got")
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
	require.Zero(t, dropped, "loading dropped events")
	assertEventsInBeatOrder(t, "loaded from a file", fromFile)

	bank, _ := quietBank(t, kit)
	scrambled := &Pattern{Events: []Event{
		{Voice: lead, Beat: stepBeat(9), Velocity: 90},
		{Voice: snare, Beat: stepBeat(2), Velocity: 90},
		{Voice: lead, Beat: stepBeat(4), Velocity: 90},
	}}
	require.NoError(t, bank.SetPattern(scrambled))
	assertEventsInBeatOrder(t, "restored into the bank", bank.CurrentPattern())
}

// assertEventsInBeatOrder fails when a pattern's events are out of beat order, and then
// checks FindBeat against what the same lookup would return from a copy that was sorted the
// slow way, so a pattern that kept its order still cannot hand back the wrong window.
func assertEventsInBeatOrder(t *testing.T, what string, p *Pattern) {
	t.Helper()
	events, _ := p.snapshot()
	for i := 1; i < len(events); i++ {
		require.GreaterOrEqualf(t, events[i].Beat, events[i-1].Beat,
			"%s: event %d sits after the beat of the event before it", what, i)
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
		require.Lenf(t, got, len(want), "%s: FindBeat(%v)", what, beat)
		for i := range got {
			require.Equalf(t, want[i].Beat, got[i].Beat, "%s: FindBeat(%v) event %d", what, beat, i)
			require.Equalf(t, want[i].Voice, got[i].Voice, "%s: FindBeat(%v) event %d", what, beat, i)
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
	require.Equal(t, defaultPatternSteps, pattern.LengthSteps())
	require.Equal(t, 4, pattern.SetLengthSteps(4))
	require.Equal(t, 4, pattern.LengthSteps())
	require.EqualValues(t, 1, pattern.Beats())
	require.Len(t, pattern.Events, 2, "events after shortening")
	require.False(t, pattern.ToggleEvent(Event{Beat: 1}), "event at the pattern boundary was accepted")
	require.Equal(t, 1, pattern.SetLengthSteps(0), "the shortest pattern")
	require.Equal(t, maxPatternSteps, pattern.SetLengthSteps(17), "the longest pattern")
}

func TestSongUsesVariablePatternLengths(t *testing.T) {
	first := &Pattern{}
	first.SetLengthSteps(4)
	second := &Pattern{}
	second.SetLengthSteps(8)
	song := &Song{}
	song.SetPattern(first, 0)
	song.SetPattern(second, 1)
	require.EqualValues(t, 1, song.IndexToBeat(1), "where the second pattern starts, in beats")
	got, index := song.BeatToPattern(1.25)
	require.Same(t, second, got)
	require.Equal(t, 1, index)
}

func TestSetPatternCopiesEvents(t *testing.T) {
	vb := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "voice", Note: testNote(60), Channel: 1}},
	}})
	pb := useController(t, NewFire(func([]byte) error { return nil }), vb).patbank
	source := &Pattern{Events: []Event{{Beat: 1}}}
	source.SetLengthSteps(8)
	require.NoError(t, pb.SetPattern(source))
	source.Events[0].Beat = 2
	source.SetLengthSteps(4)
	require.EqualValues(t, 1, pb.Patterns[1].Events[0].Beat, "the pasted pattern changed with the source")
	require.Equal(t, 8, pb.Patterns[1].LengthSteps(), "the pasted pattern length")
}

func TestChromaticPatternEditingAndTieInvariants(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	first, ok := pattern.SetChromaticNote(0, voice, 60, 90)
	require.True(t, ok, "the first chromatic edit reported no event")
	require.Equal(t, 60, first.ChromaticNote)
	require.Equal(t, 90, first.Velocity)
	pattern.SetChromaticNote(1, voice, 62, 91)
	pattern.SetChromaticNote(2, voice, 64, 92)
	require.False(t, pattern.TieEventsAtSteps(0, 2, voice), "a tie crossed an intervening event")
	require.True(t, pattern.TieEventsAtSteps(1, 0, voice), "a reversed two-pad arrival did not create a tie")
	firstEvent, _ := pattern.EventAtStep(0, voice)
	require.True(t, firstEvent.Tie, "the tie was not stored on the earlier event")
	pattern.SetChromaticNote(0, voice, 67, 90)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	require.True(t, firstEvent.Tie, "a pitch edit lost the tie")
	require.Equal(t, 67, firstEvent.ChromaticNote)
	copyPattern := pattern.Copy()
	copyPattern.SetChromaticNote(0, voice, 69, 90)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	require.Equal(t, 67, firstEvent.ChromaticNote, "the copy shared its chromatic state")
	require.True(t, firstEvent.Tie, "the copy shared its chromatic state")
	pattern.RemoveEventAtStep(1, voice)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	require.False(t, firstEvent.Tie, "removing the target left a dangling tie")
	pattern.SetChromaticNote(1, voice, 62, 91)
	pattern.TieEventsAtSteps(0, 1, voice)
	pattern.SetLengthSteps(1)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	require.False(t, firstEvent.Tie, "shortening left a tie crossing the new end")
}
