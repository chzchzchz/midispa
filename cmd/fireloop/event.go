package main

import (
	"github.com/chzchzchz/midispa/midi"
)

type Event struct {
	*Voice
	Beat     float32
	Velocity int     // [0,127]
	Pan      float32 // [-1, 1]; 0 = center
	Swing    int     // [0,50%]
}

func (ev *Event) ToMidi() [][]byte {
	ch := ev.Channel
	if ch == 0 {
		ch = ev.device.Channel
	}
	if ch == 0 {
		panic("no midi channel on voice")
	}
	return [][]byte{
		[]byte{midi.MakeNoteOff(ch - 1), byte(ev.Note), byte(ev.Velocity)},
		[]byte{midi.MakeNoteOn(ch - 1), byte(ev.Note), 64},
	}
}
