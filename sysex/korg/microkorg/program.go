package microkorg

import (
	"github.com/chzchzchz/midispa/sysex"
)

const (
	// programNameSize is the fixed width of a program name in a dump.
	programNameSize = 12

	// programSize, timbreSize, vocoderSize and globalSize are the decoded
	// payload sizes of the four dump kinds, from the sheet's tables 1 to 3
	// and 6.
	programSize = 254
	timbreSize  = 108
	globalSize  = 200

	// vocoderSize is the length of the vocoder parameter block, which the
	// sheet contradicts itself about: table 1 allocates 38 to 141, which is
	// 104 bytes, while table 3's last field ends at offset 141, which is 142.
	// The ambiguity is harmless on the wire because a program is 254 bytes
	// either way and the block starts at byte 38 in both readings, so the
	// parameters land in the same place and the tail is padding. The longer
	// layout is used because table 3 is the table that actually itemises the
	// fields, and a caller that fills in all sixteen band levels and all
	// sixteen held envelope levels needs the room for them.
	vocoderSize = 142

	// vocoderOffset is where the voices start inside a program, in front of
	// the name, the arpeggiator and the two effect blocks.
	vocoderOffset = 38

	// unnamedFlagMask covers the bits of the local control byte that the
	// sheet marks unused but the instrument sets anyway.
	unnamedFlagMask = 0xfa

	// programCount is the number of slots in internal memory, addressed by
	// the program change map rather than by the A11...b88 panel numbering.
	programCount = 128

	programBankSize = programSize * programCount
	allDataSize     = programBankSize + globalSize
)

// Voice modes. The dump skips 1, so a layer program is VoiceModeLayer and not
// VoiceModeSingle+1.
const (
	VoiceModeSingle  = 0
	VoiceModeLayer   = 2
	VoiceModeVocoder = 3
)

// Program is one of the instrument's 128 memory slots, as laid out in the
// sheet's table 1.
//
// A program holds either two synth timbres or one vocoder, selected by
// VoiceMode. Timbre1 and Timbre2 are the synth form, Vocoder the other; the
// bytes the unused form would occupy are reserved in the record and are
// written as zero, so a dump taken from a synth program and re-encoded
// unchanged still round trips.
//
// Three blocks of the record carry no parameter the sheet names, but they are
// not padding: real dumps hold firmware data in them that varies from program
// to program. They are kept as raw bytes under Reserved so that a dump taken
// off an instrument comes back out of this package unchanged.
type Program struct {
	// Name is the program name, at most twelve characters. It is stored
	// space padded, and decoding strips trailing spaces and NULs.
	Name string

	// VoiceMode is 0 for a single synth voice, 2 for a two-voice layer and
	// 3 for a vocoder.
	VoiceMode int `oneof:"0,2,3"`

	// ScaleKey is the root of the user scale, 0 being C. It is only
	// consulted by the global user scale, which the sheet marks unused and
	// which the factory bank leaves at zero.
	ScaleKey int `range:"0..15"`

	// ScaleType selects equal temperament or a variant. The sheet names only
	// equal temperament and gives no upper bound beyond the four bits it
	// occupies.
	ScaleType int `range:"0..15"`

	// Reserved holds the three bytes the sheet calls dummy, at program
	// offsets 12, 13 and 18 in that order. They are not padding: the
	// factory bank holds 0...3, 0...8 and a constant 60 in them, varying by
	// program, so they are carried through rather than zeroed.
	Reserved [3]int `range:"0..255"`

	// TriggerLength is how many steps of the arpeggiator's pattern a single
	// note holds, 0 being one step and 7 being the whole pattern. It shares
	// its byte with Reserved2's neighbours and occupies the low three bits.
	TriggerLength int `range:"0..7"`

	// TriggerPattern is the arpeggiator's trigger pattern as a raw byte. The
	// sheet describes it as eight on/off steps but writes the cell as
	// "B0~7", so it is kept uninterpreted rather than guessed at.
	TriggerPattern int `range:"0..255"`

	// Arpeggio is the sequencer that plays the program.
	Arpeggio Arpeggio

	// DelayFx is the delay stage, downstream of the modulation effect.
	DelayFx DelayFx

	// ModFx is the modulation effect.
	ModFx ModFx

	// Eq is the two band output equaliser.
	Eq Eq

	// KeyboardOctave shifts the whole keyboard, 0 being untransposed and
	// -3 three octaves down. The dump stores it as a signed byte, so
	// -1 is stored as 0xFF and the factory bank is full of 254 and 255.
	KeyboardOctave int `range:"-3..3"`

	// Timbre1 is the first voice of a single or layer program.
	Timbre1 Timbre

	// Timbre2 is the second voice of a layer program.
	Timbre2 Timbre

	// Vocoder is the vocoder program, used when VoiceMode is 3.
	Vocoder Vocoder
}

// Arpeggio is the program's step sequencer, from the sheet's table 1. The
// microKORG arpeggiator is a global instrument feature: it plays a single
// stored pattern rather than gating the incoming notes one by one, and the
// pattern can be switched to trigger mode during playback.
type Arpeggio struct {
	// Tempo is the arpeggio rate in BPM, 20 to 300. It spans two bytes in
	// the dump because 300 does not fit in one.
	Tempo int `range:"20..300"`

	// On enables the arpeggiator.
	On int `oneof:"0,1"`

	// Latch holds the arpeggio running until it is stopped.
	Latch int `oneof:"0,1"`

	// Target chooses which voice a vocoder arpeggio drives: 0 both, 1 the
	// first, 2 the second.
	Target int `range:"0..2"`

	// KeySync restarts the pattern at every note played.
	KeySync int `oneof:"0,1"`

	// Type is 0 Up, 1 Down, 2 Alt1, 3 Alt2, 4 Random or 5 Trigger.
	Type int `range:"0..5"`

	// Range is the number of octaves the pattern spans, 0 being one.
	Range int `range:"0..3"`

	// GateTime is the note length as a percentage.
	GateTime int `range:"0..100"`

	// Resolution is the step division, 0 being a 1/24 note.
	Resolution int `range:"0..5"`

	// Swing delays every other step, 64 being no swing. The factory bank
	// leaves it at zero, which is the same setting.
	Swing int `range:"0..255"`
}

// DelayFx is the delay stage, from the sheet's table 1.
type DelayFx struct {
	// Sync locks the delay time to the sequencer.
	Sync int `oneof:"0,1"`

	// TimeBase is the note division the delay follows when Sync is on, 0
	// being a 1/32 note and 14 a whole note.
	TimeBase int `range:"0..14"`

	// Time is the delay time, used when Sync is off.
	Time int `range:"0..127"`

	// Depth is the delay level against the dry signal.
	Depth int `range:"0..127"`

	// Type is 0 stereo, 1 cross or 2 left/right.
	Type int `range:"0..2"`
}

// ModFx is the modulation effect, from the sheet's table 1.
type ModFx struct {
	// LfoSpeed is the rate of the effect's own oscillator.
	LfoSpeed int `range:"0..127"`

	// Depth is how deep the effect runs.
	Depth int `range:"0..127"`

	// Type is 0 chorus/flange, 1 ensemble or 2 phaser.
	Type int `range:"0..2"`
}

// Eq is the output equaliser, from the sheet's table 1. Both gains are
// centre-zero, 64 being flat.
type Eq struct {
	// HighFrequency is the high shelf corner in kHz, 0 being 1.00 and 29
	// being 18.0.
	HighFrequency int `range:"0..29"`

	// HighGain is the high shelf level, 64 being 0 dB.
	HighGain int `range:"52..76"`

	// LowFrequency is the low shelf corner in Hz, 0 being 40 and 29 being
	// 1000.
	LowFrequency int `range:"0..29"`

	// LowGain is the low shelf level, 64 being 0 dB.
	LowGain int `range:"52..76"`
}

// Patch is one of a voice's four virtual patch routes: a modulation source
// driving a destination by a signed depth. Two routes set to opposite signs
// and aimed at the same destination is how the microKORG does anything as
// simple as a filter sweep against a pan sweep.
type Patch struct {
	// Source picks the modulator: 0 filter EG, 1 amplifier EG, 2 LFO 1,
	// 3 LFO 2, 4 velocity, 5 keyboard tracking, 6 pitch bend or 7 the
	// modulation wheel.
	Source int `range:"0..7"`

	// Destination picks what it drives: 0 pitch, 1 oscillator 2 pitch,
	// 2 oscillator 1 control 1, 3 noise level, 4 cutoff, 5 amplifier,
	// 6 pan or 7 LFO 2 frequency.
	Destination int `range:"0..7"`

	// Intensity is the depth, 64 being none. The sign chooses the
	// direction.
	Intensity int `range:"0..255"`
}

// Timbre is one 108-byte synth voice, from the sheet's table 2. A program
// carries one of these, or two in layer mode.
//
// Every centre-zero parameter is stored as a raw seven-bit byte with 64 at the
// middle, matching the control change encoding the panel uses, so Tune, Transpose
// and the rest read the same here as they do on the instrument's own CC model.
type Timbre struct {
	// Channel is the MIDI channel this voice listens on. The sheet writes
	// -1 for global, which the factory bank stores as 0xFF.
	Channel int `range:"0..255"`

	// AssignMode is 0 mono, 1 poly or 2 unison.
	AssignMode int `oneof:"0,1,2"`

	// Eg2Reset restarts the amplifier envelope at every new note.
	Eg2Reset int `oneof:"0,1"`

	// Eg1Reset restarts the filter envelope at every new note.
	Eg1Reset int `oneof:"0,1"`

	// TriggerMode is 0 single or 1 multi, used in mono and unison.
	TriggerMode int `oneof:"0,1"`

	// KeyPriority is 0 for last note priority. The sheet names no other
	// value for the second bit, unlike the fields it lists as a closed set,
	// so the bit is carried rather than refused.
	KeyPriority int `range:"0..3"`

	// UnisonDetune spreads the unison voices in cents, 0 to 99.
	UnisonDetune int `range:"0..99"`

	// Tune detunes the voice in cents, 64 being 0 and the sheet quoting a
	// range of plus or minus 50.
	Tune int `range:"0..127"`

	// BendRange is the pitch bend depth in semitones, 64 being 0.
	BendRange int `range:"0..127"`

	// Transpose shifts the voice in semitones, 64 being 0 and the sheet
	// quoting plus or minus 24.
	Transpose int `range:"0..127"`

	// VibratoInt is the depth of the dedicated LFO 2 vibrato, 64 being none.
	VibratoInt int `range:"0..127"`

	// Osc1Wave is 0 Saw, 1 Pulse, 2 Tri, 3 Sine, 4 Vox Wave, 5 DWGS,
	// 6 Noise or 7 Audio In.
	Osc1Wave int `range:"0..7"`

	// Osc1Ctrl1 morphs the shape of Osc1Wave within its family, and is the
	// pulse width of a square. It does nothing while Osc1Wave is DWGS.
	Osc1Ctrl1 int `range:"0..127"`

	// Osc1Ctrl2 sweeps Osc1Ctrl1 with LFO 1, and selects the DWGS waveform
	// 1 to 64, two values per waveform, while Osc1Wave is DWGS.
	Osc1Ctrl2 int `range:"0..127"`

	// DwgsWave is the DWGS waveform number, 0 being number 1, stored
	// separately from Osc1Ctrl2 so that both readings of the knob survive a
	// round trip through a dump.
	DwgsWave int `range:"0..63"`

	// Osc2ModSelect is 0 off, 1 ring, 2 sync or 3 ring sync.
	Osc2ModSelect int `range:"0..3"`

	// Osc2Wave is 0 Saw, 1 square or 2 triangle.
	Osc2Wave int `range:"0..2"`

	// Osc2Semitone offsets oscillator 2 in semitones, 64 being unison and
	// the sheet quoting plus or minus 24.
	Osc2Semitone int `range:"0..127"`

	// Osc2Tune trims oscillator 2 in cents, 64 being 0.
	Osc2Tune int `range:"0..127"`

	// Portamento is the glide time.
	Portamento int `range:"0..127"`

	// Osc1Level is the oscillator 1 level.
	Osc1Level int `range:"0..127"`

	// Osc2Level is the oscillator 2 level.
	Osc2Level int `range:"0..127"`

	// NoiseLevel is the noise generator level.
	NoiseLevel int `range:"0..127"`

	// FilterType is 0 a 24dB/oct low pass, 1 a 12dB/oct low pass, 2 a
	// 12dB/oct band pass or 3 a 12dB/oct high pass.
	FilterType int `range:"0..3"`

	// Cutoff is the filter cutoff frequency.
	Cutoff int `range:"0..127"`

	// Resonance is the filter resonance.
	Resonance int `range:"0..127"`

	// FilterEgIntensity is how far the filter envelope swings the cutoff,
	// 64 being none and the sign choosing the direction.
	FilterEgIntensity int `range:"0..127"`

	// FilterVelocitySense is the filter's response to note velocity, 64
	// being none.
	FilterVelocitySense int `range:"0..255"`

	// FilterKeyboardTrack is the cutoff's response to key position, 64
	// being none.
	FilterKeyboardTrack int `range:"0..127"`

	// AmpLevel is the output level.
	AmpLevel int `range:"0..127"`

	// Panpot is the stereo position, 64 being centre.
	Panpot int `range:"0..127"`

	// AmpSwitch is 0 for an amplifier envelope and 1 for a gate.
	AmpSwitch int `oneof:"0,1"`

	// Distortion is the amplifier's drive.
	Distortion int `oneof:"0,1"`

	// AmpVelocitySense is the output level's response to note velocity, 64
	// being none.
	AmpVelocitySense int `range:"0..255"`

	// AmpKeyboardTrack is the output level's response to key position, 64
	// being none.
	AmpKeyboardTrack int `range:"0..127"`

	// FilterEgAttack, FilterEgDecay, FilterEgSustain and FilterEgRelease
	// shape the filter envelope, which the sheet calls EG1.
	FilterEgAttack  int `range:"0..127"`
	FilterEgDecay   int `range:"0..127"`
	FilterEgSustain int `range:"0..127"`
	FilterEgRelease int `range:"0..127"`

	// AmpEgAttack, AmpEgDecay, AmpEgSustain and AmpEgRelease shape the
	// amplifier envelope, which the sheet calls EG2.
	AmpEgAttack  int `range:"0..127"`
	AmpEgDecay   int `range:"0..127"`
	AmpEgSustain int `range:"0..127"`
	AmpEgRelease int `range:"0..127"`

	// Lfo1KeySync is 0 off, 1 restarting per timbre or 2 per note.
	Lfo1KeySync int `range:"0..2"`

	// Lfo1Wave is 0 Saw, 1 square, 2 triangle or 3 sample and hold.
	Lfo1Wave int `range:"0..3"`

	// Lfo1Frequency is the LFO 1 rate.
	Lfo1Frequency int `range:"0..127"`

	// Lfo1TempoSync locks Lfo1Frequency to the sequencer.
	Lfo1TempoSync int `oneof:"0,1"`

	// Lfo1SyncNote is the division Lfo1Frequency follows when it is synced,
	// 0 being a whole note and 14 a 1/32.
	Lfo1SyncNote int `range:"0..14"`

	// Lfo2KeySync is 0 off, 1 restarting per timbre or 2 per note.
	Lfo2KeySync int `range:"0..2"`

	// Lfo2Wave is 0 Saw, 1 narrow square, 2 sine or 3 sample and hold. The
	// square is narrower than LFO 1's.
	Lfo2Wave int `range:"0..3"`

	// Lfo2Frequency is the LFO 2 rate.
	Lfo2Frequency int `range:"0..127"`

	// Lfo2TempoSync locks Lfo2Frequency to the sequencer.
	Lfo2TempoSync int `oneof:"0,1"`

	// Lfo2SyncNote is the division Lfo2Frequency follows when it is synced.
	Lfo2SyncNote int `range:"0..14"`

	// Patches are the four virtual patch routes. The instrument indexes
	// them as an array too, storing the intensities at global offsets 32 to
	// 35, so they are modelled as one rather than as eight named fields.
	Patches [4]Patch

	// Reserved is the 56 bytes the sheet calls dummy after the patches.
	// They are not padding: real dumps fill all 56 with a firmware pattern
	// that varies per program, so they are carried through untouched rather
	// than being written as zero.
	Reserved [56]int `range:"0..255"`
}

// Vocoder is one 142-byte vocoder program, from the sheet's table 3. It
// replaces both synth timbres in the record rather than sitting alongside them.
//
// The sixteen analysis channels have a level and a pan each, and a held
// envelope level used when EFSense is on hold. The sheet describes the held
// level as four bytes per channel but names only four of the levels it can
// take, so EfHoldLevel is kept as the raw quad rather than being flattened.
type Vocoder struct {
	// Channel is the MIDI channel this program listens on, 0xFF being
	// global.
	Channel int `range:"0..255"`

	// AssignMode is 0 mono, 1 poly or 2 unison.
	AssignMode int `oneof:"0,1,2"`

	// Eg2Reset restarts the amplifier envelope at every new note.
	Eg2Reset int `oneof:"0,1"`

	// Eg1Reset is documented as fixed off for vocoder programs.
	Eg1Reset int `oneof:"0,1"`

	// TriggerMode is 0 single or 1 multi, used in mono and unison.
	TriggerMode int `oneof:"0,1"`

	// KeyPriority is 0 for last note priority. The sheet names no other
	// value for the second bit, unlike the fields it lists as a closed set,
	// so the bit is carried rather than refused.
	KeyPriority int `range:"0..3"`

	// UnisonDetune spreads the unison voices in cents.
	UnisonDetune int `range:"0..99"`

	// Tune detunes the program in cents, 64 being 0.
	Tune int `range:"0..127"`

	// BendRange is the pitch bend depth in semitones, 64 being 0.
	BendRange int `range:"0..127"`

	// Transpose shifts the program in semitones, 64 being 0.
	Transpose int `range:"0..127"`

	// VibratoInt is the depth of the dedicated LFO 2 vibrato, 64 being none.
	VibratoInt int `range:"0..127"`

	// Wave is the carrier waveform, using the same 0 to 7 encoding as the
	// synth timbre's Osc1Wave.
	Wave int `range:"0..7"`

	// Ctrl1 morphs the carrier shape, as the synth timbre's Osc1Ctrl1 does.
	Ctrl1 int `range:"0..127"`

	// Ctrl2 sweeps Ctrl1 with LFO 1, and selects the DWGS waveform while
	// Wave is DWGS.
	Ctrl2 int `range:"0..127"`

	// DwgsWave is the DWGS waveform number, 0 being number 1.
	DwgsWave int `range:"0..63"`

	// HpfGate is the AUDIO IN 1 high pass gate, 0 disabling it.
	HpfGate int `oneof:"0,1"`

	// Portamento is the glide time.
	Portamento int `range:"0..127"`

	// Osc1Level is the level of the internal tone generator, the vocoder's
	// carrier.
	Osc1Level int `range:"0..127"`

	// Ext1Level is the level of the external carrier on AUDIO IN 2.
	Ext1Level int `range:"0..127"`

	// NoiseLevel is the noise generator level.
	NoiseLevel int `range:"0..127"`

	// HpfLevel is the modulator high pass level on AUDIO IN 1.
	HpfLevel int `range:"0..127"`

	// GateSense is how strongly the gate follows the input level.
	GateSense int `range:"0..127"`

	// Threshold is the envelope follower's detection level, which decides
	// how loudly the vocoder responds to speech.
	Threshold int `range:"0..127"`

	// Shift moves the carrier bandpass filters as a block: 0 flat, 1 and 2
	// up, 3 and 4 down.
	Shift int `range:"0..4"`

	// Cutoff offsets every carrier bandpass filter, 64 being none.
	Cutoff int `range:"0..127"`

	// Resonance is the carrier filter resonance.
	Resonance int `range:"0..127"`

	// ModSource picks what modulates the carrier: 1 amplifier EG, 2 LFO 1,
	// 3 LFO 2, 4 velocity, 5 keyboard tracking, 6 pitch bend or 7 the
	// modulation wheel. Zero is the sheet's "no source", which is also what a
	// program that has never been a vocoder holds.
	ModSource int `range:"0..7"`

	// Intensity is the depth of that modulation, 64 being none.
	Intensity int `range:"0..127"`

	// EFSense is the analysis filter's sensitivity to the modulator. 127 is
	// the special hold setting that freezes the detected spectrum, which is
	// what makes a sustained chord out of a spoken one.
	EFSense int `range:"0..127"`

	// Level is the output level.
	Level int `range:"0..127"`

	// DirectLevel is the level of the carrier that bypasses the vocoder.
	DirectLevel int `range:"0..127"`

	// Distortion is the amplifier's drive.
	Distortion int `oneof:"0,1"`

	// VelSense is the output level's response to note velocity, 64 being
	// none.
	VelSense int `range:"0..127"`

	// KeyTrack is the output level's response to key position, 64 being
	// none.
	KeyTrack int `range:"0..127"`

	// FilterEgAttack, FilterEgDecay, FilterEgSustain and FilterEgRelease are
	// a filter envelope a vocoder has no use for, and the sheet records them
	// as fixed at 0, 0, 127 and 0. They are still carried as ordinary bytes
	// rather than being pinned to those values, because fifteen of the
	// sixteen vocoders in the factory bank hold them and the sixteenth stores
	// a sustain of 3. Rejecting that would refuse a dump the instrument
	// itself produced.
	FilterEgAttack  int `range:"0..127"`
	FilterEgDecay   int `range:"0..127"`
	FilterEgSustain int `range:"0..127"`
	FilterEgRelease int `range:"0..127"`

	// AmpEgAttack, AmpEgDecay, AmpEgSustain and AmpEgRelease are the
	// vocoder's own amplifier envelope.
	AmpEgAttack  int `range:"0..127"`
	AmpEgDecay   int `range:"0..127"`
	AmpEgSustain int `range:"0..127"`
	AmpEgRelease int `range:"0..127"`

	// Lfo1KeySync, Lfo1Wave and Lfo1Frequency match the synth timbre's LFO 1.
	Lfo1KeySync   int `range:"0..2"`
	Lfo1Wave      int `range:"0..3"`
	Lfo1Frequency int `range:"0..127"`

	// Lfo1TempoSync and Lfo1SyncNote match the synth timbre's LFO 1.
	Lfo1TempoSync int `oneof:"0,1"`
	Lfo1SyncNote  int `range:"0..14"`

	// Lfo2KeySync, Lfo2Wave and Lfo2Frequency match the synth timbre's LFO 2.
	Lfo2KeySync   int `range:"0..2"`
	Lfo2Wave      int `range:"0..3"`
	Lfo2Frequency int `range:"0..127"`

	// Lfo2TempoSync and Lfo2SyncNote match the synth timbre's LFO 2.
	Lfo2TempoSync int `oneof:"0,1"`
	Lfo2SyncNote  int `range:"0..14"`

	// BandLevel is the level of each of the sixteen analysis channels.
	BandLevel [16]int `range:"0..127"`

	// BandPan is the stereo position of each of the sixteen analysis
	// channels, 64 being centre.
	BandPan [16]int `range:"0..127"`

	// EfHoldLevel is the held envelope level of each analysis channel, used
	// when EFSense is on hold. The sheet names only its LOW, MID LOW, MID
	// HIGH and HIGH ends, so the four bytes per channel are kept as stored.
	// They use the top bit, and real dumps hold values well above 127.
	EfHoldLevel [16][4]int `range:"0..255"`
}

// Global is the 200-byte global block, from the sheet's table 6. It covers
// everything that is not per-program: tuning, the MIDI routing, the map from
// the panel knobs to control change numbers, and the map from program changes
// to memory slots.
//
// KnobControlChange is the same binding the cc package's MicroKorg model
// records as a factory default, in the order the instrument indexes them. It
// is here because it lives in global memory and survives a program change, and
// because reading it back is the only way to know which parameter a given
// control change number will actually reach.
type Global struct {
	// MasterTune is the master tuning in cents, 64 being A440.
	MasterTune int `range:"0..127"`

	// Transpose is the global transpose in semitones, 64 being none.
	Transpose int `range:"0..127"`

	// Position applies global transpose before or after the trigger.
	Position int `oneof:"0,1"`

	// VelocityValue is the note velocity the keyboard reports.
	VelocityValue int `range:"1..127"`

	// VelocityCurve is the velocity response, 8 being constant.
	VelocityCurve int `range:"0..8"`

	// LocalControl lets the keyboard's own panel settings take precedence
	// over incoming MIDI.
	LocalControl int `oneof:"0,1"`

	// MemoryProtect blocks writes to program memory.
	MemoryProtect int `oneof:"0,1"`

	// UnnamedFlags holds the bits of the local control byte outside
	// LocalControl and MemoryProtect. The sheet marks them unused but the
	// factory dump sets bits 1 and 3, so they are carried through rather
	// than cleared.
	UnnamedFlags int `range:"0..255"`

	// Reserved is bytes 6 and 7, which the sheet calls dummy. Byte 7 is not
	// zero in a factory dump, so it is carried through rather than cleared.
	Reserved [2]int `range:"0..255"`

	// Clock is 0 internal, 1 external or 2 auto.
	Clock int `range:"0..2"`

	// MidiChannel is the channel the instrument transmits on, 0 being 1.
	MidiChannel int `range:"0..15"`

	// SyncControlNumber is the control change that starts and stops the
	// arpeggiator, or ControlChangeUnassigned. The sheet writes the
	// unassigned value as -1; the factory bank stores 0xFF.
	SyncControlNumber int `range:"0..255"`

	// TimbreSelectNumber is the control change that switches vocoder
	// timbre, or ControlChangeUnassigned.
	TimbreSelectNumber int `range:"0..255"`

	// Midi1ControlNumber is what the pitch bend wheel sends, 0 being pitch
	// bend itself.
	Midi1ControlNumber int `range:"0..255"`

	// Midi2ControlNumber is what the modulation wheel sends, 3 being
	// control change 1.
	Midi2ControlNumber int `range:"0..255"`

	// SystemExclusiveFilter accepts incoming dumps.
	SystemExclusiveFilter int `oneof:"0,1"`

	// NoteReceive is 0 for all notes. It occupies two bits and the sheet
	// names only that one.
	NoteReceive int `range:"0..3"`

	// PitchBendFilter accepts incoming pitch bend.
	PitchBendFilter int `oneof:"0,1"`

	// ControlChangeFilter accepts incoming control changes. This is the
	// setting the whole cc model depends on, and it defaults to on.
	ControlChangeFilter int `oneof:"0,1"`

	// ProgramChangeFilter accepts incoming program changes.
	ProgramChangeFilter int `oneof:"0,1"`

	// KnobControlChange maps each panel knob and switch to a control change
	// number, in the instrument's own order, or ControlChangeUnassigned for
	// the two knobs the factory bank leaves unbound. The order is
	// portamento, oscillator 1 wave through cutoff, the two envelopes, both
	// LFOs, the four patch intensities, and the modulation and delay depths,
	// with the amplifier's envelope or gate switch and the sequencer switch
	// left unbound.
	KnobControlChange [42]int `range:"0..255"`

	// UserScale holds the twelve scale tunings in cents. The sheet marks
	// them unused and the factory bank leaves all twelve at zero.
	UserScale [12]int `range:"0..255"`

	// ProgramChangeMap sends incoming program change n to internal memory
	// slot ProgramChangeMap[n], which is how the A11...b88 panel numbering
	// is arranged.
	ProgramChangeMap [128]int `range:"0..127"`
}

// ProgramData is the edit buffer, sent with FuncCurrentProgramDump. Writing
// one loads it into the instrument without touching internal memory, which is
// what makes it the right message for auditioning a program.
type ProgramData struct {
	Channel int `range:"0..15"`
	Program Program
}

// MarshalBinary encodes a current program data dump.
func (p *ProgramData) MarshalBinary() ([]byte, error) {
	if err := checkChannel(p.Channel); err != nil {
		return nil, err
	}
	payload, err := p.Program.encode()
	if err != nil {
		return nil, err
	}
	return pack(p.Channel, FuncCurrentProgramDump, payload), nil
}

// ProgramBank is all 128 memory slots, sent with FuncProgramDump.
type ProgramBank struct {
	Channel  int `range:"0..15"`
	Programs [programCount]Program
}

// MarshalBinary encodes a program data dump.
func (b *ProgramBank) MarshalBinary() ([]byte, error) {
	if err := checkChannel(b.Channel); err != nil {
		return nil, err
	}
	payload := make([]byte, 0, programBankSize)
	for i := range b.Programs {
		program, err := b.Programs[i].encode()
		if err != nil {
			return nil, err
		}
		payload = append(payload, program...)
	}
	return pack(b.Channel, FuncProgramDump, payload), nil
}

// GlobalData is the global block, sent with FuncGlobalDump.
type GlobalData struct {
	Channel int `range:"0..15"`
	Global  Global
}

// MarshalBinary encodes a global data dump.
func (g *GlobalData) MarshalBinary() ([]byte, error) {
	if err := checkChannel(g.Channel); err != nil {
		return nil, err
	}
	payload, err := g.Global.encode()
	if err != nil {
		return nil, err
	}
	return pack(g.Channel, FuncGlobalDump, payload), nil
}

// AllData is every memory slot followed by the global block, sent with
// FuncAllDump. It is the one dump that round trips the instrument completely,
// at 37386 MIDI bytes.
type AllData struct {
	Channel  int `range:"0..15"`
	Programs [programCount]Program
	Global   Global
}

// MarshalBinary encodes an all data dump.
func (a *AllData) MarshalBinary() ([]byte, error) {
	if err := checkChannel(a.Channel); err != nil {
		return nil, err
	}
	payload := make([]byte, 0, allDataSize)
	for i := range a.Programs {
		program, err := a.Programs[i].encode()
		if err != nil {
			return nil, err
		}
		payload = append(payload, program...)
	}
	global, err := a.Global.encode()
	if err != nil {
		return nil, err
	}
	payload = append(payload, global...)
	return pack(a.Channel, FuncAllDump, payload), nil
}

// encode lays the program out in the 254 bytes of the sheet's table 1.
func (p *Program) encode() ([]byte, error) {
	if err := sysex.CheckTaggedFields(p); err != nil {
		return nil, err
	}
	if len(p.Name) > programNameSize {
		return nil, sysex.ErrBadRange
	}

	ret := make([]byte, 0, programSize)
	// 0..11: program name, space padded to its fixed width.
	name := make([]byte, programNameSize)
	for i := range name {
		name[i] = ' '
	}
	copy(name, p.Name)
	ret = append(ret, name...)
	// 12,13: firmware data the sheet calls dummy, carried through as read.
	ret = append(ret, byte(p.Reserved[0]), byte(p.Reserved[1]))
	// 14: trigger length in the low three bits.
	ret = append(ret, byte(p.TriggerLength&0x07))
	// 15: arpeggiator trigger pattern.
	ret = append(ret, byte(p.TriggerPattern))
	// 16: a fixed 01 in bits 6 and 7, which every program in the factory
	// bank holds, and the voice mode in bits 4 and 5.
	ret = append(ret, byte(0x40)|byte(p.VoiceMode<<4))
	// 17: scale key in the high nibble, scale type in the low nibble.
	ret = append(ret, byte(p.ScaleKey<<4)|byte(p.ScaleType&0x0f))
	// 18: firmware data the sheet calls a dummy byte.
	ret = append(ret, byte(p.Reserved[2]))
	// 19: delay sync in bit 7, delay note division in the low nibble.
	ret = append(ret, byte(p.DelayFx.Sync)<<7|byte(p.DelayFx.TimeBase&0x0f))
	// 20,21,22: delay time, depth and type.
	ret = append(ret, byte(p.DelayFx.Time), byte(p.DelayFx.Depth), byte(p.DelayFx.Type))
	// 23,24,25: modulation effect LFO speed, depth and type.
	ret = append(ret, byte(p.ModFx.LfoSpeed), byte(p.ModFx.Depth), byte(p.ModFx.Type))
	// 26..29: equaliser shelf frequencies and gains.
	ret = append(ret,
		byte(p.Eq.HighFrequency),
		byte(p.Eq.HighGain),
		byte(p.Eq.LowFrequency),
		byte(p.Eq.LowGain))
	// 30,31: arpeggiator tempo as a plain sixteen bit value, most
	// significant byte first, because the 300 BPM ceiling does not fit in
	// one. The low byte is not limited to seven bits: the factory bank
	// stores tempos up to 240 there.
	ret = append(ret, byte(p.Arpeggio.Tempo>>8), byte(p.Arpeggio.Tempo))
	// 32: arpeggiator on, latch and target in bits 7, 6 and 4,5, key sync
	// in bit 0.
	ret = append(ret, byte(p.Arpeggio.On)<<7|byte(p.Arpeggio.Latch)<<6|
		byte(p.Arpeggio.Target&0x03)<<4|byte(p.Arpeggio.KeySync))
	// 33: arpeggiator type in the low nibble, range in the high nibble.
	ret = append(ret, byte(p.Arpeggio.Range&0x0f)<<4|byte(p.Arpeggio.Type&0x0f))
	// 34..36: arpeggiator gate time, resolution and swing.
	ret = append(ret, byte(p.Arpeggio.GateTime), byte(p.Arpeggio.Resolution), byte(p.Arpeggio.Swing))
	// 37: keyboard octave, stored as a signed byte so that -1 is 0xFF.
	ret = append(ret, byte(p.KeyboardOctave))
	// 38..253: the voices. A vocoder occupies the first 104 of those bytes
	// and the rest are reserved, whereas a synth program uses both halves.
	if p.VoiceMode == VoiceModeVocoder {
		vocoder, err := p.Vocoder.encode()
		if err != nil {
			return nil, err
		}
		ret = append(ret, vocoder...)
		ret = append(ret, make([]byte, programSize-vocoderOffset-vocoderSize)...)
		return ret, nil
	}
	first, err := p.Timbre1.encode()
	if err != nil {
		return nil, err
	}
	second, err := p.Timbre2.encode()
	if err != nil {
		return nil, err
	}
	return append(ret, append(first, second...)...), nil
}

// encode lays a synth voice out in the 108 bytes of the sheet's table 2.
func (t *Timbre) encode() ([]byte, error) {
	if err := sysex.CheckTaggedFields(t); err != nil {
		return nil, err
	}
	ret := make([]byte, 0, timbreSize)
	ret = append(ret, byte(t.Channel))
	// +1: assign mode, both envelope resets, trigger mode and key priority
	// share one byte.
	ret = append(ret, byte(t.AssignMode)<<6|byte(t.Eg2Reset)<<5|byte(t.Eg1Reset)<<4|
		byte(t.TriggerMode)<<3|byte(t.KeyPriority&0x03))
	// +2: unison detune.
	ret = append(ret, byte(t.UnisonDetune))
	// +3..+6: tune, bend range, transpose and vibrato intensity.
	ret = append(ret,
		byte(t.Tune),
		byte(t.BendRange),
		byte(t.Transpose),
		byte(t.VibratoInt))
	// +7..+11: oscillator 1 wave, its two controllers, the DWGS waveform
	// number and a reserved byte.
	ret = append(ret,
		byte(t.Osc1Wave),
		byte(t.Osc1Ctrl1),
		byte(t.Osc1Ctrl2),
		byte(t.DwgsWave),
		0)
	// +12: oscillator 2 mod mode in bits 4,5 and its wave in bits 0,1.
	ret = append(ret, byte(t.Osc2ModSelect&0x03)<<4|byte(t.Osc2Wave&0x03))
	// +13..+15: oscillator 2 semitone and tune, then portamento.
	ret = append(ret, byte(t.Osc2Semitone), byte(t.Osc2Tune), byte(t.Portamento&0x7f))
	// +16..+21: mixer levels, filter type, cutoff and resonance.
	ret = append(ret,
		byte(t.Osc1Level),
		byte(t.Osc2Level),
		byte(t.NoiseLevel),
		byte(t.FilterType),
		byte(t.Cutoff),
		byte(t.Resonance))
	// +22..+24: filter envelope intensity, velocity sense and key tracking.
	ret = append(ret,
		byte(t.FilterEgIntensity),
		byte(t.FilterVelocitySense),
		byte(t.FilterKeyboardTrack))
	// +25,26: amplifier level and pan.
	ret = append(ret, byte(t.AmpLevel), byte(t.Panpot))
	// +27: amplifier switch in bit 6 and drive in bit 0.
	ret = append(ret, byte(t.AmpSwitch)<<6|byte(t.Distortion))
	// +28,29: amplifier velocity sense and key tracking.
	ret = append(ret, byte(t.AmpVelocitySense), byte(t.AmpKeyboardTrack))
	// +30..+33: filter envelope, which the sheet calls EG1.
	ret = append(ret,
		byte(t.FilterEgAttack),
		byte(t.FilterEgDecay),
		byte(t.FilterEgSustain),
		byte(t.FilterEgRelease))
	// +34..+37: amplifier envelope, which the sheet calls EG2.
	ret = append(ret,
		byte(t.AmpEgAttack),
		byte(t.AmpEgDecay),
		byte(t.AmpEgSustain),
		byte(t.AmpEgRelease))
	// +38..+43: the two LFOs, each a packed sync byte, a rate and another
	// packed byte carrying tempo sync and the note division.
	ret = append(ret,
		byte(t.Lfo1KeySync&0x03)<<4|byte(t.Lfo1Wave&0x03),
		byte(t.Lfo1Frequency),
		byte(t.Lfo1TempoSync)<<7|byte(t.Lfo1SyncNote&0x0f),
		byte(t.Lfo2KeySync&0x03)<<4|byte(t.Lfo2Wave&0x03),
		byte(t.Lfo2Frequency),
		byte(t.Lfo2TempoSync)<<7|byte(t.Lfo2SyncNote&0x0f))
	// +44..+51: the four virtual patches, each a byte of destination and
	// source and a byte of intensity.
	for _, patch := range t.Patches {
		ret = append(ret, byte(patch.Destination&0x07)<<4|byte(patch.Source&0x07), byte(patch.Intensity))
	}
	// +52..+107: firmware data, carried through as read.
	for _, value := range t.Reserved {
		ret = append(ret, byte(value))
	}
	return ret, nil
}

// encode lays a vocoder program out in the bytes of the sheet's table 3.
func (v *Vocoder) encode() ([]byte, error) {
	if err := sysex.CheckTaggedFields(v); err != nil {
		return nil, err
	}
	ret := make([]byte, 0, vocoderSize)
	ret = append(ret, byte(v.Channel))
	// +1: assign mode, both envelope resets, trigger mode and key priority.
	ret = append(ret, byte(v.AssignMode)<<6|byte(v.Eg2Reset)<<5|byte(v.Eg1Reset)<<4|
		byte(v.TriggerMode)<<3|byte(v.KeyPriority&0x03))
	// +2..+6: unison detune, tune, bend range, transpose and vibrato.
	ret = append(ret,
		byte(v.UnisonDetune),
		byte(v.Tune),
		byte(v.BendRange),
		byte(v.Transpose),
		byte(v.VibratoInt))
	// +7..+11: carrier wave, its two controllers, the DWGS waveform number
	// and a reserved byte.
	ret = append(ret,
		byte(v.Wave),
		byte(v.Ctrl1),
		byte(v.Ctrl2),
		byte(v.DwgsWave),
		0)
	// +12,13: AUDIO IN 1 high pass gate in bit 0, then a reserved byte.
	ret = append(ret, byte(v.HpfGate), 0)
	// +14: portamento.
	ret = append(ret, byte(v.Portamento))
	// +15..+20: carrier and external levels, noise, and the AUDIO IN 1 high
	// pass level, gate sense and threshold.
	ret = append(ret,
		byte(v.Osc1Level),
		byte(v.Ext1Level),
		byte(v.NoiseLevel),
		byte(v.HpfLevel),
		byte(v.GateSense),
		byte(v.Threshold))
	// +21..+23: formant shift, cutoff offset and resonance.
	ret = append(ret, byte(v.Shift), byte(v.Cutoff), byte(v.Resonance))
	// +24,25: modulation source and depth.
	ret = append(ret, byte(v.ModSource), byte(v.Intensity))
	// +26: analysis filter sensitivity, where 127 is the hold setting.
	ret = append(ret, byte(v.EFSense))
	// +27..+31: output level, direct level, drive, velocity sense and key
	// tracking.
	ret = append(ret,
		byte(v.Level),
		byte(v.DirectLevel),
		byte(v.Distortion),
		byte(v.VelSense),
		byte(v.KeyTrack))
	// +32..+35: the filter envelope, which the hardware fixes at 0, 0, 127
	// and 0, and +36..+39 the real amplifier envelope.
	ret = append(ret,
		byte(v.FilterEgAttack),
		byte(v.FilterEgDecay),
		byte(v.FilterEgSustain),
		byte(v.FilterEgRelease),
		byte(v.AmpEgAttack),
		byte(v.AmpEgDecay),
		byte(v.AmpEgSustain),
		byte(v.AmpEgRelease))
	// +40..+45: the two LFOs, laid out as they are in the synth timbre.
	ret = append(ret,
		byte(v.Lfo1KeySync&0x03)<<4|byte(v.Lfo1Wave&0x03),
		byte(v.Lfo1Frequency),
		byte(v.Lfo1TempoSync)<<7|byte(v.Lfo1SyncNote&0x0f),
		byte(v.Lfo2KeySync&0x03)<<4|byte(v.Lfo2Wave&0x03),
		byte(v.Lfo2Frequency),
		byte(v.Lfo2TempoSync)<<7|byte(v.Lfo2SyncNote&0x0f))
	// +46..+61 and +62..+77: the level and the pan of each of the sixteen
	// analysis channels.
	for i := 0; i < 16; i++ {
		ret = append(ret, byte(v.BandLevel[i]), byte(v.BandPan[i]))
	}
	// +78..+141: the held envelope level of each channel, four bytes apiece.
	for i := 0; i < 16; i++ {
		ret = append(ret,
			byte(v.EfHoldLevel[i][0]),
			byte(v.EfHoldLevel[i][1]),
			byte(v.EfHoldLevel[i][2]),
			byte(v.EfHoldLevel[i][3]))
	}
	return ret, nil
}

// encode lays the global block out in the 200 bytes of the sheet's table 6.
func (g *Global) encode() ([]byte, error) {
	if err := sysex.CheckTaggedFields(g); err != nil {
		return nil, err
	}
	ret := make([]byte, 0, globalSize)
	// 0,1: master tune and transpose.
	ret = append(ret, byte(g.MasterTune), byte(g.Transpose))
	// 2: transpose position in bit 0.
	ret = append(ret, byte(g.Position))
	// 3,4: velocity value and curve.
	ret = append(ret, byte(g.VelocityValue), byte(g.VelocityCurve))
	// 5: local control in bit 2 and memory protect in bit 0, with the bits
	// the sheet calls unused in between and above them carried through.
	ret = append(ret, byte(g.LocalControl)<<2|byte(g.MemoryProtect)|byte(g.UnnamedFlags)&unnamedFlagMask)
	// 6,7: firmware data the sheet calls dummy.
	ret = append(ret, byte(g.Reserved[0]), byte(g.Reserved[1]))
	// 8: clock source in bits 0,1.
	ret = append(ret, byte(g.Clock))
	// 9: MIDI channel in the low nibble.
	ret = append(ret, byte(g.MidiChannel&0x0f))
	// 10,11: the control change numbers bound to arpeggiator sync and
	// vocoder timbre select.
	ret = append(ret, byte(g.SyncControlNumber), byte(g.TimbreSelectNumber))
	// 12,13: reserved.
	ret = append(ret, 0, 0)
	// 14,15: what the two performance wheels transmit.
	ret = append(ret, byte(g.Midi1ControlNumber), byte(g.Midi2ControlNumber))
	// 16: system exclusive filter in bit 7, note receive in bits 0,1.
	ret = append(ret, byte(g.SystemExclusiveFilter)<<7|byte(g.NoteReceive&0x03))
	// 17: pitch bend, control change and program change filters in bits 6,
	// 2 and 0.
	ret = append(ret, byte(g.PitchBendFilter)<<6|byte(g.ControlChangeFilter)<<2|byte(g.ProgramChangeFilter))
	// 18..59: the control change number bound to each panel knob.
	for _, number := range g.KnobControlChange {
		ret = append(ret, byte(number))
	}
	// 60..71: the unused user scale tunings.
	for _, cents := range g.UserScale {
		ret = append(ret, byte(cents))
	}
	// 72..199: the program change to memory slot map.
	for _, slot := range g.ProgramChangeMap {
		ret = append(ret, byte(slot))
	}
	return ret, nil
}
