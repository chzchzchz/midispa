package main

type Voice struct {
	Name    string
	Note    *int
	Channel int // [1,16] if set; otherwise use the device channel
	// Patch names a .mid or .smf file played to this voice's device when playback
	// starts. It overrides the device's Patch, the way Channel overrides its
	// channel. The file is read as a Standard MIDI File, or as a raw MIDI
	// stream when it is not one.
	Patch string
	// VelocityMin and VelocityMax bound the velocities this voice plays,
	// written in the kit file as bare field names the way the rest of the
	// voice is. Zero means unset: no floor beyond the note-off rule and no
	// cap, so the voice plays the whole data range.
	VelocityMin int
	VelocityMax int

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

// VelocityBounds resolves the voice's effective velocity range. A zero
// VelocityMax means no cap; a zero VelocityMin means no floor beyond the
// note-off rule, which keeps any voice off the note-off at velocity zero: a
// note-on with velocity zero is a note-off on the wire, for a drum as for a
// synth.
func (v *Voice) VelocityBounds() (int, int) {
	floor := max(clampMidiDataValue(v.VelocityMin), minStepVelocity)
	ceiling := midiNoteMax
	if v.VelocityMax > 0 {
		ceiling = clampMidiDataValue(v.VelocityMax)
	}
	if floor > ceiling {
		// Validation refuses an inverted range at kit load, so this only
		// guards a hand-built bank: folding rather than refusing keeps a
		// clamp from preferring a floor the voice cannot reach.
		floor = ceiling
	}
	return floor, ceiling
}

// ScaledPadVelocity maps a pad press onto the voice's range, so a soft press
// on a ranged voice stays soft relative to what that instrument can play. Pure
// integer arithmetic with truncating division: the fastest form, and exact at
// both endpoints, a press of 127 landing on the cap and a press of zero on
// the floor. With no declared range the map is the identity for every press a
// pad can send, which is what keeps a kit without ranges behaving exactly as
// before.
func (v *Voice) ScaledPadVelocity(padVelocity int) int {
	floor, ceiling := v.VelocityBounds()
	return floor + (ceiling-floor)*clampMidiDataValue(padVelocity)/midiNoteMax
}

// AbsolutePadVelocity is the inverse of ScaledPadVelocity: the absolute MIDI
// velocity that scales to the given one. Ceiling division makes the round trip
// exact, so a knob carry never stalls on a narrow range. Only the chromatic
// knob's carry is absolute, so only it calls this. A point range returns zero:
// every absolute lands on the one velocity anyway, and zero keeps the carry at
// the bottom of the scale.
func (v *Voice) AbsolutePadVelocity(velocity int) int {
	floor, ceiling := v.VelocityBounds()
	span := ceiling - floor
	if span <= 0 {
		return 0
	}
	return ((clampStepVelocity(v, velocity)-floor)*midiNoteMax + span - 1) / span
}

// clampVelocity keeps a velocity inside the voice's bounds.
func (v *Voice) clampVelocity(velocity int) int {
	floor, ceiling := v.VelocityBounds()
	return min(max(velocity, floor), ceiling)
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
