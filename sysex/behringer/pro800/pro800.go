// Package pro800 encodes and decodes the Behringer Pro 800's system exclusive
// protocol, as reconstructed by probing the instrument and published in the
// community implementation notes for it.
//
// Every message the instrument sends has the same eight byte envelope:
//
//	F0 00 20 32 00 01 24 00 <function> <arguments> <payload> F7
//
// The three manufacturer bytes are Behringer's and the four device bytes
// after them are unchanged in every message, so a message that does not open
// this way is not the instrument's. The function byte says what the message
// is, the arguments depend on it, and everything after them is one payload
// except in the version messages, whose few bytes are sent unpacked.
//
// The payload is eight bit data squeezed into seven bit MIDI bytes, seven
// decoded bytes to every eight on the wire. Each group of seven decoded bytes
// is preceded by one byte holding their eighth bits, least significant bit
// first, followed by the seven bytes with their eighth bits cleared. This is
// the same convention sysex.LoHiEncodeDataBytes implements, and the four
// captured dumps in this package's tests re-encode through it byte for byte.
//
// A dump request and its response share one fourteen bit address written low
// byte first. Addresses 0 to 399 are the 400 patches, laid out in four banks
// of 100 so that A-00 is 0, B-00 is 100, C-00 is 200 and D-00 is 300;
// addresses from 400 upwards are the instrument's other areas, and 510 is
// the system settings block.
//
// Two patch layouts exist and a dump names which it uses in the fifth decoded
// byte. Version 0x6E is firmware up to 1.2.7 and ends after a name of
// variable length; version 0x6F is firmware 1.3.6 and up, pads the name to
// sixteen bytes and adds five bytes of settings. Both share the first 150
// bytes, so the parameters land in the same place either way and a patch only
// has to be read once to know how long the rest is.
//
// Every dump taken off a real instrument so far is the older layout, which is
// what pins the shared parameters down. The newer layout's four trailing
// settings therefore rest on the notes alone, and are the one part of this
// model with nothing to check them against.
//
// What this package does not model is stated rather than guessed. The system
// settings block is carried and returned as decoded bytes because the notes
// that describe its fields contradict themselves about the byte order of the
// tempo, and a wrong field there would look like a working implementation.
// The two sequencer patterns, the firmware update messages and the function
// 0x38 messages the instrument emits while it is being tuned are all
// unaccounted for in the notes, so none of them is decoded.
package pro800

import (
	"bytes"

	"github.com/chzchzchz/midispa/sysex"
)

const (
	// headerSize covers F0, Behringer's manufacturer code and the four
	// device bytes that follow it, everything before the function ID.
	headerSize = 8

	// functionOffset is where the function ID sits in every message.
	functionOffset = headerSize

	// groupSize is how many decoded bytes share one leading eighth bit
	// byte. It is seven, because seven bytes carry exactly 49 bits, which
	// is seven collected eighth bits plus seven seven-bit remainders.
	groupSize = 7
)

// deviceID is the four byte device identifier the Pro 800 puts after its
// manufacturer code. It is the same in every message the instrument sends
// and in every message it answers, including the version reply.
var deviceID = []byte{0x00, 0x01, 0x24, 0x00}

// Function IDs. Only the functions the notes describe are named; 0x0a and
// 0x0b carry a firmware update and 0x38 is emitted while the instrument is
// tuned, neither of which is decoded here.
const (
	// FuncUnsupportedAddress is the instrument's answer to a request for an
	// address it does not implement, and it carries no payload.
	FuncUnsupportedAddress = 0x01

	FuncVersionRequest  = 0x08
	FuncVersionResponse = 0x09

	FuncDumpRequest  = 0x77
	FuncDumpResponse = 0x78
)

// Patch dump layout versions, which the fifth decoded byte names. They are
// the two values seen in real dumps, and they are the only two this package
// accepts: a dump claiming anything else is not a patch the notes describe.
const (
	// Version6E is firmware up to 1.2.7. The name is a variable length NUL
	// terminated string and nothing is known to follow it.
	Version6E = 0x6e

	// Version6F is firmware 1.3.6 and up. The name is padded to sixteen
	// bytes and four settings follow it.
	Version6F = 0x6f
)

const (
	// PatchCount is how many patches the instrument holds, in four banks of
	// a hundred: A-00 is 0 and D-99 is 399. PatchData carries the same
	// number in a range tag, which is the only kind of check the validator
	// can do, so a test pins the two to each other.
	PatchCount = 400

	// SystemAddress is the address of the system settings block. It is the
	// only address past the patches that the notes describe an area for,
	// and it is written 7E 03, which is 126 + 3*128.
	SystemAddress = 510
)

// checkHeader reports whether data is a Pro 800 message carrying the given
// function ID. The length is the caller's to establish, because the byte
// positions below only mean something once it has, and taking a length here
// would let a caller that passed len(data) silently switch the check off.
func checkHeader(data []byte, function int) error {
	if data[0] != 0xf0 ||
		!bytes.Equal(data[1:4], sysex.MfrBehringer) ||
		!bytes.Equal(data[4:headerSize], deviceID) ||
		data[functionOffset] != byte(function) ||
		data[len(data)-1] != 0xf7 {
		return sysex.ErrBadHeader
	}
	return nil
}

// build assembles a message from its function ID, its unpacked arguments and
// its already packed payload.
func build(function int, args, payload []byte) []byte {
	ret := make([]byte, 0, headerSize+1+len(args)+len(payload)+1)
	ret = append(ret, 0xf0)
	ret = append(ret, sysex.MfrBehringer...)
	ret = append(ret, deviceID...)
	ret = append(ret, byte(function))
	ret = append(ret, args...)
	ret = append(ret, payload...)
	return append(ret, 0xf7)
}

// splitHeader validates the envelope of a message with the given function ID
// and returns everything after it, up to but not including the EOX. A
// payload that is not already known to be packed is returned as nil, because
// the only unpacked payloads in the protocol are the version bytes.
func splitHeader(data []byte, function int) ([]byte, error) {
	if err := checkHeader(data, function); err != nil {
		return nil, err
	}
	return data[functionOffset+1 : len(data)-1], nil
}

// addressBytes writes a fourteen bit address low byte first, which is the
// order both the dump request and the dump response use. The system settings
// block at 510 is not a special case in the encoding, only in what it holds.
func addressBytes(address int) []byte {
	return []byte{byte(address & 0x7f), byte((address >> 7) & 0x7f)}
}

// address reads a fourteen bit address back out of the two bytes the
// instrument writes for it.
func address(data []byte) int {
	return int(data[0]&0x7f) | int(data[1]&0x7f)<<7
}

// packedSize returns the number of MIDI bytes packPayload spends on size
// decoded bytes: every full group of seven becomes eight bytes, and a short
// trailing group costs one byte for its eighth bits plus one per remaining
// byte. The four captured dumps are 179, 182, 186 and 187 bytes of payload,
// which is what a 156, 159, 162 and 163 byte patch packs into.
func packedSize(size int) int {
	total := (size / groupSize) * 8
	if rest := size % groupSize; rest > 0 {
		total += rest + 1
	}
	return total
}

// decodedSize is the inverse of packedSize, and is what lets a dump be
// unpacked without being told how long its program is. A packed payload
// always ends on a group boundary or a short group, so its length determines
// the number of decoded bytes exactly; the alternative, decoding everything
// the payload can yield, would invent trailing bytes that the instrument
// never sent.
func decodedSize(packed int) int {
	full := packed / 8
	size := full * groupSize
	if rest := packed % 8; rest > 0 {
		size += rest - 1
	}
	return size
}

// packPayload squeezes each group of up to seven decoded bytes into one byte
// holding their eighth bits followed by their seven bit remainders.
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

// unpackPayload is the inverse of packPayload. A payload carrying an eighth
// bit set is refused: that would mean it is not packed at all, and decoding
// it would silently produce a program made of the wrong bytes.
func unpackPayload(data []byte) ([]byte, error) {
	// A whole group is eight bytes and a short one is one byte of eighth
	// bits plus one per byte, so a length one past a whole group is an
	// eighth-bits byte with nothing following it. That is not a group, and
	// accepting it would drop a byte the instrument claims to have sent.
	if len(data)%8 == 1 {
		return nil, sysex.ErrBadRange
	}
	size := decodedSize(len(data))
	ret := make([]byte, 0, size)
	for i := 0; i < len(data); {
		high := data[i]
		if high&0x80 != 0 {
			return nil, sysex.ErrBadRange
		}
		i++
		for bit := 0; bit < groupSize && i < len(data); bit++ {
			low := data[i]
			if low&0x80 != 0 {
				return nil, sysex.ErrBadRange
			}
			ret = append(ret, low|((high>>bit&0x01)<<7))
			i++
		}
	}
	return ret, nil
}

// VersionRequest asks the instrument for its firmware version.
type VersionRequest struct{}

// MarshalBinary encodes a version request.
func (v *VersionRequest) MarshalBinary() ([]byte, error) {
	return build(FuncVersionRequest, []byte{0x00}, nil), nil
}

// UnmarshalBinary decodes a version request.
func (v *VersionRequest) UnmarshalBinary(data []byte) error {
	if len(data) != headerSize+3 {
		return sysex.ErrBadRange
	}
	_, err := splitHeader(data, FuncVersionRequest)
	return err
}

// VersionResponse is the instrument's firmware version as three decimal
// numbers, which the notes transcribe as 1.4.6 rather than as a packed value.
// The bytes are sent unpacked, unlike every other payload in the protocol.
type VersionResponse struct {
	Major int `range:"0..127"`
	Minor int `range:"0..127"`
	Patch int `range:"0..127"`
}

// MarshalBinary encodes a version response.
func (v *VersionResponse) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(v); err != nil {
		return nil, err
	}
	return build(FuncVersionResponse, []byte{0x00, byte(v.Major), byte(v.Minor), byte(v.Patch)}, nil), nil
}

// UnmarshalBinary decodes a version response.
func (v *VersionResponse) UnmarshalBinary(data []byte) error {
	if len(data) != headerSize+6 {
		return sysex.ErrBadRange
	}
	args, err := splitHeader(data, FuncVersionResponse)
	if err != nil {
		return err
	}
	candidate := VersionResponse{
		Major: int(args[1]),
		Minor: int(args[2]),
		Patch: int(args[3]),
	}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*v = candidate
	return nil
}

// UnsupportedAddress is the instrument's refusal of a request for an address
// it does not implement. The notes give only the one observed message and do
// not say what its two argument bytes mean, so they are carried as observed
// rather than interpreted.
type UnsupportedAddress struct{}

// MarshalBinary encodes the refusal the instrument sends.
func (u *UnsupportedAddress) MarshalBinary() ([]byte, error) {
	return build(FuncUnsupportedAddress, []byte{0x00, 0x01}, nil), nil
}

// UnmarshalBinary decodes the refusal the instrument sends.
func (u *UnsupportedAddress) UnmarshalBinary(data []byte) error {
	if len(data) != headerSize+4 {
		return sysex.ErrBadRange
	}
	_, err := splitHeader(data, FuncUnsupportedAddress)
	return err
}

// DumpRequest asks the instrument to transmit the contents of one address: a
// patch for 0 to 399, or SystemAddress for the settings block.
// A dump request carries either a patch address or the one settings
// address, and nothing else: the number is fourteen bits wide, so 0...511
// is every address the instrument can be asked about.
type DumpRequest struct {
	Address int `range:"0..511"`
}

// MarshalBinary encodes a dump request.
func (d *DumpRequest) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(d); err != nil {
		return nil, err
	}
	return build(FuncDumpRequest, addressBytes(d.Address), nil), nil
}

// UnmarshalBinary decodes a dump request. A dump response is a different
// message and is refused here: it carries a payload, which is what
// PatchData and SystemData are for.
func (d *DumpRequest) UnmarshalBinary(data []byte) error {
	if len(data) != headerSize+4 {
		return sysex.ErrBadRange
	}
	args, err := splitHeader(data, FuncDumpRequest)
	if err != nil {
		return err
	}
	candidate := DumpRequest{Address: address(args)}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*d = candidate
	return nil
}

// PatchData is one patch read back from the instrument, together with the
// address it was read from, which is the bank and program the panel shows.
type PatchData struct {
	Address int `range:"0..399"`
	Patch   Patch
}

// MarshalBinary encodes a patch dump response.
func (p *PatchData) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(p); err != nil {
		return nil, err
	}
	payload, err := p.Patch.encode()
	if err != nil {
		return nil, err
	}
	return build(FuncDumpResponse, addressBytes(p.Address), packPayload(payload)), nil
}

// UnmarshalBinary decodes a patch dump response.
func (p *PatchData) UnmarshalBinary(data []byte) error {
	if len(data) < headerSize+4 {
		return sysex.ErrBadRange
	}
	args, err := splitHeader(data, FuncDumpResponse)
	if err != nil {
		return err
	}
	payload, err := unpackPayload(args[2:])
	if err != nil {
		return err
	}
	var program Patch
	if err := program.decode(payload); err != nil {
		return err
	}
	candidate := PatchData{Address: address(args), Patch: program}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*p = candidate
	return nil
}

// SystemData is the instrument's settings block. The payload is carried and
// returned decoded rather than modelled: the notes that describe its fields
// disagree with each other about the byte order of the tempo, so a field
// layout built from them would be a guess dressed as a specification. A
// caller that needs the settings can archive and restore them exactly, which
// is the part of them that a dump is normally wanted for.
// A patch dump and the settings dump share one function ID, so the address
// is the only thing that tells them apart. The notes describe exactly one
// address past the patches, so that is the one value the address takes, and
// it is what stops a patch from being read as a settings block.
type SystemData struct {
	Address int `oneof:"510"`
	Raw     []byte
}

// maxSystemSize bounds the settings block. The settings block the notes
// transcribe decodes to 48 bytes and nothing describes a larger one, so this
// is a guard against handing the packer a slice that is not a settings block
// at all rather than a limit the instrument imposes.
const maxSystemSize = 1024

// MarshalBinary encodes a settings dump response.
func (s *SystemData) MarshalBinary() ([]byte, error) {
	if err := sysex.CheckTaggedFields(s); err != nil {
		return nil, err
	}
	if len(s.Raw) > maxSystemSize {
		return nil, sysex.ErrDataTooLarge
	}
	return build(FuncDumpResponse, addressBytes(s.Address), packPayload(s.Raw)), nil
}

// UnmarshalBinary decodes a settings dump response.
func (s *SystemData) UnmarshalBinary(data []byte) error {
	if len(data) < headerSize+4 {
		return sysex.ErrBadRange
	}
	args, err := splitHeader(data, FuncDumpResponse)
	if err != nil {
		return err
	}
	raw, err := unpackPayload(args[2:])
	if err != nil {
		return err
	}
	candidate := SystemData{Address: address(args), Raw: raw}
	if err := sysex.CheckTaggedFields(&candidate); err != nil {
		return err
	}
	*s = candidate
	return nil
}

// Decode reads any Pro 800 message and returns the kind it is, or nil when
// the bytes are a message this package does not model. It is what a caller
// reading from a MIDI port wants, since the function ID alone does not say
// which of the two dump responses a message is.
//
// A message the instrument sends that is not modelled here returns nil with
// no error, because it also sends firmware updates and a status message
// while it is being tuned, and neither is a fault.
func Decode(data []byte) (any, error) {
	if len(data) < headerSize+2 {
		return nil, sysex.ErrBadRange
	}
	switch data[functionOffset] {
	case FuncUnsupportedAddress:
		message := &UnsupportedAddress{}
		if err := message.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return message, nil
	case FuncVersionRequest:
		message := &VersionRequest{}
		if err := message.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return message, nil
	case FuncVersionResponse:
		message := &VersionResponse{}
		if err := message.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return message, nil
	case FuncDumpRequest:
		message := &DumpRequest{}
		if err := message.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return message, nil
	case FuncDumpResponse:
		if len(data) < headerSize+4 {
			return nil, sysex.ErrBadRange
		}
		// A patch dump and the settings dump share this function, so the
		// address decides which of the two it is.
		if address(data[functionOffset+1:]) == SystemAddress {
			message := &SystemData{}
			if err := message.UnmarshalBinary(data); err != nil {
				return nil, err
			}
			return message, nil
		}
		message := &PatchData{}
		if err := message.UnmarshalBinary(data); err != nil {
			return nil, err
		}
		return message, nil
	}
	return nil, nil
}
