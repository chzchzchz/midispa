package pro800

import (
	"errors"

	"github.com/chzchzchz/midispa/sysex"
)

// ErrNotRepresentable is returned when a patch holds a value that the dump
// layout it is being written in has nowhere to put. It is distinct from a
// range error because no value of the field would be acceptable: the
// information the caller set is not something a 6E dump can carry, so
// dropping it silently would hand back a message that says something the
// caller did not ask for.
var ErrNotRepresentable = errors.New("value has no place in this dump layout")

const (
	// parameterSize is where the program name starts, and so the length of
	// the block of parameters every patch carries in both layouts.
	parameterSize = 150

	// nameSize is the width of the name field. In the 6F layout the name
	// always fills it, and the instrument's own panel shows sixteen
	// characters, so a 6E name is held to the same limit.
	nameSize = 16

	// patch6FSize is the decoded length of a 6F patch: the parameters, a
	// sixteen byte name, the LFO aftertouch amount and the four settings
	// that version added.
	patch6FSize = 173
)

// LFO target bits. The later notes and the Behringer preset structure
// document agree that the amplifier is bit 5; the earlier notes place it at
// bit 6 and leave bit 5 unexplained, which is the same off by one error the
// two documents share in the program layout.
const (
	LfoTargetFrequency   = 0x01
	LfoTargetFilter      = 0x02
	LfoTargetPulseWidth  = 0x04
	LfoTargetOscillatorA = 0x08
	LfoTargetOscillatorB = 0x10
	LfoTargetAmplifier   = 0x20
)

// Oscillator is one of the instrument's two analogue oscillators. The first
// 150 bytes of a patch hold the frequency, level and pulse width of each,
// followed by the waveform switches. The notes list both oscillators as having
// a saw, a triangle and a square and nothing else, so that is all this holds.
type Oscillator struct {
	Frequency  int `range:"0..65535"`
	Volume     int `range:"0..65535"`
	PulseWidth int `range:"0..65535"`
	// The waveform switches are independent, so an oscillator mixes its
	// shapes rather than choosing one: saw plus triangle is the usual
	// starting point for a reed.
	Saw      int `oneof:"0,1"`
	Triangle int `oneof:"0,1"`
	Square   int `oneof:"0,1"`
	// PitchMode is how far the pitch wheel moves this oscillator: free
	// running, in semitones, in octaves, or fixed while the wheel does
	// something else.
	PitchMode int `oneof:"0,1,2,3"`
}

// OscillatorB is the second oscillator, the only one the record gives a fine
// tuning and a sync switch. Declaring them here rather than on the shared
// type is what lets a reader, and a generator walking the struct, see that
// the first oscillator has nowhere to put them: there is no field to refuse
// and nothing to check at encode time.
type OscillatorB struct {
	Oscillator
	// Fine is the tuning offset the sync switch is measured against.
	Fine int `range:"0..65535"`
	// Sync is whether the second oscillator locks to the first.
	Sync int `oneof:"0,1"`
}

// Envelope is a decay envelope's four times, which both the filter and the
// amplifier use, plus the two switches that change its shape. The two
// envelopes disagree about what those switches are called: the filter's are
// a linear or exponential shape and a fast or slow rate, and the amplifier's
// are the same two switches. They are named here after the filter's labels
// because the record stores them in the same order for both.
type Envelope struct {
	Attack  int `range:"0..65535"`
	Decay   int `range:"0..65535"`
	Sustain int `range:"0..65535"`
	Release int `range:"0..65535"`
	// Speed scales the whole envelope, and is the one that decides whether
	// a fast decay sounds like a percussion or a sweep.
	Speed int `oneof:"0,1"`
	// Shape is the curve: a linear one falls at a constant rate, an
	// exponential one falls fastest at first and then trails off, which is
	// what makes a filter envelope sound like it is closing rather than
	// sliding.
	Shape int `oneof:"0,1"`
}

// Filter is the instrument's voltage controlled filter and the envelope that
// sweeps it.
type Filter struct {
	Envelope
	Cutoff    int `range:"0..65535"`
	Resonance int `range:"0..65535"`
	Amount    int `range:"0..65535"`
	// KeyTrack is how far the cutoff follows the note played, so that high
	// notes stay bright without the low ones turning to mud.
	KeyTrack   int `oneof:"0,1,2"`
	Velocity   int `range:"0..65535"`
	Aftertouch int `range:"0..65535"`
}

// Amplifier is the envelope that shapes the output level.
type Amplifier struct {
	Envelope
	Velocity   int `range:"0..65535"`
	Aftertouch int `range:"0..65535"`
}

// PolyMod turns one oscillator into a modulator. The two amounts are how far
// the sources drive the destinations, and the two switches are which source
// and which destination the poly mod is routed to.
type PolyMod struct {
	FilterEnvAmount   int `range:"0..65535"`
	OscillatorBAmount int `range:"0..65535"`
	// SourceFrequencyA selects oscillator A's frequency as the modulator
	// and DestinationFilter selects the filter cutoff as what it modulates;
	// with either off, the poly mod contributes nothing.
	SourceFrequencyA  int `oneof:"0,1"`
	DestinationFilter int `oneof:"0,1"`
}

// Lfo is the instrument's low frequency oscillator, which the panel also
// calls LFO2 because the mod wheel's vibrato setting is the third routing
// option that shares this section.
type Lfo struct {
	Frequency int `range:"0..65535"`
	Amount    int `range:"0..65535"`
	// Shape is the waveform the modulation runs at. Noise is the one worth
	// reaching for when the LFO is a dither or a texture rather than a
	// pitch or cutoff movement.
	Shape int `oneof:"0,1,2,3,4,5"`
	// Speed is the rate range. The notes disagree about its name and its
	// direction: the patch record calls it a range running slow to fast,
	// and the instrument's control change table calls it a speed running
	// fast to slow. It is one bit either way, and the name here follows the
	// patch record.
	Speed int `oneof:"0,1"`
	// Target is the bit field naming the destinations the LFO reaches, one
	// bit each. The later notes and the Behringer preset structure document
	// put the amplifier at bit 5 and mark bit 6 unused; the earlier notes
	// put the amplifier at bit 6 and leave bit 5 unexplained. The whole
	// byte is carried either way, so a dump written by an instrument using
	// the earlier numbering survives a round trip.
	Target   int `range:"0..255"`
	ModDelay int `range:"0..65535"`
}

// Vibrato is the modulation the instrument applies when the mod wheel is
// aimed at it. It has a rate and a depth of its own rather than borrowing the
// LFO's, so Lfo.Frequency is not the vibrato rate.
type Vibrato struct {
	Speed  int `range:"0..65535"`
	Amount int `range:"0..65535"`
}

// ModWheel is how far the wheel reaches and what it is routed to. The amount
// is one of four steps rather than the continuous control the notes list for
// the instrument's continuous controllers, which the patch record does not
// have a field for.
type ModWheel struct {
	// Range is the depth, in quarters of the wheel's travel rather than a
	// continuous amount, so the wheel reaches 25, 50, 75 or 100 per cent
	// and nothing in between.
	Range int `oneof:"0,1,2,3"`
	// Target is the LFO or the vibrato, which are the two modulations the
	// wheel can reach.
	Target int `oneof:"0,1"`
}

// PitchBend is what the bend wheel acts on. The range exists only in the 6F
// layout and is a number of semitones.
type PitchBend struct {
	// Target chooses what the wheel moves. Sending it to the filter or the
	// level instead of the oscillators is what turns a bend into a sweep
	// rather than a note.
	Target int `oneof:"0,1,2,3"`
	// Range is the span in semitones. The notes give no scale for it, only
	// a default of 12, so the value is the instrument's own and not a
	// count of semitones.
	//
	// The span only exists in the 6F layout, so a 6E dump has nowhere to put
	// a non-zero one. It is left out of the catalog for both layouts rather
	// than for whichever the seed happened to carry, so that what a run can
	// evolve does not depend on the layout it started from.
	Range int `range:"0..65535" mutate:"skip"`
}

// Patch is one of the instrument's 400 memory slots, as laid out in the
// community notes. The first 150 bytes are the same in both dump versions;
// what follows them is not, so Version decides how the rest is read.
//
// Chord holds the eight notes of the patch as semitone offsets from the root,
// and is the unison voice pattern when Unison is on: the same eight bytes
// either spread the stack or spell a chord, and a value of 0xff is the note
// being absent, which is how a mono unison voice marks its unused notes.
//
// Tuning is the instrument's per note tuning table, twelve notes from C
// upwards, four bytes each and least significant byte first. The sign of a
// value is the top bit of its last byte, so a flat note is negative and a
// sharp one positive, with a value of zero meaning no offset. The notes are
// explicit that the scale from cents to these values is not linear and do not
// give the curve, so the numbers are the instrument's own units and not
// cents: a caller that wants to set a temperament has to interpolate from the
// table the notes publish rather than scale a value itself.
//
// The range is the whole signed 32-bit field rather than the band the
// published table covers. That table runs from -40 to +100 cents and nothing
// says the instrument stops there, so a narrower tag would reject a dump the
// instrument may well produce. It does mean a generator drawing from this
// range has a far wider space to search than the other fields on the record.
type Patch struct {
	// Version selects the dump layout, and a run keeps the one its seed
	// carries: the layouts hold different fields, so switching mid-search
	// would change which patches are representable.
	Version   int `oneof:"110,111" mutate:"skip"`
	OscA      Oscillator
	OscB      OscillatorB
	Filter    Filter
	Amp       Amplifier
	PolyMod   PolyMod
	Lfo       Lfo
	Vibrato   Vibrato
	ModWheel  ModWheel
	PitchBend PitchBend
	Glide     int `range:"0..65535"`
	// Unison turns the oscillator into a stack of detuned copies. The eight
	// chord notes below hold the stack's spread when it is on and spell a
	// chord when it is off, so the two uses cannot both be active.
	Unison int `oneof:"0,1"`
	// UnisonDetune is the width of the unison stack, which only means
	// anything while Unison is on.
	UnisonDetune int `range:"0..65535"`
	// Reserved is the one byte the notes mark unused between the mod wheel
	// target and the first chord note.
	Reserved   int       `range:"0..255" mutate:"skip"`
	Chord      [8]int    `range:"0..255"`
	Tuning     [12]int32 `range:"-2147483648..2147483647"`
	NoiseLevel int       `range:"0..65535"`
	// ArpMode is the pattern the arpeggiator plays, and is the only part
	// of the arpeggiator the patch record holds: nothing in it starts or
	// stops the arpeggiator, so a patch can name a pattern but not turn it
	// on. The last two steps disagree between the notes, one table giving
	// them as assigned order then random and another as random then
	// assigned order; the order here follows the record's own table.
	ArpMode int `oneof:"0,1,2,3,4,5,6"`

	// Name is the patch's name, without its terminator or padding. It is
	// held to sixteen characters because that is the field's width in the
	// 6F layout.
	Name string

	// LfoAftertouch is how far channel aftertouch moves the LFO. It sits
	// after the name rather than with the other LFO fields, so it is
	// carried here; in the 6F layout it always follows the padded name,
	// while a 6E patch may end without it at all, which is what
	// LfoAftertouchPresent records.
	//
	// The flag only means anything in the 6E layout. A 6F dump always
	// carries the amount, so decoding one always reports it present and a
	// hand-built 6F patch that leaves the flag unset comes back with it
	// set. The message is the same either way; only the struct differs.
	LfoAftertouch        int `range:"0..65535"`
	LfoAftertouchPresent bool

	// The four settings below exist only in the 6F layout, and are ignored
	// by an instrument still running the older firmware. All four are left
	// out of the catalog for both layouts: a run seeded from a 6E dump has
	// nowhere to put a non-zero one, and a run seeded from a 6F dump should
	// not be able to grow a field that stops existing on older firmware.
	// What a run can evolve therefore does not depend on the layout it
	// started from.

	// VoiceSpread is whether the unison stack spreads across the stereo
	// field. The notes give its two values in opposite orders, one table
	// listing it on then off and another off then on, so which value means
	// which is not settled; the name does not depend on it.
	VoiceSpread int `oneof:"0,1" mutate:"skip"`

	// TrackingReference is the key the keyboard tracking is measured from,
	// counting up from the lowest key. The notes give only that the value
	// for the fourth octave is three.
	TrackingReference int `range:"0..127" mutate:"skip"`

	// GlideMode is whether Glide is a time or a rate, which is what decides
	// whether a long glide feels the same in a low voice and a high one.
	GlideMode int `oneof:"0,1" mutate:"skip"`
}

// le16 reads the little endian sixteen bit values the record is built from.
func le16(data []byte, at int) int {
	return int(data[at]) | int(data[at+1])<<8
}

// putLE16 writes one back.
func putLE16(data []byte, at, value int) {
	data[at] = byte(value)
	data[at+1] = byte(value >> 8)
}

// le32 reads the four byte little endian tuning values.
func le32(data []byte, at int) int32 {
	return int32(data[at]) | int32(data[at+1])<<8 | int32(data[at+2])<<16 | int32(data[at+3])<<24
}

// putLE32 writes one back.
func putLE32(data []byte, at int, value int32) {
	data[at] = byte(value)
	data[at+1] = byte(value >> 8)
	data[at+2] = byte(value >> 16)
	data[at+3] = byte(value >> 24)
}

// decodeName trims the padding a fixed width name carries. The instrument
// writes NULs, and pads with spaces when a name is edited on the panel.
func decodeName(data []byte) string {
	end := len(data)
	for end > 0 && (data[end-1] == 0 || data[end-1] == ' ') {
		end--
	}
	return string(data[:end])
}

// decode reads a patch out of a decoded payload. The payload has already been
// unpacked, so its length is whatever the packed form of the message said it
// was; the version then says how much of it is a patch.
func (p *Patch) decode(data []byte) error {
	// The shortest possible patch is the parameters, a one character name
	// and its terminator.
	if len(data) < parameterSize+2 {
		return sysex.ErrBadRange
	}
	version := int(data[4])
	if version != Version6E && version != Version6F {
		return sysex.ErrBadRange
	}

	patch := Patch{
		// The first four bytes are the storage code 0x006116a5 in both
		// layouts, which is an identity check on a dump rather than a
		// setting, so it is not carried.
		Version: version,
		OscA: Oscillator{
			Frequency:  le16(data, 5),
			Volume:     le16(data, 7),
			PulseWidth: le16(data, 9),
			Saw:        int(data[55]),
			Triangle:   int(data[56]),
			Square:     int(data[57]),
			PitchMode:  int(data[74]),
		},
		OscB: OscillatorB{
			Oscillator: Oscillator{
				Frequency:  le16(data, 11),
				Volume:     le16(data, 13),
				PulseWidth: le16(data, 15),
				Saw:        int(data[58]),
				Triangle:   int(data[59]),
				Square:     int(data[60]),
				PitchMode:  int(data[75]),
			},
			Fine: le16(data, 17),
			Sync: int(data[61]),
		},
		Filter: Filter{
			Envelope: Envelope{
				Release: le16(data, 25),
				Sustain: le16(data, 27),
				Decay:   le16(data, 29),
				Attack:  le16(data, 31),
				Speed:   int(data[69]),
				Shape:   int(data[68]),
			},
			Cutoff:     le16(data, 19),
			Resonance:  le16(data, 21),
			Amount:     le16(data, 23),
			KeyTrack:   int(data[67]),
			Velocity:   le16(data, 53),
			Aftertouch: le16(data, 146),
		},
		Amp: Amplifier{
			Envelope: Envelope{
				Release: le16(data, 33),
				Sustain: le16(data, 35),
				Decay:   le16(data, 37),
				Attack:  le16(data, 39),
				Speed:   int(data[148]),
				Shape:   int(data[70]),
			},
			Velocity:   le16(data, 51),
			Aftertouch: le16(data, 144),
		},
		PolyMod: PolyMod{
			FilterEnvAmount:   le16(data, 41),
			OscillatorBAmount: le16(data, 43),
			SourceFrequencyA:  int(data[62]),
			DestinationFilter: int(data[63]),
		},
		Lfo: Lfo{
			Frequency: le16(data, 45),
			Amount:    le16(data, 47),
			Shape:     int(data[64]),
			Speed:     int(data[65]),
			Target:    int(data[66]),
			ModDelay:  le16(data, 76),
		},
		Vibrato: Vibrato{
			Speed:  le16(data, 78),
			Amount: le16(data, 80),
		},
		ModWheel: ModWheel{
			Range:  int(data[73]),
			Target: int(data[84]),
		},
		PitchBend:    PitchBend{Target: int(data[72])},
		Glide:        le16(data, 49),
		Unison:       int(data[71]),
		UnisonDetune: le16(data, 82),
		Reserved:     int(data[85]),
		NoiseLevel:   le16(data, 142),
		ArpMode:      int(data[149]),
	}
	for i := range patch.Chord {
		patch.Chord[i] = int(data[86+i])
	}
	for i := range patch.Tuning {
		patch.Tuning[i] = le32(data, 94+i*4)
	}

	if err := patch.decodeTail(data); err != nil {
		return err
	}
	if err := sysex.CheckTaggedFields(&patch); err != nil {
		return err
	}
	*p = patch
	return nil
}

// decodeTail reads the part of a patch that follows its parameters, which is
// the only part the two dump versions disagree about.
func (p *Patch) decodeTail(data []byte) error {
	if p.Version == Version6F {
		if len(data) != patch6FSize {
			return sysex.ErrBadRange
		}
		p.Name = decodeName(data[parameterSize : parameterSize+nameSize])
		p.LfoAftertouch = le16(data, 166)
		p.LfoAftertouchPresent = true
		p.VoiceSpread = int(data[168])
		p.TrackingReference = int(data[169])
		p.GlideMode = int(data[170])
		p.PitchBend.Range = le16(data, 171)
		return nil
	}

	// A 6E patch's name runs to the first NUL and nothing is known to
	// follow it, except that a patch edited on a later instrument may carry
	// the LFO aftertouch amount immediately after the name. Both lengths
	// occur in real dumps, so the payload's own length decides which.
	rest := data[parameterSize:]
	end := -1
	for i, value := range rest {
		if value == 0 {
			end = i
			break
		}
	}
	if end < 0 {
		return sysex.ErrBadRange
	}
	name := rest[:end]
	if len(name) > nameSize {
		return sysex.ErrBadRange
	}
	// Only the two lengths occur: the name and its terminator, or those
	// plus the aftertouch amount. Anything else would be dropped on the way
	// back out, so it is refused rather than quietly shortened.
	switch len(rest) {
	case end + 1:
	case end + 3:
		p.LfoAftertouchPresent = true
		p.LfoAftertouch = le16(data, parameterSize+end+1)
	default:
		return sysex.ErrBadRange
	}
	p.Name = string(name)
	return nil
}

// encode writes a patch out as a decoded payload, ready to be packed.
func (p *Patch) encode() ([]byte, error) {
	if err := sysex.CheckTaggedFields(p); err != nil {
		return nil, err
	}
	if len(p.Name) > nameSize {
		return nil, sysex.ErrBadRange
	}
	for i := range len(p.Name) {
		if p.Name[i] >= 0x80 {
			// The record's name is eight bit text, and a byte with its
			// eighth bit set would be packed as a different character.
			return nil, sysex.ErrBadRange
		}
	}

	// The first four bytes are the storage code every patch opens with.
	data := make([]byte, parameterSize, patch6FSize)
	data[0], data[1], data[2], data[3] = 0xa5, 0x16, 0x61, 0x00
	data[4] = byte(p.Version)

	putLE16(data, 5, p.OscA.Frequency)
	putLE16(data, 7, p.OscA.Volume)
	putLE16(data, 9, p.OscA.PulseWidth)
	putLE16(data, 11, p.OscB.Frequency)
	putLE16(data, 13, p.OscB.Volume)
	putLE16(data, 15, p.OscB.PulseWidth)
	putLE16(data, 17, p.OscB.Fine)
	data[55] = byte(p.OscA.Saw)
	data[56] = byte(p.OscA.Triangle)
	data[57] = byte(p.OscA.Square)
	data[58] = byte(p.OscB.Saw)
	data[59] = byte(p.OscB.Triangle)
	data[60] = byte(p.OscB.Square)
	data[61] = byte(p.OscB.Sync)

	putLE16(data, 19, p.Filter.Cutoff)
	putLE16(data, 21, p.Filter.Resonance)
	putLE16(data, 23, p.Filter.Amount)
	putLE16(data, 25, p.Filter.Release)
	putLE16(data, 27, p.Filter.Sustain)
	putLE16(data, 29, p.Filter.Decay)
	putLE16(data, 31, p.Filter.Attack)
	putLE16(data, 33, p.Amp.Release)
	putLE16(data, 35, p.Amp.Sustain)
	putLE16(data, 37, p.Amp.Decay)
	putLE16(data, 39, p.Amp.Attack)
	putLE16(data, 51, p.Amp.Velocity)
	putLE16(data, 53, p.Filter.Velocity)
	data[67] = byte(p.Filter.KeyTrack)
	data[68] = byte(p.Filter.Shape)
	data[69] = byte(p.Filter.Speed)
	data[70] = byte(p.Amp.Shape)

	putLE16(data, 41, p.PolyMod.FilterEnvAmount)
	putLE16(data, 43, p.PolyMod.OscillatorBAmount)
	data[62] = byte(p.PolyMod.SourceFrequencyA)
	data[63] = byte(p.PolyMod.DestinationFilter)

	putLE16(data, 45, p.Lfo.Frequency)
	putLE16(data, 47, p.Lfo.Amount)
	data[64] = byte(p.Lfo.Shape)
	data[65] = byte(p.Lfo.Speed)
	data[66] = byte(p.Lfo.Target)
	putLE16(data, 76, p.Lfo.ModDelay)

	putLE16(data, 78, p.Vibrato.Speed)
	putLE16(data, 80, p.Vibrato.Amount)

	putLE16(data, 49, p.Glide)
	data[71] = byte(p.Unison)
	data[72] = byte(p.PitchBend.Target)
	putLE16(data, 82, p.UnisonDetune)
	data[73] = byte(p.ModWheel.Range)
	data[74] = byte(p.OscA.PitchMode)
	data[75] = byte(p.OscB.PitchMode)
	data[84] = byte(p.ModWheel.Target)
	data[85] = byte(p.Reserved)
	for i, note := range p.Chord {
		data[86+i] = byte(note)
	}
	for i, value := range p.Tuning {
		putLE32(data, 94+i*4, value)
	}
	putLE16(data, 142, p.NoiseLevel)
	putLE16(data, 144, p.Amp.Aftertouch)
	putLE16(data, 146, p.Filter.Aftertouch)
	data[148] = byte(p.Amp.Speed)
	data[149] = byte(p.ArpMode)

	if p.Version == Version6F {
		name := make([]byte, nameSize)
		copy(name, p.Name)
		data = append(data, name...)
		data = append(data, make([]byte, patch6FSize-parameterSize-nameSize)...)
		putLE16(data, 166, p.LfoAftertouch)
		data[168] = byte(p.VoiceSpread)
		data[169] = byte(p.TrackingReference)
		data[170] = byte(p.GlideMode)
		putLE16(data, 171, p.PitchBend.Range)
		return data, nil
	}

	// A 6E dump carries the aftertouch amount only when it has one, and a
	// value set without the flag would be written nowhere. The flag is a
	// second field rather than a sentinel precisely because zero is a
	// legitimate amount, so this has to be checked rather than inferred.
	if !p.LfoAftertouchPresent && p.LfoAftertouch != 0 {
		return nil, ErrNotRepresentable
	}

	// A 6E dump ends after the name, so the four settings the newer layout
	// added have nowhere to go. Refusing is the point: a patch read from a
	// newer instrument and written back as 6E would otherwise lose them
	// without saying so.
	if p.VoiceSpread != 0 || p.TrackingReference != 0 || p.GlideMode != 0 || p.PitchBend.Range != 0 {
		return nil, ErrNotRepresentable
	}

	data = append(data, p.Name...)
	data = append(data, 0)
	if p.LfoAftertouchPresent {
		data = append(data, 0, 0)
		putLE16(data, parameterSize+len(p.Name)+1, p.LfoAftertouch)
	}
	return data, nil
}
