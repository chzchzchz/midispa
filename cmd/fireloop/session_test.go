package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A set that ended on its own is not playing, even though its session is still installed.
// That is the whole reason a session is asked rather than the field being read for nil: the
// guards used to see a handle that said otherwise, so the next Play did nothing because it
// believed a set was already running.
func TestASetThatEndedOnItsOwnIsNoLongerPlaying(t *testing.T) {
	bank, _ := chromaBank(t)
	controller := bank.controller
	controller.playback = stubSession(bank.newPlayback(), nil)

	require.NotNil(t, controller.playback, "the session was replaced without being asked to be")
	require.False(t, controller.playing(), "a set that has ended still reads as playing")

	// So pressing Play starts a new one instead of deciding there was already one there.
	writer := &captureMidiWriter{}
	controller.startPlayback(writer, bank.newPlayback())
	require.True(t, controller.playing(), "Play did not start a set")
	require.NoError(t, controller.stopPlayback())
	require.False(t, controller.playing(), "Stop left the set playing")
}

// The consequence that is audible: a pad press is not auditioned while a set is running, so
// a handle left behind by a set that had already ended would silence the instrument until
// something stopped it. Nothing has to stop this one, because it is not running.
func TestAnEndedSetDoesNotSilenceTheAudition(t *testing.T) {
	controller := useController(t, NewFire(func([]byte) error { return nil }), trackWindowKit(8, -1))
	bank := controller.patbank
	require.NoError(t, bank.SelectTrackRow(1))
	controller.playback = stubSession(bank.newPlayback(), nil)

	writer := &captureMidiWriter{}
	require.NoError(t, controller.handlePatternGrid(writer, 3, 0, 100))

	require.NotEmpty(t, writer.events, "a set that had already ended silenced the pad press")
}

// A measure seek with nothing playing schedules nothing. The gesture is not an error and it
// reports nothing, which is what it did before the seek moved behind a session.
func TestAMeasureSeekWithNothingPlayingSchedulesNothing(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(4, 0))
	songs := NewSongBank(NewFire(func([]byte) error { return nil }), bank)
	arrangement := &Song{}
	arrangement.SetPattern(bank.CurrentPattern(), 0)
	songs.installSongs(map[int]*Song{1: arrangement})
	require.Nil(t, bank.controller.playback)

	require.NoError(t, songs.JumpMeasure(0, 0))
	require.Nil(t, bank.controller.playback, "a seek started a set")
}

// Stopping is asked from both ends of an edit: a handler stops before it changes anything
// the worker reads, and the method it calls may stop again. A second stop has to be
// harmless rather than a second release or a complaint about a set that has already gone.
func TestStoppingTwiceIsHarmless(t *testing.T) {
	bank, kit := quietBank(t, trackWindowKit(4, 0))
	controller := bank.controller
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	require.NoError(t, playback.playChromaticEvent(writer, Event{
		Voice: kit.voices[0], ChromaticNote: 60, Velocity: 100,
	}))
	require.Equal(t, 1, playback.activeNoteCount(), "the test needs a note sounding")

	stops := 0
	controller.playback = stubSession(playback, func() error {
		stops++
		return nil
	})

	require.NoError(t, controller.stopPlayback())
	require.NoError(t, controller.stopPlayback())
	require.Equal(t, 1, stops, "the handle was called more than once")
	require.Zero(t, playback.activeNoteCount(), "the note was left sounding")
	require.Len(t, writer.events, 2, "the note-off should be written once")
}
