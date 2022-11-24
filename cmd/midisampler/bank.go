package main

import (
	"encoding/json"
	"os"
)

type Bank struct {
	Title    string
	Programs map[int]string
	programs map[int]*Program
}

func LoadBank(path string, pm *ProgramMap) (*Bank, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	b := &Bank{}
	if err := dec.Decode(b); err != nil {
		return nil, err
	}
	b.programs = make(map[int]*Program)
	for k, v := range b.Programs {
		pgm := pm.Lookup(v)
		if pgm == nil {
			panic("could not find program " + v)
		}
		// subtract 1 so json can have mapping to drum channel 10
		b.programs[k-1] = pgm
	}
	return b, nil
}

func BankFromProgramMap(pm *ProgramMap) *Bank {
	bank := &Bank{
		Title:    "default bank",
		Programs: make(map[int]string),
		programs: make(map[int]*Program),
	}
	pgms := pm.Instruments()
	if len(pgms) == 0 {
		panic("no programs")
	}
	// Map instruments for at least 16 channels so default always sounds.
	for len(bank.Programs) < 16 {
		oldlen := len(bank.Programs)
		for i, pgm := range pgms {
			bank.Programs[i+oldlen] = pgm
			bank.programs[i+oldlen] = pm.Lookup(pgm)
		}
	}
	return bank
}
