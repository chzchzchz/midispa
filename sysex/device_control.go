package sysex

import (
	"math"
	"time"
)

const (
	SubIdDeviceControl = 4

	DeviceControlIdMasterVolume           = 1
	DeviceControlIdMasterBalance          = 2
	DeviceControlIdGlobalParameterControl = 5
)

type MasterVolume struct {
	DeviceId int
	Volume   int
}

type MasterBalance struct {
	DeviceId int
	Balance  int
}

func (m *MasterBalance) MarshalBinary() ([]byte, error) {
	v, err := encode7bitInt(m.Balance, 2)
	if err != nil {
		return nil, err
	}
	return []byte{
		0xf0, IdRealTime, byte(m.DeviceId),
		SubIdDeviceControl, DeviceControlIdMasterBalance,
		v[0], v[1],
		0xf7}, nil
}

func (m *MasterVolume) MarshalBinary() ([]byte, error) {
	v, err := encode7bitInt(m.Volume, 2)
	if err != nil {
		return nil, err
	}
	return []byte{
		0xf0, IdRealTime, byte(m.DeviceId),
		SubIdDeviceControl, DeviceControlIdMasterVolume,
		v[0], v[1],
		0xf7}, nil
}

func MasterVolumeFromSysEx(data []byte) (*MasterVolume, error) {
	vol, err := decode7bitInt(data[3:5])
	if err != nil {
		return nil, err
	}
	return &MasterVolume{
		DeviceId: int(data[0]),
		Volume:   vol,
	}, nil
}

func (m *MasterVolume) Float32() float32 {
	return float32(m.Volume) / float32((1<<14)-1)
}

func MasterBalanceFromSysEx(data []byte) (*MasterBalance, error) {
	bal, err := decode7bitInt(data[3:5])
	if err != nil {
		return nil, err
	}
	return &MasterBalance{
		DeviceId: int(data[0]),
		Balance:  bal,
	}, nil
}

func GlobalParameterControlFromSysEx(data []byte) (interface{}, error) {
	if len(data) != 11 {
		return nil, ErrBadRange
	}
	if data[3] != 1 || data[4] != 1 || data[5] != 1 {
		return nil, ErrBadRange
	}
	effect := (int(data[6]) << 8) | int(data[7])
	switch effect {
	case 0x0101: // reverb
		switch data[8] {
		case ReverbParameterType:
			return &ReverbType{
				DeviceId: int(data[0]),
				Type:     int(data[9]),
			}, nil
		case ReverbParameterTime:
			rt := math.Exp(0.025 * (float64(data[9]) - 40))
			return &ReverbTime{
				DeviceId: int(data[0]),
				Time:     time.Duration(float64(time.Second) * rt),
			}, nil
		}
	case 0x0102: // chorus
		switch data[8] {
		case ChorusParameterType:
			return &ChorusType{
				DeviceId: int(data[0]),
				Type:     int(data[9]),
			}, nil
		case ChorusParameterModRate:
			return &ChorusModRate{
				DeviceId: int(data[0]),
				ModRate:  float32(data[9]) * 0.122,
			}, nil
		case ChorusParameterModDepth:
			return &ChorusModDepth{
				DeviceId: int(data[0]),
				ModDepth: time.Duration(float64(time.Millisecond) * (float64(data[9]) - 1.0/3.2)),
			}, nil
		case ChorusParameterFeedback:
			return &ChorusFeedback{
				DeviceId: int(data[0]),
				Feedback: float32(data[9]) * 0.763,
			}, nil
		case ChorusParameterSendToReverb:
			return &ChorusSendToReverb{
				DeviceId:     int(data[0]),
				SendToReverb: float32(data[9]) / 100.0 * 0.763,
			}, nil
		}
	}
	return nil, ErrBadRange
}
