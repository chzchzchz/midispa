package main

import (
	"sort"

	"github.com/chzchzchz/midispa/util"
)

type Program struct {
	Instrument string
	Volume     float32
	Path       string
	// loaded from json only if using per-sample ADSRs
	Notes map[int]*Sample
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

func LoadProgramMap(path string, sb *SampleBank) (ProgramMap, error) {
	ret := make(ProgramMap)
	progs, err := util.LoadJSONFile[Program](path)
	if err != nil {
		return nil, err
	}
	for i, p := range progs {
		if p.Notes == nil {
			// No notes, load from path.
			p.Notes = make(map[int]*Sample)
			pathSamples := sb.ByPrefix(p.Path)
			for i, s := range pathSamples {
				note := (i + midiMiddleC) - len(pathSamples)/2
				p.Notes[note] = s
			}
		} else if p.Path != "" {
			panic("instrument " + p.Instrument + " had path and notes")
		}
		if _, ok := ret[p.Instrument]; ok {
			panic("instrument " + p.Instrument + " defined twice")
		}
		ret[p.Instrument] = &progs[i]
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
