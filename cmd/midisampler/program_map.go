package main

import (
	"sort"

	"github.com/chzchzchz/midispa/util"
)

type ProgramMap struct {
	Map map[string]*Program
}

func NewProgramMap() *ProgramMap {
	return &ProgramMap{Map: make(map[string]*Program)}
}

func (pm *ProgramMap) Add(p *Program)           { pm.Map[p.Instrument] = p }
func (pm *ProgramMap) Lookup(p string) *Program { return pm.Map[p] }

func (pm *ProgramMap) Instruments() []string {
	ret := make([]string, 0, len(pm.Map))
	for k := range pm.Map {
		ret = append(ret, k)
	}
	sort.Strings(ret)
	return ret
}

func LoadProgramMap(path string, sb *SampleBank) (*ProgramMap, error) {
	progs, err := util.LoadJSONFile[Program](path)
	if err != nil {
		return nil, err
	}
	ret := NewProgramMap()
	for i := range progs {
		p := &progs[i]
		if p.Notes == nil {
			p.Notes = make(map[int]*Sample)
		} else if p.Path != "" {
			panic("instrument " + p.Instrument + " had path and notes")
		}
		if p.Path != "" {
			// No notes, load from path.
			pathSamples := sb.ByPrefix(p.Path)
			for i, s := range pathSamples {
				note := (i + midiMiddleC) - len(pathSamples)/2
				p.Notes[note] = s
			}
			p.Path = ""
		}
		if ret.Lookup(p.Instrument) != nil {
			panic("instrument " + p.Instrument + " defined twice")
		}
		ret.Map[p.Instrument] = p
	}
	return ret, nil
}

func (pm *ProgramMap) Save(path string) error {
	return util.SaveMapValuesJSONFile[string, Program](path, pm.Map)
}
