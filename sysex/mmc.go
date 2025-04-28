package sysex

const (
	SubIdMMCCommand = 6
	SubIDMMCReply   = 7

	MMCStop         = 1
	MMCPlay         = 2
	MMCDeferredPlay = 3
	MMCFastForward  = 4
	MMCRewind       = 5
	MMCRecordStrobe = 6
	MMCRecordExit   = 7
	MMCRecordPause  = 8
	MMCPause        = 9
	MMCEject        = 0xa
)

type RecordStrobe struct{}
type Eject struct{}
type RecordExit struct{}
type Play struct{}
type Stop struct{}
type Rewind struct{}
type FastForward struct{}
