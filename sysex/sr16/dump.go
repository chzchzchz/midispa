package sr16

import (
	"fmt"
)

type Dump struct {
	Memory []byte
}

func isHeaderOK(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if data[0] != 0xf0 || data[1] != 0 || data[2] != 0 || data[3] != 0xe {
		return false
	}
	if data[len(data)-1] != 0xf7 {
		return false
	}
	return true
}

func (d *Dump) UnmarshalBinary(data []byte) error {
	if !isHeaderOK(data) || data[4] != 0 {
		return fmt.Errorf("bad header")
	}
	payload := data[5 : len(data)-1]
	for i := range payload {
		blockIdx := i % 8
		maskHi := byte((1 << (7 - blockIdx)) - 1)
		maskLo := byte(((1 << blockIdx) - 1) << (7 - blockIdx))
		hi := (payload[i] & maskHi) << blockIdx
		lo := (payload[i] & maskLo) >> (7 - blockIdx)
		if blockIdx > 0 {
			d.Memory[len(d.Memory)-1] |= lo
		}
		if blockIdx != 7 {
			d.Memory = append(d.Memory, hi)
		}
	}
	return nil
}

func (d *Dump) MarshalBinary() ([]byte, error) {
	data := []byte{0xf0, 0, 0, 0xe, 5, 0}
	// data = append(data, payload...)
	panic("encoding")
	data = append(data, 0xf7)
	return data, nil
}

type DumpRequest struct{}

func (dr *DumpRequest) MarshalBinary() ([]byte, error) {
	return []byte{0xf0, 0, 0, 0xe, 5, 7, 0xf7}, nil
}
