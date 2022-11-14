package track

import (
	"time"
)

type MidiTimeSig struct {
	TicksPerBeat int // clocks per metronome click
	BPM          int
}

func (m *MidiTimeSig) TickDuration() time.Duration {
	cps := float64(m.TicksPerBeat*m.BPM) / 60.0
	return time.Duration(uint64(float64(time.Second) * (1.0 / cps)))
}

type TickMessage struct {
	Raw  []byte
	Tick int
}
