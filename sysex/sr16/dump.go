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
	if !isHeaderOK(data) || data[5] != 0 {
		return fmt.Errorf("bad header")
	}
	payload := data[6 : len(data)-1]
	d.Memory = decodeDataBytes(payload)
	return nil
}

func encodeDataBytes(payload []byte) (ret []byte) {
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

func decodeDataBytes(payload []byte) (ret []byte) {
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

func (d *Dump) MarshalBinary() ([]byte, error) {
	data := []byte{0xf0, 0, 0, 0xe, 5, 0}
	data = append(data, encodeDataBytes(d.Memory)...)
	data = append(data, 0xf7)
	return data, nil
}

type DumpRequest struct{}

func (dr *DumpRequest) MarshalBinary() ([]byte, error) {
	return []byte{0xf0, 0, 0, 0xe, 5, 7, 0xf7}, nil
}
