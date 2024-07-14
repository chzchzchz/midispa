package track

import (
	"context"
)

// PatternStream has a channel for tracks that it will play asynchronously.
type PatternStream struct {
	Track
	src    *Track
	trackc chan *Track
}

func NewPatternStream(ctx context.Context) *PatternStream {
	t := &PatternStream{Track: newTrack(ctx), trackc: make(chan *Track)}
	go t.read()
	return t
}

func (t *PatternStream) waitForTrack() error {
	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	case t.src = <-t.trackc:
	}
	return nil
}

func (t *PatternStream) TrackChan() chan<- *Track { return t.trackc }

func (t *PatternStream) read() {
	defer func() {
		close(t.outc)
		close(t.donec)
	}()
	for {
		if t.err = t.waitForTrack(); t.err != nil {
			return
		}
		for t.src != nil {
			select {
			case msg, ok := <-t.src.Chan():
				if !ok {
					t.src = nil
					break
				}
				select {
				case t.outc <- msg:
				case <-t.ctx.Done():
					t.err = t.Err()
					return
				}
			case <-t.ctx.Done():
				t.err = t.Err()
				return
			}
		}
	}
}
