package main

import (
	"encoding/json"
	"testing"

	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
	"github.com/stretchr/testify/require"
)

func TestVoiceNoteOmissionSelectsChromaticMode(t *testing.T) {
	var device Device
	require.NoError(t, json.Unmarshal([]byte(`{"Name":"kit","MidiPort":"out","Channel":1,"Voices":[{"Name":"lead"},{"Name":"zero","Note":0}]}`), &device))
	require.NoError(t, validateDevices([]Device{device}))
	require.True(t, device.Voices[0].IsChromatic(), "an omitted Note did not select chromatic mode")
	require.False(t, device.Voices[1].IsChromatic(), "an explicit Note 0 selected chromatic mode")
	note, ok := device.Voices[1].PercussionNote()
	require.True(t, ok, "an explicit Note 0 is not a percussion note")
	require.Zero(t, note)
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
	require.NoError(t, validateDevices([]Device{device}))
	voiceBank := NewVoiceBank([]Device{device})
	require.Equal(t, 1, voiceBank.voices[0].EffectiveChannel(), "a voice with no channel of its own")
	require.Equal(t, 9, voiceBank.voices[1].EffectiveChannel(), "a percussive voice's own channel")
	require.Equal(t, 11, voiceBank.voices[2].EffectiveChannel(), "a chromatic voice's own channel")
	percussion := Event{Voice: voiceBank.voices[1], Velocity: 90}
	messages := percussion.ToMidi()
	assertMidiData(t, alsa.SeqEvent{Data: messages[0]}, []byte{midi.MakeNoteOff(8), 36, 90})
	assertMidiData(t, alsa.SeqEvent{Data: messages[1]}, []byte{midi.MakeNoteOn(8), 36, 90})
	chromatic := Event{Voice: voiceBank.voices[2], ChromaticNote: 60, Velocity: 77}
	assertMidiData(t, alsa.SeqEvent{Data: chromatic.NoteOnMidi()}, []byte{midi.MakeNoteOn(10), 60, 77})

	device.Channel = 0
	require.Error(t, validateDevices([]Device{device}), "a device without a channel should require voice overrides")
	device.Voices[0].Channel = 1
	device.Voices[2].Channel = 0
	require.Error(t, validateDevices([]Device{device}), "a voice without an effective channel was accepted")
}
