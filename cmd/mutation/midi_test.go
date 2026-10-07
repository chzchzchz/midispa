package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/track"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingMIDIWriter struct {
	messages [][]byte
	failAt   int
	failOnce bool
}

func (writer *recordingMIDIWriter) Write(message []byte) (int, error) {
	if writer.failAt == len(writer.messages) {
		if writer.failOnce {
			writer.failAt = -1
		}
		return 0, errors.New("write failed")
	}
	writtenMessage := append([]byte(nil), message...)
	writer.messages = append(writer.messages, writtenMessage)
	return len(message), nil
}

type alwaysFailingMIDIWriter struct {
	writes int
}

func (writer *alwaysFailingMIDIWriter) Write([]byte) (int, error) {
	writer.writes++
	return 0, errors.New("write failed")
}

func newFakeMIDIPlayer(writer *recordingMIDIWriter) (*midiPlayer, *time.Time, *[]time.Duration) {
	return newFakeMIDIPlayerForChannel(writer, defaultMIDIChannelNumber)
}

func newFakeMIDIPlayerForChannel(writer *recordingMIDIWriter, channelNumber int) (*midiPlayer, *time.Time, *[]time.Duration) {
	player := newMIDIPlayerForChannel(writer, channelNumber)
	now := time.Time{}
	sleeps := make([]time.Duration, 0)
	player.now = func() time.Time {
		return now
	}
	player.waitFor = func(_ context.Context, duration time.Duration) error {
		sleeps = append(sleeps, duration)
		now = now.Add(duration)
		return nil
	}
	return player, &now, &sleeps
}

func TestMIDIPlayerPlaysProbeDurations(t *testing.T) {
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	require.NoError(t, player.playProbeNotes(context.Background()), "playProbeNotes")

	var wantSleeps []time.Duration
	var wantMessages [][]byte
	for index, step := range probeMelody {
		wantSleeps = append(wantSleeps, step.duration)
		wantMessages = append(wantMessages,
			[]byte{0x90, probeNote, step.velocity},
			[]byte{0x80, probeNote, 0},
		)
		if index+1 < len(probeMelody) {
			wantSleeps = append(wantSleeps, noteGap)
		}
	}
	assert.Equal(t, wantSleeps, *sleeps)
	assert.Equal(t, wantMessages, writer.messages)
}

func TestMIDIPlayerSchedulesPlaybackFile(t *testing.T) {
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, sleeps := newFakeMIDIPlayer(writer)
	pattern := &track.Pattern{
		MidiTimeSig: track.MidiTimeSig{TicksPerBeat: 100, BPM: 120},
		LastTick:    4,
		Msgs: []track.TickMessage{
			{Raw: []byte{0x90, 60, 100}, Tick: 0},
			{Raw: []byte{0x80, 60, 0}, Tick: 2},
		},
	}
	require.NoError(t, player.playPattern(context.Background(), pattern), "playPattern")
	assert.Equal(t, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}, *sleeps)
	require.Len(t, writer.messages, 2, "unexpected played messages")
	assert.Equal(t, pattern.Msgs[0].Raw, writer.messages[0], "unexpected played messages")
	assert.Equal(t, pattern.Msgs[1].Raw, writer.messages[1], "unexpected played messages")
}

func TestMIDIPlayerSendsPatchBeforeProbe(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	for index := range patch.genes {
		patch.genes[index].value = index
	}
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, _ := newFakeMIDIPlayer(writer)
	require.NoError(t, player.audition(context.Background(), patch, nil), "audition")
	cleanup := [][]byte{{0xb0, sustainController, 0}, {0xb0, midi.AllNotesOff, 0}}
	probeMessages := 2 * len(probeMelody)
	require.Len(t, writer.messages, len(patch.genes)+probeMessages+2*len(cleanup), "audition sent the wrong number of messages")
	for index, message := range writer.messages[2 : 2+len(patch.genes)] {
		assert.True(t, midi.IsCC(message[0]), "message %d is not CC: %v", index, message)
	}
	assert.True(t, midi.IsNoteOn(writer.messages[2+len(patch.genes)][0]), "probe note did not follow patch CCs")
	assert.Equal(t, cleanup, writer.messages[:2], "audition did not start from a safe state")
	assert.Equal(t, cleanup, writer.messages[len(writer.messages)-2:], "audition was not safely reset")
}

func TestMIDIPlayerUsesConfiguredChannel(t *testing.T) {
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, _ := newFakeMIDIPlayerForChannel(writer, 10)
	patch := newTestPatch(t, "Sound Controller")
	require.NoError(t, player.audition(context.Background(), patch, nil), "audition")
	for index, message := range writer.messages {
		assert.Equal(t, 9, midi.Channel(message[0]), "message %d does not use MIDI channel 10", index)
	}
}

func TestMIDIPlayerCleansUpAfterProbeFailure(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	writer := &recordingMIDIWriter{
		failAt:   2 + len(patch.genes) + 1,
		failOnce: true,
	}
	player, _, _ := newFakeMIDIPlayer(writer)
	assert.Error(t, player.audition(context.Background(), patch, nil), "audition ignored note-off write failure")
	cleanup := [][]byte{{0xb0, sustainController, 0}, {0xb0, midi.AllNotesOff, 0}}
	require.GreaterOrEqual(t, len(writer.messages), len(cleanup), "failed audition was not cleaned up")
	assert.Equal(t, cleanup, writer.messages[len(writer.messages)-len(cleanup):], "failed audition was not cleaned up")
}

func TestMIDIPlayerAttemptsEveryCleanupMessageAfterFailure(t *testing.T) {
	writer := &alwaysFailingMIDIWriter{}
	player := newMIDIPlayerForChannel(writer, 1)
	assert.Error(t, player.resetChannels([]int{0, 15}), "resetChannels ignored write failures")
	assert.Equal(t, 4, writer.writes, "cleanup did not attempt every message")
}

func TestMIDIPlayerWaitCanBeCanceled(t *testing.T) {
	player := newMIDIPlayerForChannel(&recordingMIDIWriter{failAt: -1}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- player.waitFor(ctx, time.Hour)
	}()
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled, "wait did not stop after cancellation")
	case <-time.After(time.Second):
		require.Fail(t, "wait did not stop after cancellation")
	}
}

func TestMIDIPlayerCleansPlaybackChannels(t *testing.T) {
	player := newMIDIPlayerForChannel(&recordingMIDIWriter{failAt: -1}, 1)
	playback := &track.Pattern{Msgs: []track.TickMessage{
		{Raw: []byte{0x95, 60, 100}},
		{Raw: []byte{0x85, 60, 0}},
		{Raw: []byte{0xb6, 1, 2}},
	}}
	assert.Equal(t, []int{0, 5}, player.auditionChannels(playback))
}

func TestMIDIPlayerRejectsInvalidPatternTiming(t *testing.T) {
	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{})
	assert.Error(t, player.playPattern(context.Background(), &track.Pattern{}), "accepted playback without timing metadata")
}

func TestMIDIPlayerPropagatesWriteError(t *testing.T) {
	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: 0})
	assert.Error(t, player.playProbeNotes(context.Background()), "ignored MIDI output error")
}

func TestOpenMIDIOutputRejectsEmptyPort(t *testing.T) {
	_, _, err := openMIDIOutput("")
	assert.Error(t, err, "accepted an empty port name")
}
