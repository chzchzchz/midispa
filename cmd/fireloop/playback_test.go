package main

import (
	"errors"
	"testing"
	"time"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/alsa/fake"
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
	session := newPlaybackSession(&Playback{})
	session.Start(&failingSequencerWriter{portErr: expected})
	require.ErrorIs(t, session.Stop(), expected)
}

func TestPlaybackReturnsEventErrorAndStops(t *testing.T) {
	note := 36
	voice := &Voice{Note: &note, Channel: 1}
	pattern := newPattern([]Event{{Voice: voice, Beat: 0, Velocity: 100}})
	expected := errors.New("event write failed")
	writer := &failingSequencerWriter{err: expected}
	playback := &Playback{nextPattern: func(float32) *Pattern { return pattern }}
	session := newPlaybackSession(playback)
	session.Start(writer)
	require.ErrorIs(t, session.Stop(), expected)
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
	controller.playback = stubSession(playback, nil)
	controller.stopPlayback()
	require.Zero(t, playback.activeNoteCount(), "stop left an active chromatic note")
	require.Len(t, writer.events, 2, "stop writes a note-on and a note-off")
	assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOff(0), 72, 0})
}

// Two devices that happen to share a channel number are two places a note can be left, so
// the sweep has to name the port as well as the channel. The other case is the reverse: one
// device with four voices on one channel is one place, and silencing it four times would be
// four messages where one does the same work.
func TestSilenceTargetsAreDistinctPlaces(t *testing.T) {
	kick := 36
	shared := alsa.SeqAddr{Client: 10, Port: 20}
	voiceBank := NewVoiceBank([]Device{
		{
			Name: "drums", MidiPort: "a", Channel: 1, SeqAddr: shared,
			Voices: []Voice{{Name: "one", Note: &kick}, {Name: "two", Note: &kick}},
		},
		{
			// The same channel on a different port, which is a kit author deliberately
			// layering two devices rather than a mistake.
			Name: "layer", MidiPort: "b", Channel: 1, SeqAddr: alsa.SeqAddr{Client: 10, Port: 21},
			Voices: []Voice{{Name: "three", Note: &kick}},
		},
		{Name: "lead", MidiPort: "c", Channel: 3, SeqAddr: alsa.SeqAddr{Client: 10, Port: 22},
			Voices: []Voice{{Name: "four"}, {Name: "five", Channel: 4}}},
	})
	playback := &Playback{vb: voiceBank}

	targets := playback.silenceTargets()

	require.ElementsMatch(t, []silenceTarget{
		{channel: 1, destination: shared},
		{channel: 1, destination: alsa.SeqAddr{Client: 10, Port: 21}},
		{channel: 3, destination: alsa.SeqAddr{Client: 10, Port: 22}},
		{channel: 4, destination: alsa.SeqAddr{Client: 10, Port: 22}},
	}, targets, "one message per place a note can be left, not one per voice")
}

// The set ending is the one moment everything has to be silent, and the release built from
// the sequencer's own record cannot get there alone: only chromatic notes are recorded, so a
// drum and anything written outside the worker outlive it. The sweep goes to every channel
// the kit plays on, because a note the sequencer never tracked has no other address.
//
// The messages are pinned as exactly one All Notes Off per channel. Sending All Sound Off
// beside it is deliberately not wanted, and asserting the count rather than searching the
// capture is what makes an extra one a failure rather than a silent addition.
func TestSilenceAllReachesEveryKitChannel(t *testing.T) {
	writer := &captureMidiWriter{}
	voiceBank := NewVoiceBank([]Device{
		{Name: "drums", MidiPort: "a", Channel: 10, SeqAddr: alsa.SeqAddr{Client: 10, Port: 20},
			Voices: []Voice{{Name: "kick", Note: testNote(36)}}},
		{Name: "lead", MidiPort: "b", Channel: 1, SeqAddr: alsa.SeqAddr{Client: 11, Port: 20},
			Voices: []Voice{{Name: "one"}, {Name: "two", Channel: 2}}},
	})
	playback := &Playback{vb: voiceBank}

	require.NoError(t, playback.silenceAll(writer))

	silence := func(protocolChannel, controller int) []byte {
		return []byte{midi.MakeCC(protocolChannel), byte(controller), 0}
	}
	require.Len(t, writer.events, 3, "one all-notes-off per channel the kit plays on")
	assertMidiData(t, writer.events[0], silence(9, midi.AllNotesOff))
	assertMidiData(t, writer.events[1], silence(0, midi.AllNotesOff))
	assertMidiData(t, writer.events[2], silence(1, midi.AllNotesOff))
	require.Equal(t, alsa.SeqAddr{Client: 10, Port: 20}, writer.events[0].SeqAddr)
	require.Equal(t, alsa.SeqAddr{Client: 11, Port: 20}, writer.events[1].SeqAddr)
	require.Equal(t, alsa.SeqAddr{Client: 11, Port: 20}, writer.events[2].SeqAddr)
}

// A client that never opened has nothing to sweep, and a playback built without a kit has
// no channels to sweep. Both ways of being absent have to be silent rather than a write to
// a sequencer that is not there, which is the same rule every other write path follows.
func TestSilenceAllIgnoresAnAbsentSequencer(t *testing.T) {
	var unopened *alsa.Seq
	playback := &Playback{vb: NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{{Name: "lead"}}}})}

	require.NoError(t, playback.silenceAll(unopened))
	require.NoError(t, playback.silenceAll(nil))
	writer := &captureMidiWriter{}
	require.NoError(t, (&Playback{}).silenceAll(writer))
	require.Empty(t, writer.events, "a playback with no kit has no channel to silence")
}

// The guarantee as a player meets it: a set is playing, the process is asked to stop the way
// ctrl+c does, and nothing is left sounding. The drum is the interesting one, because the
// sequencer never records a percussive note, so the note-off below can only be arriving from
// the sweep.
func TestLeavingSilencesEverythingTheSetWasPlaying(t *testing.T) {
	kick, crash := 36, 49
	voiceBank := NewVoiceBank([]Device{
		{Name: "drums", MidiPort: "a", Channel: 10, SeqAddr: alsa.SeqAddr{Client: 10, Port: 20},
			Voices: []Voice{{Name: "kick", Note: &kick}, {Name: "crash", Note: &crash}}},
		{Name: "lead", MidiPort: "b", Channel: 1, SeqAddr: alsa.SeqAddr{Client: 11, Port: 20},
			Voices: []Voice{{Name: "lead"}}},
	})
	controller := useController(t, NewFire(func([]byte) error { return nil }), voiceBank)
	bank := controller.patbank
	pattern := bank.CurrentPattern()
	// The drums are spread across the window so both tracks carry a hit.
	require.NoError(t, bank.SelectTrackRow(1))
	pattern.ToggleEvent(Event{Voice: voiceBank.voices[0], Beat: stepBeat(0), Velocity: 100})
	require.NoError(t, bank.SelectTrackRow(2))
	pattern.ToggleEvent(Event{Voice: voiceBank.voices[1], Beat: stepBeat(0), Velocity: 100})
	require.NoError(t, bank.SelectTrackRow(3))
	pattern.SetChromaticNote(0, voiceBank.voices[2], 60, 100)

	instrument := newSoundingInstrument()
	controller.startPlayback(instrument, bank.newPlayback())
	t.Cleanup(func() { _ = controller.stopPlayback() })
	// Wait for the first step to land on every track, so the test stops a set that is
	// genuinely sounding rather than one that has not begun.
	deadline := time.Now().Add(2 * time.Second)
	for len(instrument.stillSounding()) < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	require.Len(t, instrument.stillSounding(), 3, "the set should have three notes sounding")

	require.NoError(t, shutdown(controller, fake.New()))

	require.Emptyf(t, instrument.stillSounding(),
		"leaving left notes sounding; a note-off was only ever written for the chromatic one")
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
