package main

import (
	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/util"
)

type Device struct {
	Name     string
	MidiPort string
	Channel  int
	Voices   []Voice

	alsa.SeqAddr
}

func mustLoadDevices(path string) (devs []Device) {
	return util.MustLoadJSONFile[Device](path)
}
