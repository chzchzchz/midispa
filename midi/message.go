package midi

// First byte of a message.
const (
	SysEx             byte = 0xf0
	QuarterFrame           = 0xf1
	SongPosition           = 0xf2
	SongSelect             = 0xf3
	EndSysEx               = 0xf7
	Clock                  = 0xf8
	Tick                   = 0xf9
	Start                  = 0xfa
	Continue               = 0xfb
	Stop                   = 0xfc
	NoteOff                = 0x80
	NoteOn                 = 0x90
	KeyAftertouch          = 0xa0
	CC                     = 0xb0
	Pgm                    = 0xc0
	ChannelAftertouch      = 0xd0
	Pitch                  = 0xe0
)

// Control codes.
const (
	AllSoundOff = 120
	AllNotesOff = 123
)

func IsMessage(b byte) bool { return b&0x80 == 0x80 }
func IsNoteOn(b byte) bool  { return Message(b) == NoteOn }
func IsNoteOff(b byte) bool { return Message(b) == NoteOff }
func IsCC(b byte) bool      { return Message(b) == CC }
func IsPitch(b byte) bool   { return Message(b) == Pitch }
func IsPgm(b byte) bool     { return Message(b) == Pgm }
func Channel(b byte) int    { return int(b & 0x0f) }

func IsRealtime(b byte) bool       { return b&0xf0 == 0xf0 }
func IsClock(b byte) bool          { return b >= Clock && b <= Stop }
func IsSystemMessage(b byte) bool  { return b >= 0xf0 }
func IsChannelMessage(b byte) bool { return b >= 0x80 && b < 0xf0 }
func Message(b byte) byte {
	if IsSystemMessage(b) {
		return b
	}
	return b & 0xf0
}

func MakeNoteOn(channel int) byte  { return byte(channel) | NoteOn }
func MakeNoteOff(channel int) byte { return byte(channel) | NoteOff }
func MakeCC(channel int) byte      { return byte(channel) | CC }
func MakePgm(channel int) byte     { return byte(channel) | Pgm }
