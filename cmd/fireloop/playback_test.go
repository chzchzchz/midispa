package main

import (
	"errors"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
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
			require.Equal(t, tt.want, beatDuration(tt.bpm), "beatDuration(%d)", tt.bpm)
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

// Transport messages go out on the sync port rather than the default one, and a client
// that never opened has nothing to send them from. This is the one write path that does
// not go through writeMidiMsgs, so it is the one that has to answer for itself: both ways
// of being absent have to be silent rather than a write to a sequencer that is not there.
func TestWriteSequencerPort(t *testing.T) {
	var unopened *alsa.Seq
	for _, test := range []struct {
		name string
		aseq alsa.PortWriter
	}{
		{name: "nothing at all", aseq: nil},
		{name: "a sequencer that never opened", aseq: unopened},
	} {
		t.Run("ignores "+test.name, func(t *testing.T) {
			require.NoError(t, writeSequencerPort(test.aseq, []byte{midi.Start}))
		})
	}

	writer := &captureMidiWriter{}
	require.NoError(t, writeSequencerPort(writer, []byte{midi.Start}))
	require.Len(t, writer.events, 1, "wrote the wrong number of transport messages")
	assertMidiData(t, writer.events[0], []byte{midi.Start})
	require.Equal(t, alsa.SubsSeqAddr, writer.events[0].SeqAddr, "transport did not go to subscribers")
}

func TestPlaybackStopReturnsRunError(t *testing.T) {
	expected := errors.New("start failed")
	playback := &Playback{}
	stop := playback.start(&failingSequencerWriter{portErr: expected})
	require.ErrorIs(t, stop(), expected)
}

func TestPlaybackReturnsEventErrorAndStops(t *testing.T) {
	note := 36
	voice := &Voice{Note: &note, Channel: 1}
	pattern := &Pattern{Events: []Event{{Voice: voice, Beat: 0, Velocity: 100}}}
	expected := errors.New("event write failed")
	writer := &failingSequencerWriter{err: expected}
	playback := &Playback{nextPattern: func(float32) *Pattern { return pattern }}
	stop := playback.start(writer)
	require.ErrorIs(t, stop(), expected)
	require.Len(t, writer.port, 2, "the sync port takes a Start and a Stop")
	require.Equal(t, byte(midi.Start), writer.port[0].Data[0])
	require.Equal(t, byte(midi.Stop), writer.port[1].Data[0])
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
	require.Equal(t, 120, currentBPM())
}

func TestPatternDuration(t *testing.T) {
	short := &Pattern{}
	short.SetLengthSteps(4)
	long := &Pattern{}
	long.SetLengthSteps(8)
	require.Equal(t, 500*time.Millisecond, patternDuration(short, 120), "one-beat pattern duration")
	require.Equal(t, time.Second, patternDuration(long, 120), "two-beat pattern duration")
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
	_, err := playback.playBeat(writer, pattern)
	require.NoError(t, err)
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), byte(note), 40})
	writer.events = nil
	playback.setPosition(stepBeat(4), stepBeat(4))
	_, err = playback.playBeat(writer, pattern)
	require.NoError(t, err)
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), byte(note), 100})
}

func TestChromaticMIDIOrderingAndCleanup(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	writer := &captureMidiWriter{}
	playback := &Playback{}
	first := Event{Voice: voice, ChromaticNote: 60, Velocity: 77, Tie: true}
	require.NoError(t, playback.playChromaticEvent(writer, first))
	require.Len(t, writer.events, 1)
	assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 77})
	second := Event{Voice: voice, ChromaticNote: 60, Velocity: 78, Tie: true}
	require.NoError(t, playback.playChromaticEvent(writer, second))
	require.Len(t, writer.events, 1, "a same-pitch tie writes nothing more")
	third := Event{Voice: voice, ChromaticNote: 62, Velocity: 79, Tie: true}
	require.NoError(t, playback.playChromaticEvent(writer, third))
	require.Len(t, writer.events, 3, "a different-pitch tie retriggers")
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOn(0), 62, 79})
	assertMidiData(t, writer.events[2], []byte{midi.MakeNoteOff(0), 60, 0})
	require.NoError(t, playback.releaseAll(writer))
	require.Len(t, writer.events, 4, "cleanup releases the note left sounding")
	assertMidiData(t, writer.events[3], []byte{midi.MakeNoteOff(0), 62, 0})

	percussionNote := 60
	percussion, _ := chromaticTestVoice(t, &percussionNote)
	percussionEvent := Event{Voice: percussion, Velocity: 55}
	messages := percussionEvent.ToMidi()
	require.Len(t, messages, 2, "a percussive event is a note-off and a note-on")
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
	_, err := playback.playBeat(writer, pattern)
	require.NoError(t, err)
	require.Len(t, writer.events, 3)
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
	require.True(t, chromaticOn, "the chromatic note-on is missing")
	require.True(t, percussionOff, "the percussive note-off is missing")
	require.True(t, percussionOn, "the percussive note-on is missing")
}

func TestChromaticPlaybackCleanupOnStop(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	require.NoError(t, playback.playChromaticEvent(writer, Event{Voice: voice, ChromaticNote: 72, Velocity: 88}))
	controller := useController(t, NewFire(func([]byte) error { return nil }), stateKit())
	controller.patbank.playback = playback
	controller.stopPlayback()
	require.Zero(t, playback.activeNoteCount(), "stop left an active chromatic note")
	require.Len(t, writer.events, 2, "stop writes a note-on and a note-off")
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOff(0), 72, 0})
}

// Where the playhead lands next decides what the set sounds like, and the two rules that
// settle it pull in opposite directions: the step grid keeps the display moving even on an
// empty step, while a pattern's own event pulls the wait forward when it falls off the grid.
// Zero is the boundary, which is a different thing from the first step.
func TestNextEventBeat(t *testing.T) {
	step := float32(patternBeatsPerStep)
	tests := []struct {
		name         string
		patBeat      float32
		nextBeat     float32
		patternBeats float32
		want         float32
	}{
		{name: "an empty pattern still advances a step", patBeat: 0, patternBeats: 4, want: step},
		{name: "the grid keeps moving with nothing written", patBeat: step * 2, patternBeats: 4, want: step * 3},
		{name: "an event before the grid is not put off", patBeat: step * 2, nextBeat: step*2 + step/2, patternBeats: 4, want: step*2 + step/2},
		{name: "an event after the grid does not hold it up", patBeat: 0, nextBeat: step * 3, patternBeats: 4, want: step},
		{name: "the last step of a pattern is the boundary", patBeat: 3.75, patternBeats: 4, want: 0},
		{name: "a shortened pattern ends mid grid", patBeat: 0.375, patternBeats: 0.5, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, nextEventBeat(tt.patBeat, tt.nextBeat, tt.patternBeats),
				"nextEventBeat(%v, %v, %v)", tt.patBeat, tt.nextBeat, tt.patternBeats)
		})
	}
}
