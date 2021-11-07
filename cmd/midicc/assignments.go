package main

import (
	"encoding/json"
	"os"
)

type Assignments struct {
	Title     string
	InDevice  string
	OutDevice string
	Maps      [][2]string // in, out

	in2out map[string]string
}

func (a *Assignments) setupMap() {
	a.in2out = make(map[string]string)
	for _, v := range a.Maps {
		a.in2out[v[0]] = v[1]
	}
}

func (a *Assignments) InToOut(in string) string {
	return a.in2out[in]
}

func (a *Assignments) InCCtoOut(in int) int {
	return -1
}

func mustLoadAssignments(path string) (m []Assignments) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		m = append(m, Assignments{})
		if err := dec.Decode(&m[len(m)-1]); err != nil {
			panic(err)
		}
		m[len(m)-1].setupMap()
	}
	return m
}
