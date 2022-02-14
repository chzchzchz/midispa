package sr16

import (
	"bytes"
	"io"
)

type InquiryRequest struct{}

func (r *InquiryRequest) MarshalBinary() ([]byte, error) {
	return []byte{0xF0, 0x00, 0x00, 0x0E, 0x05, 0x09, 0x00, 0xF7}, nil
}

type InquiryResponse struct{}

func (r *InquiryResponse) UnmarshalBinary(msg []byte) error {
	if len(msg) != 8 {
		return io.EOF
	}
	cmp := bytes.Compare(
		[]byte{0xF0, 0x00, 0x00, 0x0E, 0x05, 0x09, 0x01, 0xF7}, msg)
	if cmp != 0 {
		return io.EOF
	}
	return nil
}
