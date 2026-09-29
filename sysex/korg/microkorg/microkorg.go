// Package microkorg encodes and decodes the Korg microKORG's system exclusive
// protocol, as published in the "micro KORG MIDI Implementation" sheet,
// revision 1.4.
//
// The instrument shares its format with the MS2000 and MS2000R, so a dump
// written here loads on any of the three. Everything lives in the MS2000
// family, sub-model 58 format:
//
//	F0 42 3n 58 ff <packed data> F7
//
// The channel byte carries the MIDI channel in its low nibble, and unlike
// Yamaha's instruments the format carries no checksum: a message whose payload
// length is wrong is answered with FuncDataFormatError instead, so there is
// nothing to verify beyond the length and the header.
//
// A parameter is refused when the sheet lists a closed set of values for it
// and the dump holds something else, because writing it would produce a
// program the instrument silently changes on the way in. Where the sheet names
// only one value of a multi-bit field, the field is carried at its full width
// instead, since the remaining combinations are unremarked on rather than
// excluded. Every range tag says which of the two a field is.
//
// Payloads are packed eight MIDI bytes to every seven decoded bytes, with the
// eighth bit of each byte collected into a leading high byte. The trailing
// partial group is packed as its own short group, which is what makes a
// 254-byte program occupy 291 MIDI bytes rather than a round number.
package microkorg

import (
	"fmt"

	"github.com/chzchzchz/midispa/sysex"
)

const (
	// manufacturerID is Korg's one byte manufacturer code.
	manufacturerID = 0x42

	// modelID is the MS2000 series family ID the microKORG reports and
	// requires in the header, which is what makes the dumps interchangeable
	// with the MS2000 and MS2000R.
	modelID = 0x58

	// channelBit is the flag ORed into the channel byte; the remaining four
	// bits of that byte hold the MIDI channel.
	channelBit = 0x30

	// headerSize covers F0, the manufacturer and model IDs and the channel
	// byte, everything that precedes the function ID.
	headerSize = 4

	// groupSize is how many decoded bytes share one leading high byte on the
	// wire. It is seven, because seven bytes carry exactly 49 bits, which is
	// seven collected eighth bits plus seven seven-bit remainders.
	groupSize = 7

	// ControlChangeUnassigned is the byte value Korg stores for a controller
	// number that has not been assigned. The implementation sheet writes it
	// as -1, which cannot be represented in the parameter bytes these fields
	// live in; the factory bank actually stores 0xFF, and the first
	// eighteen entries of a real global dump use it.
	ControlChangeUnassigned = 255
)

// Function IDs, split by direction. Requests go to the instrument, dumps and
// status messages come back from it.
const (
	FuncGlobalDumpRequest        = 0x0e
	FuncAllDumpRequest           = 0x0f
	FuncCurrentProgramDumpRequest = 0x10
	FuncProgramWriteRequest      = 0x11
	FuncProgramDumpRequest       = 0x1c

	FuncWriteCompleted    = 0x21
	FuncWriteError        = 0x22
	FuncDataLoadCompleted = 0x23
	FuncDataLoadError     = 0x24
	FuncDataFormatError   = 0x26

	FuncCurrentProgramDump = 0x40
	FuncProgramDump        = 0x4c
	FuncAllDump            = 0x50
	FuncGlobalDump         = 0x51
)

const (
	// requestSize is a dump request or a status message: header, function
	// ID and EOX with no payload.
	requestSize = headerSize + 2

	// writeRequestSize adds the spare byte and the destination program
	// number the write request carries after its function ID.
	writeRequestSize = headerSize + 4
)

// checkChannel is the one constraint the dump envelopes share. The payload
// behind them validates itself as it is assembled, so re-walking a hundred and
// twenty eight programs through reflection at the top of a bank dump would be
// the same work done three times over.
func checkChannel(channel int) error {
	if channel < 0 || channel > 15 {
		return fmt.Errorf("%w: channel %d is outside 0..15", sysex.ErrBadRange, channel)
	}
	return nil
}

// header builds F0, the Korg and model IDs, the channel and the function ID.
func header(channel, function int) []byte {
	return []byte{0xf0, manufacturerID, channelBit | byte(channel), modelID, byte(function)}
}

// checkHeader reports whether data is a microKORG message carrying function.
// The length is the caller's to establish: the positions below are only
// meaningful once it has, and taking a size here would let a caller that
// passed len(data) silently switch the check off.
func checkHeader(data []byte, function int) error {
	if data[0] != 0xf0 ||
		data[1] != manufacturerID ||
		data[2]&0xf0 != channelBit ||
		data[3] != modelID ||
		data[4] != byte(function) ||
		data[len(data)-1] != 0xf7 {
		return sysex.ErrBadHeader
	}
	return nil
}

// channel reads the MIDI channel out of a validated message.
func channel(data []byte) int {
	return int(data[2] & 0x0f)
}

// valid7Bit reports whether a packed payload carries no eighth bit set, which
// would mean the message is not the Korg packed format at all.
func valid7Bit(data []byte) bool {
	for _, value := range data {
		if value&0x80 != 0 {
			return false
		}
	}
	return true
}

// DumpRequest asks the instrument to transmit one of its dumps. The instrument
// answers a request with the matching dump, or with FuncDataLoadError when a
// request asks for something it cannot produce.
type DumpRequest struct {
	Channel  int `range:"0..15"`
	Function int `oneof:"14,15,16,28"`
}

// MarshalBinary encodes a microKORG dump request.
func (d *DumpRequest) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(d); err != nil {
		return nil, err
	}
	return append(header(d.Channel, d.Function), 0xf7), nil
}

// UnmarshalBinary decodes a microKORG dump request. A program write request is
// a different message and is refused here: it is two bytes longer and names a
// destination program, which is what ProgramWriteRequest is for.
func (d *DumpRequest) UnmarshalBinary(data []byte) error {
	if len(data) != requestSize {
		return sysex.ErrBadRange
	}
	if err := checkHeader(data, int(data[4])); err != nil {
		return err
	}
	candidate := DumpRequest{Channel: channel(data), Function: int(data[4])}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*d = candidate
	return nil
}

// ProgramWriteRequest asks the instrument to commit its edit buffer to a
// program in internal memory. It answers with FuncWriteCompleted, or
// FuncWriteError when the write is protected.
//
// The destination is a program number in the 0...127 range, which is the
// internal slot rather than the A11...b88 name the panel shows.
type ProgramWriteRequest struct {
	Channel int `range:"0..15"`
	Program int `range:"0..127"`
}

// MarshalBinary encodes a microKORG program write request.
func (p *ProgramWriteRequest) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(p); err != nil {
		return nil, err
	}
	ret := header(p.Channel, FuncProgramWriteRequest)
	return append(ret, 0x00, byte(p.Program), 0xf7), nil
}

// UnmarshalBinary decodes a microKORG program write request.
func (p *ProgramWriteRequest) UnmarshalBinary(data []byte) error {
	if len(data) != writeRequestSize {
		return sysex.ErrBadRange
	}
	if err := checkHeader(data, FuncProgramWriteRequest); err != nil {
		return err
	}
	if data[5] != 0 {
		return sysex.ErrBadRange
	}
	candidate := ProgramWriteRequest{Channel: channel(data), Program: int(data[6])}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*p = candidate
	return nil
}

// Status is one of the instrument's bare acknowledgements. Every message in
// this protocol that is not a request or a dump ends this way, so the function
// ID alone says whether a transfer succeeded.
type Status struct {
	Channel  int `range:"0..15"`
	Function int `oneof:"33,34,35,36,38"`
}

// MarshalBinary encodes a microKORG status message.
func (s *Status) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(s); err != nil {
		return nil, err
	}
	return append(header(s.Channel, s.Function), 0xf7), nil
}

// UnmarshalBinary decodes a microKORG status message.
func (s *Status) UnmarshalBinary(data []byte) error {
	if len(data) != requestSize {
		return sysex.ErrBadRange
	}
	if err := checkHeader(data, int(data[4])); err != nil {
		return err
	}
	candidate := Status{Channel: channel(data), Function: int(data[4])}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*s = candidate
	return nil
}

// packedSize returns the number of MIDI bytes packPayload spends on a payload
// of size decoded bytes. Every full group of seven decoded bytes becomes eight
// MIDI bytes, and a short trailing group costs one more MIDI byte for every
// seven bits it still needs. The four payload sizes the sheet quotes all check
// out against this, and so do the four factory dumps: a program is 291, the
// program bank 37157, the global block 229 and the combined dump 37386.
func packedSize(size int) int {
	total := (size / 7) * 8
	if rest := size % 7; rest > 0 {
		total += (rest*8 + 6) / 7
	}
	return total
}

// packPayload squeezes each group of up to seven decoded bytes into one high
// byte holding their eighth bits followed by their seven bit remainders, which
// is the conversion the implementation sheet describes and the one the
// instrument's own dumps use. sysex.LoHiEncodeDataBytes also spends eight
// MIDI bytes on seven decoded bytes and is equally lossless, but it scatters
// the eighth bits through the group instead of collecting them in front of it,
// so the two conventions are not interchangeable.
func packPayload(payload []byte) []byte {
	ret := make([]byte, 0, packedSize(len(payload)))
	for i := 0; i < len(payload); i += groupSize {
		group := payload[i:min(i+groupSize, len(payload))]
		var high byte
		low := make([]byte, 0, len(group))
		for bit, value := range group {
			high |= (value >> 7) << bit
			low = append(low, value&0x7f)
		}
		ret = append(ret, high)
		ret = append(ret, low...)
	}
	return ret
}

// unpackPayload is the inverse of packPayload, reading exactly size decoded
// bytes. A payload whose packed length does not match is refused rather than
// truncated, because the instrument answers a short message with a format
// error and decoding one here would only invent parameters.
func unpackPayload(data []byte, size int) ([]byte, error) {
	ret := make([]byte, 0, size)
	i := 0
	for len(ret) < size {
		if i >= len(data) {
			return nil, sysex.ErrBadRange
		}
		high := data[i]
		i++
		for bit := 0; bit < groupSize && len(ret) < size; bit++ {
			if i >= len(data) {
				return nil, sysex.ErrBadRange
			}
			ret = append(ret, data[i]&0x7f|(high>>bit&0x01)<<7)
			i++
		}
	}
	if i != len(data) {
		return nil, sysex.ErrBadRange
	}
	return ret, nil
}

// pack wraps a decoded payload in a microKORG dump message. The payload is
// already validated by whoever assembled it, so this cannot fail.
func pack(channel, function int, payload []byte) []byte {
	ret := header(channel, function)
	ret = append(ret, packPayload(payload)...)
	return append(ret, 0xf7)
}

// unpack validates the envelope of a microKORG dump message and returns the
// channel and the decoded payload of exactly size bytes.
func unpack(data []byte, function, size int) (int, []byte, error) {
	want := headerSize + 1 + packedSize(size) + 1
	if len(data) != want {
		return 0, nil, sysex.ErrBadRange
	}
	if err := checkHeader(data, function); err != nil {
		return 0, nil, err
	}
	payload := data[headerSize+1 : len(data)-1]
	if !valid7Bit(payload) {
		return 0, nil, sysex.ErrBadRange
	}
	decoded, err := unpackPayload(payload, size)
	if err != nil {
		return 0, nil, err
	}
	return channel(data), decoded, nil
}
