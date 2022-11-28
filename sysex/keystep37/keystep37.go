// Package keystep37 provides Arturia keystep 37 sysex messages.
package keystep37

import (
	"github.com/chzchzchz/midispa/sysex"
)

type ParameterRequest struct {
	Param uint
}

var header = []byte{0xf0, 0, 0x20, 0x6b, 0x7f, 0x42}

// Values derived from from the midi control center.
const (
	ParamInputChannel   = 0x1  // userchannel = 0x41
	ParamMidiThru       = 0x2  // off=0,on=1
	ParamChannel        = 0x3  // 0-15
	ParamVelocity       = 0x6  // lin=0,log=1,antilog=2
	ParamAftertouch     = 0x7  // lin=0,log=1,antilog=2,soft=3
	ParamSyncClockPPQ   = 0x8  // 0=gate,clock=1,korg=2,24=3,48=4
	ParamMidiClockStart = 0xb  // clk=0, gate=1
	ParamTransportMode  = 0x20 // both = 3, midicc = 1, mmc = 2
	ParamKnobCatchup    = 0x2e // 0=jump,1=hook,2=scale
	ParamCCNum          = 0x2f
	ParamCCMin          = 0x30
	ParamCCMax          = 0x31
	ParamCCChan         = 0x32
)

func makePacket(b []byte) []byte {
	return append(header, b...)
}

func ParamCCBankOffset(n int) int {
	if n >= 4 {
		panic("bad bank")
	}
	return n * 4 * 4
}

func ParamNthCC(n int) int { return n * 4 }

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

// PatternsDump dumps all patterns from the sequencer.
type PatternsDump struct{}

// PatternDump is taken from midi control center traffic for a single slot.
type PatternDump struct {
	Slot int /* values [1,8] */
}

func (sd *PatternDump) MarshalBinary() ([]byte, error) {
	if sd.Slot <= 0 || sd.Slot > 8 {
		return nil, sysex.ErrBadRange
	}
	var pkts []byte
	for i := byte(0x60); i < 0x63; i++ {
		pkts = append(
			pkts,
			makePacket([]byte{
				0x0b, // send is 0x0c
				0x00, i, 0x01, 0x16, byte(sd.Slot), 0x01,
				/* send has value here */
				0xf7})...)
	}
	pkt0x63 := []byte{
		0x0b,
		0x00, 0x63, 0x02, 0x16, byte(sd.Slot),
		0x01,
		0x20, 0xf7}
	pkts = append(pkts, makePacket(pkt0x63)...)

	pkt0x63[len(pkt0x63)-3] = 0x21
	pkts = append(pkts, makePacket(pkt0x63)...)

	for i := 1; i <= 40; i++ {
		pkt := []byte{
			0x0b, 0x00, 0x65, 0x16,
			byte(sd.Slot), byte(i), 0x1, 0x08, 0xf7}
		pkts = append(pkts, makePacket(pkt)...)
		pkt = []byte{
			0x0b, 0x00, 0x66, 0x16,
			byte(sd.Slot), byte(i), 0x1, 0x08, 0xf7}
		pkts = append(pkts, makePacket(pkt)...)
	}
	return pkts, nil
}

func (ssd *PatternsDump) MarshalBinary() ([]byte, error) {
	var pkts []byte
	for i := 1; i <= 8; i++ {
		sd := PatternDump{i}
		pkt, err := sd.MarshalBinary()
		if err != nil {
			return nil, err
		}
		pkts = append(pkts, pkt...)
	}
	return pkts, nil
}
