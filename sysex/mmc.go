package sysex

const (
	SubIdMMCCommand = 6
	SubIDMMCReply   = 7

	MMCRecordStrobe = 6
	MMCRecordExit   = 7
	MMCEject        = 0xa
)

type RecordStrobe struct{}
type Eject struct{}
type RecordExit struct{}
