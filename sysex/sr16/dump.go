package sr16

import (
	"fmt"

	"github.com/chzchzchz/midispa/sysex"
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
	d.Memory = sysex.LoHiDecodeDataBytes(payload)
	return nil
}

func (d *Dump) MarshalBinary() ([]byte, error) {
	data := []byte{0xf0, 0, 0, 0xe, 5, 0}
	data = append(data, sysex.LoHiEncodeDataBytes(d.Memory)...)
	data = append(data, 0xf7)
	return data, nil
}

type DumpRequest struct{}

func (dr *DumpRequest) MarshalBinary() ([]byte, error) {
	return []byte{0xf0, 0, 0, 0xe, 5, 7, 0xf7}, nil
}
