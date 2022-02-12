package sr16

import (
	"fmt"
)

const ClocksPerBeat = 96

type PatternEvent struct {
	Wait       bool // [7]
	WaitClocks int  // 0-126 [6:0]

	Dynamics int // 0-7 [6:4]
	Drum     int // 0-11 [3:0]
}

func (pe *PatternEvent) Byte() byte {
	if pe.Wait {
		return byte((1 << 7) | pe.WaitClocks)
	}
	return byte((pe.Dynamics << 4) | pe.Drum)
}

type Pattern struct {
	Drumset int // 0-47 user; 64-113 preset
	Name    string
	Main    []PatternEvent
	Fill    []PatternEvent
}

func sumClocks(pe []PatternEvent) (clocks int) {
	for _, ev := range pe {
		if ev.Wait {
			clocks += ev.WaitClocks
		}
	}
	return clocks
}

func (p *Pattern) MarshalBinary() ([]byte, error) {
	if len(p.Name) > 8 {
		return nil, fmt.Errorf("name too long")
	}
	if p.Drumset < 0 || p.Drumset > 113 {
		return nil, fmt.Errorf("drumset out of range")
	}
	patternClocks, fillClocks := sumClocks(p.Main), sumClocks(p.Fill)
	if patternClocks != fillClocks {
		return nil, fmt.Errorf(
			"pattern / fill clock mismatch (%d vs %d)",
			patternClocks,
			fillClocks)
	}

	data := make([]byte, 0xe)

	bytesInPattern := 0xe + len(p.Main) + len(p.Fill) + 2
	data[0], data[1] = byte(bytesInPattern&0xff), byte(bytesInPattern>>8)

	fillOffset := 0xe + len(p.Main) + 1 - 0x3
	data[2], data[3] = byte(fillOffset&0xff), byte(fillOffset>>8)
	data[4] = byte(patternClocks / ClocksPerBeat)
	data[5] = byte(p.Drumset)
	for i := 0; i < len(p.Name); i++ {
		data[6+i] = p.Name[i]
	}
	// TODO: error check on Byte()
	for _, ev := range p.Main {
		data = append(data, ev.Byte())
	}
	data = append(data, 0xff)
	for _, ev := range p.Fill {
		data = append(data, ev.Byte())
	}
	data = append(data, 0xff)
	return data, nil
}
