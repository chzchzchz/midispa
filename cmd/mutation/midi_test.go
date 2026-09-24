package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/track"
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
	if err := player.playProbeNotes(context.Background()); err != nil {
		t.Fatalf("playProbeNotes: %v", err)
	}

	wantSleeps := []time.Duration{
		10 * time.Millisecond,
		500 * time.Millisecond,
		100 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
	}
	if !reflect.DeepEqual(*sleeps, wantSleeps) {
		t.Fatalf("sleeps are %v, want %v", *sleeps, wantSleeps)
	}
	wantMessages := [][]byte{
		{0x90, probeNote, probeVelocity},
		{0x80, probeNote, 0},
		{0x90, probeNote, probeVelocity},
		{0x80, probeNote, 0},
		{0x90, probeNote, probeVelocity},
		{0x80, probeNote, 0},
	}
	if !reflect.DeepEqual(writer.messages, wantMessages) {
		t.Fatalf("messages are %v, want %v", writer.messages, wantMessages)
	}
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
	if err := player.playPattern(context.Background(), pattern); err != nil {
		t.Fatalf("playPattern: %v", err)
	}
	wantSleeps := []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	if !reflect.DeepEqual(*sleeps, wantSleeps) {
		t.Fatalf("sleeps are %v, want %v", *sleeps, wantSleeps)
	}
	if len(writer.messages) != 2 || !reflect.DeepEqual(writer.messages[0], pattern.Msgs[0].Raw) || !reflect.DeepEqual(writer.messages[1], pattern.Msgs[1].Raw) {
		t.Fatalf("unexpected played messages: %v", writer.messages)
	}
}

func TestMIDIPlayerSendsPatchBeforeProbe(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	for index := range patch.genes {
		patch.genes[index].value = index
	}
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, _ := newFakeMIDIPlayer(writer)
	if err := player.audition(context.Background(), patch, nil); err != nil {
		t.Fatalf("audition: %v", err)
	}
	if len(writer.messages) != len(patch.genes)+10 {
		t.Fatalf("sent %d messages, want %d", len(writer.messages), len(patch.genes)+10)
	}
	for index, message := range writer.messages[2 : 2+len(patch.genes)] {
		if !midi.IsCC(message[0]) {
			t.Fatalf("message %d is not CC: %v", index, message)
		}
	}
	if !midi.IsNoteOn(writer.messages[2+len(patch.genes)][0]) {
		t.Fatal("probe note did not follow patch CCs")
	}
	cleanup := [][]byte{{0xb0, sustainController, 0}, {0xb0, midi.AllNotesOff, 0}}
	if !reflect.DeepEqual(writer.messages[:2], cleanup) || !reflect.DeepEqual(writer.messages[len(writer.messages)-2:], cleanup) {
		t.Fatalf("audition was not safely reset: %v", writer.messages)
	}
}

func TestMIDIPlayerUsesConfiguredChannel(t *testing.T) {
	writer := &recordingMIDIWriter{failAt: -1}
	player, _, _ := newFakeMIDIPlayerForChannel(writer, 10)
	patch := newTestPatch(t, "Sound Controller")
	if err := player.audition(context.Background(), patch, nil); err != nil {
		t.Fatalf("audition: %v", err)
	}
	for index, message := range writer.messages {
		if channel := midi.Channel(message[0]); channel != 9 {
			t.Fatalf("message %d uses MIDI channel %d, want 10", index, channel+1)
		}
	}
}

func TestMIDIPlayerCleansUpAfterProbeFailure(t *testing.T) {
	patch := newTestPatch(t, "Sound Controller")
	writer := &recordingMIDIWriter{
		failAt:   2 + len(patch.genes) + 1,
		failOnce: true,
	}
	player, _, _ := newFakeMIDIPlayer(writer)
	if err := player.audition(context.Background(), patch, nil); err == nil {
		t.Fatal("audition ignored note-off write failure")
	}
	cleanup := [][]byte{{0xb0, sustainController, 0}, {0xb0, midi.AllNotesOff, 0}}
	if len(writer.messages) < len(cleanup) || !reflect.DeepEqual(writer.messages[len(writer.messages)-len(cleanup):], cleanup) {
		t.Fatalf("failed audition was not cleaned up: %v", writer.messages)
	}
}

func TestMIDIPlayerAttemptsEveryCleanupMessageAfterFailure(t *testing.T) {
	writer := &alwaysFailingMIDIWriter{}
	player := newMIDIPlayerForChannel(writer, 1)
	if err := player.resetChannels([]int{0, 15}); err == nil {
		t.Fatal("resetChannels ignored write failures")
	}
	if writer.writes != 4 {
		t.Fatalf("cleanup attempted %d writes, want 4", writer.writes)
	}
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
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wait returned %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wait did not stop after cancellation")
	}
}

func TestMIDIPlayerCleansPlaybackChannels(t *testing.T) {
	player := newMIDIPlayerForChannel(&recordingMIDIWriter{failAt: -1}, 1)
	playback := &track.Pattern{Msgs: []track.TickMessage{
		{Raw: []byte{0x95, 60, 100}},
		{Raw: []byte{0x85, 60, 0}},
		{Raw: []byte{0xb6, 1, 2}},
	}}
	if got, want := player.auditionChannels(playback), []int{0, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("audition channels are %v, want %v", got, want)
	}
}

func TestMIDIPlayerRejectsInvalidPatternTiming(t *testing.T) {
	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{})
	if err := player.playPattern(context.Background(), &track.Pattern{}); err == nil {
		t.Fatal("accepted playback without timing metadata")
	}
}

func TestMIDIPlayerPropagatesWriteError(t *testing.T) {
	player, _, _ := newFakeMIDIPlayer(&recordingMIDIWriter{failAt: 0})
	if err := player.playProbeNotes(context.Background()); err == nil {
		t.Fatal("ignored MIDI output error")
	}
}

func TestOpenMIDIOutputRejectsEmptyPort(t *testing.T) {
	if _, _, err := openMIDIOutput(""); err == nil {
		t.Fatal("accepted an empty port name")
	}
}
