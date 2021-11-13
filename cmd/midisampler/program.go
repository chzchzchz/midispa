package main

import (
	"encoding/json"
	"os"
	"sort"
)

type Program struct {
	Instrument string
	Volume     float32
	Notes      map[int]*Sample
}

type ProgramMap map[string]*Program

func (pm ProgramMap) Instruments() []string {
	ret := make([]string, 0, len(pm))
	for k := range pm {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	return ret
}

func LoadProgramMap(path string) (ProgramMap, error) {
	ret := make(ProgramMap)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		p := &Program{}
		if err := dec.Decode(p); err != nil {
			return nil, err
		}
		ret[p.Instrument] = p
	}
	return ret, nil
}

const midiMiddleC = 60

// ProgramFromSampleBank creates a program with all samples from a bank.
func ProgramFromSampleBank(sb *SampleBank) *Program {
	p := &Program{
		Instrument: "Grand Sampler",
		Volume:     1.0,
		Notes:      make(map[int]*Sample),
	}
	for i, s := range sb.slice {
		note := (i + midiMiddleC) - len(sb.slice)/2
		p.Notes[note] = s
	}
	return p
}

func (p *Program) Note2Sample(note int) *Sample {
	return p.Notes[note]
}
