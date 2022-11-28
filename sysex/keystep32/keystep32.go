// Package keystep32 provides Arturia keystep 32 sysex messages.
package keystep32

import (
	"github.com/chzchzchz/midispa/sysex"
)

type ParameterRequest struct {
	Param uint
}

var header = []byte{0xf0, 0, 0x20, 0x6b, 0x7f, 0x42}

// Values derived from from the midi control center.
const (
	ParamUserChannel   = 0x16
	ParamInputChannel  = 0x07 // userchannel = 0x41
	ParamMidiThru      = 0x08
	ParamTransportMode = 0x60 // off=0,cc=1,mmc=2,both=3
	ParamStopChannel   = 0x61
	ParamRecChannel    = 0x62
	ParamPlayChannel   = 0x63
)

func makePacket(b []byte) []byte {
	return append(header, b...)
}

func (pr *ParameterRequest) MarshalBinary() ([]byte, error) {
	if pr.Param > 0x7f {
		return nil, sysex.ErrBadRange
	}
	return makePacket([]byte{0x01, 0x00, 0x41, byte(pr.Param), 0x7f}), nil
}

// Ack is sent from the device to the host before replying with payload.
type Ack struct{}

func (a *Ack) MarshalBinary() ([]byte, error) {
	return makePacket([]byte{0x1c, 0x00, 0xf7}), nil
}

type ParameterSend struct {
	Param uint
	Value uint
}

func (ps *ParameterSend) MarshalBinary() ([]byte, error) {
	if ps.Param > 0x7f || ps.Value > 0x7f {
		return nil, sysex.ErrBadRange
	}
	return makePacket(
		[]byte{0x02, 0x00, 0x41, byte(ps.Param), byte(ps.Value), 0x7f}), nil
}
