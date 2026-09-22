package sysex

const (
	FileDumpSubIdEOF    = 0x7b
	FileDumpSubIdWait   = 0x7c
	FileDumpSubIdCancel = 0x7d
	FileDumpSubIdNAK    = 0x7e
	FileDumpSubIdACK    = 0x7f
)

type Handshake struct {
	DeviceId int
	SubId    int
	Packet   int
}

func (h *Handshake) MarshalBinary() ([]byte, error) {
	if h.SubId == 0 {
		return nil, ErrBadSubId
	}
	return []byte{
		0xF0, IdNonRealTime, byte(h.DeviceId),
		byte(h.SubId), byte(h.Packet),
		0xf7,
	}, nil
}

func HandshakeFromSysEx(data []byte) (*Handshake, error) {
	if len(data) < 3 || data[len(data)-1] != 0xf7 {
		return nil, ErrBadHeader
	}
	return &Handshake{
		DeviceId: int(data[0]),
		SubId:    int(data[1]),
		Packet:   int(data[2]),
	}, nil
}
