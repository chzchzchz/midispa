package akai

import (
	"errors"

	"github.com/chzchzchz/midispa/sysex"
)

var ErrDimensions = errors.New("bad dimensions")
var ErrBitmapSize = errors.New("bad bitmap size")

// SysEx commands for akai fire controller.

type LightPads struct {
	Pads []Pad
}

type Pad struct {
	Idx   int // 0 = top left, 3f = bottom right
	Red   int
	Green int
	Blue  int
}

func (p *LightPads) MarshalBinary() (ret []byte, err error) {
	payloadBytes := len(p.Pads) * 4
	ret = []byte{
		0xf0,
		0x47, 0x7f, 0x43, 0x65,
		byte(payloadBytes >> 7), byte(payloadBytes & 0x7f)}
	for _, k := range p.Pads {
		pkt := []byte{byte(k.Idx), byte(k.Red), byte(k.Green), byte(k.Blue)}
		ret = append(ret, pkt...)
	}
	ret = append(ret, 0xf7)
	return ret, nil
}

type ScreenUpdate struct {
	BandStart   int // 8 rows per band
	BandEnd     int
	ColumnStart int
	ColumnEnd   int
	Bitmap      []byte
}

func (p *ScreenUpdate) MarshalBinary() (ret []byte, err error) {
	if p.BandStart < 0 || p.BandEnd < 0 || p.BandStart >= 8 || p.BandEnd >= 8 {
		return nil, ErrDimensions
	}
	if p.ColumnStart < 0 || p.ColumnStart >= 128 || p.ColumnEnd < 0 || p.ColumnEnd >= 128 {
		return nil, ErrDimensions
	}
	if p.BandStart > p.BandEnd || p.ColumnStart > p.ColumnEnd {
		return nil, ErrDimensions
	}
	bits := (8 * (1 + p.BandEnd - p.BandStart)) * (1 + p.ColumnEnd - p.ColumnStart)
	if len(p.Bitmap) != bits/8 {
		return nil, ErrBitmapSize
	}
	msg := sysex.LoHiEncodeDataBytes(p.Bitmap)
	payloadSize := len(msg) + 4
	ret = []byte{
		0xf0,
		0x47, 0x7f, 0x43, 0x0e,
		byte(payloadSize >> 7), byte(payloadSize & 0x7f),
		byte(p.BandStart), byte(p.BandEnd),
		byte(p.ColumnStart), byte(p.ColumnEnd),
	}
	ret = append(ret, msg...)
	ret = append(ret, 0xf7)
	return ret, nil
}
