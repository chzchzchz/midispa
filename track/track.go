package track

import (
	"context"
)

type Track struct {
	MidiTimeSig
	ctx   context.Context
	outc  chan TickMessage
	donec chan struct{}
	err   error
}

func newTrack(ctx context.Context) Track {
	return Track{ctx: ctx, outc: make(chan TickMessage, 10), donec: make(chan struct{})}
}

func (t *Track) Chan() <-chan TickMessage { return t.outc }
func (t *Track) Err() error               { return t.err }
func (t *Track) Done() <-chan struct{}    { return t.donec }
