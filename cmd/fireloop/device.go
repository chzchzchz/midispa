package main

import (
	"github.com/chzchzchz/midispa/alsa"
)

type Device struct {
	Name     string
	MidiPort string
	Channel  int
	// Patch names a .mid or .smf file played to this device when playback starts.
	// A voice's Patch overrides it, the same way a voice's Channel overrides.
	Patch string
	// Settle is how long this device is given to absorb a vendor dump from Patch
	// before the first note, written as text such as "100ms". It is left out of a
	// kit unless it is wanted, and means no wait when it is absent.
	Settle Settle `json:",omitempty"`
	Voices []Voice

	// baseDir is the directory of the kit file that declared this device, which a
	// relative Patch resolves against. It comes from the kit path rather than the JSON.
	baseDir string

	alsa.SeqAddr
}
