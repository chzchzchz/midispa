package mmc

import "errors"

const (
	SysEx           = 0xf0
	EndSysEx        = 0xf7
	IdRealTime      = 0x7f
	SubIdMMCCommand = 6
)

var ErrBadRange = errors.New("bad value range")
var ErrBadHeader = errors.New("bad header")
var ErrNoEox = errors.New("no EOX")

// CommandMarshaler is a minimal interface for types that implement MarshalBinary.
type CommandMarshaler interface {
	MarshalBinary() ([]byte, error)
}

// Command codes
const (
	MMCStop                 = 1
	MMCPlay                 = 2
	MMCDeferredPlay         = 3
	MMCFastForward          = 4
	MMCRewind               = 5
	MMCRecordStrobe         = 6
	MMCRecordExit           = 7
	MMCRecordPause          = 8
	MMCPause                = 9
	MMCEject                = 0x0a
	MMCChase                = 0x0b
	MMCCommandErrorReset    = 0x0c
	MMCReset                = 0x0d
	MMCWrite                = 0x0e
	MMCMaskedWrite          = 0x0f
	MMCRead                 = 0x10
	MMCUpdate               = 0x11
	MMCLocate               = 0x12
	MMCSearch               = 0x13
	MMCShuttle              = 0x14
	MMCVariablePlay         = 0x15
	MMCStep                 = 0x16
	MMCAssignSystemMaster   = 0x17
	MMCGeneratorCommand     = 0x18
	MMCTimeCodeCommand      = 0x19
	MMCMove                 = 0x1a
	MMCAdd                  = 0x1b
	MMCSubtract             = 0x1c
	MMCDropFrameAdjust      = 0x1d
	MMCProcedure            = 0x1e
	MMCEvent                = 0x1f
	MMCGroup                = 0x20
	MMCCommandSegment       = 0x21
	MMCDeferredVariablePlay = 0x22
	MMCRecordStrobeVariable = 0x23
	MMCLocateIF             = 0x00
	MMCLocateTarget         = 0x01
	MMCWait                 = 0x7c
	MMCResume               = 0x7d
)

// Device types
type RecordStrobe struct{ DeviceId int }
type Eject struct{ DeviceId int }
type RecordExit struct{ DeviceId int }
type Play struct{ DeviceId int }
type Stop struct{ DeviceId int }
type Rewind struct{ DeviceId int }
type FastForward struct{ DeviceId int }
type DeferredPlay struct{ DeviceId int }
type RecordPause struct{ DeviceId int }
type Pause struct{ DeviceId int }

type StandardSpeed [3]byte
type LocateIF struct {
	DeviceId int
	Name     byte
}
type LocateTarget struct {
	DeviceId int
	TimeCode [5]byte
}
type Chase struct{ DeviceId int }
type CommandErrorReset struct{ DeviceId int }
type Reset struct{ DeviceId int }
type WritePair struct {
	Name byte
	Data []byte
}
type Write struct {
	DeviceId int
	Pairs    []WritePair
}
type MaskedWrite struct {
	DeviceId                  int
	Name, ByteNum, Mask, Data byte
}
type Read struct {
	DeviceId int
	Names    []byte
}
type Update struct {
	DeviceId int
	SubCmd   byte
	Names    []byte
}
type Search struct {
	DeviceId int
	Speed    StandardSpeed
}
type Shuttle struct {
	DeviceId int
	Speed    StandardSpeed
}
type VariablePlay struct {
	DeviceId int
	Speed    StandardSpeed
}
type Step struct {
	DeviceId int
	Steps    byte
}
type AssignSystemMaster struct {
	DeviceId int
	MasterId int
}
type GeneratorCommand struct {
	DeviceId int
	Action   byte
}
type TimeCodeCommand struct {
	DeviceId int
	Action   byte
}
type Move struct {
	DeviceId     int
	Dest, Source byte
}
type Add struct {
	DeviceId               int
	Dest, Source1, Source2 byte
}
type Subtract struct {
	DeviceId               int
	Dest, Source1, Source2 byte
}
type DropFrameAdjust struct {
	DeviceId int
	Name     byte
}
type DeferredVariablePlay struct {
	DeviceId int
	Speed    StandardSpeed
}
type RecordStrobeVariable struct {
	DeviceId int
	Speed    StandardSpeed
}
type Procedure struct {
	DeviceId int
	Action   byte
}
type Event struct {
	DeviceId int
	Action   byte
}
type Group struct {
	DeviceId int
	GroupID  byte
}
type CommandSegment struct {
	DeviceId   int
	SegmentNum byte
	Data       []byte
}

// Helpers
func MarshalMMCCommand(deviceId int, commandCode byte) ([]byte, error) {
	if deviceId < 0 || deviceId > 0x7e {
		return nil, ErrBadRange
	}
	return []byte{SysEx, IdRealTime, byte(deviceId), byte(SubIdMMCCommand), commandCode, EndSysEx}, nil
}

func MarshalMMCResponse(deviceId int, responseCode byte) ([]byte, error) {
	if deviceId < 0 || deviceId > 0x7e {
		return nil, ErrBadRange
	}
	return []byte{SysEx, IdRealTime, byte(deviceId), 0x07, responseCode, EndSysEx}, nil
}

func MarshalMMCCommandWithData(deviceId int, commandCode byte, data []byte) ([]byte, error) {
	if deviceId < 0 || deviceId > 0x7e {
		return nil, ErrBadRange
	}
	ret := []byte{SysEx, IdRealTime, byte(deviceId), byte(SubIdMMCCommand), commandCode, byte(len(data))}
	ret = append(ret, data...)
	ret = append(ret, EndSysEx)
	return ret, nil
}

func MarshalMMCResponseWithData(deviceId int, responseCode byte, data []byte) ([]byte, error) {
	if deviceId < 0 || deviceId > 0x7e {
		return nil, ErrBadRange
	}
	ret := []byte{SysEx, IdRealTime, byte(deviceId), 0x07, responseCode, byte(len(data))}
	ret = append(ret, data...)
	ret = append(ret, EndSysEx)
	return ret, nil
}

// MarshalBinary for all command types
func (r *RecordStrobe) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(r.DeviceId, MMCRecordStrobe)
}
func (e *Eject) MarshalBinary() ([]byte, error) { return MarshalMMCCommand(e.DeviceId, MMCEject) }
func (r *RecordExit) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(r.DeviceId, MMCRecordExit)
}
func (p *Play) MarshalBinary() ([]byte, error)   { return MarshalMMCCommand(p.DeviceId, MMCPlay) }
func (s *Stop) MarshalBinary() ([]byte, error)   { return MarshalMMCCommand(s.DeviceId, MMCStop) }
func (r *Rewind) MarshalBinary() ([]byte, error) { return MarshalMMCCommand(r.DeviceId, MMCRewind) }
func (f *FastForward) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(f.DeviceId, MMCFastForward)
}
func (d *DeferredPlay) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(d.DeviceId, MMCDeferredPlay)
}
func (r *RecordPause) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(r.DeviceId, MMCRecordPause)
}
func (p *Pause) MarshalBinary() ([]byte, error) { return MarshalMMCCommand(p.DeviceId, MMCPause) }
func (c *Chase) MarshalBinary() ([]byte, error) { return MarshalMMCCommand(c.DeviceId, MMCChase) }
func (c *CommandErrorReset) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(c.DeviceId, MMCCommandErrorReset)
}
func (r *Reset) MarshalBinary() ([]byte, error) { return MarshalMMCCommand(r.DeviceId, MMCReset) }
func (l *LocateIF) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(l.DeviceId, MMCLocate, []byte{MMCLocateIF, l.Name})
}
func (l *LocateTarget) MarshalBinary() ([]byte, error) {
	data := []byte{MMCLocateTarget}
	data = append(data, l.TimeCode[:]...)
	return MarshalMMCCommandWithData(l.DeviceId, MMCLocate, data)
}
func (s *Search) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(s.DeviceId, MMCSearch, s.Speed[:])
}
func (s *Shuttle) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(s.DeviceId, MMCShuttle, s.Speed[:])
}
func (v *VariablePlay) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(v.DeviceId, MMCVariablePlay, v.Speed[:])
}
func (s *Step) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(s.DeviceId, MMCStep, []byte{s.Steps})
}
func (w *Write) MarshalBinary() ([]byte, error) {
	if w.DeviceId < 0 || w.DeviceId > 0x7e {
		return nil, ErrBadRange
	}
	var ret []byte
	for _, p := range w.Pairs {
		ret = append(ret, p.Name)
		ret = append(ret, p.Data...)
	}
	return MarshalMMCCommandWithData(w.DeviceId, MMCWrite, ret)
}
func (m *MaskedWrite) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(m.DeviceId, MMCMaskedWrite, []byte{m.Name, m.ByteNum, m.Mask, m.Data})
}
func (r *Read) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(r.DeviceId, MMCRead, r.Names)
}
func (u *Update) MarshalBinary() ([]byte, error) {
	data := []byte{u.SubCmd}
	data = append(data, u.Names...)
	return MarshalMMCCommandWithData(u.DeviceId, MMCUpdate, data)
}
func (a *AssignSystemMaster) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(a.DeviceId, MMCAssignSystemMaster, []byte{byte(a.MasterId)})
}
func (g *GeneratorCommand) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(g.DeviceId, MMCGeneratorCommand, []byte{g.Action})
}
func (t *TimeCodeCommand) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(t.DeviceId, MMCTimeCodeCommand, []byte{t.Action})
}
func (m *Move) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(m.DeviceId, MMCMove, []byte{m.Dest, m.Source})
}
func (a *Add) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(a.DeviceId, MMCAdd, []byte{a.Dest, a.Source1, a.Source2})
}
func (s *Subtract) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(s.DeviceId, MMCSubtract, []byte{s.Dest, s.Source1, s.Source2})
}
func (d *DropFrameAdjust) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(d.DeviceId, MMCDropFrameAdjust, []byte{d.Name})
}
func (d *DeferredVariablePlay) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(d.DeviceId, MMCDeferredVariablePlay, d.Speed[:])
}
func (r *RecordStrobeVariable) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(r.DeviceId, MMCRecordStrobeVariable, r.Speed[:])
}
func (p *Procedure) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(p.DeviceId, MMCProcedure, []byte{p.Action})
}
func (e *Event) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(e.DeviceId, MMCEvent, []byte{e.Action})
}
func (g *Group) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommandWithData(g.DeviceId, MMCGroup, []byte{g.GroupID})
}
func (c *CommandSegment) MarshalBinary() ([]byte, error) {
	data := []byte{c.SegmentNum}
	data = append(data, c.Data...)
	return MarshalMMCCommandWithData(c.DeviceId, MMCCommandSegment, data)
}
