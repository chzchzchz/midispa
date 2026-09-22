package sysex

import (
	"github.com/chzchzchz/midispa/midi"
)

const (
	SubIdNotation = 3

	NotationBar                    = 1
	NotationTimeSignatureImmediate = 2
	NotationTimeSignatureDelayed   = 0x42
)

type TimeSignatureImmediate struct {
	Device                  int
	BeatsNumerator          int
	BeatsDenominator        int // negative power of 2
	ClocksPerClick          int // ppq
	Notated32ndNotesPerBeat int // typically 8
	// todo: slice with additional time signatures for compound time
}

// BPM returns the beats per minute implied by this time signature.
// The formula is BPM = 60 * 2^(BeatsDenominator - 2), where
// BeatsDenominator encodes the beat type as a power of 2
// (2=quarter note, 3=eighth note, etc.).
func (t *TimeSignatureImmediate) BPM() int {
	return 60 * (1 << (t.BeatsDenominator - 2))
}

// UnmarshalBinary decodes a TimeSignatureImmediate message from bytes.
// Per MIDI_Specification.pdf, Section 12 (MIDI Machine Control), Notation sub-ID #1 = 0x03:
//
//	F0 7F <device> 03 02 <count> <num> <den> <clk> <n32> F7
//
// Position 4 is the NotationTimeSignatureImmediate command code (0x02).
// Position 5 is the count of data bytes (4 for simple time, 6+ for compound).
func (t *TimeSignatureImmediate) UnmarshalBinary(data []byte) error {
	if len(data) != 11 || data[0] != midi.SysEx || data[1] != IdRealTime ||
		data[3] != SubIdNotation || data[4] != NotationTimeSignatureImmediate ||
		data[5] != 4 {
		return ErrBadHeader
	}
	if data[len(data)-1] != midi.EndSysEx {
		return ErrNoEox
	}
	t.Device = int(data[2])
	t.BeatsNumerator = int(data[6])
	t.BeatsDenominator = int(data[7])
	t.ClocksPerClick = int(data[8])
	t.Notated32ndNotesPerBeat = int(data[9])

	return nil
}

// MarshalBinary encodes a TimeSignatureImmediate message per the
// MIDI_Specification.pdf, Section 12, Notation sub-ID #1 = 0x03, command NotationTimeSignatureImmediate (0x02).
// Format: F0 7F <device> 03 02 04 <num> <den> <clk> <n32> F7
// Position 4 is the command code (0x02).
// Position 5 is the count (4 data bytes for simple time).
func (t *TimeSignatureImmediate) MarshalBinary() ([]byte, error) {
	if t.Device < 0 || t.Device > 0x7f {
		return nil, ErrBadRange
	}
	if t.BeatsNumerator > 0x7f || t.BeatsDenominator > 0x7f {
		return nil, ErrBadRange
	}
	n32 := t.Notated32ndNotesPerBeat
	if n32 == 0 {
		n32 = 8
	}
	return []byte{
		0xf0, IdRealTime, byte(t.Device), SubIdNotation,
		NotationTimeSignatureImmediate, /* command code 0x02 */
		4,                              /* count: 4 data bytes */
		byte(t.BeatsNumerator),
		byte(t.BeatsDenominator),
		byte(t.ClocksPerClick),
		byte(n32),
		0xf7}, nil
}
