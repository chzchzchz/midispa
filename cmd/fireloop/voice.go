package main

type Voice struct {
	Name    string
	Note    *int
	Channel int // [1,16] if set; otherwise use the device channel
	// Patch names a .mid or .smf file played to this voice's device when playback
	// starts. It overrides the device's Patch, the way Channel overrides its channel.
	Patch string

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

// protocolChannel converts a channel as a kit states it into the index the MIDI protocol
// numbers channels with. A kit says 1-16, because that is the numbering a musician reads
// off their controller and sees in the files they dump; the wire counts from zero. This is
// the one place the two meet, so a note and a patch sent for the same voice cannot disagree
// about which channel they are on.
func protocolChannel(channel int) int {
	return channel - 1
}

// resolvePatch is the same precedence for a patch: a voice names its own file or inherits
// the device's. There is no way back to the device patch once a voice has overridden it.
func resolvePatch(devicePatch, voicePatch string) string {
	if voicePatch != "" {
		return voicePatch
	}
	return devicePatch
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

// EffectivePatch resolves a voice override before falling back to its device.
func (v *Voice) EffectivePatch() string {
	if v == nil {
		return ""
	}
	devicePatch := ""
	if v.device != nil {
		devicePatch = v.device.Patch
	}
	return resolvePatch(devicePatch, v.Patch)
}

// patchPath is the voice's effective patch resolved to a file, or "" when it has none.
func (v *Voice) patchPath() string {
	if v == nil {
		return ""
	}
	return resolvePatchPath(v.device, v.EffectivePatch())
}

type VoiceBank struct {
	voices []*Voice
	// patches keeps the kit's patch files read across sends rather than repeated on
	// every Play. It belongs to the bank because the bank is what a kit turns into, so
	// the files live as long as the kit does.
	patches patchCache
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

// voiceAt resolves a position in the flattened bank, which is the only identity a voice
// has. An index outside the bank is reported rather than clamped: clamping would put the
// note on whichever voice happens to sit at the end, and a wrong drum is worse than a
// missing one.
func (v *VoiceBank) voiceAt(index int) (*Voice, bool) {
	if v == nil || index < 0 || index >= len(v.voices) {
		return nil, false
	}
	return v.voices[index], true
}

// voiceIndex is the same identity read the other way. A save walks every event in a
// pattern and needs the position of each voice, so the mapping is built once and reused
// rather than searched per event. Only the flattened order distinguishes two voices that
// share a name, so that is what the map is keyed by.
func (v *VoiceBank) voiceIndex() map[*Voice]int {
	if v == nil {
		return nil
	}
	indices := make(map[*Voice]int, len(v.voices))
	for index, voice := range v.voices {
		indices[voice] = index
	}
	return indices
}
