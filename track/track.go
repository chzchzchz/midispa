package track

import (
	"context"
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

type Track struct {
	MidiTimeSig
	ctx  context.Context
	outc chan TickMessage
	donec chan struct{}
	err  error
}

func newTrack(ctx context.Context) Track {
	return Track{ctx: ctx, outc: make(chan TickMessage, 10), donec: make(chan struct{})}
}

func (t *Track) Chan() <-chan TickMessage { return t.outc }
func (t *Track) Err() error               { return t.err }
func (t *Track) Done() <-chan struct{} { return t.donec }
