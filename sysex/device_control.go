package sysex

const (
	SubIdDeviceControl = 4

	DeviceControlIdMasterVolume  = 1
	DeviceControlIdMasterBalance = 2
)

type MasterVolume struct {
	DeviceId int
	Volume   int
}

type MasterBalance struct {
	DeviceId int
	Balance  int
}

func (m *MasterBalance) Encode() []byte {
	v := encode7bitInt(m.Balance, 2)
	return []byte{
		0xf0, IdRealTime, byte(m.DeviceId),
		SubIdDeviceControl, DeviceControlIdMasterBalance,
		v[0], v[1],
		0xf7}
}

func (m *MasterVolume) Encode() []byte {
	v := encode7bitInt(m.Volume, 2)
	return []byte{
		0xf0, IdRealTime, byte(m.DeviceId),
		SubIdDeviceControl, DeviceControlIdMasterVolume,
		v[0], v[1],
		0xf7}
}

func MasterVolumeFromSysEx(data []byte) *MasterVolume {
	return &MasterVolume{
		DeviceId: int(data[0]),
		Volume:   decode7bitInt(data[3:5]),
	}
}

func (m *MasterVolume) Float32() float32 {
	return float32(m.Volume) / float32((1<<14)-1)
}

func MasterBalanceFromSysEx(data []byte) *MasterBalance {
	return &MasterBalance{
		DeviceId: int(data[0]),
		Balance:  decode7bitInt(data[3:5]),
	}
}
