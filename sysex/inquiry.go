package sysex

import (
	"io"
)

const (
	SubIdDeviceInquiry      = 6
	DeviceInquiryIdRequest  = 1
	DeviceInquiryIdResponse = 2

	DeviceInquiryCallAll = 0x7f
)

type DeviceInquiryRequest struct{ DeviceId int }

func (req *DeviceInquiryRequest) MarshalBinary() ([]byte, error) {
	if req.DeviceId < 0 || req.DeviceId > 0x7f {
		return nil, ErrBadRange
	}
	return []byte{
		0xf0, IdNonRealTime, byte(req.DeviceId),
		SubIdDeviceInquiry, DeviceInquiryIdRequest, 0xf7,
	}, nil
}

type DeviceInquiryResponse struct {
	DeviceId     int
	Manufacturer int
	DeviceFamily int
	DeviceMember int
	Revision     []int
}

func (resp *DeviceInquiryResponse) UnmarshalBinary(data []byte) error {
	if len(data) < 6 || data[0] != 0xf0 || data[1] != IdNonRealTime ||
		data[3] != SubIdDeviceInquiry || data[4] != DeviceInquiryIdResponse {
		return ErrBadHeader
	}
	if data[len(data)-1] != 0xf7 {
		return ErrNoEox
	}
	resp.DeviceId = int(data[2])
	off := 5
	resp.Manufacturer = int(data[5])
	if resp.Manufacturer == 0 {
		resp.Manufacturer = int(data[6]) + int(data[7])<<7
		off += 2
	}

	off++
	if len(data) < off+4+4 {
		return io.EOF
	}
	resp.DeviceFamily = int(data[off]) + int(data[off+1])<<7
	off += 2
	resp.DeviceMember = int(data[off]) + int(data[off+1])<<7
	off += 2
	resp.Revision = make([]int, 4)
	for i := range resp.Revision {
		resp.Revision[i] = int(data[off+i])
	}

	return nil
}
