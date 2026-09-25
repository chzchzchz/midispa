package main

import (
	"errors"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

func TestBeatDuration(t *testing.T) {
	tests := []struct {
		name string
		bpm  int
		want time.Duration
	}{
		{name: "sixty", bpm: 60, want: time.Second},
		{name: "one hundred twenty", bpm: 120, want: 500 * time.Millisecond},
		{name: "invalid", bpm: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := beatDuration(tt.bpm); got != tt.want {
				t.Fatalf("beatDuration(%d) = %s, want %s", tt.bpm, got, tt.want)
			}
		})
	}
}

type failingSequencerWriter struct {
	err     error
	portErr error
	port    []alsa.SeqEvent
}

func (w *failingSequencerWriter) Write(alsa.SeqEvent) error {
	return w.err
}

func (w *failingSequencerWriter) WritePort(event alsa.SeqEvent, _ int) error {
	w.port = append(w.port, event)
	return w.portErr
}

func TestPlaybackStopReturnsRunError(t *testing.T) {
	expected := errors.New("start failed")
	playback := &Playback{}
	stop := playback.start(&failingSequencerWriter{portErr: expected})
	if err := stop(); !errors.Is(err, expected) {
		t.Fatalf("stop error = %v, want %v", err, expected)
	}
}

func TestPlaybackReturnsEventErrorAndStops(t *testing.T) {
	note := 36
	voice := &Voice{Note: &note, Channel: 1}
	pattern := &Pattern{Events: []Event{{Voice: voice, Beat: 0, Velocity: 100}}}
	expected := errors.New("event write failed")
	writer := &failingSequencerWriter{err: expected}
	playback := &Playback{nextPattern: func(float32) *Pattern { return pattern }}
	stop := playback.start(writer)
	if err := stop(); !errors.Is(err, expected) {
		t.Fatalf("stop error = %v, want %v", err, expected)
	}
	if len(writer.port) != 2 || writer.port[0].Data[0] != midi.Start || writer.port[1].Data[0] != midi.Stop {
		t.Fatalf("sync messages = %v, want Start then Stop", writer.port)
	}
}

func TestBPMAtomicAccess(t *testing.T) {
	original := currentBPM()
	t.Cleanup(func() { setBPM(original) })
	setBPM(120)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			setBPM(120 + i%2)
		}
		setBPM(120)
	}()
	for i := 0; i < 1000; i++ {
		_ = currentBPM()
	}
	<-done
	if got := currentBPM(); got != 120 {
		t.Fatalf("BPM = %d, want 120", got)
	}
}

func TestPatternDuration(t *testing.T) {
	short := &Pattern{}
	short.SetLengthSteps(4)
	long := &Pattern{}
	long.SetLengthSteps(8)
	if got := patternDuration(short, 120); got != 500*time.Millisecond {
		t.Fatalf("one-beat pattern duration = %s, want 500ms", got)
	}
	if got := patternDuration(long, 120); got != time.Second {
		t.Fatalf("two-beat pattern duration = %s, want 1s", got)
	}
}
