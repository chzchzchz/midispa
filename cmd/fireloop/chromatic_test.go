package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
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
	bank, _ := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
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
	bank, voice := chromaBankOn(t, func([]byte) error {
		writeCount++
		return nil
	})
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
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
	firstNote, _ := chromaticPaletteNote(0, 1, 0)
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
	bank, voice := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
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
	bank, voice := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}

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
	bank, _ := chromaBank(t)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
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
	bank, voice := chromaBankOn(t, func([]byte) error {
		writeCount++
		return nil
	})
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
	if got, _ := chromaticPaletteNote(0, 0, 0); got != 33 {
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
		first, ok := chromaticPaletteNote(row, 0, 0)
		if !ok {
			t.Fatalf("row %d has no first note", row)
		}
		if name := midiNoteName(first); name != fmt.Sprintf("A%d", row+1) {
			t.Fatalf("row %d starts on %s, want A%d", row, name, row+1)
		}
		last, ok := chromaticPaletteNote(row, chromaticPaletteColumns-1, 0)
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
			if note, ok := chromaticPaletteNote(row, col, 0); ok {
				t.Fatalf("row %d column %d has pitch %d, want none", row, col, note)
			}
		}
	}
	if _, ok := chromaticPaletteNote(-1, 0, 0); ok {
		t.Fatal("a row above the palette reported a pitch")
	}
	if _, ok := chromaticPaletteNote(0, -1, 0); ok {
		t.Fatal("a column left of the palette reported a pitch")
	}
}

// paletteTestBank is a bank sitting in note-edit mode on a chromatic track, which is the
// only state the palette octave is used from.
func paletteTestBank(t *testing.T) (*PatternBank, *Voice, *fireSim) {
	t.Helper()
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	return bank, kit.voices[0], sim
}

func selectKnobCC(direction int) alsa.SeqEvent {
	return alsa.SeqEvent{Data: []byte{midi.MakeCC(0), byte(CCSelect), byte(direction)}}
}

// The SELECT knob is an octave transpose for the palette: it moves which pitches the pads
// choose, and nothing that is already written into the pattern.
func TestSelectKnobShiftsPaletteByOctave(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	voiceBefore := bank.trackVoice(1)

	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	first, _ := chromaticPaletteNote(1, 3, 0)
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != first {
		t.Fatalf("palette note = %d, want %d", event.ChromaticNote, first)
	}

	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	if bank.paletteOctave != 1 {
		t.Fatalf("palette octave = %d after one detent, want 1", bank.paletteOctave)
	}
	if bank.trackVoice(1) != voiceBefore {
		t.Fatal("the knob moved the track's voice instead of the palette")
	}
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != first {
		t.Fatalf("stored note = %d after the shift, want it left at %d", event.ChromaticNote, first)
	}

	// The same pad now reaches the pitch an octave up.
	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	up, _ := chromaticPaletteNote(1, 3, bank.paletteOctave)
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != up {
		t.Fatalf("note after shifting up = %d, want %d", event.ChromaticNote, up)
	}
	if up != first+chromaticOctaveShift {
		t.Fatalf("shifted palette note = %d, want an octave above %d", up, first)
	}

	// And an octave down takes it back past where it started.
	for range 2 {
		if err := processPatternEvent(nil, selectKnobCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	if err := bank.handleNoteEditPad(nil, 1, 3, pressVelocity); err != nil {
		t.Fatal(err)
	}
	down, _ := chromaticPaletteNote(1, 3, bank.paletteOctave)
	if event, _ := bank.CurrentPattern().EventAtStep(0, voice); event.ChromaticNote != down {
		t.Fatalf("note after shifting down = %d, want %d", event.ChromaticNote, down)
	}
	if down != first-chromaticOctaveShift {
		t.Fatalf("lowered palette note = %d, want an octave below %d", down, first)
	}
}

// The palette has to say which octave it is offering, so every pad's colour travels with it
// while the steps keep the colours of the notes they hold.
func TestPaletteShiftMovesColours(t *testing.T) {
	bank, voice, sim := paletteTestBank(t)
	bank.CurrentPattern().SetChromaticNote(3, voice, 40, pressVelocity)
	if err := bank.setStepCursor(3); err != nil {
		t.Fatal(err)
	}
	if err := bank.drawNotePalette(); err != nil {
		t.Fatal(err)
	}
	stepColour := stripCell(sim, 3)
	before := paletteRegion(sim)

	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	after := paletteRegion(sim)
	changed := 0
	for i := range before {
		row, col := i/chromaticPaletteColumns, i%chromaticPaletteColumns
		index := row*padColumns + col
		if before[i] != after[i] {
			changed++
		}
		note, _ := chromaticPaletteNote(row, col, bank.paletteOctave)
		if want := chromaticPaletteColor(note); sim.pads[index] != want {
			t.Fatalf("palette pad row %d col %d = %v, want the shifted pitch colour %v",
				row, col, sim.pads[index], want)
		}
	}
	if changed != len(before) {
		t.Fatalf("%d of %d palette pads changed colour, want the shift visible on all of them",
			changed, len(before))
	}
	// A note already in the pattern keeps its own colour; only the palette moved.
	if got := stripCell(sim, 3); got != stepColour {
		t.Fatalf("step 3 cell = %v after the shift, want the note colour %v", got, stepColour)
	}
}

// A palette below the unshifted base must keep its colours apart instead of collapsing them
// onto the first entry of the table.
func TestPaletteBelowBaseKeepsDistinctColours(t *testing.T) {
	bank, _, sim := paletteTestBank(t)
	for range 2 {
		if err := processPatternEvent(nil, selectKnobCC(EncoderLeft)); err != nil {
			t.Fatal(err)
		}
	}
	if bank.paletteOctave >= 0 {
		t.Fatalf("palette octave = %d, want the palette below its base", bank.paletteOctave)
	}
	for row := 0; row < chromaticPaletteRows; row++ {
		seen := make(map[[3]int]bool, chromaticPaletteColumns)
		for col := 0; col < chromaticPaletteColumns; col++ {
			seen[sim.pads[row*padColumns+col]] = true
		}
		if len(seen) != chromaticPaletteColumns {
			t.Fatalf("row %d has %d colours across %d pads, want one each",
				row, len(seen), chromaticPaletteColumns)
		}
	}
}

// The palette may only turn as far as the MIDI range allows, or a pad would name a note
// that cannot be sent.
func TestPaletteOctaveStaysInsideMIDIRange(t *testing.T) {
	bank, _, _ := paletteTestBank(t)
	lowest, highest := chromaticOctaveBounds()
	if lowest != -2 || highest != 3 {
		t.Fatalf("octave bounds = %d to %d, want -2 to 3 for a palette from A1", lowest, highest)
	}

	for _, direction := range []int{EncoderRight, EncoderLeft} {
		for range 8 {
			if err := processPatternEvent(nil, selectKnobCC(direction)); err != nil {
				t.Fatal(err)
			}
		}
		octave := bank.paletteOctave
		// Turning further must not move it, or the palette would leave the MIDI range.
		if err := processPatternEvent(nil, selectKnobCC(direction)); err != nil {
			t.Fatal(err)
		}
		if bank.paletteOctave != octave {
			t.Fatalf("palette octave = %d after turning past the end, want it held at %d",
				bank.paletteOctave, octave)
		}
		base := bank.paletteBase()
		last, _ := chromaticPaletteNote(chromaticPaletteRows-1, chromaticPaletteColumns-1, octave)
		if base < 0 || last > midiNoteMax {
			t.Fatalf("palette spans %d to %d, outside the MIDI range", base, last)
		}
		if direction == EncoderRight && octave != highest {
			t.Fatalf("palette stopped at octave %d, want the top %d", octave, highest)
		}
		if direction == EncoderLeft && octave != lowest {
			t.Fatalf("palette stopped at octave %d, want the bottom %d", octave, lowest)
		}
	}
}

// The erase key is wherever the palette starts, so it moves with the palette.
func TestPaletteFirstPadErasesAfterShift(t *testing.T) {
	bank, voice, _ := paletteTestBank(t)
	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	base := bank.paletteBase()
	if base == chromaticBaseNote {
		t.Fatalf("palette base = %d, want it shifted off A1", base)
	}
	// A pad an octave along places the shifted pitch.
	if err := bank.handleNoteEditPad(nil, 1, 0, pressVelocity); err != nil {
		t.Fatal(err)
	}
	placed, _ := chromaticPaletteNote(1, 0, bank.paletteOctave)
	if placed == base {
		t.Fatalf("the test needs a pad that is not the palette's first one: %d", placed)
	}
	if event, ok := bank.CurrentPattern().EventAtStep(0, voice); !ok || event.ChromaticNote != placed {
		t.Fatalf("note = %d/%v, want the shifted palette pitch %d", event.ChromaticNote, ok, placed)
	}
	// The palette's first pad is the erase key wherever the palette has moved to. Placing
	// the note above also lifted the guard on that pad, which doubles as the step-one pad.
	if err := bank.handleNoteEditPad(nil, 0, 0, pressVelocity); err != nil {
		t.Fatal(err)
	}
	if _, ok := bank.CurrentPattern().EventAtStep(0, voice); ok {
		t.Fatalf("the palette's first pad at %d did not erase", base)
	}
}

// Outside note-edit mode the knob is still what chooses the track's voice, and the palette
// is left where the user put it.
func TestSelectKnobJogsVoiceOutsideNoteEdit(t *testing.T) {
	bank, _ := quietBank(t, trackWindowKit(8, 0))
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	before := bank.trackVoice(1)
	if err := processPatternEvent(nil, selectKnobCC(EncoderRight)); err != nil {
		t.Fatal(err)
	}
	if bank.trackVoice(1) == before {
		t.Fatal("the knob did not jog the track's voice")
	}
	if bank.paletteOctave != 0 {
		t.Fatalf("palette octave = %d outside note-edit mode, want it untouched", bank.paletteOctave)
	}
	if bank.NoteEditActive() {
		t.Fatal("the knob entered note-edit mode")
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
	// Column 5 on the selected row is step 6 in step mode, so that pad is guarded until a
	// note has been chosen at this step.
	guarded, _ := bank.CurrentPattern().EventAtStep(5, voice)
	if err := bank.handleNoteEditPad(nil, 0, 5, 100); err != nil {
		t.Fatal(err)
	}
	if event, _ := bank.CurrentPattern().EventAtStep(5, voice); event.ChromaticNote != guarded.ChromaticNote {
		t.Fatalf("the step's own pad changed the pitch to %d", event.ChromaticNote)
	}
	// Choosing a note elsewhere lifts the guard, and a palette pad then edits step 5.
	if err := bank.handleNoteEditPad(nil, 1, 3, 100); err != nil {
		t.Fatal(err)
	}
	chosen, _ := chromaticPaletteNote(1, 3, 0)
	if event, _ := bank.CurrentPattern().EventAtStep(5, voice); event.ChromaticNote != chosen {
		t.Fatalf("step 5 pitch = %d, want %d", event.ChromaticNote, chosen)
	}
	if err := bank.handleNoteEditPad(nil, 0, 5, 100); err != nil {
		t.Fatal(err)
	}
	event, ok := bank.CurrentPattern().EventAtStep(5, voice)
	if !ok {
		t.Fatal("a palette pad stopped editing the selected step")
	}
	want, _ := chromaticPaletteNote(0, 5, 0)
	if event.ChromaticNote != want {
		t.Fatalf("step 5 pitch = %d, want the lifted guard to allow %d", event.ChromaticNote, want)
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
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	note, _ := chromaticPaletteNote(1, 4, 0)
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

// A grid pad is a step selector in step mode and a pitch pad in note-edit mode. Pressing
// the same pad again in note-edit mode must not rewrite the note the user navigated to,
// until a note has been chosen at that step.
func TestPalettePadThatSelectedTheStepIsRefused(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	original := 40 // E2, which the palette pad below would not choose
	pattern.SetChromaticNote(6, voice, original, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	press := func(row, col, vel int) {
		if err := processPatternEvent(nil, padMessage(54+row*16+col, vel)); err != nil {
			t.Fatal(err)
		}
	}
	row, col := 0, 6 // the pad that means "step 7" in step mode
	paletteNote, _ := chromaticPaletteNote(row, col, 0)
	if paletteNote == original {
		t.Fatalf("the test needs a palette pad whose pitch differs from %d", original)
	}

	// Select the step with that pad in step mode, then release it.
	press(row, col, 100)
	press(row, col, 0)
	if bank.StepCursor() != 6 || bank.NoteEditActive() {
		t.Fatalf("after the step press: cursor=%d noteEdit=%v", bank.StepCursor(), bank.NoteEditActive())
	}
	// Enter note selection. The step's note must survive the same pad being pressed.
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	press(row, col, 100)
	press(row, col, 0)
	event, ok := pattern.EventAtStep(6, voice)
	if !ok || event.ChromaticNote != original {
		t.Fatalf("note = %d/%v, want the guarded pad to leave it at %d", event.ChromaticNote, ok, original)
	}

	// Choosing a note lifts the guard, so the same pad is a pitch pad again.
	press(1, 3, 100)
	press(1, 3, 0)
	chosen, _ := pattern.EventAtStep(6, voice)
	if chosen.ChromaticNote == original {
		t.Fatal("choosing a note on another pad did nothing")
	}
	press(row, col, 100)
	event, _ = pattern.EventAtStep(6, voice)
	if event.ChromaticNote != paletteNote {
		t.Fatalf("note = %d, want the lifted guard to allow the palette pitch %d", event.ChromaticNote, paletteNote)
	}
}

// The guard belongs to the step it was armed on: moving the cursor lifts it.
func TestPaletteGuardLiftsWhenTheCursorMoves(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(6, voice, 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	press := func(row, col, vel int) {
		if err := processPatternEvent(nil, padMessage(54+row*16+col, vel)); err != nil {
			t.Fatal(err)
		}
	}
	row, col := 0, 6
	paletteNote, _ := chromaticPaletteNote(row, col, 0)

	press(row, col, 100)
	press(row, col, 0)
	if err := processPatternEvent(nil, padMessage(NoteMode, 100)); err != nil {
		t.Fatal(err)
	}
	// The guard holds while the cursor stays on step 7.
	press(row, col, 100)
	press(row, col, 0)
	if event, _ := pattern.EventAtStep(6, voice); event.ChromaticNote != 40 {
		t.Fatalf("note = %d while the guard should hold, want 40", event.ChromaticNote)
	}
	// Moving to another step lifts it, so the pad is a pitch pad again.
	if err := bank.MoveStepCursor(2); err != nil {
		t.Fatal(err)
	}
	press(row, col, 100)
	if event, _ := pattern.EventAtStep(8, voice); event.ChromaticNote != paletteNote {
		t.Fatalf("note on the newly selected step = %d, want %d", event.ChromaticNote, paletteNote)
	}
	if event, _ := pattern.EventAtStep(6, voice); event.ChromaticNote != 40 {
		t.Fatalf("the note left behind changed to %d", event.ChromaticNote)
	}
}

// The pad standing for step 1 is also A1, the erase key, so the guard has to run before
// the removal or it deletes the note it is meant to protect. Alt still clears.
func TestStepOnePadDoesNotEraseTheNote(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 43, 100) // G2 on step 1
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	// The step pad for step 1 is palette (0,0), which is A1.
	if err := bank.handleNoteEditPad(nil, 0, 0, 100); err != nil {
		t.Fatal(err)
	}
	if event, ok := pattern.EventAtStep(0, voice); !ok || event.ChromaticNote != 43 {
		t.Fatalf("note = %d/%v, want the step's own pad to leave it at 43", event.ChromaticNote, ok)
	}
	// Alt on the same pad is a request about the step, so it clears.
	altOn = true
	if err := bank.handleNoteEditPad(nil, 0, 0, 100); err != nil {
		t.Fatal(err)
	}
	if _, ok := pattern.EventAtStep(0, voice); ok {
		t.Fatal("Alt on the step pad did not clear the note")
	}
}

// paletteRegion is the pitch palette as the unit holds it, excluding the step strip.
func paletteRegion(sim *fireSim) [][3]int {
	var out [][3]int
	for row := 0; row < chromaticPaletteRows; row++ {
		for col := 0; col < chromaticPaletteColumns; col++ {
			out = append(out, sim.pads[row*padColumns+col])
		}
	}
	return out
}

// stripCell is one step cell as the unit holds it.
func stripCell(sim *fireSim, step int) [3]int {
	row, col, ok := chromaticStepCell(step)
	if !ok {
		return [3]int{}
	}
	return sim.pads[row*padColumns+col]
}

// Playing a pattern while choosing notes must not disturb the palette. The column
// playhead repaints every pad row, which in note-edit mode is the palette.
func TestNoteEditPlayheadLeavesThePaletteAlone(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	for _, step := range []int{1, 3, 7} {
		bank.CurrentPattern().SetChromaticNote(step, voice, 36+step, 100)
	}
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NotePlay, 100)); err != nil {
		t.Fatal(err)
	}
	before := paletteRegion(sim)
	for step := 0; step < maxPatternSteps; step++ {
		if err := patbank.playback.updatePads(stepBeat(step)); err != nil {
			t.Fatal(err)
		}
	}
	for i, want := range before {
		row, col := i/chromaticPaletteColumns, i%chromaticPaletteColumns
		if got := sim.pads[row*padColumns+col]; got != want {
			t.Fatalf("palette pad row %d col %d changed from %v to %v during playback", row, col, want, got)
		}
	}
}

// In note-edit mode the playhead moves along the step strip, one cell per step, and the
// cell it leaves goes back to that step's note colour.
func TestNoteEditPlayheadLightsTheStepStrip(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().SetChromaticNote(5, voice, 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NotePlay, 100)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got == oledWhite {
		t.Fatalf("step 5 cell = %v before the playhead arrived, want its note colour", got)
	}
	if err := patbank.playback.updatePads(stepBeat(5)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got != oledWhite {
		t.Fatalf("step 5 cell = %v while the playhead is there, want white", got)
	}
	// The step the note is on keeps its colour apart from the playhead.
	noteColor := chromaticPaletteColor(40)
	if err := patbank.playback.updatePads(stepBeat(6)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 6); got != oledWhite {
		t.Fatalf("step 6 cell = %v while the playhead is there, want white", got)
	}
	if got := stripCell(sim, 5); got == oledWhite {
		t.Fatal("the cell the playhead left is still lit")
	}
	if got := stripCell(sim, 5); got != noteColor {
		t.Fatalf("step 5 cell = %v after the playhead left, want the note colour %v", got, noteColor)
	}
}

// Stopping must put the strip back, or a lit cell outlives the playback that put it there.
func TestNoteEditPlayheadClearsOnStop(t *testing.T) {
	bank, kit, sim := recordedBank(t, trackWindowKit(8, 0))
	voice := kit.voices[0]
	bank.CurrentPattern().SetChromaticNote(5, voice, 40, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	if err := processPatternEvent(nil, padMessage(NotePlay, 100)); err != nil {
		t.Fatal(err)
	}
	if err := patbank.playback.updatePads(stepBeat(5)); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got != oledWhite {
		t.Fatalf("step 5 cell = %v, want white while playing", got)
	}
	if err := stopPlayback(); err != nil {
		t.Fatal(err)
	}
	if got := stripCell(sim, 5); got == oledWhite {
		t.Fatal("stopping left a strip cell lit")
	}
	if got := stripCell(sim, 5); got != chromaticPaletteColor(40) {
		t.Fatalf("step 5 cell = %v after stopping, want the note colour", got)
	}
}

// playheadTestKit is one chromatic voice followed by one percussive voice, so the two
// colour rules can be compared side by side.
func playheadTestKit() *VoiceBank {
	kick := 36
	return NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1}, {Name: "kick", Note: &kick, Channel: 1}},
	}})
}

// The playhead must not flatten a chromatic step to green. It keeps the pitch colour, and
// the column behind it restores that exact colour rather than a flat one.
func TestPlayheadKeepsChromaticPitchColour(t *testing.T) {
	bank, kit, sim := recordedBank(t, playheadTestKit())
	chromatic, percussive := kit.voices[0], kit.voices[1]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(3, chromatic, 40, 100) // E2, untied
	pattern.ToggleEvent(Event{Voice: percussive, Beat: stepBeat(3), Velocity: 100})

	pitch := chromaticPaletteColor(40)
	drum := chromaticEventColor(Event{Voice: percussive})

	if err := bank.drawPadColumnInvert(3); err != nil {
		t.Fatal(err)
	}
	if got, want := sim.pads[3], invertColor(pitch); got != want {
		t.Fatalf("inverted chromatic step = %v, want the inverted pitch colour %v", got, want)
	}
	if got, want := sim.pads[3+padColumns], invertColor(drum); got != want {
		t.Fatalf("inverted percussive step = %v, want %v", got, want)
	}

	// The column the playhead leaves goes back to the real colours.
	if err := bank.drawPadColumn(3); err != nil {
		t.Fatal(err)
	}
	if got := sim.pads[3]; got != pitch {
		t.Fatalf("restored chromatic step = %v, want the pitch colour %v", got, pitch)
	}
	if got := sim.pads[3+padColumns]; got != drum {
		t.Fatalf("restored percussive step = %v, want %v", got, drum)
	}
}

// A tie is marked by pushing the colour away from the playhead, which means lifting it
// normally and lowering it when inverted, where lifting would be invisible.
func TestPlayheadMarksTiesBothWays(t *testing.T) {
	bank, kit, sim := recordedBank(t, playheadTestKit())
	voice := kit.voices[0]
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(3, voice, 40, 100)
	pattern.SetChromaticNote(5, voice, 43, 100)
	pattern.TieEventsAtSteps(3, 5, voice)

	if err := bank.drawPadColumn(3); err != nil {
		t.Fatal(err)
	}
	lifted := markTieColor(chromaticPaletteColor(40), false)
	if got := sim.pads[3]; got != lifted {
		t.Fatalf("tied step behind the playhead = %v, want the lifted colour %v", got, lifted)
	}
	if err := bank.drawPadColumnInvert(3); err != nil {
		t.Fatal(err)
	}
	lowered := markTieColor(invertColor(chromaticPaletteColor(40)), true)
	if got := sim.pads[3]; got != lowered {
		t.Fatalf("tied step under the playhead = %v, want the lowered colour %v", got, lowered)
	}
	// The mark has to differ from the unmarked colour in both directions.
	if lowered == invertColor(chromaticPaletteColor(40)) {
		t.Fatal("a tie under the playhead is not distinguishable")
	}
}

// stepCellPad is the grid pad note of the strip cell standing for a step, which is where the
// tie gesture is made while the palette owns the grid.
func stepCellPad(step int) int {
	row, col, ok := chromaticStepCell(step)
	if !ok {
		return 0
	}
	return 54 + row*padColumns + col
}

// A tie is what holds a note past its step, so the gesture that makes one matters as much as
// playback honouring it: two step cells held together tie the two steps, the same gesture
// step mode uses on the step grid. This drives the pads through the real handler.
func TestNoteEditStepCellsTieSteps(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(3, voice, 64, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	// Hold the cell for step 1, then press the cell for step 4 without letting go.
	press := func(step, velocity int) {
		if err := processPatternEvent(nil, padMessage(stepCellPad(step), velocity)); err != nil {
			t.Fatal(err)
		}
	}
	press(0, 100)
	press(3, 100)
	first, _ := pattern.EventAtStep(0, voice)
	second, _ := pattern.EventAtStep(3, voice)
	if !first.Tie {
		t.Fatalf("two held cells did not tie step 1 to step 4: %+v", first)
	}
	if second.Tie {
		t.Fatal("the tie landed on the later event as well")
	}
	// The edit stays on the step it came from, which is what a tie is holding.
	if bank.StepCursor() != 0 {
		t.Fatalf("the tie moved the edit to step %d", bank.StepCursor()+1)
	}
	// Letting go of both ends the gesture.
	press(0, 0)
	press(3, 0)
	if bank.noteEditHeldStep != noHeldStep {
		t.Fatalf("held step = %d after releasing the cells", bank.noteEditHeldStep)
	}

	// The tie is what the pattern plays back: the first note sounds until the tied step.
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step < 5; step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		if _, err := playback.playBeat(writer, pattern); err != nil {
			t.Fatal(err)
		}
		switch step {
		case 0:
			// The tie starts the note and nothing releases it afterwards on its own.
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 100})
			if len(writer.events) != 1 {
				t.Fatalf("step 1 wrote %d messages, want only the note-on", len(writer.events))
			}
		case 1, 2:
			// The gap the tie covers: silence on the wire, not a note-off.
			if len(writer.events) != 0 {
				t.Fatalf("step %d wrote %d messages, want none while the note is held", step+1, len(writer.events))
			}
		case 3:
			// Legato into the tied step: the next note starts before the first stops.
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 64, 100})
			assertMidiData(t, writer.events[1], []byte{midi.MakeNoteOff(0), 60, 0})
		default:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 64, 0})
		}
		writer.events = nil
	}
}

// A tie needs a note on both steps and nothing in between. A gesture that cannot tie is
// refused whole: the edit does not jump to a step whose note has nothing to hold, and the
// pattern is left as it was.
func TestNoteEditTieGestureRefusesWhenItCannotTie(t *testing.T) {
	bank, voice := chromaBank(t)
	pattern := bank.CurrentPattern()
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(4, voice, 64, 100)
	pattern.SetChromaticNote(6, voice, 65, 100)
	if err := bank.SelectTrackRow(1); err != nil {
		t.Fatal(err)
	}
	if err := bank.ToggleNoteMode(); err != nil {
		t.Fatal(err)
	}
	press := func(step, velocity int) {
		if err := processPatternEvent(nil, padMessage(stepCellPad(step), velocity)); err != nil {
			t.Fatal(err)
		}
	}

	// A cell on its own just moves the edit.
	press(0, 100)
	if bank.StepCursor() != 0 {
		t.Fatalf("the first cell press moved the edit to step %d", bank.StepCursor()+1)
	}
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("one cell press tied a step")
	}
	// Step 3 holds no note, so there is nothing for step 1 to hold on to.
	press(3, 100)
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("a step with no note was tied")
	}
	if bank.StepCursor() != 0 {
		t.Fatalf("a refused tie moved the edit to step %d", bank.StepCursor()+1)
	}
	// Step 5 would tie step 1 to step 7, but step 5's own note sits between them, and
	// tying across it would make that note unreachable. Refused, so the edit stays put.
	press(0, 0)
	press(6, 100)
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("a tie crossed an intervening note")
	}
	if bank.StepCursor() != 0 {
		t.Fatalf("a refused tie moved the edit to step %d", bank.StepCursor()+1)
	}
	if event, _ := pattern.EventAtStep(4, voice); event.Tie {
		t.Fatal("a refused tie landed on the wrong event")
	}
	// Releasing between presses ends the gesture, so two separate presses never tie.
	press(4, 0)
	press(0, 100)
	press(1, 100)
	press(1, 0)
	press(0, 0)
	if event, _ := pattern.EventAtStep(0, voice); event.Tie {
		t.Fatal("a released cell still took part in the gesture")
	}
}

// A tie between two notes of the same pitch reuses the sounding note, so the tied step sends
// nothing at all: one note-on for the whole chain, however long it runs. Releasing and
// retriggering instead would sound as two notes with a gap in them, which is what the step
// expiry would do if it did not know about the tie.
func TestSamePitchTieSendsOneNoteOnAcrossTheChain(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	for _, step := range []int{0, 3, 6} {
		pattern.SetChromaticNote(step, voice, 60, 100)
	}
	if !pattern.TieEventsAtSteps(0, 3, voice) {
		t.Fatal("tie 1-4 refused")
	}
	if !pattern.TieEventsAtSteps(3, 6, voice) {
		t.Fatal("tie 4-7 refused")
	}
	// Each tied step extends the note by one more step, so the chain covers steps 1 to 7 and
	// the note stops at step 8.
	want := map[int][]string{
		0: {"on 60"},
		1: nil,
		2: nil,
		3: nil,
		4: nil,
		5: nil,
		6: nil,
		7: {"off 60"},
	}
	assertChromaticSteps(t, voice, pattern, want)
}

// The same pitch without a tie is a fresh note, so the old one stops before the new one
// starts. The contrast is what makes the tied case above meaningful: both look alike on the
// grid until the tie is there or not.
func TestSamePitchWithoutATieRetriggers(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 100)
	pattern.SetChromaticNote(3, voice, 60, 100)
	want := map[int][]string{
		0: {"on 60"},
		1: {"off 60"},
		2: nil,
		3: {"on 60"},
		4: {"off 60"},
	}
	assertChromaticSteps(t, voice, pattern, want)
}

// assertChromaticSteps plays a pattern one step at a time and compares the MIDI each step
// wrote against what it was given, spelled as "on 60" or "off 60".
func assertChromaticSteps(t *testing.T, voice *Voice, pattern *Pattern, want map[int][]string) {
	t.Helper()
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step < len(want); step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		if _, err := playback.playBeat(writer, pattern); err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, event := range writer.events {
			kind := "on"
			if midi.IsNoteOff(event.Data[0]) {
				kind = "off"
			}
			got = append(got, fmt.Sprintf("%s %d", kind, event.Data[1]))
		}
		if strings.Join(got, ", ") != strings.Join(want[step], ", ") {
			t.Fatalf("step %d wrote [%s], want [%s]", step+1,
				strings.Join(got, ", "), strings.Join(want[step], ", "))
		}
		writer.events = nil
	}
}

// The velocity of a tied step is never heard, because a same-pitch tie writes nothing at all:
// no new note starts, so no new dynamics can. The note sounds with the velocity it was given
// at its own step, all the way to where it stops.
func TestSamePitchTieKeepsTheFirstVelocity(t *testing.T) {
	voice, _ := chromaticTestVoice(t, nil)
	pattern := &Pattern{}
	pattern.SetChromaticNote(0, voice, 60, 40)
	pattern.SetChromaticNote(3, voice, 60, midi.DataMax)
	if !pattern.TieEventsAtSteps(0, 3, voice) {
		t.Fatal("tie 1-4 refused")
	}
	writer := &captureMidiWriter{}
	playback := &Playback{active: make(map[*Voice]activeChromaticNote), writer: writer}
	for step := 0; step <= 4; step++ {
		playback.setPosition(stepBeat(step), stepBeat(step))
		if _, err := playback.playBeat(writer, pattern); err != nil {
			t.Fatal(err)
		}
		switch step {
		case 0:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOn(0), 60, 40})
			if len(writer.events) != 1 {
				t.Fatalf("step 1 wrote %d messages, want only the note-on", len(writer.events))
			}
		case 3:
			// The tied step is silent on the wire, so its velocity cannot be applied.
			if len(writer.events) != 0 {
				t.Fatalf("step 4 wrote %d messages, want none from a same-pitch tie", len(writer.events))
			}
		case 4:
			assertMidiData(t, writer.events[0], []byte{midi.MakeNoteOff(0), 60, 0})
		default:
			if len(writer.events) != 0 {
				t.Fatalf("step %d wrote %d messages, want none while the note is held", step+1, len(writer.events))
			}
		}
		writer.events = nil
	}
}
