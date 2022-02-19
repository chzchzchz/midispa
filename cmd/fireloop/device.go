package main

import (
	"encoding/json"
	"os"

	"github.com/chzchzchz/midispa/alsa"
)

type Device struct {
	Name     string
	MidiPort string
	Channel  int
	Voices   []Voice

	alsa.SeqAddr
}

func mustLoadDevices(path string) (devs []Device) {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		devs = append(devs, Device{})
		mm := &devs[len(devs)-1]
		if err := dec.Decode(mm); err != nil {
			panic(err)
		}
	}
	return devs
}
