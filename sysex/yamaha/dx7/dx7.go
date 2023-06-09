package yamaha

import (
	"fmt"
)

var errBadRange = fmt.Errorf("bad range")

const (
	parameterGroupVoice    = 0
	parameterGroupFunction = 2
	sysexHeaderSize        = 6
	packedVoiceSize        = 128
	packedOperatorSize     = 17
	singleVoiceDataSize    = 155
)

// DX7 checksums cover the data block, not the SysEx header.
func maskedChecksum(data []byte) byte {
	sum := 0
	for _, value := range data {
		sum += int(value)
	}
	return byte((-sum) & 0x7f)
}

// BulkData is a 32-voice DX7 bulk dump.
type BulkData struct {
	Channel int `range:"0..15"`
	Voices  [32]Voice
}

func (b *BulkData) encode() ([]byte, error) {
	if err := checkTaggedFields(b); err != nil {
		return nil, err
	}
	ret := []byte{
		0xf0,
		0x43,
		byte(b.Channel),
		0x09,
		0x20,
		0x00}
	for _, v := range b.Voices {
		vv, err := v.encode()
		if err != nil {
			return nil, err
		}
		ret = append(ret, vv...)
	}
	ret = append(ret, maskedChecksum(ret[sysexHeaderSize:]), 0xf7)
	return ret, nil
}

// ParameterChange addresses one voice or function parameter in the Yamaha parameter-change namespace.
type ParameterChange struct {
	Channel   int `range:"0..15"`
	Group     int `oneof:"0,2"`
	Parameter int `range:"0..155"`
	Data      int `range:"0..127"`
}

func (pc *ParameterChange) encode() ([]byte, error) {
	if err := pc.check(); err != nil {
		return nil, err
	}
	return []byte{
		0xf0,
		0x43,
		0x10 | byte(pc.Channel),
		byte(pc.Group<<2) | byte((pc.Parameter&0x180)>>7),
		byte(pc.Parameter & 0x7f),
		byte(pc.Data),
		0xf7,
	}, nil
}

func (pc *ParameterChange) check() error {
	if err := checkTaggedFields(pc); err != nil {
		return err
	}
	if pc.Group == parameterGroupFunction && (pc.Parameter < 64 || pc.Parameter > 77) {
		return errBadRange
	}
	if pc.Data > parameterDataMax(pc.Group, pc.Parameter) {
		return errBadRange
	}
	return nil
}

// Data limits depend on the parameter namespace, so they remain explicit instead of using a static field tag.
func parameterDataMax(group, parameter int) int {
	if group == parameterGroupFunction {
		switch parameter {
		case 64:
			return 1
		case 65, 66:
			return 12
		case 67, 68:
			return 1
		case 69, 70, 72, 74, 76:
			return 99
		case 71, 73, 75, 77:
			return 7
		}
		return -1
	}
	if parameter >= 0 && parameter <= 125 {
		switch parameter % 21 {
		case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 16, 19:
			return 99
		case 11, 12, 14:
			return 3
		case 13, 15:
			return 7
		case 17:
			return 1
		case 18:
			return 31
		case 20:
			return 14
		}
	}
	switch parameter {
	case 126, 127, 128, 129, 130, 131, 132, 133, 137, 138, 139, 140:
		return 99
	case 134:
		return 31
	case 135:
		return 7
	case 136, 141:
		return 1
	case 142:
		return 5
	case 143:
		return 7
	case 144:
		return 48
	case 145, 146, 147, 148, 149, 150, 151, 152, 153, 154:
		return 127
	case 155:
		return 63
	}
	return -1
}

// Voice is the packed 128-byte voice representation used in a bulk dump.
type Voice struct {
	Osc              [6]Osc
	PitchEgRate      [4]int `range:"0..99"`
	PitchEgLevel     [4]int `range:"0..99"`
	Algorithm        int    `range:"0..31"`
	Feedback         int    `range:"0..7"`
	OscSync          int    `range:"0..1"`
	LfoSpeed         int    `range:"0..99"`
	LfoDelay         int    `range:"0..99"`
	LfoPitchModDepth int    `range:"0..99"`
	LfoAmpModDepth   int    `range:"0..99"`
	LfoSync          int    `range:"0..1"`
	LfoWaveform      int    `range:"0..5"`
	PitchModSens     int    `range:"0..7"`
	Transpose        int    `range:"0..48"`
	VoiceName        [10]byte `range:"0..127"`
	OperatorOn       int    `range:"0..63"`
}

func (v *Voice) check() error {
	return checkTaggedFields(v)
}

// Voice.encode stores operators in wire order and packs the global voice parameters.
func (v *Voice) encode() ([]byte, error) {
	if err := v.check(); err != nil {
		return nil, err
	}
	var ret []byte
	for _, osc := range v.Osc {
		o, err := osc.encode()
		if err != nil {
			return nil, err
		}
		ret = append(ret, o...)
	}
	// 102
	ret = append(ret,
		byte(v.PitchEgRate[0]),
		byte(v.PitchEgRate[1]),
		byte(v.PitchEgRate[2]),
		byte(v.PitchEgRate[3]))
	ret = append(ret,
		byte(v.PitchEgLevel[0]),
		byte(v.PitchEgLevel[1]),
		byte(v.PitchEgLevel[2]),
		byte(v.PitchEgLevel[3]))
	// 110
	ret = append(ret,
		byte(v.Algorithm),
		byte((v.OscSync<<3)|v.Feedback),
		byte(v.LfoSpeed),
		byte(v.LfoDelay),
		byte(v.LfoPitchModDepth),
		byte(v.LfoAmpModDepth),
		byte((v.PitchModSens<<4)|(v.LfoWaveform<<1)|(v.LfoSync<<0)),
		byte(v.Transpose))
	ret = append(ret, v.VoiceName[:]...)
	return ret, nil
}

const OscParamOutLevel = 16
const OscParams = 21

// Osc is the 17-byte packed representation of one DX7 operator.
type Osc struct {
	EgRate     [4]int `range:"0..99"` // 0..3
	EgLevel    [4]int `range:"0..99"` // 4..7
	BrkPt      int    `range:"0..99"` // 8
	LftDepth   int    `range:"0..99"` // 9
	RhtDepth   int    `range:"0..99"` // 10
	LftCurve   int    `range:"0..3"`  // 11
	RhtCurve   int    `range:"0..3"`  // 12
	RateScale  int    `range:"0..7"`  // 13
	ModSens    int    `range:"0..3"`  // 14
	VelSens    int    `range:"0..7"`  // 15
	OutLevel   int    `range:"0..99"` // 16
	Mode       int    `range:"0..1"`  // 17; (ratio/fixed) 0-1; 0=ratio
	FreqCoarse int    `range:"0..31"` // 18
	FreqFine   int    `range:"0..99"` // 19
	Detune     int    `range:"0..14"` // 20
}

func (o *Osc) check() error {
	return checkTaggedFields(o)
}

// Osc.encode combines the parameters that share packed bytes in the bulk format.
func (o *Osc) encode() ([]byte, error) {
	// Packed osc
	if err := o.check(); err != nil {
		return nil, err
	}
	ret := make([]byte, packedOperatorSize)
	for i := range o.EgRate {
		ret[0+i] = byte(o.EgRate[i])
	}
	for i := range o.EgRate {
		ret[4+i] = byte(o.EgLevel[i])
	}
	ret[8] = byte(o.BrkPt)
	ret[9] = byte(o.LftDepth)
	ret[10] = byte(o.RhtDepth)
	ret[11] = byte(o.LftCurve<<2 | o.RhtCurve)
	ret[12] = byte(o.Detune<<3 | o.RateScale)
	ret[13] = byte(o.VelSens<<2 | o.ModSens)
	ret[14] = byte(o.OutLevel)
	ret[15] = byte(o.FreqCoarse<<1 | o.Mode)
	ret[16] = byte(o.FreqFine)
	return ret, nil
}
