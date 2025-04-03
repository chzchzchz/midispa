package main

import (
	"github.com/chzchzchz/midispa/alsa"
	"github.com/chzchzchz/midispa/midi"
)

// F0 00 30 33 00 CH CC CC PP F7
func isRouteSysEx(data []byte) bool {
	if len(data) != 10 {
		return false
	}
	if data[0] != midi.SysEx {
		return false
	}
	if data[1] != 0 || data[2] != 0x30 || data[3] != 0x33 {
		// Wrong vendor
		return false
	}
	if data[4] != 0 {
		// command byte: 0 = route channel
		return false
	}
	if data[9] != midi.EndSysEx {
		return false
	}
	return true
}

type Route struct {
	dst         alsa.SeqAddr
	midiChannel int
}

func decodeRouteSysEx(data []byte) *Route {
	channel := int(data[5])
	if channel > 15 || channel < 0 {
		return nil
	}
	client := (int(data[6]) << 7) | int(data[7])
	port := int(data[8])
	return &Route{alsa.SeqAddr{client, port}, channel}
}
