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

// rangedChromaBank is the bank a test about a chromatic voice with a
// declared range starts from: one chromatic voice bounded to [32,96].
func rangedChromaBank(t *testing.T) (*PatternBank, *Voice) {
	t.Helper()
	kit := NewVoiceBank([]Device{{
		Channel: 1,
		Voices:  []Voice{{Name: "lead", Channel: 1, VelocityMin: 32, VelocityMax: 96}},
	}})
	bank := newTestBank(t, NewFire(func([]byte) error { return nil }), kit)
	return bank, kit.voices[0]
}

// The range is written in the kit file as bare field names, the way
// the rest of the voice is, and a voice that declares none keeps
// both fields zero.
func TestVoiceVelocityRangeDecodesFromTheKitFile(t *testing.T) {
	var device Device
	contents := `{"Name":"kit","MidiPort":"out","Channel":1,` +
		`"Voices":[{"Name":"lead","VelocityMin":32,"VelocityMax":96},{"Name":"drums","Note":36}]}`
	require.NoError(t, json.Unmarshal([]byte(contents), &device))
	require.NoError(t, validateDevices([]Device{device}))
	require.Equal(t, 32, device.Voices[0].VelocityMin, "the floor")
	require.Equal(t, 96, device.Voices[0].VelocityMax, "the cap")
	require.Zero(t, device.Voices[1].VelocityMin, "a voice with no range has no floor")
	require.Zero(t, device.Voices[1].VelocityMax, "a voice with no range has no cap")
}

// VelocityBounds resolves what the two fields say into the range the
// voice actually plays: a zero field leaves the data range on its
// side, and a range that would cross itself folds to the cap, which
// is the one velocity a hand-built bank with an inverted range can
// still play.
func TestVelocityBoundsResolvesTheDeclaredRange(t *testing.T) {
	tests := []struct {
		name           string
		min, max       int
		floor, ceiling int
	}{
		{name: "no range", floor: minStepVelocity, ceiling: midiNoteMax},
		{name: "a floor only", min: 32, floor: 32, ceiling: midiNoteMax},
		{name: "a cap only", max: 96, floor: minStepVelocity, ceiling: 96},
		{name: "a floor and a cap", min: 32, max: 96, floor: 32, ceiling: 96},
		{name: "the whole range", min: 1, max: 127, floor: 1, ceiling: midiNoteMax},
		{name: "a point range", min: 40, max: 40, floor: 40, ceiling: 40},
		{name: "an inverted range folds to the cap", min: 96, max: 32, floor: 32, ceiling: 32},
		{name: "values outside the data range", min: -5, max: 200, floor: minStepVelocity, ceiling: midiNoteMax},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			voice := &Voice{VelocityMin: tt.min, VelocityMax: tt.max}
			floor, ceiling := voice.VelocityBounds()
			require.Equalf(t, tt.floor, floor, "the floor")
			require.Equalf(t, tt.ceiling, ceiling, "the ceiling")
		})
	}
}

// ScaledPadVelocity maps a pad press onto the voice's range, so a
// press of 127 is the cap and a press of zero the floor. With no
// declared range the map is the identity for every press a pad can
// send, which is what keeps a rangeless kit behaving as it did.
func TestScaledPadVelocityMapsThePadOntoTheRange(t *testing.T) {
	plain := &Voice{}
	for press := 1; press <= midiNoteMax; press++ {
		require.Equalf(t, press, plain.ScaledPadVelocity(press), "press %d on a rangeless voice", press)
	}
	require.Equal(t, minStepVelocity, plain.ScaledPadVelocity(0),
		"a press of zero is kept off the note-off")

	ranged := &Voice{VelocityMin: 32, VelocityMax: 96}
	tests := []struct {
		press int
		want  int
	}{
		{press: midiNoteMax, want: 96},
		{press: 64, want: 64},
		{press: 1, want: 32},
		{press: 0, want: 32},
		{press: 200, want: 96},
		{press: -5, want: 32},
	}
	for _, tt := range tests {
		require.Equalf(t, tt.want, ranged.ScaledPadVelocity(tt.press), "a press of %d", tt.press)
	}
}

// AbsolutePadVelocity is the inverse of ScaledPadVelocity, and the
// round trip has to be exact at every velocity the voice can play:
// the chromatic knob carries the absolute value between steps, so a
// round trip that lost a step would make a careful detent stall.
func TestAbsolutePadVelocityRoundTripsEveryPlayableVelocity(t *testing.T) {
	tests := []struct {
		name     string
		min, max int
	}{
		{name: "no range"},
		{name: "the whole range", min: 1, max: 127},
		{name: "a floor only", min: 32},
		{name: "a cap only", max: 96},
		{name: "a narrow range", min: 32, max: 96},
		{name: "a point range", min: 40, max: 40},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			voice := &Voice{VelocityMin: tt.min, VelocityMax: tt.max}
			floor, ceiling := voice.VelocityBounds()
			for stored := floor; stored <= ceiling; stored++ {
				absolute := voice.AbsolutePadVelocity(stored)
				require.Equalf(t, stored, voice.ScaledPadVelocity(absolute),
					"velocity %d came back as %d through the absolute %d",
					stored, voice.ScaledPadVelocity(absolute), absolute)
			}
		})
	}
}
