package mmc

// TimeCode represents a 5-byte Standard Time Code specification with status (type {st}).
// Bytes: hr mn sc fr st
//
//	hr: tt hhhhhh (tt=time type 2 bits, hhhhhh=hours 0-23)
//	mn: c mmmmmm (c=color frame flag, mmmmmm=minutes 0-59)
//	sc: k ssssss (k=blank/field bit, ssssss=seconds 0-59)
//	fr: g i ffffff (g=sign, i=final byte id, ffffff=frames 0-29)
//	st: e v d n xxx (status bits when i=1, or fractional frames when i=0)
type TimeCode [5]byte

// TimeCodeType encodes the time format type bits in the hours byte.
const (
	TimeCodeType24Drop = 0b00 // 24 frame
	TimeCodeType25     = 0b01 // 25 frame
	TimeCodeType30Drop = 0b10 // 30 drop frame
	TimeCodeType30     = 0b11 // 30 frame
)

// Hours returns the hours value (0-23) from the time code.
func (t TimeCode) Hours() int { return int(t[0] & 0x3f) }

// TimeType returns the time type (00=24fps, 01=25fps, 10=30drop, 11=30fps).
func (t TimeCode) TimeType() int { return int((t[0] >> 6) & 0x03) }

// Minutes returns the minutes value (0-59).
func (t TimeCode) Minutes() int { return int(t[1] & 0x3f) }

// ColorFrame returns whether the color frame flag is set.
func (t TimeCode) ColorFrame() bool { return (t[1]>>6)&0x01 == 0x01 }

// Seconds returns the seconds value (0-59).
func (t TimeCode) Seconds() int { return int(t[2] & 0x3f) }

// Blank returns whether the blank bit is set.
func (t TimeCode) Blank() bool { return (t[2]>>6)&0x01 == 0x01 }

// Frames returns the frames value (0-29).
func (t TimeCode) Frames() int { return int(t[3] & 0x1f) }

// Sign returns whether the time code is negative.
func (t TimeCode) Sign() bool { return (t[3]>>6)&0x01 == 0x01 }

// FinalByteID returns 0 for subframes, 1 for status byte.
func (t TimeCode) FinalByteID() int { return int((t[3] >> 5) & 0x01) }

// StatusByte returns the status byte (byte 4) when FinalByteID is 1.
func (t TimeCode) StatusByte() byte { return t[4] }

// FractionalFrames returns the fractional frames value (0-99) when FinalByteID is 0.
func (t TimeCode) FractionalFrames() int { return int(t[4] & 0x7f) }

// ShortTimeCode represents a 2-byte Standard Short Time Code specification.
// Contains only frames and subframes/status data.
type ShortTimeCode [2]byte

// Frames returns the frames value from the short time code.
func (s ShortTimeCode) Frames() int { return int(s[0] & 0x3f) }

// StatusByte returns the status/subframe byte.
func (s ShortTimeCode) StatusByte() byte { return s[1] }

// FractionalFrames returns the fractional frames value (0-99) when bit 7 of s[1] is 0.
func (s ShortTimeCode) FractionalFrames() int { return int(s[1] & 0x7f) }

// SelectedTimeCode (01h) — 5 bytes, Standard Time Code with status (type {st}).
// Contains the time code normally used to reference the Controlled Device's current position.
type SelectedTimeCode struct {
	DeviceId int
	Time     TimeCode
}

// SelectedMasterCode (02h) — 5 bytes, Standard Time Code with status (type {st}).
// Contains the time value relative to which all synchronization operations take place.
type SelectedMasterCode struct {
	DeviceId int
	Time     TimeCode
}

// GpoLocatePoint (08h) — 5 bytes, Standard Time Code with subframes (type {ff}).
// General Purpose time code and calculation register 0.
type GpoLocatePoint struct {
	DeviceId int
	Time     TimeCode // uses {ff} format (subframes in final byte)
}

// Gp (09h-0Fh) — 5 bytes each, General Purpose registers 1-7.
type Gp struct {
	DeviceId int
	Time     TimeCode // uses {ff} format (subframes in final byte)
}

// MotionControlTally (12h) — 3 bytes.
// ms = Most recently activated Motion Control State
// mp = Most recently activated Motion Control Process
// ss = Status and success levels (bbb.aaa format)
type MotionControlTally struct {
	DeviceId int
	MSC      byte // Most recently activated Motion Control State
	MCP      byte // Most recently activated Motion Control Process
	Status   byte // Status and success levels: bbb.aaa (3-bit MCP, 3-bit MCS)
}

// MSSuccessLevel returns the MCS success level (bits 0-2 of Status).
func (m *MotionControlTally) MSSuccessLevel() int { return int(m.Status & 0x07) }

// MCPSuccessLevel returns the MCP success level (bits 3-5 of Status).
func (m *MotionControlTally) MCPSuccessLevel() int { return int((m.Status >> 3) & 0x07) }

// VelocityTally (13h) — 1 byte.
type VelocityTally struct {
	DeviceId int
	Speed    byte // speed value
}

// StopMode (14h) — 1 byte.
type StopMode struct {
	DeviceId int
	Mode     byte
}

// FastMode (15h) — 1 byte.
type FastMode struct {
	DeviceId int
	Mode     byte
}

// RecordMode (16h) — 1 byte.
type RecordMode struct {
	DeviceId int
	Mode     byte
}

// RecordStatus (17h) — 1 byte.
type RecordStatus struct {
	DeviceId int
	Status   byte
}

// UpdateRate (41h) — 1 byte.
// Minimum time interval between repetitive UPDATE transmissions (7-bit frame count).
type UpdateRate struct {
	DeviceId int
	Interval byte // 7-bit frame count (0-127)
}

// ResponseError (42h) — identifies an unsupported Information Field.
type ResponseError struct {
	DeviceId int
	Fields   []byte // list of unsupported information field names
}

// CommandError (43h) — identifies the error and the command that caused it.
type CommandError struct {
	DeviceId int
	Code     byte // error code
	Command  byte // command that caused the error
}

// CommandErrorLevel (44h) — bitmask of enabled command errors.
type CommandErrorLevel struct {
	DeviceId int
	Level    byte // bitmask of enabled errors
}

// Signature (40h) — dual bitmap array of supported commands and response fields.
// Format: vi vf va vb <count_1> <c0...> <count_2> <r0...>
// Reference: RP-013_v1-0_MIDI_Machine_Control_Specification_96-1-4.pdf, pp. 48-49.
type Signature struct {
	DeviceId     int
	VersionMajor byte // MMC version integer part
	VersionMinor byte // MMC version fractional part (00-63h)
	// Extended version fields (va, vb) — reserved, must be 00
	CommandBitmap  []byte // contiguous command bitmap array
	ResponseBitmap []byte // contiguous response/information field bitmap array
}

// MarshalBinary encodes SelectedTimeCode.
// Format: F0 7F <device> 07 01 hr mn sc fr st F7
func (s *SelectedTimeCode) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(s.DeviceId, 0x01, s.Time[:])
}

// MarshalBinary encodes SelectedMasterCode.
func (s *SelectedMasterCode) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(s.DeviceId, 0x02, s.Time[:])
}

// MarshalBinary encodes GpoLocatePoint.
func (g *GpoLocatePoint) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(g.DeviceId, 0x08, g.Time[:])
}

// MarshalBinary encodes Gp.
func (g *Gp) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(g.DeviceId, 0x09, g.Time[:])
}

// MarshalBinary encodes MotionControlTally.
// Format: F0 7F <device> 07 12 ms mp ss F7
func (m *MotionControlTally) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(m.DeviceId, 0x12, []byte{m.MSC, m.MCP, m.Status})
}

// MarshalBinary encodes VelocityTally.
func (v *VelocityTally) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(v.DeviceId, 0x13, []byte{v.Speed})
}

// MarshalBinary encodes StopMode.
func (s *StopMode) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(s.DeviceId, 0x14, []byte{s.Mode})
}

// MarshalBinary encodes FastMode.
func (s *FastMode) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(s.DeviceId, 0x15, []byte{s.Mode})
}

// MarshalBinary encodes RecordMode.
func (s *RecordMode) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(s.DeviceId, 0x16, []byte{s.Mode})
}

// MarshalBinary encodes RecordStatus.
func (s *RecordStatus) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(s.DeviceId, 0x17, []byte{s.Status})
}

// MarshalBinary encodes UpdateRate.
func (u *UpdateRate) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(u.DeviceId, 0x41, []byte{u.Interval})
}

// MarshalBinary encodes ResponseError.
func (r *ResponseError) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(r.DeviceId, 0x42, r.Fields)
}

// MarshalBinary encodes CommandError.
func (c *CommandError) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(c.DeviceId, 0x43, []byte{c.Code, c.Command})
}

// MarshalBinary encodes CommandErrorLevel.
func (c *CommandErrorLevel) MarshalBinary() ([]byte, error) {
	return MarshalMMCResponseWithData(c.DeviceId, 0x44, []byte{c.Level})
}

// MarshalBinary encodes Signature.
func (s *Signature) MarshalBinary() ([]byte, error) {
	data := []byte{s.VersionMajor, s.VersionMinor, 0x00, 0x00}
	// RP-013_v1-0_MIDI_Machine_Control_Specification_96-1-4.pdf, pp. 48-49:
	// SIGNATURE has one length byte for each contiguous bitmap array.
	data = append(data, byte(len(s.CommandBitmap)))
	data = append(data, s.CommandBitmap...)
	data = append(data, byte(len(s.ResponseBitmap)))
	data = append(data, s.ResponseBitmap...)
	return MarshalMMCResponseWithData(s.DeviceId, 0x40, data)
}

// DecodeTimeCode decodes a 5-byte Standard Time Code from raw data.
func DecodeTimeCode(data []byte) (TimeCode, error) {
	if len(data) != 5 {
		return TimeCode{}, ErrBadRange
	}
	var tc TimeCode
	copy(tc[:], data)
	return tc, nil
}

// DecodeShortTimeCode decodes a 2-byte Short Time Code from raw data.
func DecodeShortTimeCode(data []byte) (ShortTimeCode, error) {
	if len(data) != 2 {
		return ShortTimeCode{}, ErrBadRange
	}
	var st ShortTimeCode
	copy(st[:], data)
	return st, nil
}

// UnmarshalSelectedTimeCode decodes a SELECTED TIME CODE response from raw bytes.
// Format: F0 7F <dev> 07 01 hr mn sc fr st F7
func UnmarshalSelectedTimeCode(data []byte) (*SelectedTimeCode, error) {
	if len(data) < 12 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x01 || data[5] != 0x05 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	var tc TimeCode
	copy(tc[:], data[6:11])
	return &SelectedTimeCode{DeviceId: int(data[2]), Time: tc}, nil
}

// UnmarshalSelectedMasterCode decodes a SELECTED MASTER CODE response.
func UnmarshalSelectedMasterCode(data []byte) (*SelectedMasterCode, error) {
	if len(data) < 12 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x02 || data[5] != 0x05 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	var tc TimeCode
	copy(tc[:], data[6:11])
	return &SelectedMasterCode{DeviceId: int(data[2]), Time: tc}, nil
}

// UnmarshalMotionControlTally decodes a MOTION CONTROL TALLY response.
func UnmarshalMotionControlTally(data []byte) (*MotionControlTally, error) {
	if len(data) < 10 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x12 || data[5] != 0x03 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &MotionControlTally{
		DeviceId: int(data[2]),
		MSC:      data[6],
		MCP:      data[7],
		Status:   data[8],
	}, nil
}

// UnmarshalGpoLocatePoint decodes a GPO/LOCATE POINT response.
func UnmarshalGpoLocatePoint(data []byte) (*GpoLocatePoint, error) {
	if len(data) < 12 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x08 || data[5] != 0x05 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	var tc TimeCode
	copy(tc[:], data[6:11])
	return &GpoLocatePoint{DeviceId: int(data[2]), Time: tc}, nil
}

// UnmarshalVelocityTally decodes a VELOCITY TALLY response.
func UnmarshalVelocityTally(data []byte) (*VelocityTally, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x13 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &VelocityTally{DeviceId: int(data[2]), Speed: data[6]}, nil
}

// UnmarshalStopMode decodes a STOP MODE response.
func UnmarshalStopMode(data []byte) (*StopMode, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x14 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &StopMode{DeviceId: int(data[2]), Mode: data[6]}, nil
}

// UnmarshalFastMode decodes a FAST MODE response.
func UnmarshalFastMode(data []byte) (*FastMode, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x15 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &FastMode{DeviceId: int(data[2]), Mode: data[6]}, nil
}

// UnmarshalRecordMode decodes a RECORD MODE response.
func UnmarshalRecordMode(data []byte) (*RecordMode, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x16 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &RecordMode{DeviceId: int(data[2]), Mode: data[6]}, nil
}

// UnmarshalRecordStatus decodes a RECORD STATUS response.
func UnmarshalRecordStatus(data []byte) (*RecordStatus, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x17 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &RecordStatus{DeviceId: int(data[2]), Status: data[6]}, nil
}

// UnmarshalUpdateRate decodes an UPDATE RATE response.
func UnmarshalUpdateRate(data []byte) (*UpdateRate, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x41 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &UpdateRate{DeviceId: int(data[2]), Interval: data[6]}, nil
}

// UnmarshalGp decodes a GENERAL PURPOSE REGISTER response.
// Format: F0 7F <dev> 07 09 <count=05> hr mn sc fr st F7
func UnmarshalGp(data []byte) (*Gp, error) {
	if len(data) < 12 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x09 || data[5] != 0x05 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	var tc TimeCode
	copy(tc[:], data[6:11])
	return &Gp{DeviceId: int(data[2]), Time: tc}, nil
}

// UnmarshalCommandError decodes a COMMAND ERROR response.
func UnmarshalCommandError(data []byte) (*CommandError, error) {
	if len(data) < 9 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x43 || data[5] != 0x02 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &CommandError{
		DeviceId: int(data[2]),
		Code:     data[6],
		Command:  data[7],
	}, nil
}

// UnmarshalCommandErrorLevel decodes a COMMAND ERROR LEVEL response.
func UnmarshalCommandErrorLevel(data []byte) (*CommandErrorLevel, error) {
	if len(data) < 8 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x44 || data[5] != 0x01 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	return &CommandErrorLevel{DeviceId: int(data[2]), Level: data[6]}, nil
}

// UnmarshalSignature decodes a SIGNATURE response.
func UnmarshalSignature(data []byte) (*Signature, error) {
	if len(data) < 13 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != 0x07 || data[4] != 0x40 {
		return nil, ErrBadHeader
	}
	if data[len(data)-1] != EndSysEx {
		return nil, ErrNoEox
	}
	count := int(data[5])
	payloadEnd := 6 + count
	if count < 6 || len(data) != payloadEnd+1 {
		return nil, ErrBadHeader
	}
	payload := data[6:payloadEnd]
	commandBitmapLen := int(payload[4])
	responseCountPos := 5 + commandBitmapLen
	if responseCountPos >= len(payload) {
		return nil, ErrBadHeader
	}
	responseBitmapLen := int(payload[responseCountPos])
	if responseCountPos+1+responseBitmapLen != len(payload) {
		return nil, ErrBadRange
	}
	// RP-013_v1-0_MIDI_Machine_Control_Specification_96-1-4.pdf, pp. 48-49,
	// defines one leading count byte for each contiguous bitmap array.
	return &Signature{
		DeviceId:       int(data[2]),
		VersionMajor:   payload[0],
		VersionMinor:   payload[1],
		CommandBitmap:  append([]byte(nil), payload[5:responseCountPos]...),
		ResponseBitmap: append([]byte(nil), payload[responseCountPos+1:]...),
	}, nil
}
