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

func (t *TimeSignatureImmediate) BPM() int {
	panic("STUB")
	return t.BeatsNumerator // * XXX
}

func (t *TimeSignatureImmediate) UnmarshalBinary(data []byte) error {
	if len(data) != 10 || data[0] != midi.SysEx || data[1] != IdRealTime ||
		data[3] != SubIdNotation || data[4] != NotationTimeSignatureImmediate {
		return ErrBadHeader
	}
	if data[len(data)-1] != midi.EndSysEx {
		return ErrNoEox
	}
	if data[4] != 4 {
		// TODO: support compound times
		return ErrBadRange
	}

	t.BeatsNumerator = int(data[5])
	t.BeatsDenominator = int(data[6])
	t.ClocksPerClick = int(data[7])
	t.Notated32ndNotesPerBeat = int(data[8])

	return nil
}

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
		0x04, /* number of data bytes */
		byte(t.BeatsNumerator),
		byte(t.BeatsDenominator),
		byte(t.ClocksPerClick),
		byte(n32),
		0xf7}, nil
}
