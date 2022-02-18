package sysex

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
		return DecodeNonRealTime(data)
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
	switch data[2+1] {
	case SubIdDeviceInquiry:
		switch data[2+2] {
		case DeviceInquiryIdRequest:
			return &DeviceInquiryRequest{}
		case DeviceInquiryIdResponse:
			resp := &DeviceInquiryResponse{}
			if err := resp.UnmarshalBinary(data); err != nil {
				panic(err)
			}
			return resp
		}
	case SubIdFileDump:
		switch data[2+2] {
		case FileDumpIdHeader:
			return FileDumpHeaderFromSysEx(data[2:])
		case FileDumpIdDataPacket:
			return FileDumpDataPacketFromSysEx(data[2:])
		case FileDumpIdRequest:
			return FileDumpRequestFromSysEx(data[2:])
		}
	case SubIdEOF, SubIdWait, SubIdCancel, SubIdNAK, SubIdACK:
		return HandshakeFromSysEx(data[2:])
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

type SysEx struct{ Data []byte }

func (se *SysEx) UnmarshalBinary(data []byte) error {
	if len(data) < 2 || data[0] != 0xf0 {
		return ErrBadHeader
	}
	if data[len(data)-1] != 0xf7 {
		return ErrNoEox
	}
	se.Data = data
	return nil
}

// Split packed sysex into individual sysex messages.
func (se *SysEx) Split() (ret []SysEx, err error) {
	i := 0
	for i < len(se.Data) {
		if se.Data[i] != 0xf0 {
			return nil, ErrBadHeader
		}
		j := i + 1
		for j < len(se.Data) {
			if se.Data[j] == 0xf7 {
				break
			}
			j++
		}
		if j >= len(se.Data) {
			return nil, ErrNoEox
		}
		ret = append(ret, SysEx{Data: se.Data[i : j+1]})
		i = j + 1
	}
	return ret, nil
}

// 0 A6 ... A0
// 0 A7 ... B2
// 0 B1 ... C3

func LoHiEncodeDataBytes(payload []byte) (ret []byte) {
	for i := range payload {
		bidx := i % 7
		mask := byte((1 << (bidx + 1)) - 1)
		// the most significant bits of payload[i]
		hi := (payload[i] & ^mask) >> (bidx + 1)
		// the least significant bits of payload[i]
		lo := (payload[i] & mask) << (7 - (bidx + 1))
		if bidx == 0 {
			ret = append(ret, hi)
		} else {
			ret[len(ret)-1] |= hi
		}
		ret = append(ret, lo)
	}
	return ret
}

func LoHiDecodeDataBytes(payload []byte) (ret []byte) {
	decodeBlock := func(v []byte) {
		for i := 0; i < len(v)-1; i++ {
			bidx := i % 7
			hi := v[i] << (bidx + 1)
			lo := v[i+1] >> (6 - bidx)
			ret = append(ret, hi|lo)
		}
	}
	for i := 0; i < len(payload); i += 8 {
		decodeBlock(payload[i : i+8])
	}
	return ret
}
