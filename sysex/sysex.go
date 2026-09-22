package sysex

import (
	"github.com/chzchzchz/midispa/midi"
	"github.com/chzchzchz/midispa/sysex/mmc"
)

const (
	// add vendor id's when supported

	IdNonRealTime = 0x7e
	IdRealTime    = 0x7f
)

func Decode(data []byte) interface{} {
	if len(data) <= 4 || data[0] != midi.SysEx || data[len(data)-1] != midi.EndSysEx {
		return nil
	}
	switch data[1] {
	case IdNonRealTime:
		return DecodeNonRealTime(data)
	case IdRealTime:
		return DecodeRealTime(data)
	default:
		return nil
	}
}

func DecodeRealTime(dd []byte) interface{} {
	data := dd[2:]
	switch data[1] {
	case SubIdDeviceControl:
		switch data[2] {
		case DeviceControlIdMasterVolume:
			mv, err := MasterVolumeFromSysEx(data)
			if err != nil {
				return nil
			}
			return mv
		case DeviceControlIdMasterBalance:
			mb, err := MasterBalanceFromSysEx(data)
			if err != nil {
				return nil
			}
			return mb
		case DeviceControlIdGlobalParameterControl:
			gpc, err := GlobalParameterControlFromSysEx(data)
			if err != nil {
				return nil
			}
			return gpc
		}
	case mmc.SubIdMMCCommand:
		switch data[2] {
		case mmc.MMCStop:
			return &mmc.Stop{DeviceId: int(data[0])}
		case mmc.MMCPlay:
			return &mmc.Play{DeviceId: int(data[0])}
		case mmc.MMCRewind:
			return &mmc.Rewind{DeviceId: int(data[0])}
		case mmc.MMCFastForward:
			return &mmc.FastForward{DeviceId: int(data[0])}
		case mmc.MMCRecordStrobe:
			return &mmc.RecordStrobe{DeviceId: int(data[0])}
		case mmc.MMCRecordExit:
			return &mmc.RecordExit{DeviceId: int(data[0])}
		case mmc.MMCEject:
			return &mmc.Eject{DeviceId: int(data[0])}
		case mmc.MMCDeferredPlay:
			return &mmc.DeferredPlay{DeviceId: int(data[0])}
		case mmc.MMCRecordPause:
			return &mmc.RecordPause{DeviceId: int(data[0])}
		case mmc.MMCPause:
			return &mmc.Pause{DeviceId: int(data[0])}
		case mmc.MMCWait:
			return &mmc.Wait{DeviceId: int(data[0])}
		case mmc.MMCResume:
			return &mmc.Resume{DeviceId: int(data[0])}
		case mmc.MMCChase:
			return &mmc.Chase{DeviceId: int(data[0])}
		case mmc.MMCCommandErrorReset:
			return &mmc.CommandErrorReset{DeviceId: int(data[0])}
		case mmc.MMCReset:
			return &mmc.Reset{DeviceId: int(data[0])}
		case mmc.MMCWrite:
			return &mmc.Write{DeviceId: int(data[0])}
		case mmc.MMCMaskedWrite:
			return &mmc.MaskedWrite{DeviceId: int(data[0])}
		case mmc.MMCRead:
			return &mmc.Read{DeviceId: int(data[0])}
		case mmc.MMCUpdate:
			return &mmc.Update{DeviceId: int(data[0])}
		case mmc.MMCLocate:
			if len(data) > 4 && data[4] == mmc.MMCLocateTarget {
				return &mmc.LocateTarget{DeviceId: int(data[0])}
			}
			return &mmc.LocateIF{DeviceId: int(data[0])}
		case mmc.MMCSearch:
			return &mmc.Search{DeviceId: int(data[0])}
		case mmc.MMCShuttle:
			return &mmc.Shuttle{DeviceId: int(data[0])}
		case mmc.MMCVariablePlay:
			return &mmc.VariablePlay{DeviceId: int(data[0])}
		case mmc.MMCStep:
			return &mmc.Step{DeviceId: int(data[0])}
		case mmc.MMCDeferredVariablePlay:
			return &mmc.DeferredVariablePlay{DeviceId: int(data[0])}
		case mmc.MMCRecordStrobeVariable:
			return &mmc.RecordStrobeVariable{DeviceId: int(data[0])}
		case mmc.MMCProcedure:
			return &mmc.Procedure{DeviceId: int(data[0])}
		case mmc.MMCEvent:
			return &mmc.Event{DeviceId: int(data[0])}
		case mmc.MMCGroup:
			return &mmc.Group{DeviceId: int(data[0])}
		case mmc.MMCCommandSegment:
			return &mmc.CommandSegment{DeviceId: int(data[0])}
		case mmc.MMCAdd:
			return &mmc.Add{DeviceId: int(data[0])}
		case mmc.MMCSubtract:
			return &mmc.Subtract{DeviceId: int(data[0])}
		case mmc.MMCDropFrameAdjust:
			return &mmc.DropFrameAdjust{DeviceId: int(data[0])}
		case mmc.MMCGeneratorCommand:
			return &mmc.GeneratorCommand{DeviceId: int(data[0])}
		case mmc.MMCTimeCodeCommand:
			return &mmc.TimeCodeCommand{DeviceId: int(data[0])}
		case mmc.MMCMove:
			return &mmc.Move{DeviceId: int(data[0])}
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
				return nil
			}
			return resp
		}
	case SubIdFileDump:
		switch data[2+2] {
		case FileDumpIdHeader:
			h, err := FileDumpHeaderFromSysEx(data[2:])
			if err != nil {
				return nil
			}
			return h
		case FileDumpIdDataPacket:
			return FileDumpDataPacketFromSysEx(data[2:])
		case FileDumpIdRequest:
			return FileDumpRequestFromSysEx(data[2:])
		}
	case FileDumpSubIdEOF, FileDumpSubIdWait, FileDumpSubIdCancel, FileDumpSubIdNAK, FileDumpSubIdACK:
		hs, err := HandshakeFromSysEx(data[2:])
		if err != nil {
			return nil
		}
		return hs
	}
	return nil
}

func decode7bitInt(data []byte) (ret int, err error) {
	for _, v := range data {
		if v&0x80 != 0 {
			return 0, ErrBadRange
		}
		ret <<= 7
		ret += int(uint8(v))
	}
	return ret, nil
}

func encode7bitInt(v, w int) ([]byte, error) {
	ret := make([]byte, w)
	for i := w - 1; i >= 0; i-- {
		ret[i] = byte(v & 0x7f)
		v >>= 7
	}
	if v != 0 {
		return nil, ErrBadRange
	}
	return ret, nil
}

type SysEx struct{ Data []byte }

func (se *SysEx) UnmarshalBinary(data []byte) error {
	if len(data) < 2 || data[0] != midi.SysEx {
		return ErrBadHeader
	}
	if data[len(data)-1] != midi.EndSysEx {
		return ErrNoEox
	}
	se.Data = data
	return nil
}

// Split packed sysex into individual sysex messages.
func (se *SysEx) Split() (ret []SysEx, err error) {
	i := 0
	for i < len(se.Data) {
		if se.Data[i] != midi.SysEx {
			return nil, ErrBadHeader
		}
		j := i + 1
		for j < len(se.Data) {
			if se.Data[j] == midi.EndSysEx {
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
