package main

import (
	"io"
)

type Bank struct {
	Title    string
	Programs map[int]string
	programs map[int]*Program
}

func LoadBank(path string, pm ProgramMap) (*Bank, error) {
	return nil, io.EOF
}

func BankFromProgramMap(pm ProgramMap) *Bank {
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
			bank.Programs[i+oldlen], bank.programs[i+oldlen] = pgm, pm[pgm]
		}
	}
	return bank
}
