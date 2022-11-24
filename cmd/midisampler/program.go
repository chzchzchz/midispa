package main

type Program struct {
	Instrument string
	Volume     float32

	// Path is the subdirectory of the sample directory to search.
	// If not defined, samples will be based on root directory.
	Path string

	// Notes are loaded from json only if using per-sample ADSRs
	Notes map[int]*Sample
}

// ProgramMap is the set of all programs.
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

func (p *Program) Note2Sample(note int) *Sample    { return p.Notes[note] }
func (p *Program) StoreSample(note int, s *Sample) { p.Notes[note] = s }
