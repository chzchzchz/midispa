package main

type Voice struct {
	Name    string
	Note    *int
	Channel int // [1,16] if set; otherwise use the device channel

	device *Device // backpointer
}

// IsChromatic reports whether the voice receives pitches from each event.
func (v *Voice) IsChromatic() bool {
	return v != nil && v.Note == nil
}

// PercussionNote returns the configured drum note, including explicit note zero.
func (v *Voice) PercussionNote() (int, bool) {
	if v == nil || v.Note == nil {
		return 0, false
	}
	return *v.Note, true
}

// resolveChannel centralizes the voice-over-device precedence used by validation and playback.
func resolveChannel(deviceChannel, voiceChannel int) int {
	if voiceChannel != 0 {
		return voiceChannel
	}
	return deviceChannel
}

// EffectiveChannel resolves a voice override before falling back to its device.
func (v *Voice) EffectiveChannel() int {
	if v == nil {
		return 0
	}
	deviceChannel := 0
	if v.device != nil {
		deviceChannel = v.device.Channel
	}
	return resolveChannel(deviceChannel, v.Channel)
}

type VoiceBank struct {
	voices []*Voice
}

func NewVoiceBank(devs []Device) *VoiceBank {
	vb := &VoiceBank{}
	for i := range devs {
		device := &devs[i]
		for j := range device.Voices {
			voice := &device.Voices[j]
			voice.device = device
			vb.voices = append(vb.voices, voice)
		}
	}
	return vb
}
