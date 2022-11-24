package main

import (
	"log"
	"path/filepath"
)

type Storage struct {
	ConfigPath string
	SamplePath string

	samples *SampleBank
	bank    *Bank
	pm      *ProgramMap
}

func NewStorage(configPath, samplePath string) *Storage {
	return &Storage{ConfigPath: configPath, SamplePath: samplePath}
}

func (s *Storage) ProgramsPath() string {
	return filepath.Join(s.ConfigPath, "programs.json")
}

func (s *Storage) ProgramBank() *Bank {
	if s.bank != nil {
		return s.bank
	}
	pm := s.Programs()
	path := filepath.Join(s.ConfigPath, "banks.json")
	log.Printf("loading program bank from %q", path)
	bank, err := LoadBank(path, pm)
	if err != nil {
		log.Printf(
			"failed loading %q (%v); making bank of all programs",
			path,
			err)
		bank = BankFromProgramMap(pm)
	}
	s.bank = bank
	return s.bank
}

func (s *Storage) SampleBank() *SampleBank {
	if s.samples != nil {
		return s.samples
	}
	log.Printf("loading sample bank from %q", s.SamplePath)
	s.samples = MustLoadSampleBank(s.SamplePath)
	return s.samples
}

func (s *Storage) Programs() *ProgramMap {
	if s.pm != nil {
		return s.pm
	}
	path := s.ProgramsPath()
	log.Printf("loading programs from %q", path)
	pm, err := LoadProgramMap(path, s.SampleBank())
	if err != nil {
		log.Printf(
			"failed loading %q (%v); making sample bank",
			path,
			err)
		p := ProgramFromSampleBank(s.SampleBank())
		pm = NewProgramMap()
		pm.Add(p)
	}
	s.pm = pm
	return s.pm
}
