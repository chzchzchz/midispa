package sysex

import (
	"fmt"
)

const (
	// add vendor id's when supported

	IdNonRealTime = 0x7e
	IdRealTime    = 0x7f
)

func Decode(data []byte) interface{} {
	if len(data) <= 4 || data[0] != 0xf0 || data[len(data)-1] != 0xf7 {
		return nil
	}
	switch data[1] {
	case IdNonRealTime:
		return DecodeNonRealTime(data[2:])
	case IdRealTime:
		return DecodeRealTime(data[2:])
	default:
		return nil
	}
}

func DecodeRealTime(data []byte) interface{} {
	switch data[1] {
	case SubIdDeviceControl:
		switch data[2] {
		case DeviceControlIdMasterVolume:
			return MasterVolumeFromSysEx(data)
		case DeviceControlIdMasterBalance:
			return MasterBalanceFromSysEx(data)
		case DeviceControlIdGlobalParameterControl:
			return GlobalParameterControlFromSysEx(data)
		}
	}
	return nil
}

func DecodeNonRealTime(data []byte) interface{} {
	switch data[1] {
	case SubIdFileDump:
		switch data[2] {
		case FileDumpIdHeader:
			return FileDumpHeaderFromSysEx(data)
		case FileDumpIdDataPacket:
			return FileDumpDataPacketFromSysEx(data)
		case FileDumpIdRequest:
			return FileDumpRequestFromSysEx(data)
		}
	case SubIdEOF, SubIdWait, SubIdCancel, SubIdNAK, SubIdACK:
		return HandshakeFromSysEx(data)
	}
	return nil
}

func decode7bitInt(data []byte) (ret int) {
	for _, v := range data {
		if v&0x80 != 0 {
			panic("want 7-bit")
		}
		ret <<= 7
		ret += int(uint8(v))
	}
	return ret
}

func encode7bitInt(v, w int) []byte {
	ret := make([]byte, w)
	for i := 0; i < w; i++ {
		ret[i] = byte(v & 0x7f)
		v >>= 7
	}
	if v != 0 {
		panic("value exceeded width")
	}
	return ret
}

type SysEx struct { Data []byte }

func (se *SysEx) UnmarshalBinary(data []byte) error {
	if len(data) < 2 {
		return fmt.Errorf("not enough data")
	}
	if data[0] != 0xf0 {
		return fmt.Errorf("bad sysex header")
	}
	if data[len(data)-1] != 0xf7 {
		return fmt.Errorf("missing eox")
	}
	se.Data = data
	return nil
}
