package main

import (
	"encoding/json"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

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
	assertMidiData(t, alsa.SeqEvent{Data: messages[1]}, []byte{midi.MakeNoteOn(8), 36, 90})
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
