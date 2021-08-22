package track

import (
	"context"
)

// Loop caches a track and loops it until cancled.
type Loop struct {
	Track
	src  *Track
	msgs []TickMessage
}

func NewLoop(ctx context.Context, src *Track) *Track {
	t := &Loop{Track: newTrack(ctx), src: src}
	go t.read()
	return &t.Track
}

func (t *Loop) read() {
	inc := t.src.Chan()
	defer close(t.outc)
	baseTick := 0
	for inc != nil {
		select {
		case <-t.ctx.Done():
			t.err = t.ctx.Err()
			return
		case m, ok := <-inc:
			if !ok {
				inc = nil
				break
			}
			t.msgs = append(t.msgs, m)
			t.outc <- m
			baseTick = m.Tick
		}
	}
	for {
		lastTick := 0
		for _, m := range t.msgs {
			m2 := TickMessage{Raw: m.Raw, Tick: baseTick + m.Tick}
			select {
			case <-t.ctx.Done():
				t.err = t.ctx.Err()
				return
			case t.outc <- m2:
			}
			lastTick = m2.Tick
		}
		baseTick = lastTick
	}
}
