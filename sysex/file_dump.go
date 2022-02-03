package sysex

const (
	SubIdFileDump = 7

	FileDumpIdHeader     = 1
	FileDumpIdDataPacket = 2
	FileDumpIdRequest    = 3
)

type FileDumpRequest struct {
	DeviceId int
	SourceId int
	Type     string
	Name     string
}

func (f *FileDumpRequest) MarshalBinary() ([]byte, error) {
	ret := []byte{
		0xF0, IdNonRealTime, byte(f.DeviceId),
		SubIdFileDump, FileDumpIdRequest,
		byte(f.SourceId),
	}
	if len(f.Type) != 4 {
		panic("no ftype")
	}
	ret = append(ret, []byte(f.Type)...)
	ret = append(ret, []byte(f.Name)...)
	ret = append(ret, 0xf7)
	return ret, nil
}

func FileDumpRequestFromSysEx(data []byte) *FileDumpRequest {
	return &FileDumpRequest{
		DeviceId: int(data[0]),
		SourceId: int(data[3]),
		Type:     string(data[4:7]),
		Name:     string(data[7 : len(data)-1]),
	}
}

type FileDumpHeader struct {
	FileDumpRequest
	Length int
}

func (f *FileDumpHeader) MarshalBinary() ([]byte, error) {
	ret := []byte{
		0xF0, IdNonRealTime, byte(f.DeviceId),
		SubIdFileDump, FileDumpIdRequest,
		byte(f.SourceId),
	}
	if len(f.Type) != 4 {
		panic("no ftype")
	}
	ret = append(ret, []byte(f.Type)...)
	ret = append(ret, encode7bitInt(f.Length, 4)...)
	ret = append(ret, []byte(f.Name)...)
	ret = append(ret, 0xf7)
	return ret, nil
}

func FileDumpHeaderFromSysEx(data []byte) *FileDumpHeader {
	return &FileDumpHeader{
		FileDumpRequest: FileDumpRequest{
			DeviceId: int(data[0]),
			SourceId: int(data[3]),
			Type:     string(data[4:7]),
			Name:     string(data[11 : len(data)-1]),
		},
		Length: decode7bitInt(data[7:11]),
	}
}

type FileDumpDataPacket struct {
	DeviceId int // dev id of rxer
	Packet   int
	Data     []byte
}

func (f *FileDumpDataPacket) MarshalBinary() ([]byte, error) {
	ret := []byte{
		0xF0, IdNonRealTime, byte(f.DeviceId),
		SubIdFileDump, FileDumpIdDataPacket,
		byte(f.Packet),
	}
	if len(f.Data) > 112 {
		panic("too much data to send")
	}
	var encodedBytes []byte
	for i := 0; i < len(f.Data); i += 7 {
		end := i + 7
		if end > len(f.Data) {
			end = len(f.Data)
		}
		d := f.Data[i:end]
		topBits := 0
		for j := 0; j < len(d); j++ {
			topBits |= ((int(d[j]) & 0x80) >> 7) << (6 - j)
		}
		encodedBytes = append(encodedBytes, byte(topBits))
		for _, v := range d {
			encodedBytes = append(encodedBytes, v&0x7f)
		}
	}
	ret = append(ret, byte(len(encodedBytes)-1))
	ret = append(ret, encodedBytes...)

	chksum := ret[1]
	for _, v := range ret[2:] {
		chksum ^= v
	}
	ret = append(ret, chksum)
	ret = append(ret, 0xf7)
	return ret, nil
}

func FileDumpDataPacketFromSysEx(data []byte) *FileDumpDataPacket {
	encLen := int(data[4])
	chksum := data[len(data)-2]
	payload := data[5:]
	payload = payload[:len(payload)-2]
	if encLen+1 != len(payload) {
		return nil
	}
	actualChksum := byte(IdNonRealTime)
	for _, v := range data[:len(data)-2] {
		actualChksum ^= v
	}
	if chksum != actualChksum {
		return nil
	}
	var decodedData []byte
	for i := 0; i < len(payload); i += 8 {
		topBits := payload[i]
		end := i + 8
		if end > len(payload) {
			end = len(payload)
		}
		d := payload[i+1 : end]
		for j := 0; j < len(d); j++ {
			topBit := ((topBits & (1 << (6 - j))) >> (6 - j)) << 7
			decodedData = append(decodedData, topBit|d[j])
		}
	}
	return &FileDumpDataPacket{
		DeviceId: int(data[0]),
		Packet:   int(data[3]),
		Data:     decodedData,
	}
}
