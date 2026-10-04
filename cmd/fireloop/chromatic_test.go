package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// pressVelocity is the strength the tests hit a palette pad with.
const pressVelocity = 100

type captureMidiWriter struct {
	events []alsa.SeqEvent
}

func (w *captureMidiWriter) Write(event alsa.SeqEvent) error {
	w.events = append(w.events, event)
	return nil
}

func (w *captureMidiWriter) WritePort(event alsa.SeqEvent, _ int) error {
	w.events = append(w.events, event)
	return nil
}

func TestSharedMIDIDestination(t *testing.T) {
	previous := sharedMIDIDestination
	t.Cleanup(func() { sharedMIDIDestination = previous })
	destination := alsa.SeqAddr{Client: 28, Port: 0}
	message := []byte{midi.MakeNoteOn(0), 60, 100}

	sharedMIDIDestination = false
	writer := &captureMidiWriter{}
	if err := writeMidiMsgs(writer, destination, [][]byte{message}); err != nil {
		t.Fatal(err)
	}
	if writer.events[0].SeqAddr != destination {
		t.Fatalf("per-device destination = %v, want %v", writer.events[0].SeqAddr, destination)
	}

	sharedMIDIDestination = true
	writer = &captureMidiWriter{}
	if err := writeMidiMsgs(writer, destination, [][]byte{message}); err != nil {
		t.Fatal(err)
	}
	if writer.events[0].SeqAddr != alsa.SubsSeqAddr {
		t.Fatalf("shared destination = %v, want %v", writer.events[0].SeqAddr, alsa.SubsSeqAddr)
	}
}

func chromaticTestVoice(t *testing.T, notes ...*int) (*Voice, *Device) {
	t.Helper()
	voices := make([]Voice, len(notes))
	for i, note := range notes {
		voices[i] = Voice{Name: "voice", Note: note, Channel: 1}
	}
	device := &Device{
		Name:     "chromatic-test",
		MidiPort: "out",
		Channel:  1,
		Voices:   voices,
		SeqAddr:  alsa.SeqAddr{Client: 10, Port: 20},
	}
	bank := NewVoiceBank([]Device{*device})
	return bank.voices[0], device
}

func padMessage(note, velocity int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOn(0), byte(note), byte(velocity)}}
}

func releaseMessage(note int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeNoteOff(0), byte(note), 0}}
}

func assertMidiData(t *testing.T, event alsa.SeqEvent, want []byte) {
	t.Helper()
	if !bytes.Equal(event.Data, want) {
		t.Fatalf("MIDI data = %v, want %v", event.Data, want)
	}
}

func TestVoiceNoteOmissionSelectsChromaticMode(t *testing.T) {
	var device Device
	if err := json.Unmarshal([]byte(`{"Name":"kit","MidiPort":"out","Channel":1,"Voices":[{"Name":"lead"},{"Name":"zero","Note":0}]}`), &device); err != nil {
		t.Fatal(err)
	}
	if err := validateDevices([]Device{device}); err != nil {
		t.Fatal(err)
	}
	if !device.Voices[0].IsChromatic() {
		t.Fatal("omitted Note did not select chromatic mode")
	}
	if device.Voices[1].IsChromatic() {
		t.Fatal("explicit Note 0 selected chromatic mode")
	}
	if note, ok := device.Voices[1].PercussionNote(); !ok || note != 0 {
		t.Fatalf("explicit Note 0 = %d/%v, want 0/true", note, ok)
	}
}

func TestVoiceChannelOverridesDeviceChannel(t *testing.T) {
	note := 36
	device := Device{
		Name:     "routes",
		MidiPort: "out",
		Channel:  1,
		Voices: []Voice{
			{Name: "device-channel", Note: &note},
			{Name: "voice-channel", Note: &note, Channel: 9},
			{Name: "chromatic-channel", Channel: 11},
		},
	}
	if err := validateDevices([]Device{device}); err != nil {
		t.Fatal(err)
	}
	voiceBank := NewVoiceBank([]Device{device})
	if got := voiceBank.voices[0].EffectiveChannel(); got != 1 {
		t.Fatalf("device channel = %d, want 1", got)
	}
	if got := voiceBank.voices[1].EffectiveChannel(); got != 9 {
		t.Fatalf("voice channel = %d, want 9", got)
	}
	if got := voiceBank.voices[2].EffectiveChannel(); got != 11 {
		t.Fatalf("chromatic voice channel = %d, want 11", got)
	}
	percussion := Event{Voice: voiceBank.voices[1], Velocity: 90}
	messages := percussion.ToMidi()
	assertMidiData(t, alsa.SeqEvent{Data: messages[0]}, []byte{midi.MakeNoteOff(8), 36, 90})
	assertMidiData(t, alsa.SeqEvent{Data: messages[1]}, []byte{midi.MakeNoteOn(8), 36, 64})
	chromatic := Event{Voice: voiceBank.voices[2], ChromaticNote: 60, Velocity: 77}
	assertMidiData(t, alsa.SeqEvent{Data: chromatic.NoteOnMidi()}, []byte{midi.MakeNoteOn(10), 60, 77})

	device.Channel = 0
	if err := validateDevices([]Device{device}); err == nil {
		t.Fatal("device without a channel should require voice overrides")
	}
	device.Voices[0].Channel = 1
	device.Voices[2].Channel = 0
	if err := validateDevices([]Device{device}); err == nil {
		t.Fatal("voice without an effective channel was accepted")
	}
}

func TestChromaticPatternEditingAndTieInvariants(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	first, ok := pattern.SetChromaticNote(0, voice, 60, 90)
	if !ok || first.ChromaticNote != 60 || first.Velocity != 90 {
		t.Fatalf("first chromatic edit = %+v/%v", first, ok)
	}
	pattern.SetChromaticNote(1, voice, 62, 91)
	pattern.SetChromaticNote(2, voice, 64, 92)
	if pattern.TieEventsAtSteps(0, 2, voice) {
		t.Fatal("tie crossed an intervening event")
	}
	if !pattern.TieEventsAtSteps(1, 0, voice) {
		t.Fatal("reversed two-pad arrival did not create a tie")
	}
	firstEvent, _ := pattern.EventAtStep(0, voice)
	if !firstEvent.Tie {
		t.Fatal("tie was not stored on the earlier event")
	}
	pattern.SetChromaticNote(0, voice, 67, 90)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	if !firstEvent.Tie || firstEvent.ChromaticNote != 67 {
		t.Fatalf("pitch edit lost tie: %+v", firstEvent)
	}
	copyPattern := pattern.Copy()
	copyPattern.SetChromaticNote(0, voice, 69, 90)
	if firstEvent, _ := pattern.EventAtStep(0, voice); firstEvent.ChromaticNote != 67 || !firstEvent.Tie {
		t.Fatalf("copy shared chromatic state: %+v", firstEvent)
	}
	pattern.RemoveEventAtStep(1, voice)
	firstEvent, _ = pattern.EventAtStep(0, voice)
	if firstEvent.Tie {
		t.Fatal("removing the target left a dangling tie")
	}
	pattern.SetChromaticNote(1, voice, 62, 91)
	pattern.TieEventsAtSteps(0, 1, voice)
	pattern.SetLengthSteps(1)
	if firstEvent, _ := pattern.EventAtStep(0, voice); firstEvent.Tie {
		t.Fatal("shortening left a tie crossing the new end")
	}
}

func TestModeDoesNotStopPlayback(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}},
	}})
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
	})
	patbank, songbank = bank, nil
	shiftOn, altOn = false, false
	stopCalled := false
	playbackStop = func() error {
		stopCalled = true
		return nil
	}
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	if stopCalled || playbackStop == nil {
		t.Fatal("Mode stopped or cleared active playback")
	}
}

func TestChromaticPaletteAndModeEditing(t *testing.T) {
	writeCount := 0
	fire := NewFire(func([]byte) error {
		writeCount++
		return nil
	})
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}},
	}})
	voice := voiceBank.voices[0]
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
	})
	patbank, songbank = bank, nil
	shiftOn, altOn, playbackStop = false, false, nil
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	if !bank.NoteEditActive() {
		t.Fatal("Mode did not enter note-edit mode")
	}
	if err := handlePatternGrid(nil, 1, 0, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	firstNote, _ := chromaticPaletteNote(0, 1)
	if !ok || event.ChromaticNote != firstNote {
		t.Fatalf("palette assignment = %+v/%v", event, ok)
	}
	preview := &captureMidiWriter{}
	if err := bank.auditionChromaticEvent(preview, event); err != nil {
		t.Fatal(err)
	}
	if len(preview.events) != 2 {
		t.Fatalf("audition wrote %d messages, want 2", len(preview.events))
	}
	// The palette pad was pressed at pressVelocity, so that is the note's velocity.
	assertMidiData(t, preview.events[0], []byte{midi.MakeNoteOn(0), byte(firstNote), pressVelocity})
	assertMidiData(t, preview.events[1], []byte{midi.MakeNoteOff(0), byte(firstNote), 0})
	if err := processPatternEvent(nil, padMessage(NoteGridRight, 100)); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 1 {
		t.Fatalf("grid right moved cursor to %d, want 1", bank.StepCursor())
	}
	altOn = true
	writeCount = 0
	if err := handlePatternGrid(nil, 1, 0, 100); err != nil {
		t.Fatal(err)
	}
	if writeCount != 3 {
		t.Fatalf("Alt clear wrote %d Fire messages, want palette and status writes", writeCount)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(1, voice); ok {
		t.Fatal("Alt plus palette pad did not clear the current-step event")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); !ok {
		t.Fatal("Alt plus palette pad cleared a different step")
	}
}

func TestChromaticVelocityControl(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}},
	}})
	voice := voiceBank.voices[0]
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
	})
	patbank, songbank = bank, nil
	shiftOn, altOn, playbackStop = false, false, nil
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	if err := handlePatternGrid(nil, 1, 0, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(0, voice)
	if !ok || event.Velocity != pressVelocity {
		t.Fatalf("initial velocity = %d/%v, want the pad press %d", event.Velocity, ok, pressVelocity)
	}
	if bank.chromaticVelocity != defaultChromaticVelocity {
		t.Fatalf("encoder value = %d, want it to start at %d", bank.chromaticVelocity, defaultChromaticVelocity)
	}
	// A detent moves from the encoder's own value, so the first one does not start from
	// the velocity the pad press gave the note.
	cc := alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(EncoderLeft)}}
	if err := processPatternEvent(nil, cc); err != nil {
		t.Fatal(err)
	}
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	if event.Velocity != defaultChromaticVelocity-chromaticVelocityStep {
		t.Fatalf("downward velocity = %d, want %d", event.Velocity, defaultChromaticVelocity-chromaticVelocityStep)
	}
	cc.Data[2] = byte(EncoderRight)
	if err := processPatternEvent(nil, cc); err != nil {
		t.Fatal(err)
	}
	event, _ = bank.CurrentPattern().EventAtStep(0, voice)
	if event.Velocity != defaultChromaticVelocity {
		t.Fatalf("upward velocity = %d, want %d", event.Velocity, defaultChromaticVelocity)
	}
	if event.Velocity > midiNoteMax {
		t.Fatalf("velocity above the maximum = %d", event.Velocity)
	}
}

// The encoder keeps its value across steps, so a step picked after the encoder was set
// takes that value on the next detent instead of its own.
func TestVelocityEncoderValueIsInheritedByTheNextStep(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}},
	}})
	voice := voiceBank.voices[0]
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
	})
	patbank, songbank = bank, nil
	shiftOn, altOn, playbackStop = false, false, nil

	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 40)
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	// Put the encoder at 100 by turning it, then leave that step.
	for bank.chromaticVelocity > 100 {
		if err := processPatternEvent(nil, encoderCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	if bank.chromaticVelocity != 100 {
		t.Fatalf("encoder value = %d, want 100", bank.chromaticVelocity)
	}
	// Selecting the second step must not drag the encoder back to that step's 40.
	if err := bank.MoveStepCursor(1); err != nil {
		t.Fatal(err)
	}
	if bank.chromaticVelocity != 100 {
		t.Fatalf("encoder value after selecting = %d, want the inherited 100", bank.chromaticVelocity)
	}
	if err := processPatternEvent(nil, encoderCC(EncoderLeft)); err != nil {
		t.Fatal(err)
	}
	if event, _ := pattern.EventAtStep(1, voice); event.Velocity != 99 {
		t.Fatalf("second step velocity = %d, want the encoder's 99", event.Velocity)
	}
	// The first step is untouched by the encoder moving on the second.
	if event, _ := pattern.EventAtStep(0, voice); event.Velocity != 100 {
		t.Fatalf("first step velocity = %d, want it left at 100", event.Velocity)
	}
}

func encoderCC(direction int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCVolume), byte(direction)}}
}

func TestChromaticStatusText(t *testing.T) {
	event := &Event{ChromaticNote: 60, Velocity: 90, Tie: true}
	if got := chromaticStatusText(0, event, 90, 1); got != "S01 C4@090->02" {
		t.Fatalf("tied status = %q", got)
	}
	// A step with no note must not show a velocity, or it reads as that step's own.
	if got := chromaticStatusText(1, nil, 90, -1); got != "S02 --" {
		t.Fatalf("empty status = %q", got)
	}
}

func TestChromaticPadSetsEditingStep(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}},
	}})
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
	})
	patbank, songbank = bank, nil
	shiftOn, altOn, playbackStop = false, false, nil
	if err := processPatternEvent(nil, padMessage(54+3, 100)); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 3 {
		t.Fatalf("step cursor after pad = %d, want 3", bank.StepCursor())
	}
	bank.releasePad(0, 3)
	if err := processPatternEvent(nil, padMessage(54+5, 100)); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 5 {
		t.Fatalf("step cursor after second pad = %d, want 5", bank.StepCursor())
	}
}

func TestChromaticPadGesturesAndReleases(t *testing.T) {
	writeCount := 0
	fire := NewFire(func([]byte) error {
		writeCount++
		return nil
	})
	voiceBank := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}},
	}})
	voice := voiceBank.voices[0]
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(1, voice, 62, 100)
	pattern.SetChromaticNote(2, voice, 64, 100)
	if handled, err := bank.handleChromaticStepPress(0, 0); !handled || err != nil {
		t.Fatalf("first pad press = %v/%v", handled, err)
	}
	writeCount = 0
	if handled, err := bank.handleChromaticStepPress(0, 1); !handled || err != nil {
		t.Fatalf("second pad press = %v/%v", handled, err)
	}
	if writeCount != 6 {
		t.Fatalf("tie gesture wrote %d Fire messages, want row and status redraws", writeCount)
	}
	if event, _ := pattern.EventAtStep(0, voice); !event.Tie {
		t.Fatal("two held pads did not tie adjacent events")
	}
	bank.releasePad(0, 0)
	bank.releasePad(0, 1)
	if bank.pressedPads != 0 || bank.rowPadMasks[0] != 0 {
		t.Fatalf("pad state did not clear on release: held=%x row=%x", bank.pressedPads, bank.rowPadMasks[0])
	}

	bank.clearPadState()
	pattern.UntieEventsAtSteps(0, 1, voice)
	if handled, _ := bank.handleChromaticStepPress(0, 0); !handled {
		t.Fatal("selected chromatic row was not handled")
	}
	if handled, _ := bank.handleChromaticStepPress(1, 1); !handled {
		t.Fatal("cross-row gesture was not handled as a gesture")
	}
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("cross-row gesture created a tie")
	}
	pattern.TieEventsAtSteps(0, 1, voice)
	bank.clearPadState()
	if handled, _ := bank.handleChromaticStepPress(0, 2); !handled {
		t.Fatal("third selected-row pad was not handled")
	}
	if event, _ := pattern.EventAtStep(0, voice); !event.Tie {
		t.Fatal("third held pad removed the completed tie")
	}

	bank.clearPadState()
	empty := &Pattern{}
	bank.Patterns[bank.selPatIdx] = empty
	bank.handleChromaticStepPress(0, 0)
	bank.handleChromaticStepPress(0, 1)
	if len(empty.Events) != 0 {
		t.Fatal("empty chromatic steps created events")
	}

	bank.clearPadState()
	bank.handleChromaticStepPress(0, 0)
	previousPatbank := patbank
	patbank = bank
	t.Cleanup(func() { patbank = previousPatbank })
	if err := processPatternEvent(nil, releaseMessage(54)); err != nil {
		t.Fatal(err)
	}
	if bank.pressedPads != 0 || bank.rowPadMasks[0] != 0 {
		t.Fatal("NoteOff was not treated as a release")
	}
	bank.handleChromaticStepPress(0, 0)
	if err := processPatternEvent(nil, padMessage(54, 0)); err != nil {
		t.Fatal(err)
	}
	if bank.pressedPads != 0 || bank.rowPadMasks[0] != 0 {
		t.Fatal("NoteOn velocity zero was not treated as a release")
	}
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
	assertMidiData(t, alsa.SeqEvent{Data: messages[1]}, []byte{midi.MakeNoteOn(0), 60, 64})
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

// The pitch palette is four octaves from A1, twelve semitones to the row, so every row
// starts on A and the four columns past G# carry no pitch.
func TestChromaticPaletteLayout(t *testing.T) {
	if got, _ := chromaticPaletteNote(0, 0); got != 33 {
		t.Fatalf("first palette note = %d (%s), want A1 at 33", got, midiNoteName(got))
	}
	if name := midiNoteName(33); name != "A1" {
		t.Fatalf("MIDI 33 names as %q, want A1", name)
	}
	if chromaticPaletteColumns != 12 || chromaticStepColumns != 4 {
		t.Fatalf("palette is %d columns with %d for steps, want 12 and 4", chromaticPaletteColumns, chromaticStepColumns)
	}
	if chromaticPaletteColumns+chromaticStepColumns != padColumns {
		t.Fatalf("palette %d plus steps %d does not fill a %d column row",
			chromaticPaletteColumns, chromaticStepColumns, padColumns)
	}
	if chromaticStepCells != maxPatternSteps {
		t.Fatalf("step block has %d cells, want one per step", chromaticStepCells)
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		first, ok := chromaticPaletteNote(row, 0)
		if !ok {
			t.Fatalf("row %d has no first note", row)
		}
		if name := midiNoteName(first); name != fmt.Sprintf("A%d", row+1) {
			t.Fatalf("row %d starts on %s, want A%d", row, name, row+1)
		}
		last, ok := chromaticPaletteNote(row, chromaticPaletteColumns-1)
		if !ok {
			t.Fatalf("row %d has no last note", row)
		}
		// Twelve semitones from A is a major seventh, so the row ends on the G# above
		// its starting A: row 0 runs A1 up to G#2.
		if name := midiNoteName(last); name != fmt.Sprintf("G#%d", row+2) {
			t.Fatalf("row %d ends on %s, want G#%d", row, name, row+2)
		}
		if last-first != 11 {
			t.Fatalf("row %d spans %d semitones, want the 11 from A to G#", row, last-first)
		}
		// The columns past the palette have no pitch at all.
		for _, col := range []int{12, 13, 14, 15} {
			if note, ok := chromaticPaletteNote(row, col); ok {
				t.Fatalf("row %d column %d has pitch %d, want none", row, col, note)
			}
		}
	}
	if _, ok := chromaticPaletteNote(-1, 0); ok {
		t.Fatal("a row above the palette reported a pitch")
	}
	if _, ok := chromaticPaletteNote(0, -1); ok {
		t.Fatal("a column left of the palette reported a pitch")
	}
}

// The right-hand block stands for the steps themselves: it reads in the same order as the
// step grid, four per row, and covers every step exactly once.
func TestChromaticStepBlockCoversEveryStep(t *testing.T) {
	seen := make(map[int]bool, chromaticStepCells)
	for row := 0; row < chromaticPaletteRows; row++ {
		for offset := 0; offset < chromaticStepColumns; offset++ {
			col := chromaticPaletteColumns + offset
			step := chromaticStepAt(row, col)
			if step != row*chromaticStepColumns+offset {
				t.Fatalf("cell row %d offset %d = step %d", row, offset, step)
			}
			if step < 0 || step >= maxPatternSteps {
				t.Fatalf("cell row %d offset %d maps outside the pattern: %d", row, offset, step)
			}
			if seen[step] {
				t.Fatalf("step %d is shown by more than one cell", step)
			}
			seen[step] = true
		}
	}
	if len(seen) != maxPatternSteps {
		t.Fatalf("the block covers %d steps, want %d", len(seen), maxPatternSteps)
	}
	// The palette side of the grid is not a step.
	for col := 0; col < chromaticPaletteColumns; col++ {
		if step := chromaticStepAt(0, col); step >= 0 {
			t.Fatalf("palette column %d reports step %d", col, step)
		}
	}
	for _, pad := range [][2]int{{-1, 12}, {0, -1}, {chromaticPaletteRows, 12}, {0, padColumns}} {
		if step := chromaticStepAt(pad[0], pad[1]); step >= 0 {
			t.Fatalf("pad %v reports step %d", pad, step)
		}
	}
}

// Pressing a cell in the step block moves the edit there, and A1 clears the step's note.
func TestChromaticStepBlockSelectsAndA1Removes(t *testing.T) {
	fire := NewFire(func([]byte) error { return nil })
	voiceBank := NewVoiceBank([]Device{{Channel: 1, Voices: []Voice{{Name: "lead", Channel: 1}}}})
	bank := NewPatternBank(fire, voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	previousPatbank, previousSongbank := patbank, songbank
	previousShift, previousAlt, previousCancel := shiftOn, altOn, playbackStop
	t.Cleanup(func() {
		patbank, songbank = previousPatbank, previousSongbank
		shiftOn, altOn, playbackStop = previousShift, previousAlt, previousCancel
	})
	patbank, songbank = bank, nil
	shiftOn, altOn, playbackStop = false, false, nil
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	voice := voiceBank.voices[0]

	// Put a note on step 5, then select step 5 from the block rather than the grid.
	if err := bank.setStepCursor(5); err != nil {
		t.Fatal(err)
	}
	if err := bank.handleNoteEditPad(nil, 2, 3, 100); err != nil {
		t.Fatal(err)
	}
	if err := bank.setStepCursor(0); err != nil {
		t.Fatal(err)
	}
	row, col := 1, chromaticPaletteColumns+1 // step 5
	if err := bank.handleNoteEditPad(nil, row, col, 100); err != nil {
		t.Fatal(err)
	}
	if bank.StepCursor() != 5 {
		t.Fatalf("cursor = %d after pressing the step block, want 5", bank.StepCursor())
	}
	if _, ok := bank.CurrentPattern().EventAtStep(5, voice); !ok {
		t.Fatal("the note on step 5 disappeared")
	}
	// A palette pad now edits step 5.
	if err := bank.handleNoteEditPad(nil, 0, 5, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(5, voice)
	if !ok {
		t.Fatal("a palette pad stopped editing the selected step")
	}
	want, _ := chromaticPaletteNote(0, 5)
	if event.ChromaticNote != want {
		t.Fatalf("step 5 pitch = %d, want %d", event.ChromaticNote, want)
	}

	// A1 is the palette's first pad and means no note.
	if err := bank.handleNoteEditPad(nil, 0, 0, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(5, voice); ok {
		t.Fatal("A1 did not remove the note")
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); ok {
		t.Fatal("A1 removed a note from the wrong step")
	}
}

// A step cell shows that step's note colour, dark when the step is empty.
func TestChromaticStepCellsShowNoteColours(t *testing.T) {
	sim := newFireSim()
	voiceBank := trackWindowKit(8, 0)
	bank := NewPatternBank(NewFire(sim.write), voiceBank)
	if err := bank.Jump(1); err != nil {
		t.Fatal(err)
	}
	usePatternGlobals(t, bank)
	voice := voiceBank.voices[0]
	note, _ := chromaticPaletteNote(1, 4)
	bank.CurrentPattern().SetChromaticNote(6, voice, note, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.setStepCursor(0); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := bank.drawNotePalette(); err != nil {
		t.Fatal(err)
	}
	row, col := 1, chromaticPaletteColumns+2 // step 6
	index := row*padColumns + col
	if sim.pads[index] != chromaticPaletteColor(note) {
		t.Fatalf("step 6 cell = %v, want the note colour %v", sim.pads[index], chromaticPaletteColor(note))
	}
	// An empty step is dark.
	emptyRow, emptyCol := 0, chromaticPaletteColumns // step 0
	if sim.pads[emptyRow*padColumns+emptyCol] != [3]int{} {
		t.Fatalf("empty step cell = %v, want dark", sim.pads[emptyRow*padColumns+emptyCol])
	}
}
