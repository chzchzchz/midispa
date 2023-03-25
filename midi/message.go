package midi

// First byte of a message.
const (
	SysEx    byte = 0xf0
	EndSysEx      = 0xf7
	Clock         = 0xf8
	Tick          = 0xf9
	Start         = 0xfa
	Continue      = 0xfb
	Stop          = 0xfc
	NoteOn        = 0x90
	NoteOff       = 0x80
	CC            = 0xb0
	Pgm           = 0xc0
)

func IsMessage(b byte) bool  { return b&0x80 == 0x80 }
func IsNoteOn(b byte) bool   { return Message(b) == NoteOn }
func IsNoteOff(b byte) bool  { return Message(b) == NoteOff }
func IsCC(b byte) bool       { return Message(b) == CC }
func Channel(b byte) int     { return int(b & 0x0f) }
func Message(b byte) byte    { return b & 0xf0 }
func IsRealtime(b byte) bool { return b&0xf0 == 0xf0 }

func MakeNoteOn(channel int) byte  { return byte(channel) | NoteOn }
func MakeNoteOff(channel int) byte { return byte(channel) | NoteOff }
func MakeCC(channel int) byte      { return byte(channel) | CC }
func MakePgm(channel int) byte     { return byte(channel) | Pgm }
