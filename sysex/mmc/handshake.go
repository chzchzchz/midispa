package mmc

// Wait encodes the WAIT handshake command (MMC code 0x7C).
// Format: F0 7F <device_ID> 06 7C F7
// The spec requires WAIT and RESUME to be transmitted as the only
// message in their respective System Exclusive.
type Wait struct {
	DeviceId int
}

// Resume encodes the RESUME handshake command (MMC code 0x7D).
// Format: F0 7F <device_ID> 06 7D F7
type Resume struct {
	DeviceId int
}

func (w *Wait) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(w.DeviceId, MMCWait)
}

func (r *Resume) MarshalBinary() ([]byte, error) {
	return MarshalMMCCommand(r.DeviceId, MMCResume)
}

// UnmarshalWait decodes a WAIT message from raw bytes.
func UnmarshalWait(data []byte) (*Wait, error) {
	if len(data) < 6 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != SubIdMMCCommand || data[4] != MMCWait ||
		data[len(data)-1] != EndSysEx {
		return nil, ErrBadHeader
	}
	return &Wait{DeviceId: int(data[2])}, nil
}

// UnmarshalResume decodes a RESUME message from raw bytes.
func UnmarshalResume(data []byte) (*Resume, error) {
	if len(data) < 6 || data[0] != SysEx || data[1] != IdRealTime ||
		data[3] != SubIdMMCCommand || data[4] != MMCResume ||
		data[len(data)-1] != EndSysEx {
		return nil, ErrBadHeader
	}
	return &Resume{DeviceId: int(data[2])}, nil
}
