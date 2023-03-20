package main

import (
	"github.com/chzchzchz/midispa/alsa"
)

type Device struct {
	Name     string
	MidiPort string
	Channel  int
	Voices   []Voice

	alsa.SeqAddr
}
