package stanton

import (
	"fmt"
)

type ButtonsLeftMode struct{}

func (s *ButtonsLeftMode) MarshalBinary() ([]byte, error) {
	return []byte{0xF0, 0x00, 0x01, 0x60, 0x01, 0x01, 0xF7}, nil
}

type ButtonsRightMode struct{}

func (s *ButtonsRightMode) MarshalBinary() ([]byte, error) {
	return []byte{0xF0, 0x00, 0x01, 0x60, 0x01, 0x02, 0xF7}, nil
}

type SlidersMode struct{}

func (s *SlidersMode) MarshalBinary() ([]byte, error) {
	return []byte{0xF0, 0x00, 0x01, 0x60, 0x01, 0x03, 0xF7}, nil
}

type ButtonsMode struct{}

func (s *ButtonsMode) MarshalBinary() ([]byte, error) {
	return []byte{0xF0, 0x00, 0x01, 0x60, 0x01, 0x04, 0xF7}, nil
}


type CompatibilityMode struct{ Channel int }

func (s *CompatibilityMode) MarshalBinary() ([]byte, error) {
	if s.Channel < 0 || s.Channel > 15 {
		return nil, fmt.Errorf("bad channel")
	}
	return []byte{0xF0, 0x00, 0x01, 0x60, 0x10, byte(s.Channel), 0xF7}, nil
}
