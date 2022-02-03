package sr16

import (
	"fmt"
)

type DrumSet [12]Drum

type Drum struct {
	SoundNumber  int  // 0-232
	OutputSelect bool // false = main
	Volume       int  // 0-99
	Panning      int  // 0=left,3=center,6=right
	Assignment   int  //0-3; multi, single, group1, group2
	Tuning       int  // 0=-4, 4=0, 7=+3
}

func (d *Drum) encode() ([]byte, error) {
	if d.SoundNumber < 0 || d.SoundNumber > 232 {
		return nil, fmt.Errorf("bad sound number")
	}
	if d.Volume < 0 || d.Volume > 99 {
		return nil, fmt.Errorf("bad volume")
	}
	if d.Panning < 0 || d.Panning > 6 {
		return nil, fmt.Errorf("bad panning")
	}
	if d.Assignment < 0 || d.Assignment > 6 {
		return nil, fmt.Errorf("bad assignment")
	}
	if d.Tuning < 0 || d.Tuning > 7 {
		return nil, fmt.Errorf("bad tuning")
	}
	outSel := 0
	if d.OutputSelect {
		outSel = 1
	}
	return []byte{
		byte(d.SoundNumber),
		byte(outSel<<7) | byte(d.Volume),
		byte(d.Panning<<5) | byte(d.Assignment<<3) | byte(d.Tuning),
	}, nil
}

func (ds *DrumSet) MarshalBinary() ([]byte, error) {
	var drums []byte
	for _, d := range ds {
		b, err := d.encode()
		if err != nil {
			return nil, err
		}
		drums = append(drums, b...)
	}
	// The format is two MIDI bytes per data byte, with the most significant
	// data bit transmitted in bit 0 of the first MIDI byte, and data bits
	// 0-6 transmitted in the second MIDI byte.
	var payload []byte
	for _, v := range drums {
		payload = append(payload, v&0x7f)
		payload = append(payload, (v&0x80)>>7)
	}
	if len(payload) != 72 {
		panic("payload unexpected length")
	}
	data := []byte{
		0xf0,             // sysex
		0x00, 0x00, 0x0e, // alesis
		0x05, // sr-16
		0x08, // receive drumset data
	}
	data = append(data, payload...)
	data = append(data, 0xf7)
	return data, nil
}

type DrumSetRequest struct{}

func (ds *DrumSetRequest) MarshalBinary() ([]byte, error) {
	return []byte{
		0xf0,             // sysex
		0x00, 0x00, 0x0e, // alesis
		0x05, // sr-16
		0x0a, // request drumset data
		0xf7,
	}, nil
}
