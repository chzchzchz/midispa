package sysex

const (
	SubIdEOF    = 0x7b
	SubIdWait   = 0x7c
	SubIdCancel = 0x7d
	SubIdNAK    = 0x7e
	SubIdACK    = 0x7f
)

type Handshake struct {
	DeviceId int
	SubId    int
	Packet   int
}

func (h *Handshake) Encode() []byte {
	if h.SubId == 0 {
		panic("bad sub id")
	}
	return []byte{
		0xF0, IdNonRealTime, byte(h.DeviceId),
		byte(h.SubId), byte(h.Packet),
		0xf7,
	}
}

func HandshakeFromSysEx(data []byte) *Handshake {
	return &Handshake{
		DeviceId: int(data[0]),
		SubId:    int(data[1]),
		Packet:   int(data[2]),
	}
}
