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

// A percussive step plays back with the dynamics it holds, which is the whole point of
// letting the knob set them: a soft hit and a hard hit must not sound the same.
func TestPercussionPlaybackHonoursTheStoredVelocity(t *testing.T) {
	note := 36
	voice, _ := chromaticTestVoice(t, &note)
	pattern := &Pattern{}
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(0), Velocity: 40})
	pattern.ToggleEvent(Event{Voice: voice, Beat: stepBeat(4), Velocity: 100})
	writer := &captureMidiWriter{}
	playback := &Playback{}
	playback.setPosition(0, 0)
	if _, err := playback.playBeat(writer, pattern); err != nil {
		t.Fatal(err)
	}
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), byte(note), 40})
	writer.events = nil
	playback.setPosition(stepBeat(4), stepBeat(4))
	if _, err := playback.playBeat(writer, pattern); err != nil {
		t.Fatal(err)
	}
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), byte(note), 100})
}

func TestChromaticMIDIOrderingAndCleanup(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	writer := &captureMidiWriter{}
	playback := &Playback{}
	first := Event{Voice: voice, ChromaticNote: 60, Velocity: 77, Tie: true}
	if err := playback.playChromaticEvent(writer, first); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("first event wrote %d messages, want 1", len(writer.events))
	}
	assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 77})
	second := Event{Voice: voice, ChromaticNote: 60, Velocity: 78, Tie: true}
	if err := playback.playChromaticEvent(writer, second); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) != 1 {
		t.Fatalf("same-pitch tie wrote %d messages, want 1", len(writer.events))
	}
	third := Event{Voice: voice, ChromaticNote: 62, Velocity: 79, Tie: true}
	if err := playback.playChromaticEvent(writer, third); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) != 3 {
		t.Fatalf("different-pitch tie wrote %d messages, want 3", len(writer.events))
	}
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), 62, 79})
	assertMidiData(t, writer.events[2], []byte{midi.MakeNoteOff(0), 60, 0})
	if err := playback.releaseAll(writer); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) != 4 {
		t.Fatalf("cleanup wrote %d messages, want 4", len(writer.events))
	}
	assertMidiData(t, writer.events[3], []byte{midi.MakeNoteOff(0), 62, 0})

	percussionNote := 60
	percussion, _ := chromaticTestVoice(t, &percussionNote)
	percussionEvent := Event{Voice: percussion, Velocity: 55}
	messages := percussionEvent.ToMidi()
	if len(messages) != 2 {
		t.Fatalf("percussion ToMidi returned %d messages, want 2", len(messages))
	}
	assertMidiData(t, alsa.SeqEvent{Data: messages[0]}, []byte{midi.MakeNoteOff(0), 60, 55})
	assertMidiData(t, alsa.SeqEvent{Data: messages[1]}, []byte{midi.MakeNoteOn(0), 60, 55})
}

func TestMixedPercussiveAndChromaticPlayback(t *testing.T) {
	percussionNote := 36
	device := Device{
		Name:     "mixed",
		MidiPort: "out",
		Channel:  1,
		Voices: []Voice{
			{Name: "lead"},
			{Name: "kick", Note: &percussionNote},
		},
		SeqAddr: alsa.SeqAddr{Client: 10, Port: 20},
	}
	voiceBank := NewVoiceBank([]Device{device})
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voiceBank.voices[0], 60, 91)
	pattern.ToggleEvent(Event{Voice: voiceBank.voices[1], Beat: 0, Velocity: 100})
	writer := &captureMidiWriter{}
	playback := &Playback{}
	playback.setPosition(0, 0)
	if _, err := playback.playBeat(writer, pattern); err != nil {
		t.Fatal(err)
	}
	if len(writer.events) != 3 {
		t.Fatalf("mixed pattern wrote %d messages, want 3", len(writer.events))
	}
	var chromaticOn, percussionOff, percussionOn bool
	for _, event := range writer.events {
		switch midi.Message(event.Data[0]) {
		case midi.NoteOn:
			if event.Data[1] == 60 {
				chromaticOn = true
			} else {
				percussionOn = true
			}
		case midi.NoteOff:
			percussionOff = true
		}
	}
	if !chromaticOn || !percussionOff || !percussionOn {
		t.Fatalf("mixed MIDI types missing: chromatic=%v off=%v on=%v", chromaticOn, percussionOff, percussionOn)
	}
}

func TestChromaticPlaybackCleanupOnStop(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	if err := playback.playChromaticEvent(writer, Event{Voice: voice, ChromaticNote: 72, Velocity: 88}); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank, previousCancel := patbank, songbank, playbackStop
	t.Cleanup(func() {
		patbank, songbank, playbackStop = previousPatbank, previousSongbank, previousCancel
	})
	patbank = &PatternBank{playback: playback}
	songbank = nil
	playbackStop = nil
	stopPlayback()
	if playback.activeNoteCount() != 0 {
		t.Fatal("stop left an active chromatic note")
	}
	if len(writer.events) != 2 {
		t.Fatalf("stop wrote %d messages, want note-on and note-off", len(writer.events))
	}
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOff(0), 72, 0})
}
